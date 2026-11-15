package main

import (
	"net/http"
	"strings"
	"time"
)

// maksHariAnak membatasi profil anak pada 1000 hari pertama kehidupan.
const maksHariAnak = 1000

func umurHariPada(lahir string, sekarang time.Time) (int, bool) {
	t, err := time.Parse("2006-01-02", lahir)
	if err != nil {
		return 0, false
	}
	hari := time.Date(sekarang.Year(), sekarang.Month(), sekarang.Day(), 0, 0, 0, 0, time.UTC)
	return int(hari.Sub(t).Hours() / 24), true
}

func umurHari(lahir string) (int, bool) {
	return umurHariPada(lahir, time.Now())
}

func ambilAnak(r *http.Request, userID, id string) (anakJSON, error) {
	var a anakJSON
	var lahir *string
	err := db.QueryRow(r.Context(), `
		SELECT id::text, user_id::text, nama, COALESCE(jenis_kelamin,'male'), tanggal_lahir::text,
		       hpht::text, hpl::text, pregnancy_id::text, berat_lahir_kg::float8, panjang_lahir_cm::float8,
		       lingkar_kepala_lahir_cm::float8
		FROM children_profiles WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&a.ID, &a.UserID, &a.Name, &a.Gender, &lahir,
			&a.Hpht, &a.Hpl, &a.PregnancyID, &a.BirthWeightKg, &a.BirthLengthCm, &a.BirthHeadCm)
	if err != nil {
		return a, err
	}
	a.Gender = genderDariDB(a.Gender)
	if lahir != nil {
		a = anakDariLengkap(a, *lahir)
	}
	return a, nil
}

// Kolom jenis_kelamin memakai istilah Indonesia, sedangkan API aplikasi
// memakai kode male/female. Konversi dilakukan di batas database.
func genderKeDB(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "male", "laki-laki", "laki laki", "l":
		return "Laki-laki"
	case "female", "perempuan", "p":
		return "Perempuan"
	}
	return ""
}

func genderDariDB(g string) string {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "female", "perempuan", "p":
		return "female"
	}
	return "male"
}

func anakDariLengkap(a anakJSON, lahir string) anakJSON {
	a.BirthDate = lahir
	if hari, ok := umurHari(lahir); ok {
		a.AgeInDays = hari
		a.AgeInMonths = hari / 30
		a.AgeInYears = hari / 365
	}
	return a
}

// GET /api/children
func apiDaftarAnak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	rows, err := db.Query(r.Context(), `
		SELECT id::text, user_id::text, nama, COALESCE(jenis_kelamin,'male'), tanggal_lahir::text,
		       hpht::text, hpl::text, pregnancy_id::text, berat_lahir_kg::float8, panjang_lahir_cm::float8,
		       lingkar_kepala_lahir_cm::float8
		FROM children_profiles WHERE user_id = $1 ORDER BY created_at DESC`, u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat profil anak")
		return
	}
	defer rows.Close()

	hasil := []anakJSON{}
	for rows.Next() {
		var a anakJSON
		var lahir *string
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Gender, &lahir,
			&a.Hpht, &a.Hpl, &a.PregnancyID, &a.BirthWeightKg, &a.BirthLengthCm, &a.BirthHeadCm); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat profil anak")
			return
		}
		a.Gender = genderDariDB(a.Gender)
		if lahir != nil {
			a = anakDariLengkap(a, *lahir)
		}
		hasil = append(hasil, a)
	}
	kirimData(w, hasil)
}

// POST /api/children
func apiTambahAnak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		Name          string   `json:"name"`
		Gender        string   `json:"gender"`
		BirthDate     string   `json:"birth_date"`
		Hpht          *string  `json:"hpht"`
		Hpl           *string  `json:"hpl"`
		BirthWeightKg *float64 `json:"birth_weight_kg"`
		BirthLengthCm *float64 `json:"birth_length_cm"`
		BirthHeadCm   *float64 `json:"birth_head_circumference_cm"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "name", "Format permintaan salah")
		return
	}
	isian.Name = strings.TrimSpace(isian.Name)
	lahir := tanggalSaja(isian.BirthDate)
	hpht := tanggalSaja(nilaiString(isian.Hpht))
	hplVal := tanggalSaja(nilaiString(isian.Hpl))
	if hpht != "" && hplVal == "" {
		hplVal = hpl(hpht)
	}

	if errs := validasiAnak(isian.Name, isian.Gender, lahir, true, hpht, hplVal); errs != nil {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}
	if errs := validasiLahir(isian.BirthWeightKg, isian.BirthLengthCm, isian.BirthHeadCm); errs != nil {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}

	var a anakJSON
	var hari *string
	err := db.QueryRow(r.Context(), `
		INSERT INTO children_profiles
		  (user_id, nama, jenis_kelamin, tanggal_lahir, hpht, hpl, berat_lahir_kg, panjang_lahir_cm, lingkar_kepala_lahir_cm)
		VALUES ($1,$2,$3,$4::date,$5::date,$6::date,$7,$8,$9)
		RETURNING id::text, user_id::text, nama, COALESCE(jenis_kelamin,'male'), tanggal_lahir::text,
		          hpht::text, hpl::text, pregnancy_id::text, berat_lahir_kg::float8, panjang_lahir_cm::float8,
		          lingkar_kepala_lahir_cm::float8`,
		u.ID, isian.Name, genderKeDB(isian.Gender), lahir, tanggalArg(hpht), tanggalArg(hplVal),
		isian.BirthWeightKg, isian.BirthLengthCm, isian.BirthHeadCm).
		Scan(&a.ID, &a.UserID, &a.Name, &a.Gender, &hari,
			&a.Hpht, &a.Hpl, &a.PregnancyID, &a.BirthWeightKg, &a.BirthLengthCm, &a.BirthHeadCm)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan profil anak")
		return
	}
	a.Gender = genderDariDB(a.Gender)
	if hari != nil {
		a = anakDariLengkap(a, *hari)
	}
	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true, "message": "Profil anak berhasil ditambahkan", "data": a,
	})
}

