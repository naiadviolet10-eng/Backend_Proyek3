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

// Data masih dipakai tabel lain (pelanggaran foreign key, kode 23503).
// Dipakai untuk bahan yang dipakai resep dan resep yang dipakai menu.
func bahanDipakai(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}