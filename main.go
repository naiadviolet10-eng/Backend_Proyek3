package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {

	// Membaca .env (diabaikan kalau tidak ada)
	godotenv.Load()

	// Koneksi database
	if err := initDB(); err != nil {
		log.Fatal("Gagal konek ke database: ", err)
	}
	defer db.Close()

	fmt.Println("Terhubung ke database")

	// Static file (CSS, gambar, JS)
	http.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	// =========================
	// HALAMAN PUBLIK
	// =========================

	http.HandleFunc("/login", loginHandler)   // auth.go
	http.HandleFunc("/logout", logoutHandler) // auth.go

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	// =========================
	// HALAMAN WAJIB LOGIN
	// =========================

	http.HandleFunc("/dashboard", requireLogin(dashboardPage)) // dashboard.go
	http.HandleFunc("/profil", requireLogin(profilPage))       // auth.go

	// FITUR BAHAN PANGAN (bahan.go)
	http.HandleFunc("/bahan", requireLogin(bahanPanganPage))
	http.HandleFunc("/bahan-pangan", requireLogin(bahanPanganPage))
	http.HandleFunc("/bahan/tambah", requireLogin(tambahBahanPage))
	http.HandleFunc("/bahan/", requireLogin(aksiBahanPage)) // edit & hapus

	// FITUR RESEP (resep.go)
	// Daftar resep sudah digabung ke halaman /menu.
	http.HandleFunc("/resep", requireLogin(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
	}))
	http.HandleFunc("/resep/tambah", requireLogin(tambahResepPage))
	http.HandleFunc("/resep/", requireLogin(aksiResepPage))

	// FITUR MENU (menu.go)
	http.HandleFunc("/menu", requireLogin(menuListPage))
	http.HandleFunc("/menu/tambah", requireLogin(tambahMenuPage))
	http.HandleFunc("/menu/", requireLogin(aksiMenuPage))

	// FITUR MONITORING (monitoring.go)
	http.HandleFunc("/monitoring", requireLogin(monitoringPage))

	// API HEALTH CHECK
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"message": "Backend Pangan Bijak berjalan", "status": "success"}`)
	})

	// =========================
	// MENJALANKAN SERVER
	// =========================

	fmt.Println("Server berjalan di http://localhost:8080")

	if err := http.ListenAndServe(":8080", enableCORS(http.DefaultServeMux)); err != nil {
		fmt.Println("Server error:", err)
	}
}