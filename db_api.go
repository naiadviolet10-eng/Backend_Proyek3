package main

import (
	"context"
	"time"
)

// pastikanPengguna menyisipkan baris di public.users untuk setiap pengguna
// Supabase yang baru terautentikasi. Tabel stok memakai foreign key ke sini.
func pastikanPengguna(ctx context.Context, u pengguna) error {
	_, err := db.Exec(ctx, `
		INSERT INTO public.users (id, nama, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
			SET email = EXCLUDED.email,
			    nama = COALESCE(NULLIF(EXCLUDED.nama, ''), public.users.nama)`,
		u.ID, u.Nama, u.Email)
	return err
}

// tanggalNull mengubah *time.Time menjadi string YYYY-MM-DD atau nil.
func tanggalNull(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}
