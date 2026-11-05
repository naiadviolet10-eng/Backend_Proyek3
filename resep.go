package main

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

// =========================
// STRUKTUR DATA RESEP
// Resep dibuat oleh AI dan hanya dibaca oleh aplikasi ini.
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
	MenuID   int    // 0 jika resep belum menempel ke menu
	MenuNama string // nama menu tempat resep ini menempel
}

// Pilihan bahan di dropdown form (dipakai form menu)
type BahanOpsi struct {
	ID    int
	Label string
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

// =========================
// HANDLER (HANYA BACA)
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
	perResep, _, err := muatTautan(r)
	if err != nil {
		galat(w, "Gagal membaca menu resep", err)
		return
	}

	peta := petaBahan(bahan)
	views := make([]ResepView, 0, len(daftar))
	for _, rs := range daftar {
		v := hitungView(rs, peta)
		if t, ada := perResep[rs.ID]; ada {
			v.MenuID = t.MenuID
			v.MenuNama = t.MenuNama
		}
		views = append(views, v)
	}

	renderResep(w, "resep.html", struct{ DaftarResep []ResepView }{views})
}

// /resep/{id}  (hanya detail; tidak ada edit atau hapus)
func aksiResepPage(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/resep/"), "/")
	id, err := strconv.Atoi(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	detailResep(w, r, id)
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
	perResep, _, err := muatTautan(r)
	if err != nil {
		galat(w, "Gagal membaca menu resep", err)
		return
	}

	v := hitungView(resep, petaBahan(bahan))
	if t, ada := perResep[id]; ada {
		v.MenuID = t.MenuID
		v.MenuNama = t.MenuNama
	}
	renderResep(w, "detail_resep.html", v)
}