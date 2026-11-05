package main

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Resep AI yang menempel ke sebuah menu
type ResepTaut struct {
	ResepID  int
	Nama     string
	MenuID   int
	MenuNama string
}

// Jumlah menu per kelompok, untuk empat kartu filter
func dbHitungMenu(ctx context.Context) (map[string]int, error) {
	rows, err := db.Query(ctx, `SELECT kelompok, COUNT(*) FROM menu GROUP BY kelompok`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hasil := make(map[string]int, len(daftarKelompok))
	for _, k := range daftarKelompok {
		hasil[k] = 0
	}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		hasil[k] = n
	}
	return hasil, rows.Err()
}

// Daftar menu. kelompok kosong = semua kelompok.
func dbSemuaMenu(ctx context.Context, kelompok string) ([]Menu, error) {
	rows, err := db.Query(ctx, `
		SELECT id, nama, COALESCE(deskripsi,''), kelompok,
		       COALESCE(sasaran_detail,''), porsi
		FROM menu
		WHERE ($1::text = '' OR kelompok = $1::text)
		ORDER BY nama`, kelompok)
	if err != nil {
		return nil, err
	}

	var hasil []Menu
	posisi := map[int]int{}
	for rows.Next() {
		var m Menu
		if err := rows.Scan(&m.ID, &m.Nama, &m.Deskripsi, &m.Kelompok, &m.SasaranDetail, &m.Porsi); err != nil {
			rows.Close()
			return nil, err
		}
		posisi[m.ID] = len(hasil)
		hasil = append(hasil, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// bahan dari semua menu yang termasuk filter, diambil sekaligus
	mb, err := db.Query(ctx, `
		SELECT mb.menu_id, mb.bahan_id, mb.jumlah
		FROM menu_bahan mb
		JOIN menu m ON m.id = mb.menu_id
		WHERE ($1::text = '' OR m.kelompok = $1::text)
		ORDER BY mb.id`, kelompok)
	if err != nil {
		return nil, err
	}
	defer mb.Close()

	for mb.Next() {
		var menuID int
		var bm BahanMenu
		if err := mb.Scan(&menuID, &bm.BahanID, &bm.Jumlah); err != nil {
			return nil, err
		}
		if i, ok := posisi[menuID]; ok {
			hasil[i].Isi = append(hasil[i].Isi, bm)
		}
	}
	return hasil, mb.Err()
}

func dbMenuByID(ctx context.Context, id int) (Menu, error) {
	var m Menu
	err := db.QueryRow(ctx, `
		SELECT id, nama, COALESCE(deskripsi,''), kelompok,
		       COALESCE(sasaran_detail,''), porsi
		FROM menu WHERE id=$1`, id).
		Scan(&m.ID, &m.Nama, &m.Deskripsi, &m.Kelompok, &m.SasaranDetail, &m.Porsi)
	if err != nil {
		return m, err
	}

	rows, err := db.Query(ctx, `SELECT bahan_id, jumlah FROM menu_bahan WHERE menu_id=$1 ORDER BY id`, id)
	if err != nil {
		return m, err
	}
	defer rows.Close()

	for rows.Next() {
		var bm BahanMenu
		if err := rows.Scan(&bm.BahanID, &bm.Jumlah); err != nil {
			return m, err
		}
		m.Isi = append(m.Isi, bm)
	}
	return m, rows.Err()
}

func sisipBahanMenu(ctx context.Context, tx pgx.Tx, menuID int, isi []BahanMenu) error {
	for _, b := range isi {
		if _, err := tx.Exec(ctx,
			`INSERT INTO menu_bahan (menu_id, bahan_id, jumlah) VALUES ($1,$2,$3)`,
			menuID, b.BahanID, b.Jumlah); err != nil {
			return err
		}
	}
	return nil
}

func dbTambahMenu(ctx context.Context, m Menu) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx, `
		INSERT INTO menu (nama, deskripsi, kelompok, sasaran_detail, porsi)
		VALUES ($1,$2,$3,NULLIF($4,''),$5) RETURNING id`,
		m.Nama, m.Deskripsi, m.Kelompok, m.SasaranDetail, m.Porsi).Scan(&id)
	if err != nil {
		return err
	}
	if err := sisipBahanMenu(ctx, tx, id, m.Isi); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dbUbahMenu(ctx context.Context, m Menu) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE menu
		SET nama=$1, deskripsi=$2, kelompok=$3, sasaran_detail=NULLIF($4,''), porsi=$5
		WHERE id=$6`,
		m.Nama, m.Deskripsi, m.Kelompok, m.SasaranDetail, m.Porsi, m.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	if _, err := tx.Exec(ctx, `DELETE FROM menu_bahan WHERE menu_id=$1`, m.ID); err != nil {
		return err
	}
	if err := sisipBahanMenu(ctx, tx, m.ID, m.Isi); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dbHapusMenu(ctx context.Context, id int) error {
	// menu_bahan ikut terhapus (ON DELETE CASCADE),
	// resep AI tetap ada dan menu_id-nya menjadi NULL (ON DELETE SET NULL)
	_, err := db.Exec(ctx, `DELETE FROM menu WHERE id=$1`, id)
	return err
}

// Semua resep yang sudah menempel ke sebuah menu (dibuat AI, hanya dibaca)
func dbResepTaut(ctx context.Context) ([]ResepTaut, error) {
	rows, err := db.Query(ctx, `
		SELECT r.id, r.nama, r.menu_id, m.nama
		FROM resep r
		JOIN menu m ON m.id = r.menu_id
		ORDER BY r.nama`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hasil []ResepTaut
	for rows.Next() {
		var t ResepTaut
		if err := rows.Scan(&t.ResepID, &t.Nama, &t.MenuID, &t.MenuNama); err != nil {
			return nil, err
		}
		hasil = append(hasil, t)
	}
	return hasil, rows.Err()
}