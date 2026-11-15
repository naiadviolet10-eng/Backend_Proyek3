package main

import (
	"net/http"
	"strings"
	"time"
)

type foodLogJSON struct {
	ID              string   `json:"id"`
	ChildID         string   `json:"child_id"`
	NamaMakanan     string   `json:"nama_makanan"`
	TanggalKonsumsi string   `json:"tanggal_konsumsi"`
	WaktuMakan      string   `json:"waktu_makan"`
	Porsi           float64  `json:"porsi"`
	KaloriKcal      *float64 `json:"kalori_kcal"`
	ProteinG        *float64 `json:"protein_g"`
	KarbohidratG    *float64 `json:"karbohidrat_g"`
	LemakG          *float64 `json:"lemak_g"`
	SeratG          *float64 `json:"serat_g"`
	Sumber          string   `json:"sumber"`
	ResepID         *int64   `json:"resep_id"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type maternalFoodLogJSON struct {
	ID              string   `json:"id"`
	PregnancyID     string   `json:"pregnancy_id"`
	NamaMakanan     string   `json:"nama_makanan"`
	TanggalKonsumsi string   `json:"tanggal_konsumsi"`
	WaktuMakan      string   `json:"waktu_makan"`
	Porsi           float64  `json:"porsi"`
	KaloriKcal      *float64 `json:"kalori_kcal"`
	ProteinG        *float64 `json:"protein_g"`
	KarbohidratG    *float64 `json:"karbohidrat_g"`
	LemakG          *float64 `json:"lemak_g"`
	SeratG          *float64 `json:"serat_g"`
	Sumber          string   `json:"sumber"`
	ResepID         *int64   `json:"resep_id"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

func validasiTanggal(tgl string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", tgl)
	return t, err == nil
}

func tanggalMasaDepan(tgl time.Time) bool {
	skrg := time.Now().UTC().Add(14 * time.Hour)
	batas := time.Date(skrg.Year(), skrg.Month(), skrg.Day(), 0, 0, 0, 0, time.UTC)
	return tgl.After(batas)
}

func validasiWaktuMakan(wm string) bool {
	wm = strings.TrimSpace(wm)
	return wm == "Sarapan" || wm == "Makan siang" || wm == "Makan malam" || wm == "Selingan"
}

func apiDaftarFoodLogs(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	childID := r.URL.Query().Get("child_id")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if childID == "" {
		kirimPesan(w, http.StatusBadRequest, "child_id wajib")
		return
	}
	if startDate == "" || endDate == "" {
		kirimPesan(w, http.StatusBadRequest, "start_date dan end_date wajib")
		return
	}
	if _, ok := validasiTanggal(startDate); !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid (YYYY-MM-DD)")
		return
	}
	if _, ok := validasiTanggal(endDate); !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid (YYYY-MM-DD)")
		return
	}

	var exists bool
	err := db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM children_profiles WHERE id=$1 AND user_id=$2)`, childID, u.ID).Scan(&exists)
	if err != nil || !exists {
		kirimPesan(w, http.StatusForbidden, "Anak tidak ditemukan atau bukan milik Anda")
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT id::text, child_id::text, nama_makanan, tanggal_konsumsi::text, waktu_makan, porsi::float8,
		       kalori_kcal::float8, protein_g::float8, karbohidrat_g::float8, lemak_g::float8, serat_g::float8,
		       COALESCE(sumber, 'manual'), resep_id, created_at, COALESCE(updated_at, created_at)
		FROM food_logs
		WHERE child_id = $1 AND tanggal_konsumsi >= $2 AND tanggal_konsumsi <= $3
		ORDER BY tanggal_konsumsi DESC, created_at DESC`, childID, startDate, endDate)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat catatan")
		return
	}
	defer rows.Close()

	hasil := []foodLogJSON{}
	for rows.Next() {
		var log foodLogJSON
		var createdAt, updatedAt time.Time
		err := rows.Scan(&log.ID, &log.ChildID, &log.NamaMakanan, &log.TanggalKonsumsi, &log.WaktuMakan, &log.Porsi,
			&log.KaloriKcal, &log.ProteinG, &log.KarbohidratG, &log.LemakG, &log.SeratG,
			&log.Sumber, &log.ResepID, &createdAt, &updatedAt)
		if err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat catatan")
			return
		}
		log.CreatedAt = createdAt.Format(time.RFC3339)
		log.UpdatedAt = updatedAt.Format(time.RFC3339)
		hasil = append(hasil, log)
	}
	kirimData(w, hasil)
}

