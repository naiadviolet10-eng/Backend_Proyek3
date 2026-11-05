package main

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const kolomStok = `
	s.id, s.jumlah::float8, s.satuan, s.tanggal_kadaluarsa::text,
	CASE WHEN s.tanggal_kadaluarsa IS NULL THEN 'fresh'
	     WHEN (s.tanggal_kadaluarsa - CURRENT_DATE) < 0 THEN 'expired'
	     WHEN (s.tanggal_kadaluarsa - CURRENT_DATE) <= 3 THEN 'expiring'
	     ELSE 'fresh' END,
	CASE WHEN s.tanggal_kadaluarsa IS NULL THEN NULL ELSE (s.tanggal_kadaluarsa - CURRENT_DATE) END,`

const dasarStok = `
	FROM stok s
	JOIN ingredients_master i ON i.id = s.bahan_pangan_id
	LEFT JOIN ingredient_categories c ON c.id = i.category_id`

func pindaiStok(r *http.Request, row pgx.Row) (stokJSON, error) {
	var s stokJSON
	var expiry *string
	var daysLeft *int32
	var imagePath, namaKategori *string
	err := row.Scan(&s.ID, &s.Quantity, &s.Unit, &expiry, &s.Status, &daysLeft,
		&s.Ingredient.ID, &s.Ingredient.CategoryID, &s.Ingredient.Name, &s.Ingredient.DefaultUnit,
		&s.Ingredient.CaloriesPer100g, &s.Ingredient.ProteinG, &s.Ingredient.FatG, &s.Ingredient.CarbsG,
		&s.Ingredient.IronMg, &s.Ingredient.ZincMg, &s.Ingredient.VitaminAMcg, &s.Ingredient.VitaminCMcg,
		&imagePath, &namaKategori)
	if err != nil {
		return s, err
	}
	if expiry != nil {
		s.ExpiryDate = expiry
	}
	if daysLeft != nil {
		d := int(*daysLeft)
		s.DaysLeft = &d
	}
	if namaKategori != nil {
		s.Ingredient.Category = &kategoriRingkas{ID: s.Ingredient.CategoryID, Name: *namaKategori}
	}
	s.Ingredient.ImageURL = urlGambar(r, imagePath)
	return s, nil
}

func ambilStok(ctx context.Context, r *http.Request, userID string, id int64) (stokJSON, error) {
	row := db.QueryRow(ctx, `SELECT `+kolomStok+kolomBahanAPI+dasarStok+`
		WHERE s.user_id = $1 AND s.id = $2`, userID, id)
	return pindaiStok(r, row)
}