// GET /api/children/{id}
func apiDetailAnak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	a, err := ambilAnak(r, u.ID, r.PathValue("id"))
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan")
		return
	}
	kirimData(w, a)
}

// PUT /api/children/{id}
func apiUbahAnak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	lama, err := ambilAnak(r, u.ID, id)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan")
		return
	}

	var isian struct {
		Name          *string  `json:"name"`
		Gender        *string  `json:"gender"`
		BirthDate     *string  `json:"birth_date"`
		Hpht          *string  `json:"hpht"`
		Hpl           *string  `json:"hpl"`
		BirthWeightKg *float64 `json:"birth_weight_kg"`
		BirthLengthCm *float64 `json:"birth_length_cm"`
		BirthHeadCm   *float64 `json:"birth_head_circumference_cm"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "name", "Format permintaan salah")
		return
	}

	nama := lama.Name
	gender := lama.Gender
	lahir := lama.BirthDate
	hpht := nilaiString(lama.Hpht)
	hplVal := nilaiString(lama.Hpl)
	berat := lama.BirthWeightKg
	panjang := lama.BirthLengthCm
	kepala := lama.BirthHeadCm
	if isian.Name != nil {
		nama = strings.TrimSpace(*isian.Name)
	}
	if isian.Gender != nil {
		gender = strings.TrimSpace(*isian.Gender)
	}
	if isian.BirthDate != nil {
		lahir = tanggalSaja(*isian.BirthDate)
	}
	if isian.Hpht != nil {
		hpht = tanggalSaja(*isian.Hpht)
		hplVal = ""
	}
	if isian.Hpl != nil {
		hplVal = tanggalSaja(*isian.Hpl)
	}
	if isian.BirthWeightKg != nil {
		berat = isian.BirthWeightKg
	}
	if isian.BirthLengthCm != nil {
		panjang = isian.BirthLengthCm
	}
	if isian.BirthHeadCm != nil {
		kepala = isian.BirthHeadCm
	}
	if hpht != "" && hplVal == "" {
		hplVal = hpl(hpht)
	}

	if errs := validasiAnak(nama, gender, lahir, true, hpht, hplVal); errs != nil {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}
	if errs := validasiLahir(berat, panjang, kepala); errs != nil {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}

	var a anakJSON
	var hari *string
	err = db.QueryRow(r.Context(), `
		UPDATE children_profiles SET
		  nama=$1, jenis_kelamin=$2, tanggal_lahir=$3::date,
		  hpht=$4::date, hpl=$5::date, berat_lahir_kg=$6, panjang_lahir_cm=$7,
		  lingkar_kepala_lahir_cm=$8, updated_at=now()
		WHERE id=$9 AND user_id=$10
		RETURNING id::text, user_id::text, nama, COALESCE(jenis_kelamin,'male'), tanggal_lahir::text,
		          hpht::text, hpl::text, pregnancy_id::text, berat_lahir_kg::float8, panjang_lahir_cm::float8,
		          lingkar_kepala_lahir_cm::float8`,
		nama, genderKeDB(gender), lahir, tanggalArg(hpht), tanggalArg(hplVal), berat, panjang, kepala, id, u.ID).
		Scan(&a.ID, &a.UserID, &a.Name, &a.Gender, &hari,
			&a.Hpht, &a.Hpl, &a.PregnancyID, &a.BirthWeightKg, &a.BirthLengthCm, &a.BirthHeadCm)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memperbarui profil anak")
		return
	}
	a.Gender = genderDariDB(a.Gender)
	if hari != nil {
		a = anakDariLengkap(a, *hari)
	}
	kirimJSON(w, http.StatusOK, map[string]any{
		"success": true, "message": "Profil anak berhasil diperbarui", "data": a,
	})
}

// DELETE /api/children/{id}
func apiHapusAnak(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	tag, err := db.Exec(r.Context(), `DELETE FROM children_profiles WHERE id=$1 AND user_id=$2`, r.PathValue("id"), u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menghapus profil anak")
		return
	}
	if tag.RowsAffected() == 0 {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan")
		return
	}
	kirimPesanSukses(w, http.StatusOK, "Profil anak berhasil dihapus")
}

// GET /api/children/{id}/measurements
func apiDaftarPengukuran(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	if _, err := ambilAnak(r, u.ID, id); err != nil {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan")
		return
	}
	rows, err := db.Query(r.Context(), `
		SELECT id::text, child_id::text, berat_kg::float8, panjang_tinggi_cm::float8,
		       lingkar_kepala_cm::float8, catatan, tanggal_pengukuran::text
		FROM growth_records WHERE child_id=$1 ORDER BY tanggal_pengukuran ASC`, id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat pengukuran")
		return
	}
	defer rows.Close()

	hasil := []pengukuranJSON{}
	for rows.Next() {
		var p pengukuranJSON
		if err := rows.Scan(&p.ID, &p.ChildID, &p.Weight, &p.Height,
			&p.HeadCircumference, &p.Notes, &p.MeasuredAt); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat pengukuran")
			return
		}
		hasil = append(hasil, p)
	}
	kirimData(w, hasil)
}

// POST /api/children/{id}/measurements
func apiTambahPengukuran(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")
	anak, err := ambilAnak(r, u.ID, id)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan")
		return
	}
	var isian struct {
		Weight            *float64 `json:"weight"`
		Height            *float64 `json:"height"`
		HeadCircumference *float64 `json:"head_circumference_cm"`
		Notes             *string  `json:"notes"`
		MeasuredAt        string   `json:"measured_at"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "weight", "Format permintaan salah")
		return
	}
	ukur := tanggalSaja(isian.MeasuredAt)

	errs := map[string]any{}
	if isian.Weight == nil && isian.Height == nil && isian.HeadCircumference == nil {
		errs["weight"] = []string{"Isi minimal satu ukuran: berat, tinggi, atau lingkar kepala"}
	}
	if isian.Weight != nil && (*isian.Weight <= 0 || *isian.Weight > 50) {
		errs["weight"] = []string{"Berat harus antara 0 dan 50 kg"}
	}
	if isian.Height != nil && (*isian.Height <= 0 || *isian.Height > 200) {
		errs["height"] = []string{"Tinggi harus antara 0 dan 200 cm"}
	}
	if isian.HeadCircumference != nil && (*isian.HeadCircumference <= 0 || *isian.HeadCircumference > 100) {
		errs["head_circumference_cm"] = []string{"Lingkar kepala harus antara 0 dan 100 cm"}
	}
	if ukur == "" || !tanggalValid(ukur) || ukur > hariIni() {
		errs["measured_at"] = []string{"Tanggal pengukuran tidak boleh di masa depan"}
	} else if anak.BirthDate != "" && ukur < anak.BirthDate {
		errs["measured_at"] = []string{"Tanggal pengukuran tidak boleh sebelum tanggal lahir"}
	}
	if len(errs) > 0 {
		kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{"success": false, "errors": errs})
		return
	}
	catatan := teksOpsional(isian.Notes)

	var p pengukuranJSON
	err = db.QueryRow(r.Context(), `
		INSERT INTO growth_records
		  (child_id, berat_kg, panjang_tinggi_cm, lingkar_kepala_cm, catatan, tanggal_pengukuran)
		VALUES ($1,$2,$3,$4,$5,$6::date)
		RETURNING id::text, child_id::text, berat_kg::float8, panjang_tinggi_cm::float8,
		          lingkar_kepala_cm::float8, catatan, tanggal_pengukuran::text`,
		id, isian.Weight, isian.Height, isian.HeadCircumference, catatan, ukur).
		Scan(&p.ID, &p.ChildID, &p.Weight, &p.Height, &p.HeadCircumference, &p.Notes, &p.MeasuredAt)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan pengukuran")
		return
	}
	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true, "message": "Data pengukuran ditambahkan", "data": p,
	})
}

