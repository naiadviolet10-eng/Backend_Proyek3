package main

import (
	"net/http"
	"strings"
)

// GET /api/me
func apiMe(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok {
		kirimPesan(w, http.StatusUnauthorized, "Belum login")
		return
	}

	var nama string
	var status string
	var foto *string
	err := db.QueryRow(r.Context(),
		`SELECT COALESCE(nama,''), COALESCE(status,'aktif'), foto_profil
		 FROM public.users WHERE id = $1`, u.ID).
		Scan(&nama, &status, &foto)
	if err != nil && !tidakAda(err) {
		kirimPesan(w, http.StatusInternalServerError, "Gagal membaca profil")
		return
	}
	if nama == "" {
		nama = u.Nama
	}

	kirimData(w, map[string]any{
		"id":          u.ID,
		"name":        nama,
		"email":       u.Email,
		"status":      status,
		"foto_profil": foto,
	})
}

// PUT /api/me
func apiUpdateMe(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok {
		kirimPesan(w, http.StatusUnauthorized, "Belum login")
		return
	}

	var isian struct {
		Name string `json:"name"`
	}
	if err := bacaJSON(r, &isian); err != nil {
		kirimValidasi(w, "Data tidak valid", "name", "Format permintaan salah")
		return
	}
	isian.Name = strings.TrimSpace(isian.Name)
	if isian.Name == "" {
		kirimValidasi(w, "Nama wajib diisi", "name", "Nama tidak boleh kosong")
		return
	}

	if _, err := db.Exec(r.Context(),
		`UPDATE public.users SET nama = $1, updated_at = now() WHERE id = $2`,
		isian.Name, u.ID); err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan profil")
		return
	}

	kirimData(w, map[string]any{
		"id":    u.ID,
		"name":  isian.Name,
		"email": u.Email,
	})
}

// POST /api/logout: sesi Supabase diakhiri di sisi aplikasi, server cukup mengakui.
func apiLogout(w http.ResponseWriter, r *http.Request) {
	kirimData(w, nil)
}
