package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/crypto/bcrypt"
)

// Kredensial admin dibaca dari environment lewat .env (godotenv/autoload):
// ADMIN_USERNAME dan ADMIN_PASSWORD. Tidak ada fallback ke nilai hardcoded
// supaya rahasia tidak pernah masuk ke riwayat versi.
func kredensialAdmin() (username, password string) {
	username = strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_USERNAME")))
	password = os.Getenv("ADMIN_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("ADMIN_USERNAME atau ADMIN_PASSWORD belum di-set di .env")
	}
	return username, password
}

// Kalau SESSION_KEY belum diset di terminal, pakai kunci sementara ini
// supaya session tetap bisa tersimpan (untuk belajar/testing saja).
func ambilSessionKey() []byte {
	key := os.Getenv("SESSION_KEY")
	if key == "" {
		log.Print("PERINGATAN: SESSION_KEY kosong, memakai kunci sementara")
		key = "kunci-sementara-dapur-cerdas-1234567890"
	}
	return []byte(key)
}

var store = sessions.NewCookieStore(ambilSessionKey())

// Daftar user: username -> hash bcrypt.
// Hash dibuat otomatis dari ADMIN_PASSWORD saat program dijalankan.
var users = map[string]string{}

func init() {
	username, password := kredensialAdmin()

	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	users[username] = string(h)

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 3, // login berlaku 3 jam
		HttpOnly: true,
		Secure:   false, // ganti true kalau sudah pakai HTTPS
		SameSite: http.SameSiteLaxMode,
	}
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

	hash, ok := users[username]
	if !ok || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		log.Printf("login gagal untuk user %q", username)
		tmpl.Execute(w, map[string]string{
			"Error": "Username atau password salah",
			"Email": username,
		})
		return
	}

	sess, _ := store.Get(r, "session")
	sess.Values["username"] = username
	if err := sess.Save(r, w); err != nil {
		log.Printf("gagal menyimpan session: %v", err)
		http.Error(w, "Gagal menyimpan session: "+err.Error(), http.StatusInternalServerError)
		return
	}

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
