package main

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Lama gestasi standar untuk menghitung perkiraan tanggal lahir (HPL).
const hariGestasi = 280

// statusKeDB memetakan status versi aplikasi ke nilai kolom pregnancies.
func statusKeDB(s string) (string, bool) {
	switch s {
	case "active":
		return "hamil", true
	case "delivered":
		return "lahir", true
	case "archived":
		return "selesai", true
	default:
		return "", false
	}
}

// statusKeApp memetakan status kolom pregnancies ke versi aplikasi.
func statusKeApp(s string) string {
	switch s {
	case "hamil":
		return "active"
	case "lahir":
		return "delivered"
	default:
		return "archived"
	}
}

func profilIbu(ctx context.Context, userID string) (string, string, error) {
	var id, nama string
	err := db.QueryRow(ctx,
		`SELECT id::text, nama_ibu FROM maternal_profiles WHERE user_id = $1 LIMIT 1`, userID).
		Scan(&id, &nama)
	return id, nama, err
}

func pastikanProfilIbu(ctx context.Context, userID, nama string) (string, error) {
	id, _, err := profilIbu(ctx, userID)
	if err == nil {
		nama = strings.TrimSpace(nama)
		if nama != "" {
			_, _ = db.Exec(ctx, `UPDATE maternal_profiles SET nama_ibu=$1, updated_at=now() WHERE id=$2`, nama, id)
		}
		return id, nil
	}
	if !tidakAda(err) {
		return "", err
	}
	var baru string
	err = db.QueryRow(ctx,
		`INSERT INTO maternal_profiles (user_id, nama_ibu) VALUES ($1,$2) RETURNING id::text`,
		userID, strings.TrimSpace(nama)).Scan(&baru)
	return baru, err
}

func hpl(isoDate string) string {
	t, err := time.Parse("2006-01-02", isoDate)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, hariGestasi).Format("2006-01-02")
}

func kehamilanDari(id, namaIbu, hpht string, hplVal *string, status string, tanggalLahir, catatan *string) kehamilanJSON {
	due := ""
	if hplVal != nil {
		due = *hplVal
	} else {
		due = hpl(hpht)
	}
	return kehamilanJSON{
		ID:           id,
		MotherName:   namaIbu,
		HphtDate:     hpht,
		DueDate:      due,
		Status:       statusKeApp(status),
		DeliveryDate: tanggalLahir,
		Notes:        catatan,
	}
}

func barisKehamilan(r *http.Request, userID, id string) (kehamilanJSON, error) {
	var k kehamilanJSON
	var hpht string
	var hplVal, tanggalLahir, catatan *string
	var status string
	err := db.QueryRow(r.Context(), `
		SELECT p.id::text, COALESCE(m.nama_ibu,''), p.hpht::text, p.hpl::text, p.status,
		       p.tanggal_persalinan::text, p.catatan
		FROM pregnancies p
		JOIN maternal_profiles m ON m.id = p.mother_id
		WHERE p.id = $1 AND m.user_id = $2`, id, userID).
		Scan(&k.ID, &k.MotherName, &hpht, &hplVal, &status, &tanggalLahir, &catatan)
	if err != nil {
		return k, err
	}
	return kehamilanDari(k.ID, k.MotherName, hpht, hplVal, status, tanggalLahir, catatan), nil
}

// GET /api/pregnancies/active
func apiKehamilanAktif(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	ibuID, _, err := profilIbu(r.Context(), u.ID)
	if err != nil {
		kirimData(w, nil)
		return
	}
	var k kehamilanJSON
	var hpht string
	var hplVal, tanggalLahir, catatan *string
	var status string
	err = db.QueryRow(r.Context(), `
		SELECT p.id::text, COALESCE(m.nama_ibu,''), p.hpht::text, p.hpl::text, p.status,
		       p.tanggal_persalinan::text, p.catatan
		FROM pregnancies p
		JOIN maternal_profiles m ON m.id = p.mother_id
		WHERE p.mother_id = $1 AND p.status = 'hamil'
		ORDER BY p.created_at DESC LIMIT 1`, ibuID).
		Scan(&k.ID, &k.MotherName, &hpht, &hplVal, &status, &tanggalLahir, &catatan)
	if err != nil {
		kirimData(w, nil)
		return
	}
	kirimData(w, kehamilanDari(k.ID, k.MotherName, hpht, hplVal, status, tanggalLahir, catatan))
}

