package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// =========================
// STRUKTUR DATA MENU
// =========================

type ResepMenu struct {
	ResepID int
	Porsi   float64 // jumlah porsi resep yang disajikan di menu ini
}

type Menu struct {
	ID         int
	Nama       string
	Deskripsi  string
	Tanggal    string // format YYYY-MM-DD, boleh kosong
	WaktuMakan string
	Resep      []ResepMenu
}

type BarisResep struct {
	ID    int
	Nama  string
	Porsi float64
	Gizi  Gizi // total gizi baris ini (gizi per porsi x porsi)
}

type MenuView struct {
	Menu
	Baris []BarisResep
	Total Gizi
}

type FormMenu struct {
	Judul       string
	Action      string
	Menu        Menu
	Opsi        []BahanOpsi // dropdown resep (ID + label)
	DaftarWaktu []string
}

var daftarWaktuMakan = []string{"Sarapan", "Makan Siang", "Makan Malam", "Selingan"}

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

// Menghitung gizi menu dari resep-resepnya
func hitungMenu(m Menu, resep map[int]ResepView) MenuView {
	v := MenuView{Menu: m}

	for _, rm := range m.Resep {
		rv, ada := resep[rm.ResepID]
		if !ada {
			v.Baris = append(v.Baris, BarisResep{Nama: "(resep tidak ditemukan)", Porsi: rm.Porsi})
			continue
		}
		g := tambahGizi(Gizi{}, rv.PerPorsi, rm.Porsi)
		v.Total = tambahGizi(v.Total, rv.PerPorsi, rm.Porsi)
		v.Baris = append(v.Baris, BarisResep{ID: rv.ID, Nama: rv.Nama, Porsi: rm.Porsi, Gizi: g})
	}

	return v
}

// Menyiapkan perhitungan semua resep
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

	ids := r.Form["resep_id"]
	por := r.Form["jumlah_porsi"]
	var resep []ResepMenu
	for i := range ids {
		if i >= len(por) {
			break
		}
		id, err := strconv.Atoi(ids[i])
		p := parseFloat(por[i])
		if err != nil || p <= 0 {
			continue
		}
		resep = append(resep, ResepMenu{ResepID: id, Porsi: p})
	}
	if len(resep) == 0 {
		return Menu{}, "Menu harus punya minimal satu resep dengan porsi lebih dari 0"
	}

	return Menu{
		Nama:       nama,
		Deskripsi:  strings.TrimSpace(r.FormValue("deskripsi")),
		Tanggal:    strings.TrimSpace(r.FormValue("tanggal")),
		WaktuMakan: r.FormValue("waktu_makan"),
		Resep:      resep,
	}, ""
}

func tampilkanFormMenu(w http.ResponseWriter, r *http.Request, data FormMenu) {
	daftar, err := dbSemuaResep(r.Context())
	if err != nil {
		galat(w, "Gagal membaca data resep", err)
		return
	}
	for _, rs := range daftar {
		data.Opsi = append(data.Opsi, BahanOpsi{
			ID:    rs.ID,
			Label: fmt.Sprintf("%s (%d porsi)", rs.Nama, rs.Porsi),
		})
	}
	data.DaftarWaktu = daftarWaktuMakan
	renderResep(w, "form_menu.html", data)
}

// =========================
// HANDLER
// =========================

// Halaman gabungan: daftar menu (dengan resep di dalamnya) + semua resep
func menuListPage(w http.ResponseWriter, r *http.Request) {
	peta, daftarResep, err := muatResepView(r)
	if err != nil {
		galat(w, "Gagal membaca data resep", err)
		return
	}
	daftar, err := dbSemuaMenu(r.Context())
	if err != nil {
		galat(w, "Gagal membaca data menu", err)
		return
	}

	menus := make([]MenuView, 0, len(daftar))
	for _, m := range daftar {
		menus = append(menus, hitungMenu(m, peta))
	}

	reseps := make([]ResepView, 0, len(daftarResep))
	for _, rs := range daftarResep {
		reseps = append(reseps, peta[rs.ID])
	}

	renderResep(w, "menu.html", struct {
		DaftarMenu  []MenuView
		DaftarResep []ResepView
	}{menus, reseps})
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
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
		return
	}

	tampilkanFormMenu(w, r, FormMenu{
		Judul:  "Tambah Menu",
		Action: "/menu/tambah",
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
	m, err := dbMenuByID(r.Context(), id)
	if err != nil {
		if tidakAda(err) {
			http.NotFound(w, r)
			return
		}
		galat(w, "Gagal membaca menu", err)
		return
	}
	peta, _, err := muatResepView(r)
	if err != nil {
		galat(w, "Gagal membaca data resep", err)
		return
	}
	renderResep(w, "detail_menu.html", hitungMenu(m, peta))
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
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
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