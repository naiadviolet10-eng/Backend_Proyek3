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

	fmt.Println("Server berjalan di http://localhost:8080")
	if err := http.ListenAndServe(":8080", enableCORS(http.DefaultServeMux)); err != nil {
		fmt.Println("Server error:", err)
	}
}