func apiTambahFoodLog(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		ChildID         string   `json:"child_id"`
		NamaMakanan     string   `json:"nama_makanan"`
		TanggalKonsumsi string   `json:"tanggal_konsumsi"`
		WaktuMakan      string   `json:"waktu_makan"`
		Porsi           float64  `json:"porsi"`
		KaloriKcal      *float64 `json:"kalori_kcal"`
		ProteinG        *float64 `json:"protein_g"`
		KarbohidratG    *float64 `json:"karbohidrat_g"`
		LemakG          *float64 `json:"lemak_g"`
		SeratG          *float64 `json:"serat_g"`
		Sumber          string   `json:"sumber"`
		ResepID         *int64   `json:"resep_id"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimPesan(w, http.StatusBadRequest, "Body JSON tidak valid")
		return
	}

	isian.NamaMakanan = strings.TrimSpace(isian.NamaMakanan)
	if isian.NamaMakanan == "" || len([]rune(isian.NamaMakanan)) > 120 {
		kirimPesan(w, http.StatusBadRequest, "Nama makanan wajib diisi (maks 120 karakter).")
		return
	}
	tgl, ok := validasiTanggal(isian.TanggalKonsumsi)
	if !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid.")
		return
	}
	if tanggalMasaDepan(tgl) {
		kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh di masa depan.")
		return
	}
	if !validasiWaktuMakan(isian.WaktuMakan) {
		kirimPesan(w, http.StatusBadRequest, "Waktu makan harus Sarapan, Makan siang, Makan malam, atau Selingan.")
		return
	}
	if isian.Porsi <= 0 {
		kirimPesan(w, http.StatusBadRequest, "Porsi harus lebih dari nol.")
		return
	}
	if (isian.KaloriKcal != nil && *isian.KaloriKcal < 0) ||
		(isian.ProteinG != nil && *isian.ProteinG < 0) ||
		(isian.KarbohidratG != nil && *isian.KarbohidratG < 0) ||
		(isian.LemakG != nil && *isian.LemakG < 0) ||
		(isian.SeratG != nil && *isian.SeratG < 0) {
		kirimPesan(w, http.StatusBadRequest, "Nilai gizi tidak boleh negatif.")
		return
	}

	var birthDate *string
	err := db.QueryRow(r.Context(), `
		SELECT tanggal_lahir::text FROM children_profiles WHERE id=$1 AND user_id=$2`,
		isian.ChildID, u.ID).Scan(&birthDate)
	if err != nil {
		kirimPesan(w, http.StatusForbidden, "Anak tidak ditemukan atau bukan milik Anda.")
		return
	}
	if birthDate != nil {
		if lahir, ok := validasiTanggal(*birthDate); ok && tgl.Before(lahir) {
			kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh sebelum tanggal lahir.")
			return
		}
	}

	if isian.Sumber == "" {
		isian.Sumber = "manual"
	}

	var id string
	var createdAt, updatedAt time.Time
	err = db.QueryRow(r.Context(), `
		INSERT INTO food_logs (child_id, nama_makanan, tanggal_konsumsi, waktu_makan, porsi,
		                       kalori_kcal, protein_g, karbohidrat_g, lemak_g, serat_g, sumber, resep_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id::text, created_at, COALESCE(updated_at, created_at)`,
		isian.ChildID, isian.NamaMakanan, isian.TanggalKonsumsi, isian.WaktuMakan, isian.Porsi,
		isian.KaloriKcal, isian.ProteinG, isian.KarbohidratG, isian.LemakG, isian.SeratG, isian.Sumber, isian.ResepID).
		Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan catatan")
		return
	}

	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "Catatan berhasil disimpan",
		"data": foodLogJSON{
			ID:              id,
			ChildID:         isian.ChildID,
			NamaMakanan:     isian.NamaMakanan,
			TanggalKonsumsi: isian.TanggalKonsumsi,
			WaktuMakan:      isian.WaktuMakan,
			Porsi:           isian.Porsi,
			KaloriKcal:      isian.KaloriKcal,
			ProteinG:        isian.ProteinG,
			KarbohidratG:    isian.KarbohidratG,
			LemakG:          isian.LemakG,
			SeratG:          isian.SeratG,
			Sumber:          isian.Sumber,
			ResepID:         isian.ResepID,
			CreatedAt:       createdAt.Format(time.RFC3339),
			UpdatedAt:       updatedAt.Format(time.RFC3339),
		},
	})
}

func apiUbahFoodLog(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	var isian struct {
		NamaMakanan     string   `json:"nama_makanan"`
		TanggalKonsumsi string   `json:"tanggal_konsumsi"`
		WaktuMakan      string   `json:"waktu_makan"`
		Porsi           float64  `json:"porsi"`
		KaloriKcal      *float64 `json:"kalori_kcal"`
		ProteinG        *float64 `json:"protein_g"`
		KarbohidratG    *float64 `json:"karbohidrat_g"`
		LemakG          *float64 `json:"lemak_g"`
		SeratG          *float64 `json:"serat_g"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimPesan(w, http.StatusBadRequest, "Body JSON tidak valid")
		return
	}

	isian.NamaMakanan = strings.TrimSpace(isian.NamaMakanan)
	if isian.NamaMakanan == "" || len([]rune(isian.NamaMakanan)) > 120 {
		kirimPesan(w, http.StatusBadRequest, "Nama makanan wajib diisi (maks 120 karakter).")
		return
	}
	tgl, ok := validasiTanggal(isian.TanggalKonsumsi)
	if !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid.")
		return
	}
	if tanggalMasaDepan(tgl) {
		kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh di masa depan.")
		return
	}
	if !validasiWaktuMakan(isian.WaktuMakan) {
		kirimPesan(w, http.StatusBadRequest, "Waktu makan tidak valid.")
		return
	}
	if isian.Porsi <= 0 {
		kirimPesan(w, http.StatusBadRequest, "Porsi harus lebih dari nol.")
		return
	}
	if (isian.KaloriKcal != nil && *isian.KaloriKcal < 0) ||
		(isian.ProteinG != nil && *isian.ProteinG < 0) ||
		(isian.KarbohidratG != nil && *isian.KarbohidratG < 0) ||
		(isian.LemakG != nil && *isian.LemakG < 0) ||
		(isian.SeratG != nil && *isian.SeratG < 0) {
		kirimPesan(w, http.StatusBadRequest, "Nilai gizi tidak boleh negatif.")
		return
	}

	var childID, sumber string
	var resepID *int64
	var birthDate *string
	err := db.QueryRow(r.Context(), `
		SELECT f.child_id::text, COALESCE(f.sumber, 'manual'), f.resep_id, c.tanggal_lahir::text
		FROM food_logs f
		JOIN children_profiles c ON c.id = f.child_id
		WHERE f.id = $1 AND c.user_id = $2`, id, u.ID).Scan(&childID, &sumber, &resepID, &birthDate)
	if err != nil {
		if tidakAda(err) {
			kirimPesan(w, http.StatusNotFound, "Catatan tidak ditemukan")
		} else {
			kirimPesan(w, http.StatusInternalServerError, "Gagal mengubah catatan")
		}
		return
	}

	if birthDate != nil {
		if lahir, ok := validasiTanggal(*birthDate); ok && tgl.Before(lahir) {
			kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh sebelum tanggal lahir.")
			return
		}
	}

	var createdAt, updatedAt time.Time
	err = db.QueryRow(r.Context(), `
		UPDATE food_logs
		SET nama_makanan = $2, tanggal_konsumsi = $3, waktu_makan = $4, porsi = $5,
		    kalori_kcal = $6, protein_g = $7, karbohidrat_g = $8, lemak_g = $9, serat_g = $10,
		    updated_at = now()
		WHERE id = $1
		RETURNING created_at, updated_at`, id, isian.NamaMakanan, isian.TanggalKonsumsi, isian.WaktuMakan, isian.Porsi,
		isian.KaloriKcal, isian.ProteinG, isian.KarbohidratG, isian.LemakG, isian.SeratG).Scan(&createdAt, &updatedAt)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal mengubah catatan")
		return
	}

	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Catatan berhasil diubah",
		"data": foodLogJSON{
			ID:              id,
			ChildID:         childID,
			NamaMakanan:     isian.NamaMakanan,
			TanggalKonsumsi: isian.TanggalKonsumsi,
			WaktuMakan:      isian.WaktuMakan,
			Porsi:           isian.Porsi,
			KaloriKcal:      isian.KaloriKcal,
			ProteinG:        isian.ProteinG,
			KarbohidratG:    isian.KarbohidratG,
			LemakG:          isian.LemakG,
			SeratG:          isian.SeratG,
			Sumber:          sumber,
			ResepID:         resepID,
			CreatedAt:       createdAt.Format(time.RFC3339),
			UpdatedAt:       updatedAt.Format(time.RFC3339),
		},
	})
}

