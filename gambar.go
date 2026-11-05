package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	folderGambar = "static/uploads/bahan"   // lokasi file di komputer
	urlGambar    = "/static/uploads/bahan/" // alamat yang disimpan di database
	maksGambar   = 2 << 20                  // 2 MB
)

// Jenis file yang diizinkan (dicek dari isi file, bukan dari nama)
var ekstensiGambar = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// Menyimpan file dari form dan mengembalikan alamatnya (untuk database).
// Hasil "" tanpa error berarti tidak ada file yang diunggah.
func simpanGambar(r *http.Request, field string) (string, error) {

	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	if header.Size > maksGambar {
		return "", errors.New("Ukuran gambar maksimal 2 MB")
	}

	// Cek jenis file dari 512 byte pertama
	buf := make([]byte, 512)
	n, _ := io.ReadFull(file, buf)

	ext, ok := ekstensiGambar[http.DetectContentType(buf[:n])]
	if !ok {
		return "", errors.New("Format gambar harus JPG, PNG, WEBP, atau GIF")
	}

	// Kembali ke awal file sebelum disalin
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if err := os.MkdirAll(folderGambar, 0o755); err != nil {
		return "", err
	}

	// Nama acak supaya tidak bentrok dan tidak bisa ditebak
	acak := make([]byte, 8)
	if _, err := rand.Read(acak); err != nil {
		return "", err
	}
	nama := hex.EncodeToString(acak) + ext

	dst, err := os.Create(filepath.Join(folderGambar, nama))
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", err
	}

	return urlGambar + nama, nil
}

// Menghapus file foto dari folder upload.
// Alamat di luar folder upload diabaikan, jadi file lain aman.
func hapusFileGambar(url string) {
	if !strings.HasPrefix(url, urlGambar) {
		return
	}
	os.Remove(filepath.Join(folderGambar, filepath.Base(url)))
}