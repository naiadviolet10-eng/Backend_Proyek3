package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// bahanMasak adalah satu bahan resep yang akan dipotong dari stok.
type bahanMasak struct {
	IngredientID int64
	Nama         string
	Quantity     float64
	Unit         string
	Gizi         bahanJSON
}

var kunciGizi = []string{
	"calories", "protein_g", "fat_g", "carbs_g",
	"iron_mg", "zinc_mg", "vitamin_a_mcg", "vitamin_c_mg",
}

func giziBahan(b bahanJSON, kunci string) float64 {
	switch kunci {
	case "calories":
		return b.CaloriesPer100g
	case "protein_g":
		return b.ProteinG
	case "fat_g":
		return b.FatG
	case "carbs_g":
		return b.CarbsG
	case "iron_mg":
		return b.IronMg
	case "zinc_mg":
		return b.ZincMg
	case "vitamin_a_mcg":
		return b.VitaminAMcg
	case "vitamin_c_mg":
		return b.VitaminCMcg
	default:
		return 0
	}
}

// muatBahanMasak mengambil daftar bahan sebuah resep (pustaka atau AI)
// beserta porsi dasar, nama resep, dan gizinya.
func muatBahanMasak(ctx context.Context, r *http.Request, userID string, publicID int64) (int, string, []bahanMasak, bool, error) {
	perID, perNama, err := muatPetaBahan(ctx, r)
	if err != nil {
		return 0, "", nil, false, err
	}

	if idAI, ok := publikKeTersimpan(publicID); ok {
		rt, err := bacaResepTersimpan(ctx, userID, idAI)
		if err != nil {
			return 0, "", nil, false, err
		}
		var hasil []bahanMasak
		for _, b := range rt.Bahan {
			master, ok := perNama[strings.ToLower(strings.TrimSpace(b.IngredientName))]
			if !ok {
				continue
			}
			unit := b.Unit
			if strings.TrimSpace(unit) == "" {
				unit = master.DefaultUnit
			}
			qty := b.Quantity
			if qty <= 0 {
				qty = 1
			}
			hasil = append(hasil, bahanMasak{IngredientID: master.ID, Nama: master.Name, Quantity: qty, Unit: unit, Gizi: master})
		}
		return maxInt(1, rt.Porsi), rt.Nama, hasil, true, nil
	}

	var porsi int
	var nama string
	if err := db.QueryRow(ctx, `SELECT porsi, nama FROM resep WHERE id = $1`, publicID).Scan(&porsi, &nama); err != nil {
		return 0, "", nil, false, err
	}
	rows, err := db.Query(ctx, `
		SELECT rb.bahan_id, rb.jumlah::float8, COALESCE(i.default_unit,'gram')
		FROM resep_bahan rb
		LEFT JOIN ingredients_master i ON i.id = rb.bahan_id
		WHERE rb.resep_id = $1 ORDER BY rb.id`, publicID)
	if err != nil {
		return 0, "", nil, false, err
	}
	defer rows.Close()
	var hasil []bahanMasak
	for rows.Next() {
		var bid int64
		var jumlah float64
		var unit string
		if err := rows.Scan(&bid, &jumlah, &unit); err != nil {
			return 0, "", nil, false, err
		}
		master := perID[bid]
		hasil = append(hasil, bahanMasak{IngredientID: bid, Nama: master.Name, Quantity: jumlah, Unit: unit, Gizi: master})
	}
	return maxInt(1, porsi), nama, hasil, true, nil
}