// validasiAnak memvalidasi nama, jenis kelamin, tanggal lahir, dan rentang HPHT
// dalam batas 1000 hari pertama kehidupan.
func validasiAnak(nama, gender, lahir string, wajib bool, hpht, hplVal string) map[string]any {
	errs := map[string]any{}
	if wajib || nama != "" {
		if nama == "" {
			errs["name"] = []string{"Nama anak wajib diisi"}
		} else if len([]rune(nama)) > 100 {
			errs["name"] = []string{"Nama maksimal 100 karakter"}
		}
	}
	if wajib || gender != "" {
		if gender != "male" && gender != "female" {
			errs["gender"] = []string{"Jenis kelamin harus male atau female"}
		}
	}
	if wajib || lahir != "" {
		if lahir == "" {
			errs["birth_date"] = []string{"Tanggal lahir wajib diisi"}
		} else if hari, ok := umurHari(lahir); !ok {
			errs["birth_date"] = []string{"Tanggal lahir tidak valid"}
		} else if hari < 0 {
			errs["birth_date"] = []string{"Tanggal lahir tidak boleh di masa depan"}
		} else if hari > maksHariAnak {
			errs["birth_date"] = []string{"Profil hanya untuk anak usia 0 sampai 1000 hari pertama kehidupan"}
		}
	}
	if hpht != "" {
		if !tanggalValid(hpht) {
			errs["hpht"] = []string{"Tanggal HPHT tidak valid"}
		} else if lahir != "" && hpht > lahir {
			errs["hpht"] = []string{"Tanggal HPHT tidak boleh setelah tanggal lahir"}
		} else if lahir != "" {
			tLahir, _ := time.Parse("2006-01-02", lahir)
			tHpht, _ := time.Parse("2006-01-02", hpht)
			if int(tLahir.Sub(tHpht).Hours()/24) > 320 {
				errs["hpht"] = []string{"Tanggal HPHT terlalu jauh dari tanggal lahir"}
			}
		}
	}
	if hplVal != "" {
		if !tanggalValid(hplVal) {
			errs["hpl"] = []string{"Tanggal HPL tidak valid"}
		} else if hpht != "" && hplVal < hpht {
			errs["hpl"] = []string{"Tanggal HPL tidak boleh sebelum HPHT"}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validasiLahir(berat, panjang, kepala *float64) map[string]any {
	errs := map[string]any{}
	if berat != nil && (*berat <= 0 || *berat > 15) {
		errs["birth_weight_kg"] = []string{"Berat lahir harus antara 0 dan 15 kg"}
	}
	if panjang != nil && (*panjang <= 0 || *panjang > 70) {
		errs["birth_length_cm"] = []string{"Panjang lahir harus antara 0 dan 70 cm"}
	}
	if kepala != nil && (*kepala <= 0 || *kepala > 60) {
		errs["birth_head_circumference_cm"] = []string{"Lingkar kepala lahir harus antara 0 dan 60 cm"}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func nilaiString(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func tanggalArg(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func teksOpsional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

// tanggalSaja mengambil bagian tanggal (YYYY-MM-DD) dari nilai ISO apa pun.
func tanggalSaja(s string) string {
	t := strings.TrimSpace(s)
	if len(t) >= 10 {
		t = t[:10]
	}
	return t
}