func daftarStok(ctx context.Context, r *http.Request, userID, status string, kategoriID int64) ([]stokJSON, error) {
	filterStatus := ""
	if status == "expiring" || status == "expired" {
		filterStatus = status
	}
	rows, err := db.Query(ctx, `SELECT `+kolomStok+kolomBahanAPI+dasarStok+`
		WHERE s.user_id = $1
		  AND ($2 = 0 OR i.category_id = $2)
		  AND ($3 = '' OR ($3 = 'expiring' AND s.tanggal_kadaluarsa IS NOT NULL AND (s.tanggal_kadaluarsa - CURRENT_DATE) >= 0 AND (s.tanggal_kadaluarsa - CURRENT_DATE) <= 3)
		              OR ($3 = 'expired' AND s.tanggal_kadaluarsa IS NOT NULL AND (s.tanggal_kadaluarsa - CURRENT_DATE) < 0))
		ORDER BY s.id DESC`, userID, kategoriID, filterStatus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hasil := []stokJSON{}
	for rows.Next() {
		s, err := pindaiStok(r, rows)
		if err != nil {
			return nil, err
		}
		hasil = append(hasil, s)
	}
	return hasil, nil
}

// GET /api/stocks
func apiDaftarStok(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	kategoriID, _ := strconv.ParseInt(r.URL.Query().Get("category_id"), 10, 64)
	hasil, err := daftarStok(r.Context(), r, u.ID, r.URL.Query().Get("status"), kategoriID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat stok")
		return
	}
	kirimData(w, hasil)
}

// POST /api/stocks
func apiTambahStok(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		IngredientID int64   `json:"ingredient_id"`
		Quantity     float64 `json:"quantity"`
		Unit         string  `json:"unit"`
		ExpiryDate   *string `json:"expiry_date"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "quantity", "Format permintaan salah")
		return
	}
	isian.Unit = strings.TrimSpace(isian.Unit)

	if _, err := ambilBahanAPI(r.Context(), r, isian.IngredientID); err != nil {
		kirimValidasi(w, "Bahan tidak ditemukan", "ingredient_id", "Bahan tidak terdaftar")
		return
	}
	if isian.Quantity <= 0 {
		kirimValidasi(w, "Jumlah stok harus lebih dari 0", "quantity", "Jumlah harus lebih dari 0")
		return
	}
	if isian.Unit == "" {
		kirimValidasi(w, "Satuan wajib diisi", "unit", "Satuan tidak boleh kosong")
		return
	}
	expiry := tanggalBersih(isian.ExpiryDate)
	if expiry != nil && *expiry < time.Now().Format("2006-01-02") {
		kirimValidasi(w, "Tanggal kedaluwarsa tidak boleh di masa lalu", "expiry_date", "Tanggal harus hari ini atau setelahnya")
		return
	}

	var idLama int64
	var unitLama string
	err := db.QueryRow(r.Context(),
		`SELECT id, satuan FROM stok WHERE user_id = $1 AND bahan_pangan_id = $2 LIMIT 1`,
		u.ID, isian.IngredientID).Scan(&idLama, &unitLama)

	if err == nil {
		if _, err := db.Exec(r.Context(),
			`UPDATE stok SET jumlah = jumlah + $1,
			        satuan = COALESCE(NULLIF($2,''), satuan),
			        tanggal_kadaluarsa = COALESCE($3::date, tanggal_kadaluarsa)
			 WHERE id = $4`,
			isian.Quantity, isian.Unit, expiry, idLama); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan stok")
			return
		}
	} else if tidakAda(err) {
		if err := db.QueryRow(r.Context(),
			`INSERT INTO stok (user_id, bahan_pangan_id, jumlah, satuan, tanggal_kadaluarsa)
			 VALUES ($1,$2,$3,$4,$5::date) RETURNING id`,
			u.ID, isian.IngredientID, isian.Quantity, isian.Unit, expiry).Scan(&idLama); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan stok")
			return
		}
	} else {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan stok")
		return
	}

	s, err := ambilStok(r.Context(), r, u.ID, idLama)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat stok")
		return
	}
	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true, "message": "Stok berhasil ditambahkan", "data": s,
	})
}

// GET /api/stocks/{id}
func apiDetailStok(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	s, err := ambilStok(r.Context(), r, u.ID, id)
	if err != nil {
		if tidakAda(err) {
			kirimPesan(w, http.StatusNotFound, "Stok tidak ditemukan")
			return
		}
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat stok")
		return
	}
	kirimData(w, s)
}

// PUT /api/stocks/{id}
func apiUbahStok(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)

	var isian struct {
		Quantity   *float64 `json:"quantity"`
		Unit       *string  `json:"unit"`
		ExpiryDate *string  `json:"expiry_date"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "quantity", "Format permintaan salah")
		return
	}

	if _, err := ambilStok(r.Context(), r, u.ID, id); err != nil {
		if tidakAda(err) {
			kirimPesan(w, http.StatusNotFound, "Stok tidak ditemukan")
			return
		}
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat stok")
		return
	}

	if isian.Quantity != nil && *isian.Quantity <= 0 {
		kirimValidasi(w, "Jumlah stok harus lebih dari 0", "quantity", "Jumlah harus lebih dari 0")
		return
	}
	expiry := tanggalBersih(isian.ExpiryDate)
	if expiry != nil && *expiry < time.Now().Format("2006-01-02") {
		kirimValidasi(w, "Tanggal kedaluwarsa tidak boleh di masa lalu", "expiry_date", "Tanggal harus hari ini atau setelahnya")
		return
	}

	unitBaru := ""
	if isian.Unit != nil {
		unitBaru = strings.TrimSpace(*isian.Unit)
	}

	if _, err := db.Exec(r.Context(),
		`UPDATE stok SET
		    jumlah = COALESCE($1::numeric, jumlah),
		    satuan = COALESCE(NULLIF($2,''), satuan),
		    tanggal_kadaluarsa = $3::date
		 WHERE id = $4 AND user_id = $5`,
		isian.Quantity, unitBaru, expiry, id, u.ID); err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui stok")
		return
	}

	s, err := ambilStok(r.Context(), r, u.ID, id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat stok")
		return
	}
	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true, "message": "Stok berhasil diperbarui", "data": s,
	})
}

