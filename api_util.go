package main

import (
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Resep AI disimpan di resep_tersimpan (id bigint) sedangkan pustaka di resep
// (id integer). Keduanya dipakai aplikasi lewat satu kolom id integer, jadi id
// AI digeser ke rentang besar supaya tidak bertabrakan dengan id pustaka.
const offsetResepAI int64 = 1_000_000_000

func idTersimpanKePublik(id int64) int64 { return offsetResepAI + id }

func publikKeTersimpan(id int64) (int64, bool) {
	if id >= offsetResepAI {
		return id - offsetResepAI, true
	}
	return 0, false
}

type kategoriRingkas struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type bahanJSON struct {
	ID              int64            `json:"id"`
	CategoryID      int64            `json:"category_id"`
	Name            string           `json:"name"`
	DefaultUnit     string           `json:"default_unit"`
	CaloriesPer100g float64          `json:"calories_per_100g"`
	ProteinG        float64          `json:"protein_g"`
	FatG            float64          `json:"fat_g"`
	CarbsG          float64          `json:"carbs_g"`
	IronMg          float64          `json:"iron_mg"`
	ZincMg          float64          `json:"zinc_mg"`
	VitaminAMcg     float64          `json:"vitamin_a_mcg"`
	VitaminCMcg     float64          `json:"vitamin_c_mg"`
	Category        *kategoriRingkas `json:"category"`
	ImageURL        *string          `json:"image_url"`
}

type stokJSON struct {
	ID         int64     `json:"id"`
	Ingredient bahanJSON `json:"ingredient"`
	Quantity   float64   `json:"quantity"`
	Unit       string    `json:"unit"`
	ExpiryDate *string   `json:"expiry_date"`
	Status     string    `json:"status"`
	DaysLeft   *int      `json:"days_left"`
}

type resepBahanJSON struct {
	IngredientID   int64   `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	QuantityNeeded float64 `json:"quantity_needed"`
	Unit           string  `json:"unit"`
	ImageURL       *string `json:"image_url"`
	InStock        bool    `json:"in_stock"`
}

type resepJSON struct {
	ID              int64            `json:"id"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	Instructions    string           `json:"instructions"`
	CookTimeMinutes int              `json:"cook_time_minutes"`
	Servings        int              `json:"servings"`
	Source          string           `json:"source"`
	Ingredients     []resepBahanJSON `json:"ingredients"`
}

type anakJSON struct {
	ID            string   `json:"id"`
	UserID        string   `json:"user_id"`
	Name          string   `json:"name"`
	Gender        string   `json:"gender"`
	BirthDate     string   `json:"birth_date"`
	Hpht          *string  `json:"hpht"`
	Hpl           *string  `json:"hpl"`
	PregnancyID   *string  `json:"pregnancy_id"`
	BirthWeightKg *float64 `json:"birth_weight_kg"`
	BirthLengthCm *float64 `json:"birth_length_cm"`
	BirthHeadCm   *float64 `json:"birth_head_circumference_cm"`
	AgeInDays     int      `json:"age_in_days"`
	AgeInMonths   int      `json:"age_in_months"`
	AgeInYears    int      `json:"age_in_years"`
}

type pengukuranJSON struct {
	ID                string   `json:"id"`
	ChildID           string   `json:"child_id"`
	Weight            *float64 `json:"weight"`
	Height            *float64 `json:"height"`
	HeadCircumference *float64 `json:"head_circumference_cm"`
	Notes             *string  `json:"notes"`
	MeasuredAt        string   `json:"measured_at"`
}

type kehamilanJSON struct {
	ID           string  `json:"id"`
	MotherName   string  `json:"mother_name"`
	HphtDate     string  `json:"hpht_date"`
	DueDate      string  `json:"due_date"`
	Status       string  `json:"status"`
	DeliveryDate *string `json:"delivery_date"`
	Notes        *string `json:"notes"`
}

// hariIni mengembalikan tanggal lokal dalam format YYYY-MM-DD.
func hariIni() string {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func tanggalValid(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// urlGambar menyusun URL gambar bahan dari path relatif (mis. ingredients/x.jpg).
// Path absolut http(s) dikembalikan apa adanya.
func urlGambar(r *http.Request, imagePath *string) *string {
	if imagePath == nil || strings.TrimSpace(*imagePath) == "" {
		return nil
	}
	p := strings.TrimSpace(*imagePath)
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return &p
	}
	nama := path.Base(p)
	skema := "http"
	if r.TLS != nil {
		skema = "https"
	}
	u := skema + "://" + r.Host + "/api/ingredients/image/" + url.PathEscape(nama)
	return &u
}

// angka membaca nilai numerik dari any (json / pgx) sebagai float64.
func angka(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		return parseAngka(t)
	default:
		return 0
	}
}

func parseAngka(s string) float64 {
	var b strings.Builder
	dot := false
	for _, c := range strings.TrimSpace(s) {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		} else if (c == '.' || c == ',') && !dot {
			b.WriteRune('.')
			dot = true
		} else if b.Len() > 0 {
			break
		}
	}
	if b.Len() == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0
	}
	return f
}

// sisaString mengembalikan sisa teks setelah angka pertama, mis. "200 gram" -> "gram".
func sisaString(s string) string {
	t := strings.TrimSpace(s)
	i := 0
	for i < len(t) && (t[i] >= '0' && t[i] <= '9' || t[i] == '.' || t[i] == ',') {
		i++
	}
	return strings.TrimSpace(t[i:])
}
