package main

import (
	"html/template"
	"net/http"
)

// =========================
// HALAMAN MONITORING
// =========================

func monitoringPage(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("templates/monitoring.html")
	if err != nil {
		http.Error(w, "Template monitoring error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Gagal menampilkan halaman monitoring: "+err.Error(), http.StatusInternalServerError)
	}
}