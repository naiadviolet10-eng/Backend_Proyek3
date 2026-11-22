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

func init() {
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

	if username == "" || password == "" {
		tmpl.Execute(w, map[string]string{
			"Error": "Username dan password wajib diisi",
			"Email": username,
		})
		return
	}

	// Verifikasi kredensial admin dari database (tabel public.admin_users)
	var passwordHash string
	err = db.QueryRow(r.Context(), `
		SELECT password_hash
		FROM public.admin_users
		WHERE LOWER(username) = $1
	`, username).Scan(&passwordHash)

	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
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
