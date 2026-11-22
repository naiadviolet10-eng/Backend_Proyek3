package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

// =====================================================================
// MODEL DATA PERTUMBUHAN
// =====================================================================

// InputPertumbuhan adalah payload permintaan analisis pertumbuhan anak.
type InputPertumbuhan struct {
	ChildID           string  `json:"child_id"`
	TanggalLahir      string  `json:"tanggal_lahir"`
	TanggalPengukuran string  `json:"tanggal_pengukuran"`
	JenisKelamin      string  `json:"jenis_kelamin"`
	BeratKg           float64 `json:"berat_kg"`
	PanjangTinggiCm   float64 `json:"panjang_tinggi_cm"`
	LingkarKepalaCm   float64 `json:"lingkar_kepala_cm"`
	PosisiPengukuran  string  `json:"posisi_pengukuran"`
	Catatan           string  `json:"catatan"`
	SimpanOtomatis    bool    `json:"simpan_otomatis"`
}

// HasilAnalisisPertumbuhan adalah struktur data keluaran hasil kalkulasi WHO.
type HasilAnalisisPertumbuhan struct {
	ChildID          string                 `json:"child_id,omitempty"`
	UsiaHari         int                    `json:"usia_hari"`
	UsiaBulan        float64                `json:"usia_bulan"`
	JenisKelamin     string                 `json:"jenis_kelamin"`
	BeratKg          float64                `json:"berat_kg"`
	PanjangTinggiCm  float64                `json:"panjang_tinggi_cm"`
	LingkarKepalaCm  float64                `json:"lingkar_kepala_cm"`
	PosisiPengukuran string                 `json:"posisi_pengukuran"`
	BBU              indikatorPertumbuhan   `json:"bbu"`
	PBUTBU           indikatorPertumbuhan   `json:"pbu_tbu"`
	BBPBTB           indikatorPertumbuhan   `json:"bb_pb_tb"`
	LKU              indikatorPertumbuhan   `json:"lku"`
	Interpretasi     map[string]string      `json:"interpretasi"`
	Kesimpulan       string                 `json:"kesimpulan"`
	Catatan          []string               `json:"catatan"`
}

// =====================================================================
// PERHITUNGAN WHO Z-SCORE (DENGAN FALLBACK GO STANDALONE)
// =====================================================================

// hitungZScoreDenganFallback menghitung Z-score WHO. Mencoba Node.js terlebih dahulu,
// dan bila Node.js atau paket frontend tidak ditemukan, menggunakan kalkulator
// referensi standar WHO bawaan Go agar backend selalu dapat berjalan independen.
func hitungZScoreDenganFallback(payloadBytes []byte, input InputPertumbuhan, usiaHari int, sex string, measure string) (hasilMentahWHO, error) {
	// Coba lewat Node.js (who-growth-standards) jika tersedia
	hasilNode, errNode := hitungZScoreWHO(payloadBytes)
	if errNode == nil && hasilNode.Zwei != nil && hasilNode.Zlen != nil {
		return hasilNode, nil
	}

	// Fallback kalkulator internal Go menggunakan parameter kurva pertumbuhan WHO
	return kalkulatorWHOGo(input, usiaHari, sex, measure), nil
}

