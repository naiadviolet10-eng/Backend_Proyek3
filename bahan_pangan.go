package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// =========================
// STRUKTUR DATA BAHAN PANGAN
// =========================

type Bahan struct {
	ID             int
	NamaBahan      string
	Kategori       string
	Satuan         string
	BeratPerSatuan float64
	BDD            float64
	Kalori         float64
	Protein        float64
	Lemak          float64
	Karbohidrat    float64
	Serat          float64
	Gula           float64
	Natrium        float64
	Kolesterol     float64
	Kalsium        float64
	ZatBesi        float64
	Kalium         float64
	VitaminA       float64
	VitaminC       float64
	Alergen        []string
	SumberData     string
}

// Data yang dikirim ke form tambah/edit
type FormBahan struct {
	Judul          string
	Action         string
	Bahan          Bahan
	DaftarKategori []string
	DaftarAlergen  []string
}

var daftarKategori = []string{
	"Karbohidrat",
	"Protein Hewani",
	"Protein Nabati",
	"Sayuran",
	"Buah",
	"Susu & Olahannya",
	"Lemak & Minyak",
	"Bumbu",
	"Lainnya",
}

var daftarAlergen = []string{
	"Kacang Tanah",
	"Kacang Pohon",
	"Kedelai",
	"Susu",
	"Telur",
	"Ikan",
	"Krustasea",
	"Moluska",
	"Gluten",
	"Wijen",
}

// =========================
// HELPER
// =========================

// Mengubah teks form menjadi angka (kosong atau salah = 0)
func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// Menampilkan error asli di browser dan mencatatnya di terminal
func galat(w http.ResponseWriter, pesan string, err error) {
	log.Println(pesan+":", err)
	http.Error(w, pesan+": "+err.Error(), http.StatusInternalServerError)
}

// Membaca isian form menjadi struct Bahan (ID diisi oleh pemanggil)
func bacaForm(r *http.Request) (Bahan, string) {

	if err := r.ParseForm(); err != nil {
		return Bahan{}, "Form tidak valid: " + err.Error()
	}

	nama := strings.TrimSpace(r.FormValue("nama_bahan"))
	if nama == "" {
		return Bahan{}, "Nama bahan wajib diisi"
	}

	// BDD default 100% kalau dikosongkan
	bdd := 100.0
	if strings.TrimSpace(r.FormValue("bdd")) != "" {
		bdd = parseFloat(r.FormValue("bdd"))
	}

	return Bahan{
		NamaBahan:      nama,
		Kategori:       r.FormValue("kategori"),
		Satuan:         r.FormValue("satuan"),
		BeratPerSatuan: parseFloat(r.FormValue("berat_per_satuan")),
		BDD:            bdd,
		Kalori:         parseFloat(r.FormValue("kalori")),
		Protein:        parseFloat(r.FormValue("protein")),
		Lemak:          parseFloat(r.FormValue("lemak")),
		Karbohidrat:    parseFloat(r.FormValue("karbohidrat")),
		Serat:          parseFloat(r.FormValue("serat")),
		Gula:           parseFloat(r.FormValue("gula")),
		Natrium:        parseFloat(r.FormValue("natrium")),
		Kolesterol:     parseFloat(r.FormValue("kolesterol")),
		Kalsium:        parseFloat(r.FormValue("kalsium")),
		ZatBesi:        parseFloat(r.FormValue("zat_besi")),
		Kalium:         parseFloat(r.FormValue("kalium")),
		VitaminA:       parseFloat(r.FormValue("vitamin_a")),
		VitaminC:       parseFloat(r.FormValue("vitamin_c")),
		Alergen:        r.Form["alergen"],
		SumberData:     r.FormValue("sumber_data"),
	}, ""
}

// =========================
// FORM TAMBAH / EDIT BAHAN
// =========================

