package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

// =====================================================
// UBAH DI SINI kalau mau ganti username dan password
// (username harus huruf kecil semua)
// =====================================================
const (
	loginUsername = "admindapurcerdas"
	loginPassword = "Admin123"
)

// Kalau SESSION_KEY belum diset di terminal, pakai kunci sementara ini
// supaya session tetap bisa tersimpan (untuk belajar/testing saja).
func ambilSessionKey() []byte {
	key := os.Getenv("SESSION_KEY")
	if key == "" {
		fmt.Println("PERINGATAN: SESSION_KEY kosong, memakai kunci sementara")
		key = "kunci-sementara-dapur-cerdas-1234567890"
	}
	return []byte(key)
}

var store = sessions.NewCookieStore(ambilSessionKey())

// Daftar user: username -> hash bcrypt.
// Hash dibuat otomatis dari loginPassword saat program dijalankan.
var users = map[string]string{}

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte(loginPassword), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	users[loginUsername] = string(h)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/login.html")
	if err != nil {
		http.Error(w, "Template login error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodGet {
		tmpl.Execute(w, map[string]string{})
		return
	}

	// Spasi di awal/akhir dibuang, huruf besar/kecil username diabaikan
	username := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	fmt.Printf("DEBUG: form diterima, user=%q\n", username)

	hash, ok := users[username]
	if !ok {
		fmt.Println("DEBUG: user tidak ditemukan:", username)
	}

	if !ok || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		fmt.Println("DEBUG: login gagal (user/password salah)")
		tmpl.Execute(w, map[string]string{
			"Error": "Username atau password salah",
			"Email": username,
		})
		return
	}

	sess, _ := store.Get(r, "session")
	sess.Values["username"] = username
	if err := sess.Save(r, w); err != nil {
		fmt.Println("DEBUG: gagal simpan session:", err)
		http.Error(w, "Gagal menyimpan session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println("DEBUG: login berhasil, redirect ke /dashboard")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	sess, _ := store.Get(r, "session")
	sess.Options.MaxAge = -1
	sess.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Halaman yang dibungkus fungsi ini wajib login
func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, _ := store.Get(r, "session")
		if sess.Values["username"] == nil {
			fmt.Println("DEBUG: belum login, dialihkan ke /login dari", r.URL.Path)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// Halaman profil: menampilkan user yang sedang login
func profilPage(w http.ResponseWriter, r *http.Request) {
	sess, _ := store.Get(r, "session")
	username, _ := sess.Values["username"].(string)

	inisial := "?"
	if username != "" {
		inisial = strings.ToUpper(username[:1])
	}

	tmpl, err := template.ParseFiles("templates/profil.html")
	if err != nil {
		http.Error(w, "Template profil error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Username string
		Inisial  string
	}{
		Username: username,
		Inisial:  inisial,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Gagal menampilkan halaman profil: "+err.Error(), http.StatusInternalServerError)
	}
}