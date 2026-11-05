package main

import (
	"net/http"
	"time"
)

// GET /api/health
func apiHealth(w http.ResponseWriter, r *http.Request) {
	kirimData(w, map[string]any{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}
