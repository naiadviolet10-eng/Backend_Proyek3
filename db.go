package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

// Membuka koneksi ke Supabase (PostgreSQL)
func initDB() error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL belum diatur (cek file .env)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return err
	}
	db = pool
	return nil
}

// Data tidak ditemukan
func tidakAda(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// Bahan masih dipakai resep (pelanggaran foreign key)
func bahanDipakai(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// =========================
// BAHAN PANGAN
// =========================

const kolomBahan = `id, nama_bahan, COALESCE(kategori,''), COALESCE(satuan,''),
	berat_per_satuan, bdd, kalori, protein, lemak, karbohidrat, serat, gula,
	natrium, kolesterol, kalsium, zat_besi, kalium, vitamin_a, vitamin_c,
	alergen, COALESCE(sumber_data,'')`

type pemindai interface {
	Scan(dest ...any) error
}

func pindaiBahan(row pemindai) (Bahan, error) {
	var b Bahan
	err := row.Scan(
		&b.ID, &b.NamaBahan, &b.Kategori, &b.Satuan,
		&b.BeratPerSatuan, &b.BDD, &b.Kalori, &b.Protein, &b.Lemak,
		&b.Karbohidrat, &b.Serat, &b.Gula, &b.Natrium, &b.Kolesterol,
		&b.Kalsium, &b.ZatBesi, &b.Kalium, &b.VitaminA, &b.VitaminC,
		&b.Alergen, &b.SumberData,
	)
	return b, err
}

// Urutan harus sama dengan kolom di INSERT/UPDATE (20 nilai)
func argsBahan(b Bahan) []any {
	if b.Alergen == nil {
		b.Alergen = []string{} // kolom alergen NOT NULL
	}
	return []any{
		b.NamaBahan, b.Kategori, b.Satuan, b.BeratPerSatuan, b.BDD,
		b.Kalori, b.Protein, b.Lemak, b.Karbohidrat, b.Serat, b.Gula,
		b.Natrium, b.Kolesterol, b.Kalsium, b.ZatBesi, b.Kalium,
		b.VitaminA, b.VitaminC, b.Alergen, b.SumberData,
	}
}

func dbSemuaBahan(ctx context.Context) ([]Bahan, error) {
	rows, err := db.Query(ctx, `SELECT `+kolomBahan+` FROM bahan_pangan ORDER BY nama_bahan`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hasil []Bahan
	for rows.Next() {
		b, err := pindaiBahan(rows)
		if err != nil {
			return nil, err
		}
		hasil = append(hasil, b)
	}
	return hasil, rows.Err()
}

func dbBahanByID(ctx context.Context, id int) (Bahan, error) {
	return pindaiBahan(db.QueryRow(ctx, `SELECT `+kolomBahan+` FROM bahan_pangan WHERE id=$1`, id))
}

func dbTambahBahan(ctx context.Context, b Bahan) error {
	_, err := db.Exec(ctx, `
		INSERT INTO bahan_pangan (
			nama_bahan, kategori, satuan, berat_per_satuan, bdd,
			kalori, protein, lemak, karbohidrat, serat, gula,
			natrium, kolesterol, kalsium, zat_besi, kalium,
			vitamin_a, vitamin_c, alergen, sumber_data
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		argsBahan(b)...)
	return err
}

func dbUbahBahan(ctx context.Context, b Bahan) error {
	args := append(argsBahan(b), b.ID)
	tag, err := db.Exec(ctx, `
		UPDATE bahan_pangan SET
			nama_bahan=$1, kategori=$2, satuan=$3, berat_per_satuan=$4, bdd=$5,
			kalori=$6, protein=$7, lemak=$8, karbohidrat=$9, serat=$10, gula=$11,
			natrium=$12, kolesterol=$13, kalsium=$14, zat_besi=$15, kalium=$16,
			vitamin_a=$17, vitamin_c=$18, alergen=$19, sumber_data=$20
		WHERE id=$21`, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func dbHapusBahan(ctx context.Context, id int) error {
	_, err := db.Exec(ctx, `DELETE FROM bahan_pangan WHERE id=$1`, id)
	return err
}

// =========================
// RESEP
// =========================

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