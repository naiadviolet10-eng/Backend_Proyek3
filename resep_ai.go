package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

type StokResep struct {
	ID           int64   `json:"id"`
	IngredientID int64   `json:"ingredient_id"`
	Nama         string  `json:"nama"`
	Jumlah       float64 `json:"jumlah"`
	Satuan       string  `json:"satuan"`
	Kategori     string  `json:"kategori"`
	ImagePath    string  `json:"image_path"`
	Kadaluarsa   string  `json:"tanggal_kadaluarsa"`
	Kalori100g   float64 `json:"kalori_per_100g"`
	Protein100g  float64 `json:"protein_per_100g"`
	Lemak100g    float64 `json:"lemak_per_100g"`
	Karbo100g    float64 `json:"karbohidrat_per_100g"`
	ZatBesi100g  float64 `json:"zat_besi_per_100g"`
	Zinc100g     float64 `json:"zinc_per_100g"`
	VitaminA100g float64 `json:"vitamin_a_per_100g"`
	VitaminC100g float64 `json:"vitamin_c_per_100g"`
}

// Struktur profil keluarga yang dikirim oleh RacikResepAI.jsx.
type ProfilIbu struct {
	ID string `json:"id"`
}

type DataKehamilan struct {
	ID       string `json:"id"`
	MotherID string `json:"mother_id"`
}

type DataAnak struct {
	ID           string `json:"id"`
	Nama         string `json:"nama"`
	TanggalLahir string `json:"tanggal_lahir"`
	PregnancyID  string `json:"pregnancy_id"`
}

type FamilyContext struct {
	Ibu       *ProfilIbu      `json:"ibu"`
	Kehamilan []DataKehamilan `json:"kehamilan"`
	Anak      []DataAnak      `json:"anak"`
}

type PermintaanResep struct {
	SelectedStockIDs []int64       `json:"selected_stock_ids"`
	Target           string        `json:"target"`
	Servings         int           `json:"servings"`
	FamilyContext    FamilyContext `json:"family_context"`
}

type BahanHasilAI struct {
	Nama     string `json:"nama"`
	Jumlah   string `json:"jumlah"`
	DariStok bool   `json:"dari_stok"`
}

type NutrisiHasilAI struct {
	Kalori      float64 `json:"kalori"`
	Protein     float64 `json:"protein"`
	Lemak       float64 `json:"lemak"`
	Karbohidrat float64 `json:"karbohidrat"`
	Serat       float64 `json:"serat"`
}

type HasilResepAI struct {
	NamaResep  string         `json:"nama_resep"`
	Deskripsi  string         `json:"deskripsi"`
	WaktuMenit int            `json:"waktu_menit"`
	Porsi      int            `json:"porsi"`
	Tag        string         `json:"tag"`
	Nutrisi    NutrisiHasilAI `json:"nutrisi"`
	Bahan      []BahanHasilAI `json:"bahan"`
	Langkah    []string       `json:"langkah"`
	Tips       string         `json:"tips"`
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"message": message})
}

// Verifikasi token melalui Supabase Auth.
// ID pengguna diambil dari respons Supabase, bukan dari request React.
func verifikasiPengguna(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return "", fmt.Errorf("token login tidak ditemukan")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	baseURL := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	anonKey := os.Getenv("SUPABASE_ANON_KEY")

	if baseURL == "" || anonKey == "" {
		return "", fmt.Errorf("konfigurasi Supabase belum lengkap")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/auth/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", anonKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal memverifikasi login")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sesi login tidak valid atau sudah berakhir")
	}

	var user struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("respons autentikasi tidak valid")
	}
	if user.ID == "" {
		return "", fmt.Errorf("identitas pengguna tidak ditemukan")
	}
	return user.ID, nil
}

