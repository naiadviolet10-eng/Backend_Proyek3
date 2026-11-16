package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	if err := initDB(); err != nil {
		log.Fatal("Gagal konek ke database: ", err)
	}
	defer db.Close()
	fmt.Println("Terhubung ke database")

	jalankanMigrasi(context.Background())

	daftarkanAPI()

	// Jalankan cron job untuk hapus bahan kadaluarsa
	go jalankanCronJob()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)

	http.HandleFunc("/api/resep/bahan", resepBahanHandler)
	http.HandleFunc("/api/resep/racik", resepRacikHandler)
	http.HandleFunc("/api/resep/tersimpan", resepTersimpanHandler)
	http.HandleFunc("/api/resep/tersimpan/", hapusResepTersimpanHandler)

	http.HandleFunc("/api/pertumbuhan/analisis", wajibLoginAPI(analisisPertumbuhanHandler))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	http.HandleFunc("/dashboard", requireLogin(dashboardPage))
	http.HandleFunc("/profil", requireLogin(profilPage))
	http.HandleFunc("/bahan", requireLogin(bahanPanganPage))
	http.HandleFunc("/bahan-pangan", requireLogin(bahanPanganPage))
	http.HandleFunc("/bahan/tambah", requireLogin(tambahBahanPage))
	http.HandleFunc("/bahan/", requireLogin(aksiBahanPage))

	// FITUR RESEP (resep.go)
	// Daftar resep sudah digabung ke halaman /menu.
	// /resep diarahkan ke /menu; detail, tambah, edit & hapus tetap tersedia.
	http.HandleFunc("/resep", requireLogin(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/menu", http.StatusSeeOther)
	}))
	http.HandleFunc("/resep/tambah", requireLogin(tambahResepPage))
	http.HandleFunc("/resep/", requireLogin(aksiResepPage)) // detail

	http.HandleFunc("/menu", requireLogin(menuListPage))
	http.HandleFunc("/menu/tambah", requireLogin(tambahMenuPage))
	http.HandleFunc("/menu/", requireLogin(aksiMenuPage)) // detail, edit & hapus

	http.HandleFunc("/monitoring", requireLogin(monitoringPage))

	fmt.Println("Server berjalan di http://localhost:8080")
	if err := http.ListenAndServe(":8080", enableCORS(http.DefaultServeMux)); err != nil {
		fmt.Println("Server error:", err)
	}
}
