package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

// =====================================================
// MODEL RESEP DAN GIZI
// =====================================================

type Gizi struct {
	Kalori      float64
	Protein     float64
	Lemak       float64
	Karbohidrat float64
	Serat       float64
	Gula        float64
	Natrium     float64
	Kolesterol  float64
	ZatBesi     float64
	Seng        float64
	VitaminA    float64
	VitaminC    float64
}

type Resep struct {
	ID        int
	Nama      string
	Deskripsi string
	Porsi     int
	Langkah   string
	Bahan     []BahanResep
}

type BahanResep struct {
	BahanID int
	Jumlah  float64
}

type BarisBahan struct {
	Nama   string
	Jumlah float64
	Satuan string
	Gram   float64
}

type ResepView struct {
	Resep
	Baris    []BarisBahan
	Total    Gizi
	PerPorsi Gizi
	Alergen  []string
}

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

// =====================================================
// TEMPLATE
// =====================================================

var funcsResep = template.FuncMap{
	"inc": func(i int) int {
		return i + 1
	},
	"f1": func(n float64) string {
		return strconv.FormatFloat(n, 'f', 1, 64)
	},
	"angka": func(n float64) string {
		return strconv.FormatFloat(n, 'f', -1, 64)
	},
}

func renderResep(w http.ResponseWriter, namaTemplate string, data any) {
	t, err := template.New(namaTemplate).
		Funcs(funcsResep).
		ParseFiles("templates/" + namaTemplate)
	if err != nil {
		http.Error(w, "Gagal membaca template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.ExecuteTemplate(w, namaTemplate, data); err != nil {
		http.Error(w, "Gagal menampilkan halaman: "+err.Error(), http.StatusInternalServerError)
	}
}

// =====================================================
// DATA BAHAN DAN HITUNG GIZI
// =====================================================

func petaBahan(daftar []Bahan) map[int]Bahan {
	hasil := make(map[int]Bahan, len(daftar))
	for _, bahan := range daftar {
		hasil[bahan.ID] = bahan
	}
	return hasil
}

// Jangan mendefinisikan tambahGizi di sini karena fungsi
// dengan nama tersebut sudah ada di menu.go.
func hitungView(r Resep, bahanMap map[int]Bahan) ResepView {
	view := ResepView{
		Resep:   r,
		Baris:   make([]BarisBahan, 0, len(r.Bahan)),
		Alergen: []string{},
	}

	for _, item := range r.Bahan {
		bahan, ditemukan := bahanMap[item.BahanID]
		if !ditemukan {
			continue
		}

		view.Baris = append(view.Baris, BarisBahan{
			Nama:   bahan.NamaBahan,
			Jumlah: item.Jumlah,
			Satuan: bahan.Satuan,
			Gram:   item.Jumlah,
		})

		// Data gizi bahan saat ini diasumsikan per 100 gram.
		faktor := item.Jumlah / 100.0

		view.Total.Kalori += bahan.Kalori * faktor
		view.Total.Protein += bahan.Protein * faktor
		view.Total.Lemak += bahan.Lemak * faktor
		view.Total.Karbohidrat += bahan.Karbohidrat * faktor
		view.Total.ZatBesi += bahan.ZatBesi * faktor
		view.Total.Seng += bahan.Seng * faktor
		view.Total.VitaminA += bahan.VitaminA * faktor
		view.Total.VitaminC += bahan.VitaminC * faktor

		// Serat, gula, natrium, kolesterol, dan alergen
		// perlu tersedia pada data bahan agar dapat dihitung.
	}

	porsi := r.Porsi
	if porsi < 1 {
		porsi = 1
	}

	view.PerPorsi = Gizi{
		Kalori:      view.Total.Kalori / float64(porsi),
		Protein:     view.Total.Protein / float64(porsi),
		Lemak:       view.Total.Lemak / float64(porsi),
		Karbohidrat: view.Total.Karbohidrat / float64(porsi),
		Serat:       view.Total.Serat / float64(porsi),
		Gula:        view.Total.Gula / float64(porsi),
		Natrium:     view.Total.Natrium / float64(porsi),
		Kolesterol:  view.Total.Kolesterol / float64(porsi),
		ZatBesi:     view.Total.ZatBesi / float64(porsi),
		Seng:        view.Total.Seng / float64(porsi),
		VitaminA:    view.Total.VitaminA / float64(porsi),
		VitaminC:    view.Total.VitaminC / float64(porsi),
	}

	return view
}

// =====================================================
// FORM RESEP
// =====================================================

func bacaFormResep(r *http.Request) (Resep, string) {
	if err := r.ParseForm(); err != nil {
		return Resep{}, "Data formulir tidak dapat dibaca."
	}

	nama := strings.TrimSpace(r.FormValue("nama"))
	deskripsi := strings.TrimSpace(r.FormValue("deskripsi"))
	langkah := strings.TrimSpace(r.FormValue("langkah"))

	if nama == "" {
		return Resep{}, "Nama resep wajib diisi."
	}

	porsi, err := strconv.Atoi(r.FormValue("porsi"))
	if err != nil || porsi < 1 {
		return Resep{}, "Jumlah porsi harus berupa angka minimal 1."
	}

	ids := r.Form["bahan_id"]
	jumlahs := r.Form["jumlah"]

	if len(ids) != len(jumlahs) {
		return Resep{}, "Data bahan dan jumlah tidak sesuai."
	}

	resep := Resep{
		Nama:      nama,
		Deskripsi: deskripsi,
		Porsi:     porsi,
		Langkah:   langkah,
		Bahan:     []BahanResep{},
	}

	for i := range ids {
		idText := strings.TrimSpace(ids[i])
		jumlahText := strings.TrimSpace(jumlahs[i])

		if idText == "" && jumlahText == "" {
			continue
		}

		id, errID := strconv.Atoi(idText)
		jumlah, errJumlah := strconv.ParseFloat(jumlahText, 64)

		if errID != nil || id < 1 {
			return Resep{}, fmt.Sprintf("Bahan pada baris %d tidak valid.", i+1)
		}
		if errJumlah != nil || jumlah <= 0 {
			return Resep{}, fmt.Sprintf("Jumlah bahan pada baris %d harus lebih dari 0.", i+1)
		}

		resep.Bahan = append(resep.Bahan, BahanResep{
			BahanID: id,
			Jumlah:  jumlah,
		})
	}

	if len(resep.Bahan) == 0 {
		return Resep{}, "Tambahkan minimal satu bahan ke resep."
	}

	return resep, ""
}

func tampilkanFormResep(
	w http.ResponseWriter,
	judul string,
	action string,
	resep Resep,
) {
	daftarBahan, err := dbSemuaBahan(context.Background())
	if err != nil {
		http.Error(w, "Gagal mengambil data bahan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	opsi := make([]BahanOpsi, 0, len(daftarBahan))
	for _, bahan := range daftarBahan {
		opsi = append(opsi, BahanOpsi{
			ID:    bahan.ID,
			Label: fmt.Sprintf("%s (%s)", bahan.NamaBahan, bahan.Satuan),
		})
	}

	renderResep(w, "form_resep.html", FormResep{
		Judul:  judul,
		Action: action,
		Resep:  resep,
		Opsi:   opsi,
	})
}

// =====================================================
// DAFTAR RESEP
// =====================================================

// Fungsi ini memerlukan templates/resep.html.
// Jika route /resep masih diarahkan ke /menu di main.go,
// halaman daftar resep ini belum dipanggil.
func resepListPage(w http.ResponseWriter, r *http.Request) {
	daftar, err := dbSemuaResep(context.Background())
	if err != nil {
		http.Error(w, "Gagal mengambil data resep: "+err.Error(), http.StatusInternalServerError)
		return
	}

	daftarBahan, err := dbSemuaBahan(context.Background())
	if err != nil {
		http.Error(w, "Gagal mengambil data bahan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	bahanMap := petaBahan(daftarBahan)
	views := make([]ResepView, 0, len(daftar))

	for _, resep := range daftar {
		views = append(views, hitungView(resep, bahanMap))
	}

	renderResep(w, "resep.html", views)
}

// =====================================================
// TAMBAH RESEP
// =====================================================

func tambahResepPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		resep, pesan := bacaFormResep(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}

		if err := dbTambahResep(context.Background(), resep); err != nil {
			http.Error(w, "Gagal menyimpan resep: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/resep", http.StatusSeeOther)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Metode tidak diizinkan.", http.StatusMethodNotAllowed)
		return
	}

	tampilkanFormResep(w, "Tambah Resep", "/resep/tambah", Resep{Porsi: 1})
}

// =====================================================
// DETAIL, EDIT, HAPUS
// =====================================================

func aksiResepPage(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/resep/"), "/")
	bagian := strings.Split(path, "/")

	if len(bagian) == 0 || bagian[0] == "" {
		http.NotFound(w, r)
		return
	}

	id, err := strconv.Atoi(bagian[0])
	if err != nil || id < 1 {
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
	resep, err := dbResepByID(context.Background(), id)
	if err != nil {
		http.Error(w, "Resep tidak ditemukan atau gagal dibaca.", http.StatusNotFound)
		return
	}

	daftarBahan, err := dbSemuaBahan(context.Background())
	if err != nil {
		http.Error(w, "Gagal mengambil data bahan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	view := hitungView(resep, petaBahan(daftarBahan))
	renderResep(w, "detail_resep.html", view)
}

func editResep(w http.ResponseWriter, r *http.Request, id int) {
	resep, err := dbResepByID(context.Background(), id)
	if err != nil {
		http.Error(w, "Resep tidak ditemukan.", http.StatusNotFound)
		return
	}

	if r.Method == http.MethodGet {
		tampilkanFormResep(
			w,
			"Edit Resep",
			fmt.Sprintf("/resep/%d/edit", id),
			resep,
		)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Metode tidak diizinkan.", http.StatusMethodNotAllowed)
		return
	}

	resepBaru, pesan := bacaFormResep(r)
	if pesan != "" {
		http.Error(w, pesan, http.StatusBadRequest)
		return
	}

	resepBaru.ID = id

	if err := dbUbahResep(context.Background(), resepBaru); err != nil {
		http.Error(w, "Gagal mengubah resep: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/resep/%d", id), http.StatusSeeOther)
}

func hapusResep(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "Gunakan metode POST untuk menghapus resep.", http.StatusMethodNotAllowed)
		return
	}

	if err := dbHapusResep(context.Background(), id); err != nil {
		http.Error(w, "Gagal menghapus resep: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/resep", http.StatusSeeOther)
}
