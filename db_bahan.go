package main

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Kolom yang dibaca dari tabel ingredients_master (alias m)
// dan nama kategori dari ingredient_categories (alias c).
// Angka numeric diubah ke float8 supaya mudah dibaca ke float64.
const kolomBahan = `m.id, m.category_id, COALESCE(c.name,''), m.name, m.default_unit,
	m.calories_per_100g::float8, m.protein_g::float8, m.fat_g::float8, m.carbs_g::float8,
	m.iron_mg::float8, m.zinc_mg::float8, m.vitamin_a_mcg::float8, m.vitamin_c_mg::float8`

type pemindai interface {
	Scan(dest ...any) error
}

func pindaiBahan(row pemindai) (Bahan, error) {
	var b Bahan
	err := row.Scan(
		&b.ID, &b.KategoriID, &b.Kategori, &b.NamaBahan, &b.Satuan,
		&b.Kalori, &b.Protein, &b.Lemak, &b.Karbohidrat,
		&b.ZatBesi, &b.Seng, &b.VitaminA, &b.VitaminC,
	)
	return b, err
}

// Urutan harus sama dengan kolom di INSERT/UPDATE (11 nilai)
func argsBahan(b Bahan) []any {
	return []any{
		b.KategoriID, b.NamaBahan, b.Satuan,
		b.Kalori, b.Protein, b.Lemak, b.Karbohidrat,
		b.ZatBesi, b.Seng, b.VitaminA, b.VitaminC,
	}
}

// Daftar kategori untuk dropdown di form
func dbSemuaKategori(ctx context.Context) ([]Kategori, error) {
	rows, err := db.Query(ctx, `SELECT id, name FROM ingredient_categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hasil []Kategori
	for rows.Next() {
		var k Kategori
		if err := rows.Scan(&k.ID, &k.Nama); err != nil {
			return nil, err
		}
		hasil = append(hasil, k)
	}
	return hasil, rows.Err()
}

func dbSemuaBahan(ctx context.Context) ([]Bahan, error) {
	rows, err := db.Query(ctx, `
		SELECT `+kolomBahan+`
		FROM ingredients_master m
		JOIN ingredient_categories c ON c.id = m.category_id
		ORDER BY m.name`)
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
	return pindaiBahan(db.QueryRow(ctx, `
		SELECT `+kolomBahan+`
		FROM ingredients_master m
		JOIN ingredient_categories c ON c.id = m.category_id
		WHERE m.id = $1`, id))
}

func dbTambahBahan(ctx context.Context, b Bahan) error {
	_, err := db.Exec(ctx, `
		INSERT INTO ingredients_master (
			category_id, name, default_unit,
			calories_per_100g, protein_g, fat_g, carbs_g,
			iron_mg, zinc_mg, vitamin_a_mcg, vitamin_c_mg
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		argsBahan(b)...)
	return err
}

func dbUbahBahan(ctx context.Context, b Bahan) error {
	args := append(argsBahan(b), b.ID)
	tag, err := db.Exec(ctx, `
		UPDATE ingredients_master SET
			category_id=$1, name=$2, default_unit=$3,
			calories_per_100g=$4, protein_g=$5, fat_g=$6, carbs_g=$7,
			iron_mg=$8, zinc_mg=$9, vitamin_a_mcg=$10, vitamin_c_mg=$11
		WHERE id=$12`, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func dbHapusBahan(ctx context.Context, id int) error {
	_, err := db.Exec(ctx, `DELETE FROM ingredients_master WHERE id=$1`, id)
	return err
}