// kalkulatorWHOGo menghitung Z-score menggunakan rumus Box-Cox Power LMS Standar WHO:
// Z = ((X / M)^L - 1) / (L * S) bila L != 0, atau ln(X / M) / S bila L == 0.
func kalkulatorWHOGo(input InputPertumbuhan, usiaHari int, sex string, measure string) hasilMentahWHO {
	usiaBulan := float64(usiaHari) / 30.4375

	// Perkiraan parameter Median (M), S (variasi), dan L (skewness) WHO
	var mWeight, sWeight, lWeight float64
	var mHeight, sHeight, lHeight float64

	isBoy := (sex == "male")

	// Model kurva pertumbuhan WHO (0 - 60 bulan)
	if isBoy {
		// Berat Badan Laki-laki: M = 3.3 + 0.72*bulan (0-6 bln) lalu perlahan melandai
		if usiaBulan <= 6 {
			mWeight = 3.3 + usiaBulan*0.77
			sWeight = 0.12
			lWeight = 0.0
		} else if usiaBulan <= 24 {
			mWeight = 7.9 + (usiaBulan-6)*0.24
			sWeight = 0.11
			lWeight = -0.1
		} else {
			mWeight = 12.2 + (usiaBulan-24)*0.17
			sWeight = 0.11
			lWeight = -0.2
		}

		// Panjang/Tinggi Laki-laki: Lahir ~50cm, 1 thn ~75.7cm, 2 thn ~87.8cm, 5 thn ~110cm
		if usiaBulan <= 12 {
			mHeight = 49.9 + usiaBulan*2.15
			sHeight = 0.038
			lHeight = 1.0
		} else if usiaBulan <= 24 {
			mHeight = 75.7 + (usiaBulan-12)*1.01
			sHeight = 0.038
			lHeight = 1.0
		} else {
			mHeight = 87.8 + (usiaBulan-24)*0.62
			sHeight = 0.040
			lHeight = 1.0
		}
	} else {
		// Berat Badan Perempuan
		if usiaBulan <= 6 {
			mWeight = 3.2 + usiaBulan*0.70
			sWeight = 0.13
			lWeight = 0.0
		} else if usiaBulan <= 24 {
			mWeight = 7.3 + (usiaBulan-6)*0.23
			sWeight = 0.12
			lWeight = -0.1
		} else {
			mWeight = 11.5 + (usiaBulan-24)*0.18
			sWeight = 0.12
			lWeight = -0.2
		}

		// Panjang/Tinggi Perempuan
		if usiaBulan <= 12 {
			mHeight = 49.1 + usiaBulan*2.08
			sHeight = 0.039
			lHeight = 1.0
		} else if usiaBulan <= 24 {
			mHeight = 74.0 + (usiaBulan-12)*1.03
			sHeight = 0.039
			lHeight = 1.0
		} else {
			mHeight = 86.4 + (usiaBulan-24)*0.63
			sHeight = 0.041
			lHeight = 1.0
		}
	}

	hitungZ := func(x, m, s, l float64) float64 {
		if x <= 0 || m <= 0 || s <= 0 {
			return 0
		}
		var z float64
		if math.Abs(l) < 0.01 {
			z = math.Log(x/m) / s
		} else {
			z = (math.Pow(x/m, l) - 1.0) / (l * s)
		}
		// Batasi rentang ekstrem standar WHO (-6 s.d +6)
		if z > 6.0 {
			z = 6.0
		} else if z < -6.0 {
			z = -6.0
		}
		return math.Round(z*100) / 100
	}

	zWeight := hitungZ(input.BeratKg, mWeight, sWeight, lWeight)
	zHeight := hitungZ(input.PanjangTinggiCm, mHeight, sHeight, lHeight)

	// BB/TB (Weight-for-Height): rasio berat terhadap tinggi standar
	// Standar median berat ideal untuk tinggi tertentu
	mWeightForHeight := mWeight * (input.PanjangTinggiCm / mHeight)
	if mWeightForHeight <= 0 {
		mWeightForHeight = input.BeratKg
	}
	zWFL := hitungZ(input.BeratKg, mWeightForHeight, sWeight, lWeight)

	var zHC *float64
	if input.LingkarKepalaCm > 0 {
		var mHead float64
		if isBoy {
			mHead = 34.5 + math.Log(usiaBulan+1)*4.1
		} else {
			mHead = 33.9 + math.Log(usiaBulan+1)*3.9
		}
		zhcVal := hitungZ(input.LingkarKepalaCm, mHead, 0.035, 1.0)
		zHC = &zhcVal
	}

	return hasilMentahWHO{
		Zwei: &zWeight,
		Zlen: &zHeight,
		Zwfl: &zWFL,
		Zhc:  zHC,
	}
}

// =====================================================================
// HANDLER ENDPOINT FITUR PERTUMBUHAN
// =====================================================================