// GET /api/pregnancies
func apiDaftarKehamilan(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	ibuID, _, err := profilIbu(r.Context(), u.ID)
	if err != nil {
		kirimData(w, []kehamilanJSON{})
		return
	}
	rows, err := db.Query(r.Context(), `
		SELECT p.id::text, COALESCE(m.nama_ibu,''), p.hpht::text, p.hpl::text, p.status,
		       p.tanggal_persalinan::text, p.catatan
		FROM pregnancies p
		JOIN maternal_profiles m ON m.id = p.mother_id
		WHERE p.mother_id = $1
		ORDER BY (p.status = 'hamil') DESC, p.created_at DESC`, ibuID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat data kehamilan")
		return
	}
	defer rows.Close()

	hasil := []kehamilanJSON{}
	for rows.Next() {
		var k kehamilanJSON
		var hpht string
		var hplVal, tanggalLahir, catatan *string
		var status string
		if err := rows.Scan(&k.ID, &k.MotherName, &hpht, &hplVal, &status, &tanggalLahir, &catatan); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat data kehamilan")
			return
		}
		hasil = append(hasil, kehamilanDari(k.ID, k.MotherName, hpht, hplVal, status, tanggalLahir, catatan))
	}
	kirimData(w, hasil)
}

// POST /api/pregnancies
func apiTambahKehamilan(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		MotherName string  `json:"mother_name"`
		HphtDate   string  `json:"hpht_date"`
		HplDate    *string `json:"hpl_date"`
		Notes      *string `json:"notes"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "mother_name", "Format permintaan salah")
		return
	}
	isian.MotherName = strings.TrimSpace(isian.MotherName)
	hpht := tanggalSaja(isian.HphtDate)
	hplBaru := hpl(hpht)
	if isian.HplDate != nil {
		hplBaru = tanggalSaja(*isian.HplDate)
	}
	catatan := teksOpsional(isian.Notes)

	errs := map[string]any{}
	if isian.MotherName == "" {
		errs["mother_name"] = []string{"Nama ibu wajib diisi"}
	} else if len([]rune(isian.MotherName)) > 100 {
		errs["mother_name"] = []string{"Nama maksimal 100 karakter"}
	}
	if hpht == "" || !tanggalValid(hpht) {
		errs["hpht_date"] = []string{"Tanggal HPHT wajib diisi dengan format yang benar"}
	} else if hpht > hariIni() {
		errs["hpht_date"] = []string{"Tanggal HPHT tidak boleh di masa depan"}
	}
	if hplBaru == "" || !tanggalValid(hplBaru) {
		errs["hpl_date"] = []string{"Tanggal HPL tidak valid"}
	} else if hpht != "" && hplBaru < hpht {
		errs["hpl_date"] = []string{"Tanggal HPL tidak boleh sebelum HPHT"}
	}
	if len(errs) > 0 {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}

	if hari, ok := umurHari(hpht); ok && hari > hariGestasi+14 {
		kirimPesan(w, http.StatusUnprocessableEntity,
			"Tanggal HPHT sudah lewat lebih dari 42 minggu. Perbarui tanggalnya atau tandai kehamilan sebelumnya sebagai selesai.")
		return
	}

	ibuID, err := pastikanProfilIbu(r.Context(), u.ID, isian.MotherName)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan profil kehamilan")
		return
	}

	_, _ = db.Exec(r.Context(),
		`UPDATE pregnancies SET status='selesai', updated_at=now() WHERE mother_id=$1 AND status='hamil'`, ibuID)

	var id string
	err = db.QueryRow(r.Context(), `
		INSERT INTO pregnancies (mother_id, hpht, hpl, status, catatan)
		VALUES ($1,$2::date,$3::date,'hamil',$4) RETURNING id::text`,
		ibuID, hpht, hplBaru, catatan).Scan(&id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan profil kehamilan")
		return
	}

	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "Profil kehamilan berhasil disimpan",
		"data":    kehamilanDari(id, isian.MotherName, hpht, &hplBaru, "hamil", nil, catatan),
	})
}

// GET /api/pregnancies/{id}
func apiDetailKehamilan(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	k, err := barisKehamilan(r, u.ID, r.PathValue("id"))
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Data kehamilan tidak ditemukan")
		return
	}
	kirimData(w, k)
}

// PUT /api/pregnancies/{id}
func apiUbahKehamilan(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	lama, err := barisKehamilan(r, u.ID, id)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Data kehamilan tidak ditemukan")
		return
	}

	var isian struct {
		MotherName *string `json:"mother_name"`
		HphtDate   *string `json:"hpht_date"`
		HplDate    *string `json:"hpl_date"`
		Status     *string `json:"status"`
		Notes      *string `json:"notes"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "mother_name", "Format permintaan salah")
		return
	}

	nama := lama.MotherName
	hpht := lama.HphtDate
	hplBaru := lama.DueDate
	if isian.MotherName != nil {
		nama = strings.TrimSpace(*isian.MotherName)
	}
	if isian.HphtDate != nil {
		hpht = tanggalSaja(*isian.HphtDate)
		hplBaru = hpl(hpht)
	}
	if isian.HplDate != nil {
		hplBaru = tanggalSaja(*isian.HplDate)
	}
	if hplBaru == "" {
		hplBaru = hpl(hpht)
	}
	catatan := teksOpsional(isian.Notes)
	if nama == "" || len([]rune(nama)) > 100 {
		kirimValidasi(w, "Nama ibu tidak valid", "mother_name", "Nama wajib 1 sampai 100 karakter")
		return
	}
	if hpht == "" || !tanggalValid(hpht) || hpht > hariIni() {
		kirimValidasi(w, "Tanggal HPHT tidak valid", "hpht_date", "Tanggal HPHT tidak boleh di masa depan")
		return
	}
	if !tanggalValid(hplBaru) || hplBaru < hpht {
		kirimValidasi(w, "Tanggal HPL tidak valid", "hpl_date", "Tanggal HPL tidak boleh sebelum HPHT")
		return
	}

	ibuID, err := pastikanProfilIbu(r.Context(), u.ID, nama)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui data kehamilan")
		return
	}

	status := ""
	if isian.Status != nil {
		s, ok := statusKeDB(strings.TrimSpace(*isian.Status))
		if !ok {
			kirimValidasi(w, "Status tidak valid", "status", "Status harus active, delivered, atau archived")
			return
		}
		status = s
	}

	if status == "hamil" {
		_, _ = db.Exec(r.Context(),
			`UPDATE pregnancies SET status='selesai', updated_at=now() WHERE mother_id=$1 AND status='hamil' AND id<>$2`, ibuID, id)
	}

	var tanggalPersalinan *string
	if status == "lahir" {
		hari := time.Now().Format("2006-01-02")
		tanggalPersalinan = &hari
	}

	var statusArg *string
	if status != "" {
		statusArg = &status
	}

	err = db.QueryRow(r.Context(), `
		UPDATE pregnancies SET
		  mother_id = $1,
		  hpht = $2::date,
		  hpl = $3::date,
		  status = COALESCE($4, status),
		  tanggal_persalinan = COALESCE($5::date, tanggal_persalinan),
		  catatan = COALESCE($7, catatan),
		  updated_at = now()
		WHERE id = $6
		RETURNING id::text`, ibuID, hpht, hplBaru, statusArg, tanggalPersalinan, id, catatan).Scan(&id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui data kehamilan")
		return
	}

	k, err := barisKehamilan(r, u.ID, id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat data kehamilan")
		return
	}
	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Profil kehamilan berhasil diperbarui",
		"data":    k,
	})
}