// GET /api/resep/bahan
func resepBahanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		errorResponse(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan")
		return
	}

	userID, err := verifikasiPengguna(r)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}

	rows, err := db.Query(r.Context(), `
		SELECT
			s.id,
			m.id,
			m.name,
			s.jumlah::float8,
			s.satuan,
			COALESCE(c.name, ''),
			 COALESCE(m.image_path, ''),
			COALESCE(s.tanggal_kadaluarsa::text, ''),
			COALESCE(m.calories_per_100g, 0)::float8,
			COALESCE(m.protein_g, 0)::float8,
			COALESCE(m.fat_g, 0)::float8,
			COALESCE(m.carbs_g, 0)::float8,
			COALESCE(m.iron_mg, 0)::float8,
			COALESCE(m.zinc_mg, 0)::float8,
			COALESCE(m.vitamin_a_mcg, 0)::float8,
			COALESCE(m.vitamin_c_mg, 0)::float8
		FROM public.stok s
		JOIN public.ingredients_master m ON m.id = s.bahan_pangan_id
		LEFT JOIN public.ingredient_categories c ON c.id = m.category_id
		WHERE s.user_id = $1
			AND s.jumlah > 0
			AND (s.tanggal_kadaluarsa IS NULL OR s.tanggal_kadaluarsa >= CURRENT_DATE)
		ORDER BY s.tanggal_kadaluarsa ASC NULLS LAST, m.name ASC
	`, userID)
	if err != nil {
		log.Printf("ERROR query stok resep: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal mengambil stok bahan")
		return
	}
	defer rows.Close()

	items := make([]StokResep, 0)
	for rows.Next() {
		var item StokResep
		err := rows.Scan(
    &item.ID,
    &item.IngredientID,
    &item.Nama,
    &item.Jumlah,
    &item.Satuan,
    &item.Kategori,
    &item.ImagePath,
    &item.Kadaluarsa,
    &item.Kalori100g,
    &item.Protein100g,
    &item.Lemak100g,
    &item.Karbo100g,
    &item.ZatBesi100g,
    &item.Zinc100g,
    &item.VitaminA100g,
    &item.VitaminC100g,
		)
		if err != nil {
			log.Printf("ERROR membaca stok resep: %v", err)
			errorResponse(w, http.StatusInternalServerError, "Gagal membaca data stok")
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("ERROR hasil query stok resep: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal membaca hasil stok")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/resep/racik
func resepRacikHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		errorResponse(w, http.StatusMethodNotAllowed, "Metode tidak diizinkan")
		return
	}

	userID, err := verifikasiPengguna(r)
	if err != nil {
		errorResponse(w, http.StatusUnauthorized, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input PermintaanResep
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		errorResponse(w, http.StatusBadRequest, "Data permintaan tidak valid")
		return
	}
	if len(input.SelectedStockIDs) == 0 {
		errorResponse(w, http.StatusBadRequest, "Pilih minimal satu bahan")
		return
	}
	if len(input.SelectedStockIDs) > 100 {
		errorResponse(w, http.StatusBadRequest, "Terlalu banyak bahan dipilih")
		return
	}
	if input.Servings < 1 || input.Servings > 20 {
		errorResponse(w, http.StatusBadRequest, "Jumlah porsi harus 1 sampai 20")
		return
	}

	// Ambil stok milik pengguna yang sudah diverifikasi.
	rows, err := db.Query(r.Context(), `
		SELECT
			s.id,
			m.id,
			m.name,
			s.jumlah::float8,
			s.satuan,
			COALESCE(c.name, ''),
			COALESCE(s.tanggal_kadaluarsa::text, ''),
			COALESCE(m.calories_per_100g, 0)::float8,
			COALESCE(m.protein_g, 0)::float8,
			COALESCE(m.fat_g, 0)::float8,
			COALESCE(m.carbs_g, 0)::float8,
			COALESCE(m.iron_mg, 0)::float8,
			COALESCE(m.zinc_mg, 0)::float8,
			COALESCE(m.vitamin_a_mcg, 0)::float8,
			COALESCE(m.vitamin_c_mg, 0)::float8
		FROM public.stok s
		JOIN public.ingredients_master m ON m.id = s.bahan_pangan_id
		LEFT JOIN public.ingredient_categories c ON c.id = m.category_id
		WHERE s.user_id = $1
			AND s.jumlah > 0
			AND (s.tanggal_kadaluarsa IS NULL OR s.tanggal_kadaluarsa >= CURRENT_DATE)
	`, userID)
	if err != nil {
		log.Printf("ERROR query stok untuk racik resep: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal mengambil stok")
		return
	}

	var stokTersedia []StokResep
	for rows.Next() {
		var item StokResep
		if err := rows.Scan(
			&item.ID, &item.IngredientID, &item.Nama, &item.Jumlah, &item.Satuan,
			&item.Kategori, &item.Kadaluarsa, &item.Kalori100g, &item.Protein100g,
			&item.Lemak100g, &item.Karbo100g, &item.ZatBesi100g, &item.Zinc100g,
			&item.VitaminA100g, &item.VitaminC100g,
		); err != nil {
			rows.Close()
			log.Printf("ERROR scan stok untuk resep: %v", err)
			errorResponse(w, http.StatusInternalServerError, "Gagal membaca stok")
			return
		}
		stokTersedia = append(stokTersedia, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		log.Printf("ERROR membaca hasil stok resep: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal membaca hasil stok")
		return
	}

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
			errorResponse(w, http.StatusBadRequest, "Salah satu bahan tidak tersedia atau bukan milik akun kamu")
			return
		}
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		errorResponse(w, http.StatusInternalServerError, "GEMINI_API_KEY belum diatur")
		return
	}

	stokJSON, err := json.Marshal(terpilih)
	if err != nil {
		log.Printf("ERROR marshal stok resep: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal menyiapkan data bahan")
		return
	}

	familyJSON, err := json.Marshal(input.FamilyContext)
	if err != nil {
		log.Printf("ERROR marshal profil keluarga: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal menyiapkan data profil keluarga")
		return
	}

	target := strings.TrimSpace(input.Target)
	if target == "" {
		target = "Semua"
	}

	prompt := fmt.Sprintf(`
Kamu adalah asisten resep sehat untuk aplikasi Dapur Cerdas.
Buat satu resep realistis berdasarkan bahan stok yang diberikan.
Jumlah porsi: %d.
Target resep: %s.

DATA STOK:
%s

DATA PROFIL KELUARGA:
%s

Aturan:
- Prioritaskan bahan dari stok yang dipilih.
- Jangan menggunakan bahan dari stok yang tidak diberikan.
- Boleh menambahkan bumbu dapur umum, tetapi tandai dari_stok=false.
- Jangan menyatakan bahan tambahan tersedia di kulkas.
- Sesuaikan takaran dengan jumlah porsi dan jumlah stok.
- Gunakan data nutrisi per 100 gram sebagai referensi jika relevan.
- Nilai nutrisi hasil adalah perkiraan, bukan hasil uji laboratorium.
- Jangan membuat klaim medis atau menjanjikan pencegahan stunting.
- Gunakan profil keluarga hanya untuk membantu memahami target resep.
- Jika data profil tidak tersedia atau kosong, jangan mengarang data keluarga.
- Resep untuk keluarga tidak otomatis berarti aman untuk bayi.
- Jangan menyarankan MPASI atau makanan padat untuk bayi berusia di bawah 6 bulan.
- Untuk bayi di bawah 6 bulan, jangan membuat resep makanan pendamping; ikuti anjuran tenaga kesehatan terkait pemberian ASI.
- Langkah memasak harus jelas dan aman, termasuk memastikan protein hewani matang.
- Jawab hanya dalam JSON valid dengan struktur:
{
  "nama_resep": "string",
  "deskripsi": "string",
  "waktu_menit": 20,
  "porsi": %d,
  "tag": "string",
  "nutrisi": {
    "kalori": 0,
    "protein": 0,
    "lemak": 0,
    "karbohidrat": 0,
    "serat": 0
  },
  "bahan": [
    {"nama": "string", "jumlah": "string", "dari_stok": true}
  ],
  "langkah": ["langkah 1", "langkah 2"],
  "tips": "string"
}
Nilai nutrisi adalah total perkiraan untuk seluruh resep.
`, input.Servings, target, string(stokJSON), string(familyJSON), input.Servings)

	// Timeout keseluruhan untuk proses Gemini.
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		log.Printf("ERROR menyiapkan Gemini client: %v", err)
		errorResponse(w, http.StatusInternalServerError, "Gagal menyiapkan Gemini AI")
		return
	}

	// Pemanggilan Gemini dengan retry untuk error sementara.
	var result *genai.GenerateContentResponse
	var errGemini error
	const maksimalPercobaan = 3
	waktuMulai := time.Now()

	for percobaan := 1; percobaan <= maksimalPercobaan; percobaan++ {
		if ctx.Err() != nil {
			errGemini = ctx.Err()
			break
		}

		ctxPercobaan, cancelPercobaan := context.WithTimeout(ctx, 25*time.Second)
		result, errGemini = client.Models.GenerateContent(
			ctxPercobaan,
			"gemini-3.1-flash-lite",
			genai.Text(prompt),
			&genai.GenerateContentConfig{ResponseMIMEType: "application/json"},
		)
		cancelPercobaan()

		if errGemini == nil {
			break
		}

		pesanError := strings.ToUpper(errGemini.Error())
		bisaRetry :=
			strings.Contains(pesanError, "503") ||
				strings.Contains(pesanError, "UNAVAILABLE") ||
				strings.Contains(pesanError, "429") ||
				strings.Contains(pesanError, "RESOURCE_EXHAUSTED") ||
				strings.Contains(pesanError, "504") ||
				strings.Contains(pesanError, "DEADLINE_EXCEEDED") ||
				strings.Contains(pesanError, "500") ||
				strings.Contains(pesanError, "502")

		log.Printf("Gemini percobaan %d/%d gagal setelah %s: %v",
			percobaan, maksimalPercobaan, time.Since(waktuMulai), errGemini)

		if !bisaRetry || percobaan == maksimalPercobaan {
			break
		}

		jeda := time.Duration(1<<uint(percobaan)) * time.Second
		timer := time.NewTimer(jeda)
		select {
		case <-ctx.Done():
			timer.Stop()
			errGemini = ctx.Err()
		case <-timer.C:
		}
		if ctx.Err() != nil {
			errGemini = ctx.Err()
			break
		}
	}

	log.Printf("Durasi total proses Gemini: %s", time.Since(waktuMulai))
	if errGemini != nil {
		log.Printf("ERROR Gemini setelah retry: %v", errGemini)
		errorResponse(w, http.StatusBadGateway, "AI sedang sibuk atau membutuhkan waktu lebih lama. Silakan coba lagi beberapa saat.")
		return
	}

	if result == nil || len(result.Candidates) == 0 || result.Candidates[0].Content == nil {
		errorResponse(w, http.StatusBadGateway, "Gemini tidak memberikan hasil resep")
		return
	}

	var teks strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		if part != nil {
			teks.WriteString(part.Text)
		}
	}

	jsonText := strings.TrimSpace(teks.String())
	jsonText = strings.TrimPrefix(jsonText, "```json")
	jsonText = strings.TrimPrefix(jsonText, "```")
	jsonText = strings.TrimSuffix(jsonText, "```")
	jsonText = strings.TrimSpace(jsonText)

	var resep HasilResepAI
	if err := json.NewDecoder(bytes.NewBufferString(jsonText)).Decode(&resep); err != nil {
		log.Printf("ERROR format JSON dari Gemini: %v", err)
		errorResponse(w, http.StatusBadGateway, "Format resep dari AI tidak valid")
		return
	}

	if resep.NamaResep == "" || len(resep.Bahan) == 0 || len(resep.Langkah) == 0 {
		errorResponse(w, http.StatusBadGateway, "Hasil resep AI belum lengkap")
		return
	}

	// Mengembalikan hasil resep, bukan data contoh dari React.
	jsonResponse(w, http.StatusOK, resep)
}