// DELETE /api/stocks/{id}
func apiHapusStok(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	tag, err := db.Exec(r.Context(), `DELETE FROM stok WHERE id = $1 AND user_id = $2`, id, u.ID)
	if err != nil || tag.RowsAffected() == 0 {
		kirimPesan(w, http.StatusNotFound, "Stok tidak ditemukan")
		return
	}
	kirimPesanSukses(w, http.StatusOK, "Stok berhasil dihapus")
}

// GET /api/notifications/stock
func apiNotifikasiStok(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	semua, err := daftarStok(r.Context(), r, u.ID, "", 0)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat notifikasi")
		return
	}

	expiring := []stokJSON{}
	for _, s := range semua {
		if s.Status == "expiring" || s.Status == "expired" {
			expiring = append(expiring, s)
		}
	}

	rendah := []stokJSON{}
	for _, s := range semua {
		if s.Status != "expired" && stokRendah(s.Quantity, s.Unit) {
			rendah = append(rendah, s)
		}
	}
	sort.SliceStable(rendah, func(i, j int) bool { return rendah[i].Quantity < rendah[j].Quantity })
	if len(rendah) > 8 {
		rendah = rendah[:8]
	}

	pengingat, err := pengingatPengukuran(r, u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat notifikasi")
		return
	}

	kirimData(w, map[string]any{
		"expiring":        expiring,
		"low_stock":       rendah,
		"measurement_due": pengingat,
	})
}

func stokRendah(qty float64, unit string) bool {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch {
	case strings.Contains(u, "kg"), strings.Contains(u, "kilogram"):
		return qty < 0.5
	case strings.HasSuffix(u, "l") && !strings.Contains(u, "ml"):
		return qty < 0.5
	case strings.Contains(u, "liter"):
		return qty < 0.5
	case strings.Contains(u, "ml"):
		return qty < 250
	case strings.Contains(u, "g"):
		return qty < 100
	default:
		return qty <= 1
	}
}

func pengingatPengukuran(r *http.Request, userID string) ([]map[string]any, error) {
	rows, err := db.Query(r.Context(), `
		SELECT c.id::text, c.nama, COALESCE(c.jenis_kelamin,'male'),
		       COALESCE(c.tanggal_lahir::text,''),
		       g.tanggal_pengukuran::text, g.berat_kg::float8, g.panjang_tinggi_cm::float8,
		       CASE WHEN g.tanggal_pengukuran IS NULL THEN NULL
		            ELSE (CURRENT_DATE - g.tanggal_pengukuran) END
		FROM children_profiles c
		LEFT JOIN LATERAL (
			SELECT tanggal_pengukuran, berat_kg, panjang_tinggi_cm
			FROM growth_records WHERE child_id = c.id
			ORDER BY tanggal_pengukuran DESC LIMIT 1
		) g ON true
		WHERE c.user_id = $1
		ORDER BY c.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hasil := []map[string]any{}
	for rows.Next() {
		var id, nama, gender, lahir string
		var tglUkur *string
		var berat, tinggi *float64
		var selisih *int32
		if err := rows.Scan(&id, &nama, &gender, &lahir, &tglUkur, &berat, &tinggi, &selisih); err != nil {
			return nil, err
		}
		var selisihInt *int
		if selisih != nil {
			v := int(*selisih)
			selisihInt = &v
		}
		if tglUkur != nil && selisihInt != nil && *selisihInt < 30 {
			continue
		}
		item := map[string]any{
			"child_id":         id,
			"name":             nama,
			"gender":           genderDariDB(gender),
			"birth_date":       lahir,
			"last_measured_at": tglUkur,
			"days_since_last":  selisihInt,
			"last_weight":      berat,
			"last_height":      tinggi,
		}
		hasil = append(hasil, item)
	}
	return hasil, nil
}

// tanggalBersih mengubah string tanggal menjadi *string yang aman untuk SQL,
// memperlakukan teks kosong sebagai NULL.
func tanggalBersih(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	if len(t) >= 10 {
		t = t[:10]
	}
	return &t
}
