package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// =========================
// KELOMPOK MENU
// =========================

var daftarKelompok = []string{"Umum", "Ibu Hamil", "Ibu Menyusui", "Balita"}

func kelompokValid(k string) bool {
	for _, x := range daftarKelompok {
		if x == k {
			return true
		}
	}
	return false
}

// =========================
// STRUKTUR DATA MENU
// =========================

// Satu baris bahan di dalam menu (referensi ke ingredients_master)
type BahanMenu struct {
	BahanID int
	Jumlah  float64 // dalam gram
}

type Menu struct {
	ID            int
	Nama          string
	Deskripsi     string
	Kelompok      string // Umum / Ibu Hamil / Ibu Menyusui / Balita
	SasaranDetail string // contoh: Trimester 2, Usia 6-8 bulan (boleh kosong)
	Porsi         int
	Isi           []BahanMenu
}

// Baris bahan yang sudah siap ditampilkan (dengan gizinya)
type BarisBahanMenu struct {
	Nama   string
	Jumlah float64
	Gizi   Gizi // gizi baris ini = nilai per 100 g x gram / 100
}

// Resep AI yang menempel ke sebuah menu (hanya untuk ditampilkan)
type ResepRingkas struct {
	ID   int
	Nama string
}

// Menu + hasil perhitungan, dikirim ke template
type MenuView struct {
	Menu
	Bahan    []BarisBahanMenu
	Total    Gizi
	PerPorsi Gizi
	Resep    []ResepRingkas
}

type FormMenu struct {
	Judul          string
	Action         string
	Menu           Menu
	Opsi           []BahanOpsi // dropdown bahan (ID + nama)
	DaftarKelompok []string
}

// Menambahkan gizi b (dikali k) ke gizi a
func tambahGizi(a, b Gizi, k float64) Gizi {
	a.Kalori += b.Kalori * k
	a.Protein += b.Protein * k
	a.Lemak += b.Lemak * k
	a.Karbohidrat += b.Karbohidrat * k
	a.ZatBesi += b.ZatBesi * k
	a.Seng += b.Seng * k
	a.VitaminA += b.VitaminA * k
	a.VitaminC += b.VitaminC * k
	return a
}

// Nilai gizi bahan per 100 g
func giziPer100(b Bahan) Gizi {
	return Gizi{
		Kalori: b.Kalori, Protein: b.Protein, Lemak: b.Lemak, Karbohidrat: b.Karbohidrat,
		ZatBesi: b.ZatBesi, Seng: b.Seng, VitaminA: b.VitaminA, VitaminC: b.VitaminC,
	}
}

// Menghitung gizi menu langsung dari bahan-bahannya.
// gizi = nilai per 100 g x jumlah gram / 100
func hitungMenu(m Menu, bahan map[int]Bahan, resep []ResepRingkas) MenuView {
	v := MenuView{Menu: m, Resep: resep}

	for _, bm := range m.Isi {
		b, ada := bahan[bm.BahanID]
		if !ada {
			v.Bahan = append(v.Bahan, BarisBahanMenu{Nama: "(bahan tidak ditemukan)", Jumlah: bm.Jumlah})
			continue
		}
		faktor := bm.Jumlah / 100
		g := tambahGizi(Gizi{}, giziPer100(b), faktor)
		v.Total = tambahGizi(v.Total, giziPer100(b), faktor)
		v.Bahan = append(v.Bahan, BarisBahanMenu{Nama: b.NamaBahan, Jumlah: bm.Jumlah, Gizi: g})
	}

	porsi := float64(m.Porsi)
	if porsi < 1 {
		porsi = 1
	}
	v.PerPorsi = tambahGizi(Gizi{}, v.Total, 1/porsi)

	return v
}

// Menyiapkan perhitungan semua resep (resep hanya dibaca)
func muatResepView(r *http.Request) (map[int]ResepView, []Resep, error) {
	ctx := r.Context()
	daftar, err := dbSemuaResep(ctx)
	if err != nil {
		return nil, nil, err
	}
	bahan, err := dbSemuaBahan(ctx)
	if err != nil {
		return nil, nil, err
	}
	peta := petaBahan(bahan)
	hasil := make(map[int]ResepView, len(daftar))
	for _, rs := range daftar {
		hasil[rs.ID] = hitungView(rs, peta)
	}
	return hasil, daftar, nil
}

