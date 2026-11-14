
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ResepTersimpan struct {
	ID        int64           `json:"id"`
	NamaResep string          `json:"nama_resep"`
	DataResep json.RawMessage `json:"data_resep"`
	CreatedAt time.Time       `json:"created_at"`
}

// GET dan POST /api/resep/tersimpan
func resepTersimpanHandler(w http.ResponseWriter, r *http.Request) {
	userID, err := verifikasiPengguna(r)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(r.Context(), `
			SELECT id, nama_resep, data_resep, created_at
			FROM public.resep_tersimpan
			WHERE user_id = $1
			ORDER BY created_at DESC
		`, userID)
		if err != nil {
			log.Printf("ERROR daftar resep tersimpan: %v", err)
			errorResponse(w, http.StatusInternalServerError,
				"Gagal mengambil resep tersimpan")
			return
		}
		defer rows.Close()

		items := make([]ResepTersimpan, 0)

		for rows.Next() {
			var item ResepTersimpan
			if err := rows.Scan(
				&item.ID,
				&item.NamaResep,
				&item.DataResep,
				&item.CreatedAt,
			); err != nil {
				log.Printf("ERROR membaca resep tersimpan: %v", err)
				errorResponse(w, http.StatusInternalServerError,
					"Gagal membaca resep tersimpan")
				return
			}
			items = append(items, item)
		}

		if err := rows.Err(); err != nil {
			log.Printf("ERROR hasil resep tersimpan: %v", err)
			errorResponse(w, http.StatusInternalServerError,
				"Gagal membaca hasil resep")
			return
		}

		jsonResponse(w, http.StatusOK, map[string]any{
			"items": items,
		})

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var input struct {
			NamaResep string          `json:"nama_resep"`
			DataResep json.RawMessage `json:"data_resep"`
		}

		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			errorResponse(w, http.StatusBadRequest,
				"Data resep tidak valid")
			return
		}

		input.NamaResep = strings.TrimSpace(input.NamaResep)

		if input.NamaResep == "" || len(input.DataResep) == 0 ||
			!json.Valid(input.DataResep) {
			errorResponse(w, http.StatusBadRequest,
				"Nama atau data resep belum valid")
			return
		}

		// Pastikan data yang disimpan benar-benar berbentuk objek JSON.
		var data map[string]any
		if err := json.Unmarshal(input.DataResep, &data); err != nil || data == nil {
			errorResponse(w, http.StatusBadRequest,
				"Data resep harus berupa objek JSON")
			return
		}

		var item ResepTersimpan
		err := db.QueryRow(r.Context(), `
			INSERT INTO public.resep_tersimpan
				(user_id, nama_resep, data_resep)
			VALUES ($1, $2, $3::jsonb)
			RETURNING id, nama_resep, data_resep, created_at
		`, userID, input.NamaResep, string(input.DataResep)).Scan(
			&item.ID,
			&item.NamaResep,
			&item.DataResep,
			&item.CreatedAt,
		)
		if err != nil {
			log.Printf("ERROR menyimpan resep: %v", err)
			errorResponse(w, http.StatusInternalServerError,
				"Gagal menyimpan resep")
			return
		}

		jsonResponse(w, http.StatusCreated, map[string]any{
			"message": "Resep berhasil disimpan",
			"item":    item,
		})

	default:
		w.Header().Set("Allow", "GET, POST")
		errorResponse(w, http.StatusMethodNotAllowed,
			"Metode tidak diizinkan")
	}
}

// DELETE /api/resep/tersimpan/{id}
func hapusResepTersimpanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		errorResponse(w, http.StatusMethodNotAllowed,
			"Metode tidak diizinkan")
		return
	}

	userID, err := verifikasiPengguna(r)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}

	idText := strings.TrimPrefix(
		r.URL.Path,
		"/api/resep/tersimpan/",
	)
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		errorResponse(w, http.StatusBadRequest, "ID resep tidak valid")
		return
	}

	tag, err := db.Exec(r.Context(), `
		DELETE FROM public.resep_tersimpan
		WHERE id = $1 AND user_id = $2
	`, id, userID)
	if err != nil {
		log.Printf("ERROR menghapus resep tersimpan: %v", err)
		errorResponse(w, http.StatusInternalServerError,
			"Gagal menghapus resep")
		return
	}

	if tag.RowsAffected() == 0 {
		errorResponse(w, http.StatusNotFound,
			"Resep tidak ditemukan")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Resep berhasil dihapus",
	})
}