func apiHapusFoodLog(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")

	var exists bool
	err := db.QueryRow(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM food_logs f
			JOIN children_profiles c ON c.id = f.child_id
			WHERE f.id = $1 AND c.user_id = $2
		)`, id, u.ID).Scan(&exists)
	if err != nil || !exists {
		kirimPesan(w, http.StatusNotFound, "Catatan tidak ditemukan")
		return
	}

	_, err = db.Exec(r.Context(), `DELETE FROM food_logs WHERE id = $1`, id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menghapus catatan")
		return
	}

	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Catatan berhasil dihapus",
	})
}

func apiDaftarMaternalFoodLogs(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	pregnancyID := r.URL.Query().Get("pregnancy_id")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if pregnancyID == "" {
		kirimPesan(w, http.StatusBadRequest, "pregnancy_id wajib")
		return
	}
	if startDate == "" || endDate == "" {
		kirimPesan(w, http.StatusBadRequest, "start_date dan end_date wajib")
		return
	}
	if _, ok := validasiTanggal(startDate); !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid (YYYY-MM-DD)")
		return
	}
	if _, ok := validasiTanggal(endDate); !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid (YYYY-MM-DD)")
		return
	}

	var exists bool
	err := db.QueryRow(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM pregnancies p
			JOIN maternal_profiles m ON m.id = p.mother_id
			WHERE p.id = $1 AND m.user_id = $2
		)`, pregnancyID, u.ID).Scan(&exists)
	if err != nil || !exists {
		kirimPesan(w, http.StatusForbidden, "Kehamilan tidak ditemukan atau bukan milik Anda")
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT id::text, pregnancy_id::text, nama_makanan, tanggal_konsumsi::text, waktu_makan, porsi::float8,
		       kalori_kcal::float8, protein_g::float8, karbohidrat_g::float8, lemak_g::float8, serat_g::float8,
		       COALESCE(sumber, 'manual'), resep_id, created_at, COALESCE(updated_at, created_at)
		FROM maternal_food_logs
		WHERE pregnancy_id = $1 AND tanggal_konsumsi >= $2 AND tanggal_konsumsi <= $3
		ORDER BY tanggal_konsumsi DESC, created_at DESC`, pregnancyID, startDate, endDate)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat catatan")
		return
	}
	defer rows.Close()

	hasil := []maternalFoodLogJSON{}
	for rows.Next() {
		var log maternalFoodLogJSON
		var createdAt, updatedAt time.Time
		err := rows.Scan(&log.ID, &log.PregnancyID, &log.NamaMakanan, &log.TanggalKonsumsi, &log.WaktuMakan, &log.Porsi,
			&log.KaloriKcal, &log.ProteinG, &log.KarbohidratG, &log.LemakG, &log.SeratG,
			&log.Sumber, &log.ResepID, &createdAt, &updatedAt)
		if err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat catatan")
			return
		}
		log.CreatedAt = createdAt.Format(time.RFC3339)
		log.UpdatedAt = updatedAt.Format(time.RFC3339)
		hasil = append(hasil, log)
	}
	kirimData(w, hasil)
}

func apiTambahMaternalFoodLog(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		PregnancyID     string   `json:"pregnancy_id"`
		NamaMakanan     string   `json:"nama_makanan"`
		TanggalKonsumsi string   `json:"tanggal_konsumsi"`
		WaktuMakan      string   `json:"waktu_makan"`
		Porsi           float64  `json:"porsi"`
		KaloriKcal      *float64 `json:"kalori_kcal"`
		ProteinG        *float64 `json:"protein_g"`
		KarbohidratG    *float64 `json:"karbohidrat_g"`
		LemakG          *float64 `json:"lemak_g"`
		SeratG          *float64 `json:"serat_g"`
		Sumber          string   `json:"sumber"`
		ResepID         *int64   `json:"resep_id"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimPesan(w, http.StatusBadRequest, "Body JSON tidak valid")
		return
	}

	isian.NamaMakanan = strings.TrimSpace(isian.NamaMakanan)
	if isian.NamaMakanan == "" || len([]rune(isian.NamaMakanan)) > 120 {
		kirimPesan(w, http.StatusBadRequest, "Nama makanan wajib diisi (maks 120 karakter).")
		return
	}
	tgl, ok := validasiTanggal(isian.TanggalKonsumsi)
	if !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid.")
		return
	}
	if tanggalMasaDepan(tgl) {
		kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh di masa depan.")
		return
	}
	if !validasiWaktuMakan(isian.WaktuMakan) {
		kirimPesan(w, http.StatusBadRequest, "Waktu makan harus Sarapan, Makan siang, Makan malam, atau Selingan.")
		return
	}
	if isian.Porsi <= 0 {
		kirimPesan(w, http.StatusBadRequest, "Porsi harus lebih dari nol.")
		return
	}
	if (isian.KaloriKcal != nil && *isian.KaloriKcal < 0) ||
		(isian.ProteinG != nil && *isian.ProteinG < 0) ||
		(isian.KarbohidratG != nil && *isian.KarbohidratG < 0) ||
		(isian.LemakG != nil && *isian.LemakG < 0) ||
		(isian.SeratG != nil && *isian.SeratG < 0) {
		kirimPesan(w, http.StatusBadRequest, "Nilai gizi tidak boleh negatif.")
		return
	}

	var hpht *string
	err := db.QueryRow(r.Context(), `
		SELECT p.hpht::text
		FROM pregnancies p
		JOIN maternal_profiles m ON m.id = p.mother_id
		WHERE p.id = $1 AND m.user_id = $2`, isian.PregnancyID, u.ID).Scan(&hpht)
	if err != nil {
		kirimPesan(w, http.StatusForbidden, "Kehamilan tidak ditemukan atau bukan milik Anda.")
		return
	}
	if hpht != nil {
		if hphtDate, ok := validasiTanggal(*hpht); ok && tgl.Before(hphtDate) {
			kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh sebelum HPHT.")
			return
		}
	}

	if isian.Sumber == "" {
		isian.Sumber = "manual"
	}

	var id string
	var createdAt, updatedAt time.Time
	err = db.QueryRow(r.Context(), `
		INSERT INTO maternal_food_logs (pregnancy_id, nama_makanan, tanggal_konsumsi, waktu_makan, porsi,
		                                 kalori_kcal, protein_g, karbohidrat_g, lemak_g, serat_g, sumber, resep_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id::text, created_at, COALESCE(updated_at, created_at)`,
		isian.PregnancyID, isian.NamaMakanan, isian.TanggalKonsumsi, isian.WaktuMakan, isian.Porsi,
		isian.KaloriKcal, isian.ProteinG, isian.KarbohidratG, isian.LemakG, isian.SeratG, isian.Sumber, isian.ResepID).
		Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan catatan")
		return
	}

	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "Catatan berhasil disimpan",
		"data": maternalFoodLogJSON{
			ID:              id,
			PregnancyID:     isian.PregnancyID,
			NamaMakanan:     isian.NamaMakanan,
			TanggalKonsumsi: isian.TanggalKonsumsi,
			WaktuMakan:      isian.WaktuMakan,
			Porsi:           isian.Porsi,
			KaloriKcal:      isian.KaloriKcal,
			ProteinG:        isian.ProteinG,
			KarbohidratG:    isian.KarbohidratG,
			LemakG:          isian.LemakG,
			SeratG:          isian.SeratG,
			Sumber:          isian.Sumber,
			ResepID:         isian.ResepID,
			CreatedAt:       createdAt.Format(time.RFC3339),
			UpdatedAt:       updatedAt.Format(time.RFC3339),
		},
	})
}

