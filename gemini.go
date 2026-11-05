package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Klien Google Gemini. Model dicoba berurutan (utama lalu cadangan); status
// sementara dicoba ulang dengan jeda bertingkat sebelum pindah model.
type gemini struct {
	apiKey   string
	model    string
	cadangan []string
	baseURL  string
	client   *http.Client
}

var jedaUlang = []int{2, 4, 8, 12}

const batasWaktuGemini = 90 * time.Second

func geminiDariEnv() *gemini {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("GEMINI_BASE_URL")), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &gemini{
		apiKey:   strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		model:    strings.TrimSpace(os.Getenv("GEMINI_MODEL")),
		cadangan: pisahDaftarModel(os.Getenv("GEMINI_FALLBACK_MODELS")),
		baseURL:  base,
		client:   &http.Client{Timeout: 45 * time.Second},
	}
}

func pisahDaftarModel(raw string) []string {
	var hasil []string
	for _, s := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(s); t != "" {
			hasil = append(hasil, t)
		}
	}
	return hasil
}

func (g *gemini) daftarModel() []string {
	unik := map[string]bool{}
	var hasil []string
	for _, m := range append([]string{g.model}, g.cadangan...) {
		if m != "" && !unik[m] {
			unik[m] = true
			hasil = append(hasil, m)
		}
	}
	if len(hasil) == 0 {
		hasil = []string{"gemini-3.6-flash"}
	}
	return hasil
}

func (g *gemini) generate(prompt string) (string, error) {
	if strings.TrimSpace(g.apiKey) == "" {
		return "", errors.New("Kunci API Gemini belum diatur. Isi GEMINI_API_KEY di file .env backend.")
	}

	batas := time.Now().Add(batasWaktuGemini)
	var pesanTerakhir string

luar:
	for _, model := range g.daftarModel() {
		for percobaan := 1; percobaan <= len(jedaUlang); percobaan++ {
			body, status, err := g.kirim(model, prompt)
			if err != nil {
				pesanTerakhir = "Tidak bisa menghubungi server Gemini."
				break
			}

			if status >= 200 && status < 300 {
				teks := ambilTeksGemini(body)
				if strings.TrimSpace(teks) != "" {
					return teks, nil
				}
				return "", errors.New("Gemini tidak mengembalikan jawaban yang valid. Coba lagi.")
			}

			if status == 404 || status == 403 {
				pesanTerakhir = fmt.Sprintf("Model %s tidak tersedia (kode %d).", model, status)
				break
			}

			if status != 429 && status != 500 && status != 502 && status != 503 && status != 504 {
				return "", errors.New(pesanErrorGemini(body, status))
			}

			pesanTerakhir = pesanErrorGemini(body, status)

			if status == 429 {
				return "", errors.New("Kuota AI sudah habis untuk hari ini. Coba lagi besok.")
			}

			wait := jedaUlang[percobaan-1]
			if time.Now().Add(time.Duration(wait) * time.Second).After(batas) {
				break luar
			}
			time.Sleep(time.Duration(wait) * time.Second)
		}
	}

	log.Printf("gemini gagal setelah semua percobaan: %s", pesanTerakhir)
	return "", errors.New("Layanan AI sedang sibuk. Tunggu sebentar lalu coba lagi.")
}

func (g *gemini) kirim(model, prompt string) ([]byte, int, error) {
	permintaan := map[string]any{
		"contents": []any{
			map[string]any{"parts": []any{map[string]any{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":      0.7,
			"maxOutputTokens":  2048,
			"responseMimeType": "application/json",
		},
	}
	isi, _ := json.Marshal(permintaan)

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.baseURL, model, g.apiKey)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(isi))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode, nil
}

func ambilTeksGemini(body []byte) string {
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return parsed.Candidates[0].Content.Parts[0].Text
}

func pesanErrorGemini(body []byte, status int) string {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	return "Permintaan Gemini ditolak (kode " + strconv.Itoa(status) + ")."
}

var polaKomaBerlebih = regexp.MustCompile(`,\s*([\]}])`)
var polaBlokJSON = regexp.MustCompile(`(?s)\{.*\}`)

func (g *gemini) generateJSON(prompt string) (map[string]any, error) {
	mentah, err := g.generate(prompt)
	if err != nil {
		return nil, err
	}
	teks := strings.TrimSpace(mentah)

	if hasil := cobaDecodeJSON(teks); hasil != nil {
		return hasil, nil
	}
	if m := polaBlokJSON.FindString(teks); m != "" {
		if hasil := cobaDecodeJSON(m); hasil != nil {
			return hasil, nil
		}
	}
	return nil, errors.New("Format jawaban AI tidak bisa dibaca. Coba lagi.")
}

func cobaDecodeJSON(teks string) map[string]any {
	bersih := strings.TrimSpace(teks)
	bersih = strings.TrimPrefix(bersih, "```json")
	bersih = strings.TrimPrefix(bersih, "```")
	bersih = strings.TrimSuffix(bersih, "```")
	bersih = strings.TrimSpace(bersih)
	bersih = polaKomaBerlebih.ReplaceAllString(bersih, "$1")

	var hasil map[string]any
	if err := json.Unmarshal([]byte(bersih), &hasil); err != nil {
		return nil
	}
	return hasil
}
