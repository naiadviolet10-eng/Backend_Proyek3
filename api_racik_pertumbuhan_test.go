package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHitungWHOInternal(t *testing.T) {
	// Tes anak laki-laki usia 12 bulan (365 hari)
	// Berat 10 kg, Panjang 75.7 cm
	in := InputPertumbuhan{
		JenisKelamin:    "L",
		BeratKg:         10.0,
		PanjangTinggiCm: 75.7,
		LingkarKepalaCm: 46.0,
	}

	res := kalkulatorWHOGo(in, 365, "M", "berbaring")

	if res.Zwei == nil || *res.Zwei < -3 || *res.Zwei > 3 {
		t.Errorf("Z-Score BBU diluar batas wajar untuk berat normal: %v", res.Zwei)
	}

	if res.Zlen == nil || *res.Zlen < -3 || *res.Zlen > 3 {
		t.Errorf("Z-Score TBU diluar batas wajar untuk tinggi normal: %v", res.Zlen)
	}
}

func TestJSONRacikAI(t *testing.T) {
	reqJSON := `{
		"selected_stock_ids": [10, 25],
		"target": "Anak (1-3 tahun)",
		"servings": 2,
		"family_context": {
			"total_anggota": 3,
			"target_usia": ["balita"]
		}
	}`

	var input PermintaanRacikAI
	if err := json.Unmarshal([]byte(reqJSON), &input); err != nil {
		t.Fatalf("Gagal unmarshal PermintaanRacikAI: %v", err)
	}

	if len(input.SelectedStockIDs) != 2 || input.SelectedStockIDs[0] != 10 {
		t.Errorf("SelectedStockIDs tidak sesuai: %+v", input.SelectedStockIDs)
	}

	if input.Target != "Anak (1-3 tahun)" {
		t.Errorf("Target tidak sesuai: %s", input.Target)
	}

	if input.Servings != 2 {
		t.Errorf("Servings tidak sesuai: %d", input.Servings)
	}
}

func TestURLGambarDeployment(t *testing.T) {
	// Simpan env asli
	origBase := os.Getenv("BASE_URL")
	origApp := os.Getenv("APP_URL")
	defer func() {
		os.Setenv("BASE_URL", origBase)
		os.Setenv("APP_URL", origApp)
	}()

	pathGambar := "images/wortel.png"

	// Kasus 1: Menggunakan BASE_URL https://api.nutripath.my.id
	os.Setenv("BASE_URL", "https://api.nutripath.my.id")
	os.Unsetenv("APP_URL")

	req1 := httptest.NewRequest("GET", "http://localhost:8080/api/racik-ai", nil)
	hasil1 := urlGambar(req1, &pathGambar)
	expected1 := "https://api.nutripath.my.id/api/ingredients/image/wortel.png"
	if hasil1 == nil || *hasil1 != expected1 {
		t.Errorf("urlGambar salah. Diharapkan: %s, Didapat: %v", expected1, hasil1)
	}

	// Kasus 2: Tanpa env var, request localhost
	os.Unsetenv("BASE_URL")
	os.Unsetenv("APP_URL")
	req2 := httptest.NewRequest("GET", "http://localhost:8080/api/racik-ai", nil)
	hasil2 := urlGambar(req2, &pathGambar)
	expected2 := "http://localhost:8080/api/ingredients/image/wortel.png"
	if hasil2 == nil || *hasil2 != expected2 {
		t.Errorf("urlGambar salah. Diharapkan: %s, Didapat: %v", expected2, hasil2)
	}

	// Kasus 3: Di balik reverse proxy HTTPS (X-Forwarded-Proto)
	req3 := httptest.NewRequest("GET", "http://domainanda.com/api/racik-ai", nil)
	req3.Header.Set("X-Forwarded-Proto", "https")
	hasil3 := urlGambar(req3, &pathGambar)
	expected3 := "https://domainanda.com/api/ingredients/image/wortel.png"
	if hasil3 == nil || *hasil3 != expected3 {
		t.Errorf("urlGambar salah. Diharapkan: %s, Didapat: %v", expected3, hasil3)
	}
}

func TestCORSHeaders(t *testing.T) {
	origOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	defer os.Setenv("CORS_ALLOWED_ORIGINS", origOrigins)

	os.Setenv("CORS_ALLOWED_ORIGINS", "https://app.nutripath.my.id,https://admin.nutripath.my.id")

	handler := enableCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("OPTIONS", "/api/racik-ai", nil)
	req.Header.Set("Origin", "https://app.nutripath.my.id")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "https://app.nutripath.my.id" {
		t.Errorf("Access-Control-Allow-Origin salah: %s", allowOrigin)
	}
}

func TestInputPertumbuhanValidation(t *testing.T) {
	reqBody := []byte(`{
		"tanggal_lahir": "2024-01-01",
		"tanggal_pengukuran": "2025-01-01",
		"jenis_kelamin": "L",
		"berat_kg": 9.5,
		"panjang_tinggi_cm": 75.0,
		"lingkar_kepala_cm": 45.0
	}`)

	req := httptest.NewRequest("POST", "/api/pertumbuhan/analisis", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	apiAnalisisPertumbuhan(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("apiAnalisisPertumbuhan status bukan 200: %d, body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Sukses bool                     `json:"sukses"`
		Pesan  string                   `json:"pesan"`
		Data   HasilAnalisisPertumbuhan `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Gagal unmarshal respon pertumbuhan: %v", err)
	}

	if resp.Data.BBU.ZScore == nil {
		t.Errorf("Status BBU ZScore tidak boleh nil")
	}

	if resp.Data.PBUTBU.ZScore == nil {
		t.Errorf("Status PBUTBU ZScore tidak boleh nil")
	}

	if resp.Data.Kesimpulan == "" {
		t.Errorf("Kesimpulan tidak boleh kosong")
	}
}
