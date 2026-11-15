package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// bahanAI adalah bentuk takaran bahan resep AI yang disimpan di resep_tersimpan.
type bahanAI struct {
	IngredientName string
	Quantity       float64
	Unit           string
}

type resepTersimpan struct {
	ID        int64
	Nama      string
	Deskripsi string
	Waktu     int
	Porsi     int
	Langkah   []string
	Bahan     []bahanAI
}

// muatPetaBahan memuat seluruh ingredients_master untuk pencocokan nama dan
// pengambilan gizi, dikunci per id dan per nama huruf kecil.
func muatPetaBahan(ctx context.Context, r *http.Request) (map[int64]bahanJSON, map[string]bahanJSON, error) {
	rows, err := db.Query(ctx, `SELECT `+kolomBahanAPI+`
		FROM ingredients_master i
		LEFT JOIN ingredient_categories c ON c.id = i.category_id`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	perID := map[int64]bahanJSON{}
	perNama := map[string]bahanJSON{}
	for rows.Next() {
		b, err := pindaiBahanAPI(r, rows)
		if err != nil {
			return nil, nil, err
		}
		perID[b.ID] = b
		perNama[strings.ToLower(b.Name)] = b
	}
	return perID, perNama, nil
}

func stokIDSet(ctx context.Context, userID string) (map[int64]bool, error) {
	rows, err := db.Query(ctx, `SELECT DISTINCT bahan_pangan_id FROM stok WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		set[id] = true
	}
	return set, nil
}

// bacaResepTersimpan mengambil satu resep AI milik user dan menguraikan
// kolom kolomnya, dengan cadangan ke data_resep untuk baris lama.
func bacaResepTersimpan(ctx context.Context, userID string, id int64) (resepTersimpan, error) {
	var rt resepTersimpan
	var deskripsi *string
	var waktu, porsi *int
	var bahanRaw, langkahRaw, dataResep string

	err := db.QueryRow(ctx, `
		SELECT id, nama_resep, deskripsi, waktu_memasak, porsi,
		       COALESCE(bahan::text,'[]'), COALESCE(langkah::text,'[]'), data_resep::text
		FROM resep_tersimpan WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&rt.ID, &rt.Nama, &deskripsi, &waktu, &porsi, &bahanRaw, &langkahRaw, &dataResep)
	if err != nil {
		return rt, err
	}
	if deskripsi != nil {
		rt.Deskripsi = *deskripsi
	}
	rt.Waktu = 15
	if waktu != nil {
		rt.Waktu = *waktu
	}
	rt.Porsi = 1
	if porsi != nil {
		rt.Porsi = *porsi
	}

	rt.Bahan = uraiBahan(bahanRaw)
	rt.Langkah = uraiLangkah(langkahRaw)

	// Baris lama (mis. hasil seed) menyimpan seluruhnya di data_resep.
	if len(rt.Bahan) == 0 || len(rt.Langkah) == 0 {
		var dr map[string]any
		if json.Unmarshal([]byte(dataResep), &dr) == nil {
			if len(rt.Bahan) == 0 {
				rt.Bahan = uraiBahanDariAny(dr["bahan"])
			}
			if len(rt.Langkah) == 0 {
				rt.Langkah = uraiLangkahDariAny(dr["langkah"])
			}
			if rt.Deskripsi == "" {
				rt.Deskripsi = teksNilai(dr["deskripsi"])
			}
			if rt.Nama == "" {
				rt.Nama = teksNilai(dr["nama_resep"])
			}
		}
	}
	return rt, nil
}

func uraiBahan(raw string) []bahanAI {
	var items []any
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	return uraiBahanDariAny(items)
}

// uraiBahanDariAny menerima dua bentuk item: kanonik baru
// (ingredient_name/quantity/unit) dan bentuk lama (nama/jumlah/dari_stok).
func uraiBahanDariAny(v any) []bahanAI {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var hasil []bahanAI
	for _, it := range arr {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		nama := teksNilai(m["ingredient_name"])
		if nama == "" {
			nama = teksNilai(m["nama"])
		}
		if strings.TrimSpace(nama) == "" {
			continue
		}
		q := angka(m["quantity"])
		unit := teksNilai(m["unit"])
		if j := teksNilai(m["jumlah"]); j != "" {
			if q == 0 {
				q = parseAngka(j)
			}
			if unit == "" {
				unit = sisaString(j)
			}
		}
		hasil = append(hasil, bahanAI{IngredientName: strings.TrimSpace(nama), Quantity: q, Unit: unit})
	}
	return hasil
}

func uraiLangkah(raw string) []string {
	var items []any
	if json.Unmarshal([]byte(raw), &items) != nil {
		return nil
	}
	return uraiLangkahDariAny(items)
}

func uraiLangkahDariAny(v any) []string {
	switch t := v.(type) {
	case []any:
		var hasil []string
		for _, s := range t {
			if teks := strings.TrimSpace(teksNilai(s)); teks != "" {
				hasil = append(hasil, teks)
			}
		}
		return hasil
	case string:
		var hasil []string
		for _, s := range strings.Split(t, "\n") {
			if teks := strings.TrimSpace(s); teks != "" {
				hasil = append(hasil, teks)
			}
		}
		return hasil
	default:
		return nil
	}
}

func teksNilai(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// resepDariTersimpan mengubah resep AI menjadi bentuk yang dipakai aplikasi.
func resepDariTersimpan(rt resepTersimpan, perNama map[string]bahanJSON, stokSet map[int64]bool) resepJSON {
	hasil := resepJSON{
		ID:              idTersimpanKePublik(rt.ID),
		Name:            rt.Nama,
		Description:     rt.Deskripsi,
		Instructions:    strings.Join(rt.Langkah, "\n"),
		CookTimeMinutes: rt.Waktu,
		Servings:        rt.Porsi,
		Source:          "ai_generated",
		Ingredients:     []resepBahanJSON{},
	}
	if hasil.Name == "" {
		hasil.Name = "Resep Tanpa Nama"
	}
	for _, b := range rt.Bahan {
		master, ok := perNama[strings.ToLower(strings.TrimSpace(b.IngredientName))]
		if !ok {
			// Bahan pelengkap yang tidak ada di master tidak ditampilkan,
			// sama seperti backend lama.
			continue
		}
		unit := b.Unit
		if strings.TrimSpace(unit) == "" {
			unit = master.DefaultUnit
		}
		hasil.Ingredients = append(hasil.Ingredients, resepBahanJSON{
			IngredientID:   master.ID,
			IngredientName: master.Name,
			QuantityNeeded: b.Quantity,
			Unit:           unit,
			ImageURL:       master.ImageURL,
			InStock:        stokSet[master.ID],
		})
	}
	return hasil
}

// resepPustaka mengambil resep dari tabel pustaka beserta bahannya.
func daftarResepPustaka(ctx context.Context, r *http.Request, perID map[int64]bahanJSON, stokSet map[int64]bool) ([]resepJSON, error) {
	rows, err := db.Query(ctx, `SELECT id, nama, COALESCE(deskripsi,''), porsi, COALESCE(langkah,'') FROM resep ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type barisResep struct {
		id              int64
		nama, deskripsi string
		porsi           int
		langkah         string
	}
	var daftar []barisResep
	for rows.Next() {
		var b barisResep
		if err := rows.Scan(&b.id, &b.nama, &b.deskripsi, &b.porsi, &b.langkah); err != nil {
			return nil, err
		}
		daftar = append(daftar, b)
	}
	rows.Close()

	hasil := []resepJSON{}
	for _, b := range daftar {
		rj := resepJSON{
			ID: b.id, Name: b.nama, Description: b.deskripsi, Instructions: b.langkah,
			Servings: b.porsi, Source: "manual", Ingredients: []resepBahanJSON{},
		}
		bahanRows, err := db.Query(ctx, `
			SELECT rb.bahan_id, rb.jumlah::float8, COALESCE(i.default_unit,'gram')
			FROM resep_bahan rb
			LEFT JOIN ingredients_master i ON i.id = rb.bahan_id
			WHERE rb.resep_id = $1 ORDER BY rb.id`, b.id)
		if err != nil {
			return nil, err
		}
		for bahanRows.Next() {
			var bid int64
			var jumlah float64
			var unit string
			if err := bahanRows.Scan(&bid, &jumlah, &unit); err != nil {
				bahanRows.Close()
				return nil, err
			}
			master := perID[bid]
			rj.Ingredients = append(rj.Ingredients, resepBahanJSON{
				IngredientID:   bid,
				IngredientName: master.Name,
				QuantityNeeded: jumlah,
				Unit:           unit,
				ImageURL:       master.ImageURL,
				InStock:        stokSet[bid],
			})
		}
		bahanRows.Close()
		hasil = append(hasil, rj)
	}
	return hasil, nil
}

// GET /api/recipes
func apiDaftarResep(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	perID, perNama, err := muatPetaBahan(r.Context(), r)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}
	stokSet, err := stokIDSet(r.Context(), u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}

	hasil, err := daftarResepPustaka(r.Context(), r, perID, stokSet)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}

	rows, err := db.Query(r.Context(), `SELECT id FROM resep_tersimpan WHERE user_id = $1 ORDER BY id DESC`, u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}
	var idTersimpan []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
			return
		}
		idTersimpan = append(idTersimpan, id)
	}
	rows.Close()

	for _, id := range idTersimpan {
		rt, err := bacaResepTersimpan(r.Context(), u.ID, id)
		if err != nil {
			continue
		}
		hasil = append(hasil, resepDariTersimpan(rt, perNama, stokSet))
	}

	kirimData(w, hasil)
}

// GET /api/recipes/{id}
func apiDetailResep(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Resep tidak ditemukan")
		return
	}
	perID, perNama, err := muatPetaBahan(r.Context(), r)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}
	stokSet, err := stokIDSet(r.Context(), u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}

	if idAI, ok := publikKeTersimpan(id); ok {
		rt, err := bacaResepTersimpan(r.Context(), u.ID, idAI)
		if err != nil {
			kirimPesan(w, http.StatusNotFound, "Resep tidak ditemukan")
			return
		}
		kirimData(w, resepDariTersimpan(rt, perNama, stokSet))
		return
	}

	pustaka, err := daftarResepPustaka(r.Context(), r, perID, stokSet)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep")
		return
	}
	for _, rj := range pustaka {
		if rj.ID == id {
			kirimData(w, rj)
			return
		}
	}
	kirimPesan(w, http.StatusNotFound, "Resep tidak ditemukan")
}

const promptResep = `Kamu adalah koki rumah tangga untuk ibu Indonesia yang ingin menyajikan makanan bergizi murah untuk balita. Bahan yang tersedia: {{BAHAN}}.

Buatlah SATU resep masakan untuk 2 porsi yang memanfaatkan bahan-bahan tersebut. Boleh menambahkan bahan pelengkap umum seperti bumbu dapur, garam, minyak, bawang, dsb.

Kembalikan HANYA JSON tanpa penjelasan lain, dengan format persis:
{
  "name": "Nama masakan",
  "description": "1-2 kalimat tentang masakan ini",
  "servings": 2,
  "instructions": ["langkah 1", "langkah 2", "..."],
  "cook_time_minutes": 30,
  "ingredients": [
    {"ingredient_name": "nama bahan", "quantity": 1.5, "unit": "gram"},
    {"ingredient_name": "nama bahan", "quantity": 2, "unit": "siung"}
  ]
}

Aturan takaran:
- Takaran ditulis untuk 2 porsi.
- Maksimal 8 langkah, setiap langkah satu kalimat singkat.
- Unit hanya salah satu dari: gram, kg, ml, liter, sdm, sdt, siung, butir, buah, gelas, SENDOK, SEDEKAP.
- ingredient_name memakai nama bahan yang tersedia di atas, atau nama bumbu umum yang singkat (contoh: Bawang Putih, Garam, Minyak Goreng).`

// POST /api/recipes/generate
func apiGenerateResep(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	var isian struct {
		IngredientIDs []int64 `json:"ingredient_ids"`
	}
	if err := bacaJSON(r, &isian); err != nil || len(isian.IngredientIDs) == 0 {
		kirimValidasi(w, "Pilih minimal satu bahan", "ingredient_ids", "Daftar bahan tidak boleh kosong")
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT i.name FROM stok s
		JOIN ingredients_master i ON i.id = s.bahan_pangan_id
		WHERE s.user_id = $1 AND s.bahan_pangan_id = ANY($2)`, u.ID, isian.IngredientIDs)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal membaca stok")
		return
	}
	var namaBahan []string
	for rows.Next() {
		var nama string
		if err := rows.Scan(&nama); err == nil {
			namaBahan = append(namaBahan, nama)
		}
	}
	rows.Close()

	if len(namaBahan) == 0 {
		kirimPesan(w, http.StatusUnprocessableEntity,
			"Tidak ada bahan yang cocok dengan stok kamu. Pilih bahan dari daftar stok.")
		return
	}

	prompt := strings.ReplaceAll(promptResep, "{{BAHAN}}", strings.Join(namaBahan, ", "))

	gem := geminiDariEnv()
	hasil, err := gem.generateJSON(prompt)
	if err != nil {
		pesan := err.Error()
		status := http.StatusBadGateway
		if strings.Contains(pesan, "sibuk") {
			status = http.StatusServiceUnavailable
		}
		kirimPesan(w, status, pesan)
		return
	}

	nama := strings.TrimSpace(teksNilai(hasil["name"]))
	if nama == "" {
		nama = "Resep Tanpa Nama"
	}
	deskripsi := strings.TrimSpace(teksNilai(hasil["description"]))
	langkah := uraiLangkahDariAny(hasil["instructions"])
	waktu := int(angka(hasil["cook_time_minutes"]))
	if waktu < 1 {
		waktu = 30
	}
	porsi := int(angka(hasil["servings"]))
	if porsi < 1 {
		porsi = 2
	}
	if porsi > 8 {
		porsi = 8
	}
	bahan := uraiBahanDariAny(hasil["ingredients"])
	bahan = normalBahanAI(bahan)

	bahanJSONStr := marshalJSON(bahanKanonik(bahan))
	langkahJSONStr := marshalJSON(langkah)
	dataResepStr := marshalJSON(hasil)

	var id int64
	err = db.QueryRow(r.Context(), `
		INSERT INTO resep_tersimpan
		  (user_id, nama_resep, data_resep, deskripsi, waktu_memasak, porsi, bahan, langkah, nutrisi, tag, tips)
		VALUES ($1,$2,$3::jsonb,$4,$5,$6,$7::jsonb,$8::jsonb,'{}'::jsonb,'[]'::jsonb,'[]'::jsonb)
		RETURNING id`,
		u.ID, nama, dataResepStr, nullIfEmpty(deskripsi), waktu, porsi,
		bahanJSONStr, langkahJSONStr).Scan(&id)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan resep")
		return
	}

	perNama := map[string]bahanJSON{}
	if _, pn, err := muatPetaBahan(r.Context(), r); err == nil {
		perNama = pn
	}
	rt := resepTersimpan{ID: id, Nama: nama, Deskripsi: deskripsi, Waktu: waktu, Porsi: porsi, Langkah: langkah, Bahan: bahan}
	kirimJSON(w, http.StatusCreated, map[string]any{
		"success": true, "message": "Resep berhasil dibuat", "data": resepDariTersimpan(rt, perNama, map[int64]bool{}),
	})
}

// normalBahanAI membuang bahan tanpa nama dan memastikan jumlah positif.
func normalBahanAI(bahan []bahanAI) []bahanAI {
	hasil := []bahanAI{}
	for _, b := range bahan {
		b.IngredientName = strings.TrimSpace(b.IngredientName)
		if b.IngredientName == "" {
			continue
		}
		if b.Quantity <= 0 {
			b.Quantity = 1
		}
		if strings.TrimSpace(b.Unit) == "" {
			b.Unit = "gram"
		}
		hasil = append(hasil, b)
	}
	return hasil
}

type bahanKanonikItem struct {
	IngredientName string  `json:"ingredient_name"`
	Quantity       float64 `json:"quantity"`
	Unit           string  `json:"unit"`
}

func bahanKanonik(bahan []bahanAI) []bahanKanonikItem {
	hasil := make([]bahanKanonikItem, 0, len(bahan))
	for _, b := range bahan {
		hasil = append(hasil, bahanKanonikItem{IngredientName: b.IngredientName, Quantity: b.Quantity, Unit: b.Unit})
	}
	return hasil
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
