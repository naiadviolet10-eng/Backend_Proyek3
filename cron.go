package main

import (
	"log"
	"time"
)

// Jalankan cron job untuk hapus bahan kadaluarsa setiap hari jam 3 pagi
func jalankanCronJob() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Jalankan pertama kali saat startup
	go func() {
		log.Println("Menjalankan cron job: hapus bahan kadaluarsa...")
		if err := hapusBahanKadaluarsa(); err != nil {
			log.Printf("Error cron job: %v\n", err)
		} else {
			log.Println("Cron job selesai")
		}
	}()

	// Lalu jalankan setiap 24 jam
	for range ticker.C {
		log.Println("Menjalankan cron job: hapus bahan kadaluarsa...")
		if err := hapusBahanKadaluarsa(); err != nil {
			log.Printf("Error cron job: %v\n", err)
		} else {
			log.Println("Cron job selesai")
		}
	}
}
