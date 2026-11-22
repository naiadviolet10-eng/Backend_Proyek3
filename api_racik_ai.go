package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"
)

// =====================================================================
// MODEL DATA RACIK AI
// =====================================================================

// PermintaanRacikAI adalah payload yang diterima dari frontend saat meminta
// AI meracik resep berdasarkan stok bahan yang dipilih.
type PermintaanRacikAI struct {
	SelectedStockIDs []int64       `json:"selected_stock_ids"`
	Target           string        `json:"target"`
	Servings         int           `json:"servings"`
	FamilyContext    FamilyContext `json:"family_context"`
}

// ResepRacikSimpan adalah payload untuk menyimpan resep hasil racikan AI.
type ResepRacikSimpan struct {
	NamaResep string          `json:"nama_resep"`
	DataResep json.RawMessage `json:"data_resep"`
}

// =====================================================================
// SERVICE INTEGRASI GEMINI AI UNTUK RACIK RESEP
// =====================================================================

// panggilGeminiRacik memanggil Gemini AI untuk meracik resep berdasarkan bahan
// stok yang dipilih dan target sasaran (keluarga/anak/ibu hamil/menyusui).
func panggilGeminiRacik(ctx context.Context, input PermintaanRacikAI, terpilih []StokResep) (HasilResepAI, error) {
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		return HasilResepAI{}, errors.New("GEMINI_API_KEY belum dikonfigurasi di backend (.env)")
	}

	bahanJSON, err := json.Marshal(terpilih)
	if err != nil {
		return HasilResepAI{}, fmt.Errorf("gagal memproses data bahan stok: %w", err)
	}

	familyJSON, err := json.Marshal(input.FamilyContext)
	if err != nil {
		return HasilResepAI{}, fmt.Errorf("gagal memproses data profil keluarga: %w", err)
	}

	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = "Semua"
	}
	porsi := input.Servings
	if porsi < 1 {
		porsi = 2
	}
	if porsi > 20 {
		porsi = 20
	}

	prompt := fmt.Sprintf(`
Kamu adalah asisten ahli gizi dan koki resep sehat untuk aplikasi Dapur Cerdas.
Buat SATU resep masakan realistis, lezat, dan bergizi seimbang berdasarkan bahan stok yang dipilih.

Jumlah Porsi: %d porsi.
Target Sasaran: %s.

DATA STOK BAHAN YANG DIPILIH:
%s

DATA PROFIL KELUARGA:
%s

Pedoman Pembuatan Resep:
1. Prioritaskan bahan dari stok yang dipilih (beri tanda "dari_stok": true).
2. Boleh menambahkan bumbu dapur umum (garam, gula, minyak, bawang merah/putih, merica), beri tanda "dari_stok": false.
3. Sesuaikan takaran bahan dan langkah dengan jumlah porsi (%d porsi).
4. Pastikan langkah memasak higienis, aman, dan mematangkan protein dengan baik.
5. Bila target adalah bayi di bawah 6 bulan, jangan buat MPASI padat; anjurkan pemberian ASI eksklusif.
6. Kembalikan HANYA format JSON valid tanpa tanda markdown (tanpa `+"`"+`json ... `+"`"+`) dengan struktur persis:
{
  "nama_resep": "Nama Masakan",
  "deskripsi": "Deskripsi singkat 1-2 kalimat tentang menu dan manfaatnya",
  "waktu_menit": 25,
  "porsi": %d,
  "tag": "%s",
  "nutrisi": {
    "kalori": 250.0,
    "protein": 12.5,
    "lemak": 8.0,
    "karbohidrat": 30.0,
    "serat": 4.5
  },
  "bahan": [
    {"nama": "Nama Bahan", "jumlah": "100 gram", "dari_stok": true}
  ],
  "langkah": [
    "Langkah 1...",
    "Langkah 2..."
  ],
  "tips": "Tips pengolahan agar gizi tetap optimal"
}
`, porsi, target, string(bahanJSON), string(familyJSON), porsi, porsi, target)

	// Metode 1: Coba menggunakan Google GenAI SDK (gemini-3.1-flash-lite atau model konfigurasi)
	resepSDK, errSDK := panggilGeminiSDK(ctx, apiKey, prompt)
	if errSDK == nil && resepSDK.NamaResep != "" {
		return resepSDK, nil
	}

	log.Printf("[Racik AI] Panggilan SDK gagal (%v), beralih ke REST client cadangan...", errSDK)

	// Metode 2: Fallback menggunakan klien REST gemini.go (mencoba model utama dan cadangan)
	gem := geminiDariEnv()
	teksResep, errREST := gem.generate(prompt)
	if errREST != nil {
		return HasilResepAI{}, fmt.Errorf("AI sedang sibuk atau tidak tersedia: %v", errREST)
	}

	return uraiHasilResepJSON(teksResep)
}

