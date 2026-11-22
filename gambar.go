package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	folderGambar   = "static/uploads/bahan"   // lokasi file lokal (dev)
	prefixGambar   = "/static/uploads/bahan/" // prefix URL lokal
	bucketSupabase = "Bahan Pangan"           // nama bucket di Supabase Storage
	maksGambar     = 2 << 20                  // 2 MB
)

// Jenis file yang diizinkan (dicek dari isi file, bukan dari nama)
var ekstensiGambar = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// namaAcak menghasilkan nama file acak berekstensi tertentu.
func namaAcak(ext string) (string, error) {
	acak := make([]byte, 8)
	if _, err := rand.Read(acak); err != nil {
		return "", err
	}
	return hex.EncodeToString(acak) + ext, nil
}

// bacaIsiFile membaca seluruh isi file upload setelah deteksi mime type.
// File dikembalikan ke posisi awal sebelum dibaca ulang.
func bacaIsiFile(file io.ReadSeeker, header int64) ([]byte, string, error) {
	buf := make([]byte, 512)
	n, _ := io.ReadFull(file, buf)
	mimeType := http.DetectContentType(buf[:n])

	ext, ok := ekstensiGambar[mimeType]
	if !ok {
		return nil, "", errors.New("Format gambar harus JPG, PNG, WEBP, atau GIF")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", err
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, "", err
	}
	return data, ext, nil
}

// =====================================================================
// UPLOAD KE SUPABASE STORAGE
// =====================================================================

// uploadKeSupabase mengunggah data gambar ke Supabase Storage bucket "Bahan Pangan"
// dan mengembalikan URL publik permanen.
// Membutuhkan env: SUPABASE_URL dan SUPABASE_SERVICE_KEY.
func uploadKeSupabase(data []byte, nama, mimeType string) (string, error) {
	supabaseURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	serviceKey := os.Getenv("SUPABASE_SERVICE_KEY")
	if supabaseURL == "" || serviceKey == "" {
		return "", errors.New("SUPABASE_URL atau SUPABASE_SERVICE_KEY belum diset")
	}

	// URL-encode nama bucket agar spasi dan karakter khusus aman di URL
	bucketEnc := url.PathEscape(bucketSupabase)
	namaEnc := url.PathEscape(nama)

	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", supabaseURL, bucketEnc, namaEnc)

	req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("gagal membuat request upload: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("x-upsert", "true") // timpa jika sudah ada (aman untuk retry)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal upload ke Supabase Storage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Supabase Storage error %d: %s", resp.StatusCode, string(body))
	}

	// URL publik permanen — gunakan nama asli (tidak di-encode) agar bisa diakses browser
	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", supabaseURL, bucketEnc, namaEnc)
	return publicURL, nil
}

// hapusFromSupabase menghapus file dari Supabase Storage.
func hapusFromSupabase(publicURL string) {
	supabaseURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	serviceKey := os.Getenv("SUPABASE_SERVICE_KEY")
	if supabaseURL == "" || serviceKey == "" {
		return
	}

	bucketEnc := url.PathEscape(bucketSupabase)

	// Ambil nama file dari URL
	prefix := fmt.Sprintf("%s/storage/v1/object/public/%s/", supabaseURL, bucketEnc)
	if !strings.HasPrefix(publicURL, prefix) {
		return
	}
	nama := strings.TrimPrefix(publicURL, prefix)

	deleteURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", supabaseURL, bucketEnc, nama)
	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		log.Println("Gagal membuat request hapus gambar Supabase:", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+serviceKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("Gagal hapus gambar dari Supabase:", err)
		return
	}
	resp.Body.Close()
}

// =====================================================================
// SIMPAN GAMBAR — otomatis pilih Supabase atau lokal
// =====================================================================

// simpanGambar menerima file upload dari form.
// Jika SUPABASE_URL dan SUPABASE_SERVICE_KEY tersedia → upload ke Supabase Storage (permanen).
// Jika tidak → simpan ke disk lokal /static/uploads/bahan/ (hanya untuk dev).
// Mengembalikan URL yang disimpan ke database, atau "" jika tidak ada file.
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

	data, ext, err := bacaIsiFile(file, header.Size)
	if err != nil {
		return "", err
	}

	nama, err := namaAcak(ext)
	if err != nil {
		return "", err
	}

	mimeType := http.DetectContentType(data[:min512(data)])

	// Mode Supabase Storage (production)
	if os.Getenv("SUPABASE_URL") != "" && os.Getenv("SUPABASE_SERVICE_KEY") != "" {
		url, err := uploadKeSupabase(data, nama, mimeType)
		if err != nil {
			log.Printf("Upload Supabase gagal, fallback ke lokal: %v", err)
			// jatuh ke lokal sebagai cadangan
		} else {
			return url, nil
		}
	}

	// Mode lokal (development / fallback)
	if err := os.MkdirAll(folderGambar, 0o755); err != nil {
		return "", err
	}
	dst, err := os.Create(filepath.Join(folderGambar, nama))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := dst.Write(data); err != nil {
		return "", err
	}
	return prefixGambar + nama, nil
}

// min512 mengembalikan min(len(data), 512) untuk DetectContentType.
func min512(data []byte) int {
	if len(data) < 512 {
		return len(data)
	}
	return 512
}

// =====================================================================
// HAPUS GAMBAR — otomatis pilih Supabase atau lokal
// =====================================================================

// hapusFileGambar menghapus file gambar, baik dari Supabase maupun disk lokal.
func hapusFileGambar(url string) {
	if url == "" {
		return
	}
	// Supabase URL
	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL != "" && strings.HasPrefix(url, strings.TrimRight(supabaseURL, "/")) {
		hapusFromSupabase(url)
		return
	}
	// Lokal
	if strings.HasPrefix(url, prefixGambar) {
		os.Remove(filepath.Join(folderGambar, filepath.Base(url)))
	}
}