// POST /api/cooking-logs
func apiCatatMasak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		RecipeID        int64   `json:"recipe_id"`
		Servings        float64 `json:"servings"`
		DurationMinutes *int    `json:"duration_minutes"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "servings", "Format permintaan salah")
		return
	}
	if isian.Servings < 1 || isian.Servings > 8 {
		kirimValidasi(w, "Porsi harus antara 1 dan 8", "servings", "Porsi tidak valid")
		return
	}
	if isian.DurationMinutes != nil && (*isian.DurationMinutes < 1 || *isian.DurationMinutes > 600) {
		kirimValidasi(w, "Durasi tidak valid", "duration_minutes", "Durasi antara 1 dan 600 menit")
		return
	}

	base, namaResep, bahan, ok, err := muatBahanMasak(r.Context(), r, u.ID, isian.RecipeID)
	if err != nil || !ok {
		kirimValidasi(w, "Resep tidak ditemukan", "recipe_id", "Resep tidak terdaftar")
		return
	}

	factor := isian.Servings / float64(base)

	tx, err := db.Begin(r.Context())
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal mencatat masak")
		return
	}
	defer tx.Rollback(r.Context())

	type stokBaris struct {
		id       int64
		quantity float64
		unit     string
	}
	stok := map[int64]stokBaris{}
	rows, err := tx.Query(r.Context(),
		`SELECT id, bahan_pangan_id, jumlah::float8, satuan FROM stok WHERE user_id = $1 FOR UPDATE`, u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal membaca stok")
		return
	}
	for rows.Next() {
		var sid, bid int64
		var qty float64
		var unit string
		if err := rows.Scan(&sid, &bid, &qty, &unit); err != nil {
			rows.Close()
			kirimPesan(w, http.StatusInternalServerError, "Gagal membaca stok")
			return
		}
		stok[bid] = stokBaris{id: sid, quantity: qty, unit: unit}
	}
	rows.Close()

	deducted := []map[string]any{}
	removed := []map[string]any{}
	skipped := []map[string]any{}
	insufficient := []map[string]any{}
	excluded := []map[string]any{}
	total := map[string]float64{}
	for _, k := range kunciGizi {
		total[k] = 0
	}

	for _, b := range bahan {
		amount := b.Quantity * factor
		s, ada := stok[b.IngredientID]
		if !ada {
			skipped = append(skipped, map[string]any{"ingredient": b.Nama, "reason": "tidak ada di stok"})
			excluded = append(excluded, map[string]any{"ingredient": b.Nama, "reason": "tidak ada di stok"})
			continue
		}

		if grams, okG := gramUntukGizi(amount, b.Unit); okG {
			f := grams / 100
			for _, k := range kunciGizi {
				total[k] += giziBahan(b.Gizi, k) * f
			}
		} else {
			excluded = append(excluded, map[string]any{
				"ingredient": b.Nama,
				"reason":     "satuan " + b.Unit + " tidak bisa diubah ke gram",
			})
		}

		needed, okU := ubahSatuan(amount, b.Unit, s.unit)
		if !okU {
			skipped = append(skipped, map[string]any{
				"ingredient": b.Nama,
				"reason":     "satuan " + b.Unit + " tidak bisa ke " + s.unit,
			})
			continue
		}
		if needed <= 0 {
			continue
		}

		before := s.quantity
		after := bulat2(before - needed)
		if after > 0 {
			if _, err := tx.Exec(r.Context(), `UPDATE stok SET jumlah = $1 WHERE id = $2`, after, s.id); err != nil {
				kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui stok")
				return
			}
			deducted = append(deducted, map[string]any{
				"ingredient": b.Nama, "used": needed, "from": before, "to": after, "unit": s.unit,
			})
		} else {
			if _, err := tx.Exec(r.Context(), `DELETE FROM stok WHERE id = $1`, s.id); err != nil {
				kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui stok")
				return
			}
			removed = append(removed, map[string]any{
				"ingredient": b.Nama, "unit": s.unit, "short": needed - before,
			})
			insufficient = append(insufficient, map[string]any{
				"ingredient": b.Nama, "needed": needed, "available": before, "unit": s.unit,
			})
		}
	}

	perPorsi := map[string]float64{}
	for _, k := range kunciGizi {
		perPorsi[k] = bulat2(total[k] / math.Max(isian.Servings, 0.0001))
	}
	totalBulat := map[string]float64{}
	for _, k := range kunciGizi {
		totalBulat[k] = bulat2(total[k])
	}
	nutrisi := map[string]any{
		"total":       totalBulat,
		"per_serving": perPorsi,
		"servings":    isian.Servings,
		"excluded":    excluded,
	}

	logNutrisi := marshalJSON(map[string]any{
		"total": totalBulat, "per_serving": perPorsi, "servings": isian.Servings, "excluded": excluded,
	})
	logRincian := marshalJSON(map[string]any{"deducted": deducted, "removed": removed, "skipped": skipped})

	var logID string
	var cookedAt time.Time
	err = tx.QueryRow(r.Context(), `
		INSERT INTO cooking_logs (user_id, recipe_id, nama_resep, servings, duration_minutes, nutrition, rincian)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb)
		RETURNING id::text, cooked_at`,
		u.ID, isian.RecipeID, namaResep, isian.Servings, isian.DurationMinutes, logNutrisi, logRincian).Scan(&logID, &cookedAt)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal mencatat masak")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal mencatat masak")
		return
	}

	pesan := "Memasak tercatat."
	if len(insufficient) > 0 {
		pesan = "Memasak tercatat, tapi sebagian bahan kurang."
	}
	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": pesan,
		"data": map[string]any{
			"id":               logID,
			"recipe_id":        isian.RecipeID,
			"recipe_name":      namaResep,
			"servings":         isian.Servings,
			"cooked_at":        cookedAt.Format(time.RFC3339),
			"duration_minutes": isian.DurationMinutes,
			"deducted":         deducted,
			"removed":          removed,
			"skipped":          skipped,
			"insufficient":     insufficient,
			"nutrition":        nutrisi,
		},
	})
}

// GET /api/cooking-logs?days=30
func apiRiwayatMasak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	days := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil {
		days = v
	}
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}

	rows, err := db.Query(r.Context(), `
		SELECT l.id::text, l.recipe_id, l.servings::float8, l.duration_minutes, l.cooked_at,
		       COALESCE(NULLIF(l.nama_resep,''), ''), l.nutrition::text
		FROM cooking_logs l
		WHERE l.user_id = $1 AND l.cooked_at >= now() - make_interval(days => $2)
		ORDER BY l.cooked_at DESC`, u.ID, days)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat riwayat")
		return
	}
	defer rows.Close()

	type riwayat struct {
		ID              string
		RecipeID        int64
		Servings        float64
		DurationMinutes *int
		CookedAt        time.Time
		Nutrition       string
	}
	var daftar []riwayat
	for rows.Next() {
		var it riwayat
		if err := rows.Scan(&it.ID, &it.RecipeID, &it.Servings, &it.DurationMinutes, &it.CookedAt, new(string), &it.Nutrition); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat riwayat")
			return
		}
		daftar = append(daftar, it)
	}
	rows.Close()

	// Nama resep diambil dari pustaka atau resep AI milik user.
	namaPustaka := map[int64]string{}
	if nama, err := muatNamaPustaka(r.Context()); err == nil {
		namaPustaka = nama
	}

	hasil := []map[string]any{}
	for _, it := range daftar {
		nama := namaPustaka[it.RecipeID]
		if nama == "" {
			if idAI, ok := publikKeTersimpan(it.RecipeID); ok {
				var n string
				if err := db.QueryRow(r.Context(), `SELECT nama_resep FROM resep_tersimpan WHERE id = $1 AND user_id = $2`, idAI, u.ID).Scan(&n); err == nil {
					nama = n
				}
			}
		}
		var nutrisi any
		json.Unmarshal([]byte(it.Nutrition), &nutrisi)
		item := map[string]any{
			"id":               it.ID,
			"recipe_id":        it.RecipeID,
			"servings":         it.Servings,
			"duration_minutes": it.DurationMinutes,
			"cooked_at":        it.CookedAt.Format(time.RFC3339),
			"nutrition":        nutrisi,
			"recipe": map[string]any{
				"id":   it.RecipeID,
				"name": nama,
			},
		}
		hasil = append(hasil, item)
	}
	kirimData(w, hasil)
}

func muatNamaPustaka(ctx context.Context) (map[int64]string, error) {
	rows, err := db.Query(ctx, `SELECT id, nama FROM resep`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hasil := map[int64]string{}
	for rows.Next() {
		var id int64
		var nama string
		if err := rows.Scan(&id, &nama); err != nil {
			return nil, err
		}
		hasil[id] = nama
	}
	return hasil, nil
}

func bulat2(v float64) float64 { return math.Round(v*100) / 100 }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
