package main

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func dbSemuaMenu(ctx context.Context) ([]Menu, error) {
	rows, err := db.Query(ctx, `
		SELECT id, nama, COALESCE(deskripsi,''),
		       COALESCE(to_char(tanggal,'YYYY-MM-DD'),''), COALESCE(waktu_makan,'')
		FROM menu ORDER BY tanggal DESC NULLS LAST, id DESC`)
	if err != nil {
		return nil, err
	}

	var hasil []Menu
	posisi := map[int]int{}
	for rows.Next() {
		var m Menu
		if err := rows.Scan(&m.ID, &m.Nama, &m.Deskripsi, &m.Tanggal, &m.WaktuMakan); err != nil {
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

	mr, err := db.Query(ctx, `SELECT menu_id, resep_id, porsi FROM menu_resep ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer mr.Close()

	for mr.Next() {
		var mid int
		var rm ResepMenu
		if err := mr.Scan(&mid, &rm.ResepID, &rm.Porsi); err != nil {
			return nil, err
		}
		if i, ok := posisi[mid]; ok {
			hasil[i].Resep = append(hasil[i].Resep, rm)
		}
	}
	return hasil, mr.Err()
}

func dbMenuByID(ctx context.Context, id int) (Menu, error) {
	var m Menu
	err := db.QueryRow(ctx, `
		SELECT id, nama, COALESCE(deskripsi,''),
		       COALESCE(to_char(tanggal,'YYYY-MM-DD'),''), COALESCE(waktu_makan,'')
		FROM menu WHERE id=$1`, id).
		Scan(&m.ID, &m.Nama, &m.Deskripsi, &m.Tanggal, &m.WaktuMakan)
	if err != nil {
		return m, err
	}

	rows, err := db.Query(ctx, `SELECT resep_id, porsi FROM menu_resep WHERE menu_id=$1 ORDER BY id`, id)
	if err != nil {
		return m, err
	}
	defer rows.Close()

	for rows.Next() {
		var rm ResepMenu
		if err := rows.Scan(&rm.ResepID, &rm.Porsi); err != nil {
			return m, err
		}
		m.Resep = append(m.Resep, rm)
	}
	return m, rows.Err()
}

func sisipResepMenu(ctx context.Context, tx pgx.Tx, menuID int, resep []ResepMenu) error {
	for _, r := range resep {
		if _, err := tx.Exec(ctx,
			`INSERT INTO menu_resep (menu_id, resep_id, porsi) VALUES ($1,$2,$3)`,
			menuID, r.ResepID, r.Porsi); err != nil {
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
		INSERT INTO menu (nama, deskripsi, tanggal, waktu_makan)
		VALUES ($1,$2,NULLIF($3,'')::date,$4) RETURNING id`,
		m.Nama, m.Deskripsi, m.Tanggal, m.WaktuMakan).Scan(&id)
	if err != nil {
		return err
	}
	if err := sisipResepMenu(ctx, tx, id, m.Resep); err != nil {
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
		UPDATE menu SET nama=$1, deskripsi=$2, tanggal=NULLIF($3,'')::date, waktu_makan=$4
		WHERE id=$5`, m.Nama, m.Deskripsi, m.Tanggal, m.WaktuMakan, m.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	if _, err := tx.Exec(ctx, `DELETE FROM menu_resep WHERE menu_id=$1`, m.ID); err != nil {
		return err
	}
	if err := sisipResepMenu(ctx, tx, m.ID, m.Resep); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func dbHapusMenu(ctx context.Context, id int) error {
	_, err := db.Exec(ctx, `DELETE FROM menu WHERE id=$1`, id)
	return err
}