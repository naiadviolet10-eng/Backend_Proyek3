package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

// =========================
// STRUKTUR DATA RESEP
// =========================

// Satu baris bahan di dalam resep (hanya menyimpan referensi ke bahan)
type BahanResep struct {
	BahanID int
	Jumlah  float64 // dalam gram
}

type Resep struct {
	ID        int
	Nama      string
	Deskripsi string
	Porsi     int
	Langkah   string
	Bahan     []BahanResep
}

// Total gizi (sesuai kolom di tabel ingredients_master)
type Gizi struct {
	Kalori, Protein, Lemak, Karbohidrat float64
	ZatBesi, Seng, VitaminA, VitaminC   float64
}

// Baris bahan yang sudah siap ditampilkan
type BarisBahan struct {
	Nama   string
	Jumlah float64
	Satuan string
	Gram   float64
}

// Resep + hasil perhitungan, dikirim ke template
type ResepView struct {
	Resep
	Baris    []BarisBahan
	Total    Gizi
	PerPorsi Gizi
}

// Pilihan bahan di dropdown form
type BahanOpsi struct {
	ID    int
	Label string
}

type FormResep struct {
	Judul  string
	Action string
	Resep  Resep
	Opsi   []BahanOpsi
}

var funcsResep = template.FuncMap{
	"inc": func(i int) int { return i + 1 },
	// Satu angka desimal untuk tampilan gizi
	"f1": func(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) },
	// Angka 0 ditampilkan kosong di form
	"angka": func(f float64) string {
		if f == 0 {
			return ""
		}
		return strconv.FormatFloat(f, 'f', -1, 64)
	},
}

func petaBahan(daftar []Bahan) map[int]Bahan {
	m := make(map[int]Bahan, len(daftar))
	for _, b := range daftar {
		m[b.ID] = b
	}
	return m
}

// Menghitung gizi resep dari data bahan terbaru.
// Jumlah bahan dalam gram, nilai gizi bahan per 100 g:
// gizi = nilai per 100 g x jumlah gram / 100
func hitungView(r Resep, bahan map[int]Bahan) ResepView {
	v := ResepView{Resep: r}

	for _, br := range r.Bahan {
		b, ada := bahan[br.BahanID]
		if !ada {
			v.Baris = append(v.Baris, BarisBahan{Nama: "(bahan tidak ditemukan)", Jumlah: br.Jumlah, Satuan: "g"})
			continue
		}

		gram := br.Jumlah
		f := gram / 100

		v.Total.Kalori += b.Kalori * f
		v.Total.Protein += b.Protein * f
		v.Total.Lemak += b.Lemak * f
		v.Total.Karbohidrat += b.Karbohidrat * f
		v.Total.ZatBesi += b.ZatBesi * f
		v.Total.Seng += b.Seng * f
		v.Total.VitaminA += b.VitaminA * f
		v.Total.VitaminC += b.VitaminC * f

		v.Baris = append(v.Baris, BarisBahan{Nama: b.NamaBahan, Jumlah: br.Jumlah, Satuan: "g", Gram: gram})
	}

	porsi := float64(r.Porsi)
	if porsi < 1 {
		porsi = 1
	}
	t := v.Total
	v.PerPorsi = Gizi{
		Kalori: t.Kalori / porsi, Protein: t.Protein / porsi, Lemak: t.Lemak / porsi,
		Karbohidrat: t.Karbohidrat / porsi,
		ZatBesi:     t.ZatBesi / porsi, Seng: t.Seng / porsi,
		VitaminA: t.VitaminA / porsi, VitaminC: t.VitaminC / porsi,
	}

	return v
}

func renderResep(w http.ResponseWriter, file string, data any) {
	tmpl, err := template.New(file).Funcs(funcsResep).ParseFiles("templates/" + file)
	if err != nil {
		http.Error(w, "Template "+file+" error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Gagal menampilkan "+file+": "+err.Error(), http.StatusInternalServerError)
	}
}