func tampilkanForm(w http.ResponseWriter, data FormBahan) {

	data.DaftarKategori = daftarKategori
	data.DaftarAlergen = daftarAlergen

	funcs := template.FuncMap{
		"angka": func(f float64) string {
			if f == 0 {
				return ""
			}
			return strconv.FormatFloat(f, 'f', -1, 64)
		},
		"has": func(list []string, s string) bool {
			for _, v := range list {
				if v == s {
					return true
				}
			}
			return false
		},
	}

	tmpl, err := template.New("tambah_bahan.html").
		Funcs(funcs).
		ParseFiles("templates/tambah_bahan.html")
	if err != nil {
		http.Error(w, "Template form bahan error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Gagal menampilkan form: "+err.Error(), http.StatusInternalServerError)
	}
}

// =========================
// HALAMAN BAHAN PANGAN
// =========================

func bahanPanganPage(w http.ResponseWriter, r *http.Request) {

	funcs := template.FuncMap{
		"inc": func(i int) int { return i + 1 },
	}

	tmpl, err := template.New("bahan_pangan.html").
		Funcs(funcs).
		ParseFiles("templates/bahan_pangan.html")
	if err != nil {
		http.Error(w, "Template bahan pangan error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	daftar, err := dbSemuaBahan(r.Context())
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}

	data := struct {
		DaftarBahan []Bahan
	}{
		DaftarBahan: daftar,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Gagal menampilkan halaman bahan pangan: "+err.Error(), http.StatusInternalServerError)
	}
}

// =========================
// HALAMAN TAMBAH BAHAN
// =========================

func tambahBahanPage(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodPost {

		bahan, pesan := bacaForm(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}

		if err := dbTambahBahan(r.Context(), bahan); err != nil {
			galat(w, "Gagal menyimpan bahan", err)
			return
		}

		http.Redirect(w, r, "/bahan", http.StatusSeeOther)
		return
	}

	tampilkanForm(w, FormBahan{
		Judul:  "Tambah Bahan",
		Action: "/bahan/tambah",
		Bahan:  Bahan{BDD: 100},
	})
}

// =========================
// EDIT & HAPUS BAHAN
// =========================

func aksiBahanPage(w http.ResponseWriter, r *http.Request) {

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/bahan/"), "/")
	bagian := strings.Split(path, "/")

	if len(bagian) != 2 {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(bagian[0])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	switch bagian[1] {
	case "edit":
		editBahan(w, r, id)
	case "hapus":
		hapusBahan(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func editBahan(w http.ResponseWriter, r *http.Request, id int) {

	if r.Method == http.MethodPost {

		baru, pesan := bacaForm(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}

		baru.ID = id

		if err := dbUbahBahan(r.Context(), baru); err != nil {
			if tidakAda(err) {
				http.NotFound(w, r)
				return
			}
			galat(w, "Gagal menyimpan perubahan", err)
			return
		}

		http.Redirect(w, r, "/bahan", http.StatusSeeOther)
		return
	}

	bahan, err := dbBahanByID(r.Context(), id)
	if err != nil {
		if tidakAda(err) {
			http.NotFound(w, r)
			return
		}
		galat(w, "Gagal membaca data bahan", err)
		return
	}

	tampilkanForm(w, FormBahan{
		Judul:  "Edit Bahan",
		Action: fmt.Sprintf("/bahan/%d/edit", id),
		Bahan:  bahan,
	})
}

func hapusBahan(w http.ResponseWriter, r *http.Request, id int) {

	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/bahan", http.StatusSeeOther)
		return
	}

	if err := dbHapusBahan(r.Context(), id); err != nil {
		if bahanDipakai(err) {
			http.Error(w,
				"Bahan tidak bisa dihapus karena masih dipakai oleh resep. Hapus dulu dari resep yang memakainya.",
				http.StatusConflict)
			return
		}
		galat(w, "Gagal menghapus bahan", err)
		return
	}

	http.Redirect(w, r, "/bahan", http.StatusSeeOther)
}