func bacaFormMenu(r *http.Request) (Menu, string) {
	if err := r.ParseForm(); err != nil {
		return Menu{}, "Form tidak valid: " + err.Error()
	}

	nama := strings.TrimSpace(r.FormValue("nama"))
	if nama == "" {
		return Menu{}, "Nama menu wajib diisi"
	}

	kelompok := r.FormValue("kelompok")
	if !kelompokValid(kelompok) {
		return Menu{}, "Kelompok menu tidak valid"
	}

	porsi, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("porsi")))
	if porsi < 1 {
		porsi = 1
	}

	// bahan_id dan jumlah dikirim berurutan, satu pasang per baris
	ids := r.Form["bahan_id"]
	jml := r.Form["jumlah"]
	var isi []BahanMenu
	for i := range ids {
		if i >= len(jml) {
			break
		}
		id, err := strconv.Atoi(ids[i])
		j := parseFloat(jml[i])
		if err != nil || j <= 0 {
			continue
		}
		isi = append(isi, BahanMenu{BahanID: id, Jumlah: j})
	}
	if len(isi) == 0 {
		return Menu{}, "Menu harus punya minimal satu bahan dengan jumlah lebih dari 0"
	}

	return Menu{
		Nama:          nama,
		Deskripsi:     strings.TrimSpace(r.FormValue("deskripsi")),
		Kelompok:      kelompok,
		SasaranDetail: strings.TrimSpace(r.FormValue("sasaran_detail")),
		Porsi:         porsi,
		Isi:           isi,
	}, ""
}

func tampilkanFormMenu(w http.ResponseWriter, r *http.Request, data FormMenu) {
	daftar, err := dbSemuaBahan(r.Context())
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}
	for _, b := range daftar {
		data.Opsi = append(data.Opsi, BahanOpsi{ID: b.ID, Label: b.NamaBahan})
	}
	data.DaftarKelompok = daftarKelompok
	renderResep(w, "form_menu.html", data)
}

// =========================
// HANDLER
// =========================

// Halaman gabungan: daftar menu (filter kelompok) + semua resep (hanya dilihat)
// /menu  atau  /menu?kelompok=Ibu%20Hamil
func menuListPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	kelompok := r.URL.Query().Get("kelompok")
	if !kelompokValid(kelompok) {
		kelompok = "" // tampilkan semua
	}

	bahan, err := dbSemuaBahan(ctx)
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}
	peta := petaBahan(bahan)

	daftar, err := dbSemuaMenu(ctx, kelompok)
	if err != nil {
		galat(w, "Gagal membaca data menu", err)
		return
	}
	hitung, err := dbHitungMenu(ctx)
	if err != nil {
		galat(w, "Gagal menghitung menu per kelompok", err)
		return
	}
	perResep, perMenu, err := muatTautan(r)
	if err != nil {
		galat(w, "Gagal membaca resep per menu", err)
		return
	}

	menus := make([]MenuView, 0, len(daftar))
	for _, m := range daftar {
		menus = append(menus, hitungMenu(m, peta, perMenu[m.ID]))
	}

	petaResep, daftarResep, err := muatResepView(r)
	if err != nil {
		galat(w, "Gagal membaca data resep", err)
		return
	}
	reseps := make([]ResepView, 0, len(daftarResep))
	for _, rs := range daftarResep {
		rv := petaResep[rs.ID]
		if t, ada := perResep[rs.ID]; ada {
			rv.MenuID = t.MenuID
			rv.MenuNama = t.MenuNama
		}
		reseps = append(reseps, rv)
	}

	renderResep(w, "menu.html", struct {
		Kelompok    string
		Hitung      map[string]int
		DaftarMenu  []MenuView
		DaftarResep []ResepView
	}{kelompok, hitung, menus, reseps})
}

