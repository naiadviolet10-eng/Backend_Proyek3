package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"
)

type Aktivitas struct {
	Ikon       string
	Judul      string
	Keterangan string
	Waktu      string
}

type Ringkasan struct {
	TotalBahan     int
	TotalResep     int
	TotalMenu      int
	MenuTanpaBahan int // menu yang belum punya bahan
	MenuTanpaResep int // menu yang belum punya resep dari AI
	Aktivitas      []Aktivitas
}

// Dipanggil langsung dari template: {{.PerluPerhatian}}
func (r Ringkasan) PerluPerhatian() int {
	return r.MenuTanpaBahan + r.MenuTanpaResep
}

// =========================
// HELPER
// =========================

// Mencatat aktivitas admin. Kegagalan hanya dicatat di log,
// supaya proses utama (simpan/hapus data) tidak ikut gagal.
func catatAktivitas(ctx context.Context, jenis, aksi, nama string) {
	if nama == "" {
		nama = "(tanpa nama)"
	}
	_, err := db.Exec(ctx,
		`INSERT INTO aktivitas (jenis, aksi, nama) VALUES ($1,$2,$3)`,
		jenis, aksi, nama)
	if err != nil {
		log.Println("Gagal mencatat aktivitas:", err)
	}
}

func hitung(ctx context.Context, query string) int {
	var n int
	if err := db.QueryRow(ctx, query).Scan(&n); err != nil {
		log.Println("Gagal menghitung ringkasan dashboard:", err)
		return 0
	}
	return n
}

func waktuLalu(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "Baru saja"
	case d < time.Hour:
		return fmt.Sprintf("%d menit lalu", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d jam lalu", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d hari lalu", int(d.Hours()/24))
	default:
		return t.Format("02-01-2006")
	}
}

// Ikon dan teks untuk tiap jenis aktivitas
func tampilanAktivitas(jenis, aksi, nama string) (ikon, judul, ket string) {
	objek := map[string]string{"bahan": "Bahan pangan", "menu": "Menu"}[jenis]
	ikon = map[string]string{"bahan": "🌿", "menu": "🍽️"}[jenis]
	if objek == "" {
		objek, ikon = jenis, "📌"
	}

	switch aksi {
	case "tambah":
		return ikon, objek + " ditambahkan", fmt.Sprintf("Admin menambahkan \"%s\"", nama)
	case "ubah":
		return ikon, objek + " diperbarui", fmt.Sprintf("Admin memperbarui \"%s\"", nama)
	case "hapus":
		return "🗑️", objek + " dihapus", fmt.Sprintf("Admin menghapus \"%s\"", nama)
	}
	return ikon, objek, nama
}

// =========================
// RINGKASAN DASHBOARD
// =========================

func dbRingkasan(ctx context.Context) Ringkasan {
	var r Ringkasan

	r.TotalBahan = hitung(ctx, `SELECT count(*) FROM ingredients_master`)
	r.TotalResep = hitung(ctx, `SELECT count(*) FROM resep`)
	r.TotalMenu = hitung(ctx, `SELECT count(*) FROM menu`)

	r.MenuTanpaBahan = hitung(ctx, `
		SELECT count(*) FROM menu m
		WHERE NOT EXISTS (SELECT 1 FROM menu_bahan mb WHERE mb.menu_id = m.id)`)

	r.MenuTanpaResep = hitung(ctx, `
		SELECT count(*) FROM menu m
		WHERE NOT EXISTS (SELECT 1 FROM resep rs WHERE rs.menu_id = m.id)`)

	r.Aktivitas = dbAktivitas(ctx)
	return r
}

type barisAktivitas struct {
	a Aktivitas
	t time.Time
}

func dbAktivitas(ctx context.Context) []Aktivitas {
	var semua []barisAktivitas

	// 1) Catatan aktivitas admin (tambah, ubah, hapus)
	rows, err := db.Query(ctx, `
		SELECT jenis, aksi, nama, created_at
		FROM aktivitas ORDER BY created_at DESC LIMIT 8`)
	if err != nil {
		log.Println("Gagal membaca aktivitas:", err)
	} else {
		for rows.Next() {
			var jenis, aksi, nama string
			var t time.Time
			if err := rows.Scan(&jenis, &aksi, &nama, &t); err != nil {
				break
			}
			ikon, judul, ket := tampilanAktivitas(jenis, aksi, nama)
			semua = append(semua, barisAktivitas{
				a: Aktivitas{Ikon: ikon, Judul: judul, Keterangan: ket, Waktu: waktuLalu(t)},
				t: t,
			})
		}
		rows.Close()
	}

	// 2) Resep yang masuk dari AI (tidak lewat aplikasi, jadi dibaca dari tabel resep)
	rr, err := db.Query(ctx, `SELECT nama, created_at FROM resep ORDER BY created_at DESC LIMIT 8`)
	if err != nil {
		log.Println("Gagal membaca resep terbaru:", err)
	} else {
		for rr.Next() {
			var nama string
			var t time.Time
			if err := rr.Scan(&nama, &t); err != nil {
				break
			}
			semua = append(semua, barisAktivitas{
				a: Aktivitas{
					Ikon:       "🍲",
					Judul:      "Resep baru dari AI",
					Keterangan: fmt.Sprintf("Resep \"%s\" ditambahkan", nama),
					Waktu:      waktuLalu(t),
				},
				t: t,
			})
		}
		rr.Close()
	}

	// Gabung dan urutkan dari yang terbaru, ambil 6
	sort.Slice(semua, func(i, j int) bool { return semua[i].t.After(semua[j].t) })
	if len(semua) > 6 {
		semua = semua[:6]
	}

	hasil := make([]Aktivitas, 0, len(semua))
	for _, b := range semua {
		hasil = append(hasil, b.a)
	}
	return hasil
}