package main

import (
	"context"

	"github.com/jackc/pgx/v5"
)

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