func tambahMenuPage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		m, pesan := bacaFormMenu(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}
		if err := dbTambahMenu(r.Context(), m); err != nil {
			galat(w, "Gagal menyimpan menu", err)
			return
		}
		http.Redirect(w, r, "/menu?kelompok="+m.Kelompok, http.StatusSeeOther)
		return
	}

	// kelompok awal diambil dari ?kelompok= (kartu yang sedang dipilih)
	kelompok := r.URL.Query().Get("kelompok")
	if !kelompokValid(kelompok) {
		kelompok = "Umum"
	}

	tampilkanFormMenu(w, r, FormMenu{
		Judul:  "Tambah Menu",
		Action: "/menu/tambah",
		Menu:   Menu{Kelompok: kelompok, Porsi: 1},
	})
}

// /menu/{id}  /menu/{id}/edit  /menu/{id}/hapus
func aksiMenuPage(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/menu/"), "/")
	bagian := strings.Split(path, "/")

	id, err := strconv.Atoi(bagian[0])
	if err != nil || len(bagian) > 2 {
		http.NotFound(w, r)
		return
	}

	if len(bagian) == 1 {
		detailMenu(w, r, id)
		return
	}

	switch bagian[1] {
	case "edit":
		editMenu(w, r, id)
	case "hapus":
		hapusMenu(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func detailMenu(w http.ResponseWriter, r *http.Request, id int) {
	ctx := r.Context()

	m, err := dbMenuByID(ctx, id)
	if err != nil {
		if tidakAda(err) {
			http.NotFound(w, r)
			return
		}
		galat(w, "Gagal membaca menu", err)
		return
	}
	bahan, err := dbSemuaBahan(ctx)
	if err != nil {
		galat(w, "Gagal membaca data bahan", err)
		return
	}
	_, perMenu, err := muatTautan(r)
	if err != nil {
		galat(w, "Gagal membaca resep menu", err)
		return
	}

	renderResep(w, "detail_menu.html", hitungMenu(m, petaBahan(bahan), perMenu[m.ID]))
}

func editMenu(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method == http.MethodPost {
		baru, pesan := bacaFormMenu(r)
		if pesan != "" {
			http.Error(w, pesan, http.StatusBadRequest)
			return
		}
		baru.ID = id
		if err := dbUbahMenu(r.Context(), baru); err != nil {
			if tidakAda(err) {
				http.NotFound(w, r)
				return
			}
			galat(w, "Gagal menyimpan perubahan menu", err)
			return
		}
		http.Redirect(w, r, "/menu?kelompok="+baru.Kelompok, http.StatusSeeOther)
		return
	}

	m, err := dbMenuByID(r.Context(), id)
	if err != nil {
		if tidakAda(err) {
			http.NotFound(w, r)
			return
		}
		galat(w, "Gagal membaca menu", err)
		return
	}

	tampilkanFormMenu(w, r, FormMenu{
		Judul:  "Edit Menu",
		Action: fmt.Sprintf("/menu/%d/edit", id),
		Menu:   m,
	})
}

func hapusMenu(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}
	if err := dbHapusMenu(r.Context(), id); err != nil {
		galat(w, "Gagal menghapus menu", err)
		return
	}
	http.Redirect(w, r, "/menu", http.StatusSeeOther)
}

// =========================
// TAUTAN RESEP <-> MENU (hanya baca)
// =========================

// Mengembalikan resep per resep_id (untuk kolom "Untuk Menu")
// dan daftar resep per menu_id (untuk bagian "Resep dari AI").
func muatTautan(r *http.Request) (map[int]ResepTaut, map[int][]ResepRingkas, error) {
	daftar, err := dbResepTaut(r.Context())
	if err != nil {
		return nil, nil, err
	}
	perResep := make(map[int]ResepTaut, len(daftar))
	perMenu := make(map[int][]ResepRingkas)
	for _, t := range daftar {
		perResep[t.ResepID] = t
		perMenu[t.MenuID] = append(perMenu[t.MenuID], ResepRingkas{ID: t.ResepID, Nama: t.Nama})
	}
	return perResep, perMenu, nil
}