// panggilGeminiSDK memanggil Google Gemini lewat official Go SDK google.golang.org/genai.
func panggilGeminiSDK(ctx context.Context, apiKey, prompt string) (HasilResepAI, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctxTimeout, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return HasilResepAI{}, err
	}

	modelName := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if modelName == "" {
		modelName = "gemini-3.1-flash-lite"
	}

	res, err := client.Models.GenerateContent(
		ctxTimeout,
		modelName,
		genai.Text(prompt),
		&genai.GenerateContentConfig{ResponseMIMEType: "application/json"},
	)
	if err != nil {
		return HasilResepAI{}, err
	}

	if res == nil || len(res.Candidates) == 0 || res.Candidates[0].Content == nil {
		return HasilResepAI{}, errors.New("tidak ada konten yang dihasilkan oleh Gemini")
	}

	var sb strings.Builder
	for _, part := range res.Candidates[0].Content.Parts {
		if part != nil {
			sb.WriteString(part.Text)
		}
	}

	return uraiHasilResepJSON(sb.String())
}

// uraiHasilResepJSON membersihkan format markdown dan membaca JSON menjadi HasilResepAI.
func uraiHasilResepJSON(raw string) (HasilResepAI, error) {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)

	var resep HasilResepAI
	if err := json.NewDecoder(bytes.NewBufferString(clean)).Decode(&resep); err != nil {
		return HasilResepAI{}, fmt.Errorf("format JSON dari AI tidak valid: %w", err)
	}

	if strings.TrimSpace(resep.NamaResep) == "" {
		resep.NamaResep = "Resep Racik Sehat"
	}
	if len(resep.Bahan) == 0 {
		return HasilResepAI{}, errors.New("resep dari AI tidak memuat daftar bahan")
	}
	if len(resep.Langkah) == 0 {
		return HasilResepAI{}, errors.New("resep dari AI tidak memuat langkah memasak")
	}

	return resep, nil
}

// =====================================================================
// HANDLER ENDPOINT RACIK AI
// =====================================================================

// GET /api/racik-ai/bahan
// Mengambil daftar bahan pangan dari stok pengguna untuk diracik, lengkap dengan gizi.
func apiRacikBahan(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid atau token belum disertakan")
			return
		}
	}

	items, err := ambilStokResep(r.Context(), u.ID)
	if err != nil {
		log.Printf("[Racik AI] Gagal mengambil stok: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal mengambil daftar stok bahan pangan")
		return
	}

	kirimDataPesan(w, http.StatusOK, "Daftar bahan stok berhasil dimuat", map[string]any{
		"items": items,
		"total": len(items),
	})
}

// POST /api/racik-ai atau POST /api/racik-ai/generate
// Meracik resep masakan berbasis AI dari stok bahan yang dipilih pengguna.
func apiRacikAI(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid atau token belum disertakan")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maksBacaBody)
	defer r.Body.Close()

	var input PermintaanRacikAI
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		kirimValidasi(w, "Format data permintaan tidak valid", "body", err.Error())
		return
	}

	if len(input.SelectedStockIDs) == 0 {
		kirimValidasi(w, "Pilih minimal 1 bahan dari stok kamu", "selected_stock_ids", "Daftar ID bahan tidak boleh kosong")
		return
	}
	if len(input.SelectedStockIDs) > 50 {
		kirimValidasi(w, "Maksimal 50 bahan dapat dipilih sekaligus", "selected_stock_ids", "Bahan terlalu banyak")
		return
	}
	if input.Servings < 1 || input.Servings > 20 {
		kirimValidasi(w, "Jumlah porsi harus antara 1 sampai 20", "servings", "Nilai porsi di luar batas")
		return
	}

	// Ambil stok bahan milik pengguna yang valid
	stokTersedia, err := ambilStokResep(r.Context(), u.ID)
	if err != nil {
		log.Printf("[Racik AI] Error baca stok: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal membaca stok bahan pengguna")
		return
	}

	// Filter hanya bahan yang dipilih dan pastikan milik user
	terpilih := make([]StokResep, 0, len(input.SelectedStockIDs))
	seen := make(map[int64]bool)
	for _, id := range input.SelectedStockIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		found := false
		for _, item := range stokTersedia {
			if item.ID == id {
				terpilih = append(terpilih, item)
				found = true
				break
			}
		}
		if !found {
			kirimPesan(w, http.StatusBadRequest, fmt.Sprintf("Bahan dengan ID %d tidak tersedia atau bukan milik akun kamu", id))
			return
		}
	}

	// Panggil service AI Gemini
	resep, err := panggilGeminiRacik(r.Context(), input, terpilih)
	if err != nil {
		log.Printf("[Racik AI] Error generate resep: %v", err)
		kirimPesan(w, http.StatusBadGateway, err.Error())
		return
	}

	// Kembalikan response JSON terstandarisasi
	kirimDataPesan(w, http.StatusOK, "Resep berhasil diracik oleh AI", resep)
}