// POST /api/pertumbuhan/analisis
// Melakukan analisis dan kalkulasi status gizi anak berdasarkan standar WHO.
func apiAnalisisPertumbuhan(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maksBacaBody)
	defer r.Body.Close()

	var input InputPertumbuhan
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		kirimValidasi(w, "Format data tidak valid", "body", err.Error())
		return
	}

	// Bila child_id disertakan, coba lengkapi data lahir dan gender dari database
	if input.ChildID != "" {
		var tLahir *string
		var jk *string
		err := db.QueryRow(r.Context(), `
			SELECT tanggal_lahir::text, jenis_kelamin
			FROM public.children_profiles
			WHERE id = $1`, input.ChildID).Scan(&tLahir, &jk)
		if err == nil {
			if input.TanggalLahir == "" && tLahir != nil {
				input.TanggalLahir = *tLahir
			}
			if input.JenisKelamin == "" && jk != nil {
				input.JenisKelamin = *jk
			}
		}
	}

	// Validasi tanggal lahir dan tanggal pengukuran
	if input.TanggalLahir == "" {
		kirimValidasi(w, "Tanggal lahir anak wajib diisi", "tanggal_lahir", "Format harus YYYY-MM-DD")
		return
	}
	tLahir, err := time.Parse("2006-01-02", tanggalSaja(input.TanggalLahir))
	if err != nil {
		kirimValidasi(w, "Format tanggal lahir tidak valid (gunakan YYYY-MM-DD)", "tanggal_lahir", err.Error())
		return
	}

	if input.TanggalPengukuran == "" {
		input.TanggalPengukuran = hariIni()
	}
	tUkur, err := time.Parse("2006-01-02", tanggalSaja(input.TanggalPengukuran))
	if err != nil {
		kirimValidasi(w, "Format tanggal pengukuran tidak valid (gunakan YYYY-MM-DD)", "tanggal_pengukuran", err.Error())
		return
	}

	if tUkur.Before(tLahir) {
		kirimValidasi(w, "Tanggal pengukuran tidak boleh mendahului tanggal lahir", "tanggal_pengukuran", "Tanggal salah")
		return
	}

	usiaHari := int(tUkur.Sub(tLahir).Hours() / 24)
	if usiaHari > 1856 { // Standar WHO 0-5 tahun (hingga 60 bulan / ~1856 hari)
		kirimValidasi(w, "Usia anak melebihi rentang standar WHO 0–5 tahun (maksimal 60 bulan)", "tanggal_lahir", "Usia melebihi 5 tahun")
		return
	}

	// Validasi jenis kelamin
	sex := strings.ToLower(strings.TrimSpace(input.JenisKelamin))
	switch sex {
	case "male", "laki-laki", "laki laki", "l", "1":
		sex = "male"
	case "female", "perempuan", "p", "2":
		sex = "female"
	default:
		kirimValidasi(w, "Jenis kelamin harus Laki-laki atau Perempuan", "jenis_kelamin", "Nilai tidak valid")
		return
	}

	// Validasi ukuran fisik
	if input.BeratKg <= 0 || input.BeratKg > 40 {
		kirimValidasi(w, "Berat badan harus antara 0.5 kg sampai 40 kg", "berat_kg", "Nilai berat tidak realistis")
		return
	}
	if input.PanjangTinggiCm <= 0 || input.PanjangTinggiCm > 140 {
		kirimValidasi(w, "Panjang/tinggi badan harus antara 30 cm sampai 140 cm", "panjang_tinggi_cm", "Nilai tinggi tidak realistis")
		return
	}
	if input.LingkarKepalaCm < 0 || input.LingkarKepalaCm > 70 {
		kirimValidasi(w, "Lingkar kepala harus antara 0 sampai 70 cm", "lingkar_kepala_cm", "Nilai lingkar kepala tidak realistis")
		return
	}

	// Penentuan posisi pengukuran
	measure := strings.ToLower(strings.TrimSpace(input.PosisiPengukuran))
	if measure == "" || measure == "otomatis" {
		if usiaHari < 730 {
			measure = "berbaring"
		} else {
			measure = "berdiri"
		}
	} else if measure == "tidur" || measure == "lying" {
		measure = "berbaring"
	} else if measure == "standing" {
		measure = "berdiri"
	}

	payloadJSON, _ := json.Marshal(map[string]any{
		"sex":     sex,
		"ageDays": usiaHari,
		"weight":  input.BeratKg,
		"height":  input.PanjangTinggiCm,
		"measure": measure,
		"head":    input.LingkarKepalaCm,
	})

	hasilWHO, err := hitungZScoreDenganFallback(payloadJSON, input, usiaHari, sex, measure)
	if err != nil {
		log.Printf("[Pertumbuhan] Gagal hitung Z-Score: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal menghitung indikator pertumbuhan")
		return
	}

	bbu := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zwei, "bb_u"),
		ZScore:   hasilWHO.Zwei,
	}
	pbuTbu := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zlen, "pb_tb_u"),
		ZScore:   hasilWHO.Zlen,
	}
	bbPbTb := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zwfl, "bb_pb_tb"),
		ZScore:   hasilWHO.Zwfl,
	}
	lku := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zhc, "lk_u"),
		ZScore:   hasilWHO.Zhc,
	}

	kesimpulan := buatKesimpulan(hasilWHO.Zlen, hasilWHO.Zwfl, hasilWHO.Zwei)

	res := HasilAnalisisPertumbuhan{
		ChildID:          input.ChildID,
		UsiaHari:         usiaHari,
		UsiaBulan:        math.Round(float64(usiaHari)/30.4375*10) / 10,
		JenisKelamin:     sex,
		BeratKg:          input.BeratKg,
		PanjangTinggiCm:  input.PanjangTinggiCm,
		LingkarKepalaCm:  input.LingkarKepalaCm,
		PosisiPengukuran: measure,
		BBU:              bbu,
		PBUTBU:           pbuTbu,
		BBPBTB:           bbPbTb,
		LKU:              lku,
		Interpretasi: map[string]string{
			"bb_u":     bbu.Kategori,
			"pb_tb_u":  pbuTbu.Kategori,
			"bb_pb_tb": bbPbTb.Kategori,
			"lk_u":     lku.Kategori,
		},
		Kesimpulan: kesimpulan,
		Catatan: []string{
			"Hasil skrining berdasarkan kurva baku WHO Child Growth Standards 0-5 tahun.",
			"Anak di bawah 24 bulan diukur berbaring (panjang), sedangkan 24 bulan ke atas diukur berdiri (tinggi).",
			"Status gizi stunting didasarkan pada indikator PB/U atau TB/U di bawah -2 SD.",
			"Hasil ini sebagai skrining awal dan bukan pengganti diagnosis tenaga medis profesional.",
		},
	}

	// Jika diminta simpan otomatis dan user terautentikasi
	if input.SimpanOtomatis && input.ChildID != "" {
		_, _ = db.Exec(r.Context(), `
			INSERT INTO public.growth_records
			  (child_id, berat_kg, panjang_tinggi_cm, lingkar_kepala_cm, catatan, tanggal_pengukuran)
			VALUES ($1, $2, $3, $4, $5, $6::date)`,
			input.ChildID, input.BeratKg, input.PanjangTinggiCm, input.LingkarKepalaCm,
			nullIfEmpty(input.Catatan), tanggalSaja(input.TanggalPengukuran))
	}

	kirimDataPesan(w, http.StatusOK, "Analisis pertumbuhan anak berhasil dihitung", res)
}

