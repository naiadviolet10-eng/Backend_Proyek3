package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const kolomBahanAPI = `
	i.id, i.category_id, i.name, i.default_unit,
	i.calories_per_100g::float8, i.protein_g::float8, i.fat_g::float8, i.carbs_g::float8,
	i.iron_mg::float8, i.zinc_mg::float8, i.vitamin_a_mcg::float8, i.vitamin_c_mg::float8,
	i.image_path, c.name`

func pindaiBahanAPI(r *http.Request, row pgx.Row) (bahanJSON, error) {
	var b bahanJSON
	var imagePath, namaKategori *string
	err := row.Scan(&b.ID, &b.CategoryID, &b.Name, &b.DefaultUnit,
		&b.CaloriesPer100g, &b.ProteinG, &b.FatG, &b.CarbsG,
		&b.IronMg, &b.ZincMg, &b.VitaminAMcg, &b.VitaminCMcg,
		&imagePath, &namaKategori)
	if err != nil {
		return b, err
	}
	if namaKategori != nil {
		b.Category = &kategoriRingkas{ID: b.CategoryID, Name: *namaKategori}
	}
	b.ImageURL = urlGambar(r, imagePath)
	return b, nil
}

func ambilBahanAPI(ctx context.Context, r *http.Request, id int64) (bahanJSON, error) {
	row := db.QueryRow(ctx, `SELECT `+kolomBahanAPI+`
		FROM ingredients_master i
		LEFT JOIN ingredient_categories c ON c.id = i.category_id
		WHERE i.id = $1`, id)
	return pindaiBahanAPI(r, row)
}

// GET /api/categories
func apiKategori(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(r.Context(), `SELECT id, name FROM ingredient_categories ORDER BY name`)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat kategori")
		return
	}
	defer rows.Close()

	hasil := []kategoriRingkas{}
	for rows.Next() {
		var k kategoriRingkas
		if err := rows.Scan(&k.ID, &k.Name); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal membaca kategori")
			return
		}
		hasil = append(hasil, k)
	}
	kirimData(w, hasil)
}

// GET /api/ingredients?q=&category_id=
func apiDaftarBahan(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	kategori := int64(0)
	if v, err := strconv.ParseInt(r.URL.Query().Get("category_id"), 10, 64); err == nil {
		kategori = v
	}

	rows, err := db.Query(r.Context(), `SELECT `+kolomBahanAPI+`
		FROM ingredients_master i
		LEFT JOIN ingredient_categories c ON c.id = i.category_id
		WHERE ($1 = '' OR i.name ILIKE '%' || $1 || '%')
		  AND ($2 = 0 OR i.category_id = $2)
		ORDER BY i.name`, q, kategori)
	if err != nil {
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat bahan")
		return
	}
	defer rows.Close()

	hasil := []bahanJSON{}
	for rows.Next() {
		b, err := pindaiBahanAPI(r, rows)
		if err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal membaca bahan")
			return
		}
		hasil = append(hasil, b)
	}
	kirimData(w, hasil)
}

// GET /api/ingredients/{id}
func apiDetailBahan(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		kirimPesan(w, http.StatusNotFound, "Bahan tidak ditemukan")
		return
	}
	b, err := ambilBahanAPI(r.Context(), r, id)
	if err != nil {
		if tidakAda(err) {
			kirimPesan(w, http.StatusNotFound, "Bahan tidak ditemukan")
			return
		}
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat bahan")
		return
	}
	kirimData(w, b)
}

// GET /api/ingredients/image/{file}
func apiGambarBahan(w http.ResponseWriter, r *http.Request) {
	file := filepath.Base(r.PathValue("file"))
	if file == "." || file == "" || file == string(filepath.Separator) {
		http.NotFound(w, r)
		return
	}
	lokasi := filepath.Join("static", "ingredients", file)
	if _, err := os.Stat(lokasi); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, lokasi)
}