// POST /api/pregnancies/{id}/delivery
func apiLahirkan(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	var isian struct {
		DeliveryDate  string   `json:"delivery_date"`
		BabyName      string   `json:"baby_name"`
		Gender        string   `json:"gender"`
		BirthWeightKg *float64 `json:"birth_weight_kg"`
		BirthLengthCm *float64 `json:"birth_length_cm"`
		BirthHeadCm   *float64 `json:"birth_head_circumference_cm"`
		Notes         *string  `json:"notes"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "baby_name", "Format permintaan salah")
		return
	}
	isian.BabyName = strings.TrimSpace(isian.BabyName)
	isian.Gender = strings.TrimSpace(isian.Gender)
	lahir := tanggalSaja(isian.DeliveryDate)
	if lahir == "" {
		lahir = hariIni()
	}
	catatan := teksOpsional(isian.Notes)

	errs := map[string]any{}
	if isian.BabyName == "" {
		errs["baby_name"] = []string{"Nama bayi wajib diisi"}
	} else if len([]rune(isian.BabyName)) > 100 {
		errs["baby_name"] = []string{"Nama bayi maksimal 100 karakter"}
	}
	if isian.Gender != "Laki-laki" && isian.Gender != "Perempuan" {
		errs["gender"] = []string{"Jenis kelamin harus Laki-laki atau Perempuan"}
	}
	if !tanggalValid(lahir) || lahir > hariIni() {
		errs["delivery_date"] = []string{"Tanggal lahir tidak boleh di masa depan"}
	}
	for k, v := range validasiLahir(isian.BirthWeightKg, isian.BirthLengthCm, isian.BirthHeadCm) {
		errs[k] = v
	}
	if len(errs) > 0 {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}

	tx, err := db.Begin(r.Context())
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan data kelahiran")
		return
	}
	defer tx.Rollback(r.Context())

	var ibuID, namaIbu, hpht, status string
	var hplVal, catatanLama *string
	err = tx.QueryRow(r.Context(), `
		SELECT p.mother_id::text, COALESCE(m.nama_ibu,''), p.hpht::text, p.hpl::text, p.status, p.catatan
		FROM pregnancies p
		JOIN maternal_profiles m ON m.id = p.mother_id
		WHERE p.id = $1 AND m.user_id = $2
		FOR UPDATE OF p`, id, u.ID).
		Scan(&ibuID, &namaIbu, &hpht, &hplVal, &status, &catatanLama)
	if err != nil {
		if tidakAda(err) {
			kirimPesan(w, http.StatusNotFound, "Data kehamilan tidak ditemukan")
			return
		}
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat data kehamilan")
		return
	}
	if status != "hamil" {
		kirimPesan(w, http.StatusConflict, "Kehamilan ini sudah ditandai lahir atau selesai")
		return
	}
	if lahir < hpht {
		kirimValidasi(w, "Tanggal lahir tidak valid", "delivery_date", "Tanggal lahir tidak boleh sebelum HPHT")
		return
	}
	if hplVal == nil || *hplVal == "" {
		nilai := hpl(hpht)
		hplVal = &nilai
	}
	if catatan != nil {
		catatanLama = catatan
	}

	var a anakJSON
	var hari *string
	err = tx.QueryRow(r.Context(), `
		INSERT INTO children_profiles
		  (user_id, nama, jenis_kelamin, tanggal_lahir, hpht, hpl, pregnancy_id,
		   berat_lahir_kg, panjang_lahir_cm, lingkar_kepala_lahir_cm)
		VALUES ($1,$2,$3,$4::date,$5::date,$6::date,$7::uuid,$8,$9,$10)
		RETURNING id::text, user_id::text, nama, COALESCE(jenis_kelamin,'male'), tanggal_lahir::text,
		          hpht::text, hpl::text, pregnancy_id::text, berat_lahir_kg::float8, panjang_lahir_cm::float8,
		          lingkar_kepala_lahir_cm::float8`,
		u.ID, isian.BabyName, isian.Gender, lahir, hpht, *hplVal, id,
		isian.BirthWeightKg, isian.BirthLengthCm, isian.BirthHeadCm).
		Scan(&a.ID, &a.UserID, &a.Name, &a.Gender, &hari,
			&a.Hpht, &a.Hpl, &a.PregnancyID, &a.BirthWeightKg, &a.BirthLengthCm, &a.BirthHeadCm)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan profil bayi")
		return
	}
	a.Gender = genderDariDB(a.Gender)
	if hari != nil {
		a = anakDariLengkap(a, *hari)
	}

	if _, err := tx.Exec(r.Context(), `
		UPDATE pregnancies SET status='lahir', tanggal_persalinan=$1::date,
		       catatan=COALESCE($2, catatan), updated_at=now()
		WHERE id=$3`, lahir, catatan, id); err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui status kehamilan")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan data kelahiran")
		return
	}

	k := kehamilanDari(id, namaIbu, hpht, hplVal, "lahir", &lahir, catatanLama)
	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "Data kelahiran berhasil disimpan",
		"data": map[string]any{
			"pregnancy": k,
			"child":     a,
		},
	})
}

// DELETE /api/pregnancies/{id}
func apiHapusKehamilan(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	ibuID, _, err := profilIbu(r.Context(), u.ID)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Data kehamilan tidak ditemukan")
		return
	}
	tag, err := db.Exec(r.Context(), `DELETE FROM pregnancies WHERE id=$1 AND mother_id=$2`, id, ibuID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menghapus data kehamilan")
		return
	}
	if tag.RowsAffected() == 0 {
		kirimPesan(w, http.StatusNotFound, "Data kehamilan tidak ditemukan")
		return
	}
	kirimPesanSukses(w, http.StatusOK, "Data kehamilan berhasil dihapus")
}
