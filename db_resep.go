package main

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func dbSemuaResep(ctx context.Context) ([]Resep, error) {
	rows, err := db.Query(ctx,
		`SELECT id, nama, COALESCE(deskripsi,''), porsi, COALESCE(langkah,'') FROM resep ORDER BY id`)
	if err != nil {
		return nil, err
	}

	var hasil []Resep
	posisi := map[int]int{}
	for rows.Next() {
		var r Resep
		if err := rows.Scan(&r.ID, &r.Nama, &r.Deskripsi, &r.Porsi, &r.Langkah); err != nil {
			rows.Close()
			return nil, err
		}
		posisi[r.ID] = len(hasil)
		hasil = append(hasil, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rb, err := db.Query(ctx, `SELECT resep_id, bahan_id, jumlah FROM resep_bahan ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rb.Close()

	for rb.Next() {
		var rid int
		var br BahanResep
		if err := rb.Scan(&rid, &br.BahanID, &br.Jumlah); err != nil {
			return nil, err
		}
		if i, ok := posisi[rid]; ok {
			hasil[i].Bahan = append(hasil[i].Bahan, br)
		}
	}
	return hasil, rb.Err()
}

func dbResepByID(ctx context.Context, id int) (Resep, error) {
	var r Resep
	err := db.QueryRow(ctx,
		`SELECT id, nama, COALESCE(deskripsi,''), porsi, COALESCE(langkah,'') FROM resep WHERE id=$1`, id).
		Scan(&r.ID, &r.Nama, &r.Deskripsi, &r.Porsi, &r.Langkah)
	if err != nil {
		return r, err
	}

	rows, err := db.Query(ctx,
		`SELECT bahan_id, jumlah FROM resep_bahan WHERE resep_id=$1 ORDER BY id`, id)
	if err != nil {
		return r, err
	}
	defer rows.Close()

	for rows.Next() {
		var br BahanResep
		if err := rows.Scan(&br.BahanID, &br.Jumlah); err != nil {
			return r, err
		}
		r.Bahan = append(r.Bahan, br)
	}
	return r, rows.Err()
}

func sisipBahanResep(ctx context.Context, tx pgx.Tx, resepID int, bahan []BahanResep) error {
	for _, b := range bahan {
		_, err := tx.Exec(ctx,
			`INSERT INTO resep_bahan (resep_id, bahan_id, jumlah) VALUES ($1,$2,$3)`,
			resepID, b.BahanID, b.Jumlah)
		if err != nil {
			return err
		}
	}
	return nil
}

func dbTambahResep(ctx context.Context, r Resep) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id int
	err = tx.QueryRow(ctx,
		`INSERT INTO resep (nama, deskripsi, porsi, langkah) VALUES ($1,$2,$3,$4) RETURNING id`,
		r.Nama, r.Deskripsi, r.Porsi, r.Langkah).Scan(&id)
	if err != nil {
		return err
	}
	if err := sisipBahanResep(ctx, tx, id, r.Bahan); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dbUbahResep(ctx context.Context, r Resep) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE resep SET nama=$1, deskripsi=$2, porsi=$3, langkah=$4 WHERE id=$5`,
		r.Nama, r.Deskripsi, r.Porsi, r.Langkah, r.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	// Ganti seluruh daftar bahan dengan yang baru
	if _, err := tx.Exec(ctx, `DELETE FROM resep_bahan WHERE resep_id=$1`, r.ID); err != nil {
		return err
	}
	if err := sisipBahanResep(ctx, tx, r.ID, r.Bahan); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dbHapusResep(ctx context.Context, id int) error {
	_, err := db.Exec(ctx, `DELETE FROM resep WHERE id=$1`, id)
	return err
}