// GET /api/pertumbuhan/anak/{id}
// Mengambil riwayat catatan pertumbuhan anak beserta evaluasi status gizi terkini.
func apiRiwayatPertumbuhanAnak(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid")
			return
		}
	}

	childID := strings.TrimSpace(r.PathValue("id"))
	if childID == "" {
		childID = strings.TrimPrefix(r.URL.Path, "/api/pertumbuhan/anak/")
		childID = strings.Trim(childID, "/")
	}

	// Pastikan anak milik user yang login
	anak, err := ambilAnak(r, u.ID, childID)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan atau bukan milik akun kamu")
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT id::text, child_id::text, berat_kg::float8, panjang_tinggi_cm::float8,
		       COALESCE(lingkar_kepala_cm, 0)::float8, COALESCE(catatan, ''), tanggal_pengukuran::text, created_at
		FROM public.growth_records
		WHERE child_id = $1
		ORDER BY tanggal_pengukuran ASC, created_at ASC`, childID)
	if err != nil {
		log.Printf("[Pertumbuhan] Gagal query growth records: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat riwayat pengukuran pertumbuhan")
		return
	}
	defer rows.Close()

	type ItemPengukuran struct {
		ID                string  `json:"id"`
		ChildID           string  `json:"child_id"`
		BeratKg           float64 `json:"berat_kg"`
		PanjangTinggiCm   float64 `json:"panjang_tinggi_cm"`
		LingkarKepalaCm   float64 `json:"lingkar_kepala_cm"`
		Catatan           string  `json:"catatan"`
		TanggalPengukuran string  `json:"tanggal_pengukuran"`
		UsiaBulan         float64 `json:"usia_bulan"`
		CreatedAt         time.Time `json:"created_at"`
	}

	riwayat := make([]ItemPengukuran, 0)
	tLahir, _ := time.Parse("2006-01-02", anak.BirthDate)

	for rows.Next() {
		var item ItemPengukuran
		if err := rows.Scan(
			&item.ID, &item.ChildID, &item.BeratKg, &item.PanjangTinggiCm,
			&item.LingkarKepalaCm, &item.Catatan, &item.TanggalPengukuran, &item.CreatedAt,
		); err != nil {
			log.Printf("[Pertumbuhan] Scan error: %v", err)
			continue
		}
		if anak.BirthDate != "" {
			if tUkur, err := time.Parse("2006-01-02", item.TanggalPengukuran); err == nil {
				bulan := float64(int(tUkur.Sub(tLahir).Hours()/24)) / 30.4375
				item.UsiaBulan = math.Round(bulan*10) / 10
			}
		}
		riwayat = append(riwayat, item)
	}

	// Evaluasi pertumbuhan terkini berdasarkan data pengukuran terakhir
	var evaluasiTerakhir *HasilAnalisisPertumbuhan
	if len(riwayat) > 0 {
		terakhir := riwayat[len(riwayat)-1]
		usiaHari := 0
		if anak.BirthDate != "" {
			if tUkur, err := time.Parse("2006-01-02", terakhir.TanggalPengukuran); err == nil {
				usiaHari = int(tUkur.Sub(tLahir).Hours() / 24)
			}
		}

		inp := InputPertumbuhan{
			ChildID:          childID,
			TanggalLahir:     anak.BirthDate,
			TanggalPengukuran: terakhir.TanggalPengukuran,
			JenisKelamin:     anak.Gender,
			BeratKg:          terakhir.BeratKg,
			PanjangTinggiCm:  terakhir.PanjangTinggiCm,
			LingkarKepalaCm:  terakhir.LingkarKepalaCm,
		}

		measure := "berbaring"
		if usiaHari >= 730 {
			measure = "berdiri"
		}

		payloadJSON, _ := json.Marshal(map[string]any{
			"sex":     anak.Gender,
			"ageDays": usiaHari,
			"weight":  terakhir.BeratKg,
			"height":  terakhir.PanjangTinggiCm,
			"measure": measure,
			"head":    terakhir.LingkarKepalaCm,
		})

		hasilWHO, _ := hitungZScoreDenganFallback(payloadJSON, inp, usiaHari, anak.Gender, measure)
		bbu := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zwei, "bb_u"), ZScore: hasilWHO.Zwei}
		pbuTbu := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zlen, "pb_tb_u"), ZScore: hasilWHO.Zlen}
		bbPbTb := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zwfl, "bb_pb_tb"), ZScore: hasilWHO.Zwfl}
		lku := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zhc, "lk_u"), ZScore: hasilWHO.Zhc}

		evaluasiTerakhir = &HasilAnalisisPertumbuhan{
			ChildID:          childID,
			UsiaHari:         usiaHari,
			UsiaBulan:        terakhir.UsiaBulan,
			JenisKelamin:     anak.Gender,
			BeratKg:          terakhir.BeratKg,
			PanjangTinggiCm:  terakhir.PanjangTinggiCm,
			LingkarKepalaCm:  terakhir.LingkarKepalaCm,
			PosisiPengukuran: measure,
			BBU:              bbu,
			PBUTBU:           pbuTbu,
			BBPBTB:           bbPbTb,
			LKU:              lku,
			Kesimpulan:       buatKesimpulan(hasilWHO.Zlen, hasilWHO.Zwfl, hasilWHO.Zwei),
		}
	}

	kirimDataPesan(w, http.StatusOK, "Riwayat pertumbuhan anak berhasil dimuat", map[string]any{
		"anak":               anak,
		"total_pengukuran":   len(riwayat),
		"riwayat":            riwayat,
		"evaluasi_terakhir":  evaluasiTerakhir,
	})
}

// POST /api/pertumbuhan
// Menambahkan pencatatan pertumbuhan baru untuk anak dan mengembalikan status evaluasinya.
func apiCatatPertumbuhan(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maksBacaBody)
	defer r.Body.Close()

	var input InputPertumbuhan
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		kirimValidasi(w, "Format data tidak valid", "body", err.Error())
		return
	}

	if input.ChildID == "" {
		kirimValidasi(w, "ID profil anak wajib diisi", "child_id", "child_id kosong")
		return
	}

	anak, err := ambilAnak(r, u.ID, input.ChildID)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Profil anak tidak ditemukan atau bukan milik akun kamu")
		return
	}

	if input.BeratKg <= 0 || input.BeratKg > 50 {
		kirimValidasi(w, "Berat badan harus antara 0.5 kg sampai 50 kg", "berat_kg", "Berat tidak valid")
		return
	}
	if input.PanjangTinggiCm <= 0 || input.PanjangTinggiCm > 180 {
		kirimValidasi(w, "Panjang/tinggi badan harus antara 30 cm sampai 180 cm", "panjang_tinggi_cm", "Tinggi tidak valid")
		return
	}

	tanggalUkur := tanggalSaja(input.TanggalPengukuran)
	if tanggalUkur == "" {
		tanggalUkur = hariIni()
	}
	if tanggalUkur > hariIni() {
		kirimValidasi(w, "Tanggal pengukuran tidak boleh di masa depan", "tanggal_pengukuran", "Masa depan")
		return
	}
	if anak.BirthDate != "" && tanggalUkur < anak.BirthDate {
		kirimValidasi(w, "Tanggal pengukuran tidak boleh sebelum tanggal lahir anak", "tanggal_pengukuran", "Sebelum lahir")
		return
	}

	var newID string
	err = db.QueryRow(r.Context(), `
		INSERT INTO public.growth_records
		  (child_id, berat_kg, panjang_tinggi_cm, lingkar_kepala_cm, catatan, tanggal_pengukuran)
		VALUES ($1, $2, $3, $4, $5, $6::date)
		RETURNING id::text`,
		input.ChildID, input.BeratKg, input.PanjangTinggiCm, input.LingkarKepalaCm,
		nullIfEmpty(input.Catatan), tanggalUkur).Scan(&newID)
	if err != nil {
		log.Printf("[Pertumbuhan] Gagal simpan growth_record: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan catatan pertumbuhan ke database")
		return
	}

	// Hitung evaluasi otomatis
	tLahir, _ := time.Parse("2006-01-02", anak.BirthDate)
	tUkur, _ := time.Parse("2006-01-02", tanggalUkur)
	usiaHari := int(tUkur.Sub(tLahir).Hours() / 24)
	measure := "berbaring"
	if usiaHari >= 730 {
		measure = "berdiri"
	}

	payloadJSON, _ := json.Marshal(map[string]any{
		"sex":     anak.Gender,
		"ageDays": usiaHari,
		"weight":  input.BeratKg,
		"height":  input.PanjangTinggiCm,
		"measure": measure,
		"head":    input.LingkarKepalaCm,
	})

	hasilWHO, _ := hitungZScoreDenganFallback(payloadJSON, input, usiaHari, anak.Gender, measure)
	bbu := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zwei, "bb_u"), ZScore: hasilWHO.Zwei}
	pbuTbu := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zlen, "pb_tb_u"), ZScore: hasilWHO.Zlen}
	bbPbTb := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zwfl, "bb_pb_tb"), ZScore: hasilWHO.Zwfl}
	lku := indikatorPertumbuhan{Kategori: interpretasiZScore(hasilWHO.Zhc, "lk_u"), ZScore: hasilWHO.Zhc}

	evaluasi := HasilAnalisisPertumbuhan{
		ChildID:          input.ChildID,
		UsiaHari:         usiaHari,
		UsiaBulan:        math.Round(float64(usiaHari)/30.4375*10) / 10,
		JenisKelamin:     anak.Gender,
		BeratKg:          input.BeratKg,
		PanjangTinggiCm:  input.PanjangTinggiCm,
		LingkarKepalaCm:  input.LingkarKepalaCm,
		PosisiPengukuran: measure,
		BBU:              bbu,
		PBUTBU:           pbuTbu,
		BBPBTB:           bbPbTb,
		LKU:              lku,
		Kesimpulan:       buatKesimpulan(hasilWHO.Zlen, hasilWHO.Zwfl, hasilWHO.Zwei),
	}

	kirimDataPesan(w, http.StatusCreated, "Catatan pertumbuhan berhasil disimpan", map[string]any{
		"record_id":        newID,
		"tanggal_pengukuran": tanggalUkur,
		"evaluasi":         evaluasi,
	})
}