// GET /api/racik-ai/tersimpan
// Mengambil daftar resep hasil racik AI yang telah disimpan pengguna.
func apiRacikTersimpan(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid")
			return
		}
	}

	rows, err := db.Query(r.Context(), `
		SELECT id, nama_resep, data_resep, created_at
		FROM public.resep_tersimpan
		WHERE user_id = $1
		ORDER BY created_at DESC`, u.ID)
	if err != nil {
		log.Printf("[Racik AI] Error query resep tersimpan: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal memuat resep tersimpan")
		return
	}
	defer rows.Close()

	items := make([]ResepTersimpan, 0)
	for rows.Next() {
		var item ResepTersimpan
		if err := rows.Scan(&item.ID, &item.NamaResep, &item.DataResep, &item.CreatedAt); err != nil {
			log.Printf("[Racik AI] Error scan resep tersimpan: %v", err)
			kirimPesan(w, http.StatusInternalServerError, "Gagal membaca data resep")
			return
		}
		items = append(items, item)
	}

	kirimDataPesan(w, http.StatusOK, "Daftar resep tersimpan berhasil dimuat", map[string]any{
		"items": items,
		"total": len(items),
	})
}

// POST /api/racik-ai/simpan
// Menyimpan resep hasil racikan AI ke tabel resep_tersimpan.
func apiSimpanRacik(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid")
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maksBacaBody)
	defer r.Body.Close()

	var input ResepRacikSimpan
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		kirimValidasi(w, "Format data resep tidak valid", "body", err.Error())
		return
	}

	input.NamaResep = strings.TrimSpace(input.NamaResep)
	if input.NamaResep == "" {
		kirimValidasi(w, "Nama resep wajib diisi", "nama_resep", "Nama resep kosong")
		return
	}
	if len(input.DataResep) == 0 || !json.Valid(input.DataResep) {
		kirimValidasi(w, "Data resep harus berformat JSON valid", "data_resep", "JSON tidak valid")
		return
	}

	var parsed map[string]any
	if err := json.Unmarshal(input.DataResep, &parsed); err != nil {
		kirimValidasi(w, "Data resep harus berupa objek JSON", "data_resep", "Bukan objek JSON")
		return
	}

	deskripsi := teksNilai(parsed["deskripsi"])
	waktu := int(angka(parsed["waktu_menit"]))
	porsi := int(angka(parsed["porsi"]))
	if waktu <= 0 {
		waktu = 20
	}
	if porsi <= 0 {
		porsi = 2
	}

	bahanJSONBytes, _ := json.Marshal(parsed["bahan"])
	langkahJSONBytes, _ := json.Marshal(parsed["langkah"])
	nutrisiJSONBytes, _ := json.Marshal(parsed["nutrisi"])

	var id int64
	err := db.QueryRow(r.Context(), `
		INSERT INTO public.resep_tersimpan
		  (user_id, nama_resep, data_resep, deskripsi, waktu_memasak, porsi, bahan, langkah, nutrisi, tag, tips)
		VALUES ($1, $2, $3::jsonb, $4, $5, $6, $7::jsonb, $8::jsonb, $9::jsonb, '[]'::jsonb, '[]'::jsonb)
		RETURNING id`,
		u.ID, input.NamaResep, string(input.DataResep), nullIfEmpty(deskripsi), waktu, porsi,
		string(bahanJSONBytes), string(langkahJSONBytes), string(nutrisiJSONBytes)).Scan(&id)
	if err != nil {
		log.Printf("[Racik AI] Gagal menyimpan resep: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal menyimpan resep ke database")
		return
	}

	kirimDataPesan(w, http.StatusCreated, "Resep berhasil disimpan", map[string]any{
		"id":         id,
		"nama_resep": input.NamaResep,
	})
}

// DELETE /api/racik-ai/tersimpan/{id}
// Menghapus resep hasil racikan AI milik pengguna.
func apiHapusRacik(w http.ResponseWriter, r *http.Request) {
	u, ok := penggunaKonteks(r.Context())
	if !ok || u.ID == "" {
		var err error
		u.ID, err = verifikasiPengguna(r)
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Sesi login tidak valid")
			return
		}
	}

	idText := strings.TrimSpace(r.PathValue("id"))
	if idText == "" {
		idText = strings.TrimPrefix(r.URL.Path, "/api/racik-ai/tersimpan/")
		idText = strings.TrimPrefix(idText, "/api/resep/tersimpan/")
		idText = strings.Trim(idText, "/")
	}

	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		kirimPesan(w, http.StatusBadRequest, "ID resep tidak valid")
		return
	}

	tag, err := db.Exec(r.Context(), `
		DELETE FROM public.resep_tersimpan
		WHERE id = $1 AND user_id = $2`, id, u.ID)
	if err != nil {
		log.Printf("[Racik AI] Error hapus resep: %v", err)
		kirimPesan(w, http.StatusInternalServerError, "Gagal menghapus resep")
		return
	}

	if tag.RowsAffected() == 0 {
		kirimPesan(w, http.StatusNotFound, "Resep tidak ditemukan atau bukan milik akun kamu")
		return
	}

	kirimPesanSukses(w, http.StatusOK, "Resep tersimpan berhasil dihapus")
}