// Membaca isian form resep (ID diisi oleh pemanggil)
func bacaFormResep(r *http.Request) (Resep, string) {
	if err := r.ParseForm(); err != nil {
		return Resep{}, "Form tidak valid: " + err.Error()
	}

	nama := strings.TrimSpace(r.FormValue("nama"))
	if nama == "" {
		return Resep{}, "Nama resep wajib diisi"
	}

	porsi, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("porsi")))
	if porsi < 1 {
		porsi = 1
	}

	// bahan_id dan jumlah dikirim berurutan, satu pasang per baris
	ids := r.Form["bahan_id"]
	jml := r.Form["jumlah"]
	var bahan []BahanResep
	for i := range ids {
		if i >= len(jml) {
			break
		}
		id, err := strconv.Atoi(ids[i])
		j := parseFloat(jml[i])
		if err != nil || j <= 0 {
			continue
		}
		bahan = append(bahan, BahanResep{BahanID: id, Jumlah: j})
	}
	if len(bahan) == 0 {
		return Resep{}, "Resep harus punya minimal satu bahan dengan jumlah lebih dari 0"
	}

	return Resep{
		Nama:      nama,
		Deskripsi: strings.TrimSpace(r.FormValue("deskripsi")),
		Porsi:     porsi,
		Langkah:   strings.TrimSpace(r.FormValue("langkah")),
		Bahan:     bahan,
	}, ""
}

func tampilkanFormResep(w http.ResponseWriter, r *http.Request, data FormResep) {
	daftar, err := dbSemuaBahan(r.Context())
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}

	// Jumlah bahan di resep dihitung dalam gram
	for _, b := range daftar {
		data.Opsi = append(data.Opsi, BahanOpsi{ID: b.ID, Label: b.NamaBahan})
	}

	renderResep(w, "form_resep.html", data)
}

// =========================
// HANDLER
// =========================

// Daftar resep: /resep
func resepListPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	daftar, err := dbSemuaResep(ctx)
	if err != nil {
		galat(w, "Gagal membaca data resep", err)
		return
	}
	bahan, err := dbSemuaBahan(ctx)
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}

	peta := petaBahan(bahan)
	views := make([]ResepView, 0, len(daftar))
	for _, rs := range daftar {
		views = append(views, hitungView(rs, peta))
	}

	renderResep(w, "resep.html", struct{ DaftarResep []ResepView }{views})
}

// Tambah resep: /resep/tambah
func tambahResepPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		resep, pesan := bacaFormResep(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}
		if err := dbTambahResep(r.Context(), resep); err != nil {
			galat(w, "Gagal menyimpan resep", err)
			return
		}
		http.Redirect(w, r, "/resep", http.StatusSeeOther)
		return
	}

	tampilkanFormResep(w, r, FormResep{
		Judul:  "Tambah Resep",
		Action: "/resep/tambah",
		Resep:  Resep{Porsi: 1},
	})
}

// /resep/{id}  /resep/{id}/edit  /resep/{id}/hapus
func aksiResepPage(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/resep/"), "/")
	bagian := strings.Split(path, "/")

	id, err := strconv.Atoi(bagian[0])
	if err != nil || len(bagian) > 2 {
		http.NotFound(w, r)
		return
	}

	if len(bagian) == 1 {
		detailResep(w, r, id)
		return
	}

	switch bagian[1] {
	case "edit":
		editResep(w, r, id)
	case "hapus":
		hapusResep(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func detailResep(w http.ResponseWriter, r *http.Request, id int) {
	ctx := r.Context()

	resep, err := dbResepByID(ctx, id)
	if err != nil {
		if tidakAda(err) {
			http.NotFound(w, r)
			return
		}
		galat(w, "Gagal membaca resep", err)
		return
	}

	bahan, err := dbSemuaBahan(ctx)
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}

	renderResep(w, "detail_resep.html", hitungView(resep, petaBahan(bahan)))
}

func editResep(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method == http.MethodPost {
		baru, pesan := bacaFormResep(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}
		baru.ID = id

		if err := dbUbahResep(r.Context(), baru); err != nil {
			if tidakAda(err) {
				http.NotFound(w, r)
				return
			}
			galat(w, "Gagal menyimpan perubahan resep", err)
			return
		}

		http.Redirect(w, r, "/resep", http.StatusSeeOther)
		return
	}

	resep, err := dbResepByID(r.Context(), id)
	if err != nil {
		if tidakAda(err) {
			http.NotFound(w, r)
			return
		}
		galat(w, "Gagal membaca resep", err)
		return
	}

	tampilkanFormResep(w, r, FormResep{
		Judul:  "Edit Resep",
		Action: fmt.Sprintf("/resep/%d/edit", id),
		Resep:  resep,
	})
}

func hapusResep(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/resep", http.StatusSeeOther)
		return
	}

	if err := dbHapusResep(r.Context(), id); err != nil {
		galat(w, "Gagal menghapus resep", err)
		return
	}

	http.Redirect(w, r, "/resep", http.StatusSeeOther)
}