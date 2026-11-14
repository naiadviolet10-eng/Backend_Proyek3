package main

import (
	"fmt"
	"html/template"
	"net/http"
	"time"
)

// =========================
// HALAMAN DASHBOARD
// =========================

func dashboardPage(w http.ResponseWriter, r *http.Request) {

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		http.Error(w, "Zona waktu tidak ditemukan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now().In(loc)

	bulanIndonesia := map[time.Month]string{
		time.January:   "Januari",
		time.February:  "Februari",
		time.March:     "Maret",
		time.April:     "April",
		time.May:       "Mei",
		time.June:      "Juni",
		time.July:      "Juli",
		time.August:    "Agustus",
		time.September: "September",
		time.October:   "Oktober",
		time.November:  "November",
		time.December:  "Desember",
	}

	data := struct {
		Tanggal string
		Waktu   string
	}{
		Tanggal: fmt.Sprintf("%02d %s %d", now.Day(), bulanIndonesia[now.Month()], now.Year()),
		Waktu:   now.Format("15:04:05"),
	}

	tmpl, err := template.ParseFiles("templates/dashboard.html")
	if err != nil {
		http.Error(w, "Template dashboard error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Gagal menampilkan halaman dashboard: "+err.Error(), http.StatusInternalServerError)
	}
}