func apiUbahMaternalFoodLog(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	var isian struct {
		NamaMakanan     string   `json:"nama_makanan"`
		TanggalKonsumsi string   `json:"tanggal_konsumsi"`
		WaktuMakan      string   `json:"waktu_makan"`
		Porsi           float64  `json:"porsi"`
		KaloriKcal      *float64 `json:"kalori_kcal"`
		ProteinG        *float64 `json:"protein_g"`
		KarbohidratG    *float64 `json:"karbohidrat_g"`
		LemakG          *float64 `json:"lemak_g"`
		SeratG          *float64 `json:"serat_g"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimPesan(w, http.StatusBadRequest, "Body JSON tidak valid")
		return
	}

	isian.NamaMakanan = strings.TrimSpace(isian.NamaMakanan)
	if isian.NamaMakanan == "" || len([]rune(isian.NamaMakanan)) > 120 {
		kirimPesan(w, http.StatusBadRequest, "Nama makanan wajib diisi (maks 120 karakter).")
		return
	}
	tgl, ok := validasiTanggal(isian.TanggalKonsumsi)
	if !ok {
		kirimPesan(w, http.StatusBadRequest, "Format tanggal tidak valid.")
		return
	}
	if tanggalMasaDepan(tgl) {
		kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh di masa depan.")
		return
	}
	if !validasiWaktuMakan(isian.WaktuMakan) {
		kirimPesan(w, http.StatusBadRequest, "Waktu makan tidak valid.")
		return
	}
	if isian.Porsi <= 0 {
		kirimPesan(w, http.StatusBadRequest, "Porsi harus lebih dari nol.")
		return
	}
	if (isian.KaloriKcal != nil && *isian.KaloriKcal < 0) ||
		(isian.ProteinG != nil && *isian.ProteinG < 0) ||
		(isian.KarbohidratG != nil && *isian.KarbohidratG < 0) ||
		(isian.LemakG != nil && *isian.LemakG < 0) ||
		(isian.SeratG != nil && *isian.SeratG < 0) {
		kirimPesan(w, http.StatusBadRequest, "Nilai gizi tidak boleh negatif.")
		return
	}

	var pregnancyID, sumber string
	var resepID *int64
	var hpht *string
	err := db.QueryRow(r.Context(), `
		SELECT f.pregnancy_id::text, COALESCE(f.sumber, 'manual'), f.resep_id, p.hpht::text
		FROM maternal_food_logs f
		JOIN pregnancies p ON p.id = f.pregnancy_id
		JOIN maternal_profiles m ON m.id = p.mother_id
		WHERE f.id = $1 AND m.user_id = $2`, id, u.ID).Scan(&pregnancyID, &sumber, &resepID, &hpht)
	if err != nil {
		if tidakAda(err) {
			kirimPesan(w, http.StatusNotFound, "Catatan tidak ditemukan")
		} else {
			kirimPesan(w, http.StatusInternalServerError, "Gagal mengubah catatan")
		}
		return
	}

	if hpht != nil {
		if hphtDate, ok := validasiTanggal(*hpht); ok && tgl.Before(hphtDate) {
			kirimPesan(w, http.StatusBadRequest, "Tanggal tidak boleh sebelum HPHT.")
			return
		}
	}

	var createdAt, updatedAt time.Time
	err = db.QueryRow(r.Context(), `
		UPDATE maternal_food_logs
		SET nama_makanan = $2, tanggal_konsumsi = $3, waktu_makan = $4, porsi = $5,
		    kalori_kcal = $6, protein_g = $7, karbohidrat_g = $8, lemak_g = $9, serat_g = $10,
		    updated_at = now()
		WHERE id = $1
		RETURNING created_at, updated_at`, id, isian.NamaMakanan, isian.TanggalKonsumsi, isian.WaktuMakan, isian.Porsi,
		isian.KaloriKcal, isian.ProteinG, isian.KarbohidratG, isian.LemakG, isian.SeratG).Scan(&createdAt, &updatedAt)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal mengubah catatan")
		return
	}

	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Catatan berhasil diubah",
		"data": maternalFoodLogJSON{
			ID:              id,
			PregnancyID:     pregnancyID,
			NamaMakanan:     isian.NamaMakanan,
			TanggalKonsumsi: isian.TanggalKonsumsi,
			WaktuMakan:      isian.WaktuMakan,
			Porsi:           isian.Porsi,
			KaloriKcal:      isian.KaloriKcal,
			ProteinG:        isian.ProteinG,
			KarbohidratG:    isian.KarbohidratG,
			LemakG:          isian.LemakG,
			SeratG:          isian.SeratG,
			Sumber:          sumber,
			ResepID:         resepID,
			CreatedAt:       createdAt.Format(time.RFC3339),
			UpdatedAt:       updatedAt.Format(time.RFC3339),
		},
	})
}

func apiHapusMaternalFoodLog(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")

	var exists bool
	err := db.QueryRow(r.Context(), `
		SELECT EXISTS(
			SELECT 1 FROM maternal_food_logs f
			JOIN pregnancies p ON p.id = f.pregnancy_id
			JOIN maternal_profiles m ON m.id = p.mother_id
			WHERE f.id = $1 AND m.user_id = $2
		)`, id, u.ID).Scan(&exists)
	if err != nil || !exists {
		kirimPesan(w, http.StatusNotFound, "Catatan tidak ditemukan")
		return
	}

	_, err = db.Exec(r.Context(), `DELETE FROM maternal_food_logs WHERE id = $1`, id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menghapus catatan")
		return
	}

	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Catatan berhasil dihapus",
	})
}
