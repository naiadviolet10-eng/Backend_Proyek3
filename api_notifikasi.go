package main

import (
	"context"
	"net/http"
	"time"
)

type notifikasiJSON struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	IsRead    bool   `json:"is_read"`
	CreatedAt string `json:"created_at"`
}

// GET /api/notifications
func apiNotifikasi(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())

	rows, err := db.Query(r.Context(), `
		SELECT id::text, user_id::text, title, message, type, is_read, created_at::text
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 50
	`, u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat notifikasi")
		return
	}
	defer rows.Close()

	var list []notifikasiJSON
	for rows.Next() {
		var n notifikasiJSON
		if err := rows.Scan(&n.ID, &n.UserID, &n.Title, &n.Message, &n.Type, &n.IsRead, &n.CreatedAt); err != nil {
			continue
		}
		list = append(list, n)
	}
	if list == nil {
		list = []notifikasiJSON{}
	}
	kirimJSON(w, http.StatusOK, map[string]any{"success": true, "data": list})
}

// PUT /api/notifications/{id}/read
func apiTandaiDibaca(w http.ResponseWriter, r *http.Request) {
	u, _ := penggunaKonteks(r.Context())
	id := r.PathValue("id")

	_, err := db.Exec(r.Context(), `
		UPDATE notifications
		SET is_read = true
		WHERE id = $1 AND user_id = $2
	`, id, u.ID)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal menandai notifikasi")
		return
	}
	kirimJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Fungsi helper untuk membuat notifikasi
func buatNotifikasi(userID, title, message, tipe string) error {
	_, err := db.Exec(context.Background(), `
		INSERT INTO notifications (user_id, title, message, type)
		VALUES ($1, $2, $3, $4)
	`, userID, title, message, tipe)
	return err
}

// Cron job: hapus bahan expired >3 hari dan kirim notifikasi
func hapusBahanKadaluarsa() error {
	// Tanggal 3 hari yang lalu dari expired_date
	cutoff := time.Now().AddDate(0, 0, -3).Format("2006-01-02")

	// Ambil bahan yang akan dihapus
	rows, err := db.Query(context.Background(), `
		SELECT s.id::text, s.user_id::text, i.nama, s.expired_date::text
		FROM ingredients_stock s
		JOIN ingredients i ON i.id = s.ingredient_id
		WHERE s.expired_date IS NOT NULL 
		  AND s.expired_date < $1::date
	`, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()

	type stokHapus struct {
		id         string
		userID     string
		namaBahan  string
		expiredAt  string
	}
	var toDelete []stokHapus
	for rows.Next() {
		var s stokHapus
		if err := rows.Scan(&s.id, &s.userID, &s.namaBahan, &s.expiredAt); err != nil {
			continue
		}
		toDelete = append(toDelete, s)
	}

	// Hapus dan buat notifikasi
	for _, s := range toDelete {
		_, err := db.Exec(context.Background(), `
			DELETE FROM ingredients_stock WHERE id = $1
		`, s.id)
		if err != nil {
			continue
		}

		// Buat notifikasi
		title := "Bahan otomatis dihapus"
		message := s.namaBahan + " sudah lebih dari 3 hari melewati tanggal kadaluarsa (" + s.expiredAt + ") dan telah dihapus otomatis dari stok."
		_ = buatNotifikasi(s.userID, title, message, "warning")
	}

	return nil
}
