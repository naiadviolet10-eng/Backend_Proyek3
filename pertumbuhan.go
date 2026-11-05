
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// =====================================================
// STRUKTUR DATA
// =====================================================

type PermintaanAnalisisPertumbuhan struct {
	ChildID           string  `json:"child_id"`
	TanggalLahir      string  `json:"tanggal_lahir"`
	TanggalPengukuran string  `json:"tanggal_pengukuran"`
	JenisKelamin      string  `json:"jenis_kelamin"`
	BeratKg           float64 `json:"berat_kg"`
	PanjangTinggiCm   float64 `json:"panjang_tinggi_cm"`
	LingkarKepalaCm   float64 `json:"lingkar_kepala_cm"`
	PosisiPengukuran  string  `json:"posisi_pengukuran"`
}

type hasilMentahWHO struct {
	Zwei *float64 `json:"zwei"`
	Zlen *float64 `json:"zlen"`
	Zwfl *float64 `json:"zwfl"`
	Zhc  *float64 `json:"zhc"`
}

type indikatorPertumbuhan struct {
	Kategori string   `json:"kategori"`
	ZScore   *float64 `json:"z_score"`
}

// =====================================================
// HANDLER ANALISIS PERTUMBUHAN
// =====================================================

func analisisPertumbuhanHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		kirimJSONPertumbuhan(w, http.StatusMethodNotAllowed, map[string]interface{}{
			"status": "error",
			"error":  "Gunakan metode POST.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	var input PermintaanAnalisisPertumbuhan

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Format data tidak valid: " + err.Error(),
		})
		return
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Body hanya boleh berisi satu objek JSON.",
		})
		return
	}

	// -------------------------------------------------
	// VALIDASI TANGGAL
	// -------------------------------------------------

	tanggalLahir, err := time.Parse("2006-01-02", input.TanggalLahir)
	if err != nil {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Tanggal lahir harus berformat YYYY-MM-DD.",
		})
		return
	}

	tanggalUkur, err := time.Parse("2006-01-02", input.TanggalPengukuran)
	if err != nil {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Tanggal pengukuran harus berformat YYYY-MM-DD.",
		})
		return
	}

	if tanggalUkur.Before(tanggalLahir) {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Tanggal pengukuran tidak boleh sebelum tanggal lahir.",
		})
		return
	}

	usiaHari := int(tanggalUkur.Sub(tanggalLahir).Hours() / 24)

	if usiaHari > 1856 {
		kirimJSONPertumbuhan(w, http.StatusUnprocessableEntity, map[string]interface{}{
			"status": "error",
			"error":  "Usia melebihi rentang WHO Child Growth Standards 0–5 tahun.",
		})
		return
	}

	// -------------------------------------------------
	// VALIDASI JENIS KELAMIN
	// -------------------------------------------------

	sex := strings.ToLower(strings.TrimSpace(input.JenisKelamin))

	switch sex {
	case "laki-laki", "laki laki", "laki", "male", "m", "1":
		sex = "male"

	case "perempuan", "female", "f", "2":
		sex = "female"

	default:
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Jenis kelamin harus Laki-laki atau Perempuan.",
		})
		return
	}

	// -------------------------------------------------
	// VALIDASI BERAT BADAN
	// -------------------------------------------------

	if !angkaValid(input.BeratKg) ||
		input.BeratKg <= 0 ||
		input.BeratKg > 40 {

		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Berat badan harus lebih dari 0 dan maksimal 40 kg.",
		})
		return
	}

	// -------------------------------------------------
	// VALIDASI PANJANG / TINGGI BADAN
	// -------------------------------------------------

	if !angkaValid(input.PanjangTinggiCm) ||
		input.PanjangTinggiCm <= 0 ||
		input.PanjangTinggiCm > 130 {

		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Panjang atau tinggi badan harus lebih dari 0 dan maksimal 130 cm.",
		})
		return
	}

	// -------------------------------------------------
	// VALIDASI LINGKAR KEPALA
	// -------------------------------------------------

	if !angkaValid(input.LingkarKepalaCm) ||
		input.LingkarKepalaCm < 0 ||
		input.LingkarKepalaCm > 65 {

		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Lingkar kepala harus 0 jika tidak diukur, atau maksimal 65 cm.",
		})
		return
	}

	// -------------------------------------------------
	// POSISI PENGUKURAN
	// -------------------------------------------------

	measure := strings.ToLower(strings.TrimSpace(input.PosisiPengukuran))

	switch measure {
	case "", "otomatis":
		if usiaHari < 730 {
			measure = "berbaring"
		} else {
			measure = "berdiri"
		}

	case "berbaring", "tidur", "lying", "l":
		measure = "berbaring"

	case "berdiri", "standing", "h":
		measure = "berdiri"

	default:
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Posisi pengukuran harus berbaring, berdiri, atau otomatis.",
		})
		return
	}

	// Pastikan posisi pengukuran sesuai usia.
	if usiaHari < 730 && measure == "berdiri" {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Anak di bawah 24 bulan sebaiknya diukur berbaring.",
		})
		return
	}

	if usiaHari >= 730 && measure == "berbaring" {
		kirimJSONPertumbuhan(w, http.StatusBadRequest, map[string]interface{}{
			"status": "error",
			"error":  "Anak berusia 24 bulan atau lebih sebaiknya diukur berdiri.",
		})
		return
	}

	// -------------------------------------------------
	// SIAPKAN DATA UNTUK NODE.JS
	// -------------------------------------------------

	payload := map[string]interface{}{
		"sex":     sex,
		"ageDays": usiaHari,
		"weight":  input.BeratKg,
		"height":  input.PanjangTinggiCm,
		"measure": measure,
		"head":    input.LingkarKepalaCm,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		kirimJSONPertumbuhan(w, http.StatusInternalServerError, map[string]interface{}{
			"status": "error",
			"error":  "Gagal menyiapkan data analisis.",
		})
		return
	}

	// -------------------------------------------------
	// HITUNG Z-SCORE WHO
	// -------------------------------------------------

	hasilWHO, err := hitungZScoreWHO(payloadBytes)
	if err != nil {
		kirimJSONPertumbuhan(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status": "error",
			"error":  "Perhitungan WHO gagal dijalankan.",
			"detail": err.Error(),
		})
		return
	}

	// -------------------------------------------------
	// BENTUK HASIL INDIKATOR
	// -------------------------------------------------

	bbu := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zwei, "bb_u"),
		ZScore:   hasilWHO.Zwei,
	}

	pbuTbu := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zlen, "pb_tb_u"),
		ZScore:   hasilWHO.Zlen,
	}

	bbPbTb := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zwfl, "bb_pb_tb"),
		ZScore:   hasilWHO.Zwfl,
	}

	lku := indikatorPertumbuhan{
		Kategori: interpretasiZScore(hasilWHO.Zhc, "lk_u"),
		ZScore:   hasilWHO.Zhc,
	}

	kesimpulan := buatKesimpulan(
		hasilWHO.Zlen,
		hasilWHO.Zwfl,
		hasilWHO.Zwei,
	)

	// -------------------------------------------------
	// HASIL ANALISIS
	// -------------------------------------------------

	hasil := map[string]interface{}{
		"bbu":        bbu,
		"pbu_tbu":    pbuTbu,
		"bb_pb_tb":   bbPbTb,
		"lku":        lku,
		"kesimpulan": kesimpulan,
		"usia_hari":  usiaHari,
	}

	response := map[string]interface{}{
		"status":     "success",
		"child_id":   input.ChildID,
		"usia_hari":  usiaHari,
		"usia_bulan": math.Round(float64(usiaHari)/30.4375*100) / 100,

		// Indikator utama.
		"bbu":      bbu,
		"pbu_tbu":  pbuTbu,
		"bb_pb_tb": bbPbTb,
		"lku":      lku,

		// Hasil bersarang.
		"hasil": hasil,

		// Nilai Z-score mentah.
		"bb_u_zscore":     hasilWHO.Zwei,
		"pb_tb_u_zscore":  hasilWHO.Zlen,
		"bb_pb_tb_zscore": hasilWHO.Zwfl,
		"lk_u_zscore":     hasilWHO.Zhc,

		"interpretasi": map[string]string{
			"bb_u":     bbu.Kategori,
			"pb_tb_u":  pbuTbu.Kategori,
			"bb_pb_tb": bbPbTb.Kategori,
			"lk_u":     lku.Kategori,
		},

		"kesimpulan": kesimpulan,

		"catatan": []string{
			"Hasil merupakan skrining berdasarkan standar pertumbuhan WHO, bukan diagnosis medis.",
			"Pastikan tanggal lahir, jenis kelamin, berat, panjang atau tinggi badan sudah benar.",
			"Anak di bawah 24 bulan umumnya diukur berbaring, sedangkan mulai 24 bulan diukur berdiri.",
			"Jika anak lahir prematur, hasil perlu ditinjau bersama tenaga kesehatan.",
		},
	}

	kirimJSONPertumbuhan(w, http.StatusOK, response)
}

// =====================================================
// VALIDASI ANGKA
// =====================================================

func angkaValid(nilai float64) bool {
	return !math.IsNaN(nilai) && !math.IsInf(nilai, 0)
}

// =====================================================
// CARI FOLDER FRONTEND
// =====================================================

func cariFolderFrontend() (string, error) {
	if folderEnv := strings.TrimSpace(os.Getenv("WHO_NODE_PROJECT_DIR")); folderEnv != "" {
		folder, err := filepath.Abs(folderEnv)
		if err != nil {
			return "", err
		}

		if packageTersedia(folder) {
			return folder, nil
		}

		return "", fmt.Errorf(
			"package who-growth-standards tidak ditemukan di %s",
			folder,
		)
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	kandidat := []string{
		workingDir,
		filepath.Join(workingDir, "..", "project_3", "pangan_bijak"),
		filepath.Join(workingDir, "..", "pangan_bijak"),
	}

	for _, path := range kandidat {
		absolute, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		if packageTersedia(absolute) {
			return absolute, nil
		}
	}

	return "", fmt.Errorf(
		"folder frontend tidak ditemukan. Atur WHO_NODE_PROJECT_DIR ke folder pangan_bijak",
	)
}

func packageTersedia(folder string) bool {
	packagePath := filepath.Join(
		folder,
		"node_modules",
		"who-growth-standards",
		"package.json",
	)

	_, err := os.Stat(packagePath)
	return err == nil
}

// =====================================================
// PERHITUNGAN Z-SCORE WHO MELALUI NODE.JS
// =====================================================

func hitungZScoreWHO(input []byte) (hasilMentahWHO, error) {
	folderFrontend, err := cariFolderFrontend()
	if err != nil {
		return hasilMentahWHO{}, err
	}

	script := `
import {
  weightForAge,
  heightForAge,
  headCircumferenceForAge,
  weightForLength,
  weightForHeight
} from "who-growth-standards";

const input = JSON.parse(await new Promise((resolve, reject) => {
  let data = "";
  process.stdin.setEncoding("utf8");
  process.stdin.on("data", chunk => data += chunk);
  process.stdin.on("end", () => resolve(data));
  process.stdin.on("error", reject);
}));

const hasil = {
  zwei: null,
  zlen: null,
  zwfl: null,
  zhc: null
};

function ambilZScore(callback) {
  try {
    const result = callback();

    if (
      result &&
      typeof result.zScore === "number" &&
      Number.isFinite(result.zScore)
    ) {
      return result.zScore;
    }
  } catch (error) {
    throw new Error(error.message);
  }

  return null;
}

const opsiUsia = {
  sex: input.sex,
  ageDays: input.ageDays
};

// Berat badan menurut umur.
hasil.zwei = ambilZScore(() =>
  weightForAge(input.weight, opsiUsia)
);

// Panjang atau tinggi badan menurut umur.
hasil.zlen = ambilZScore(() =>
  heightForAge(input.height, opsiUsia)
);

// Berat badan menurut panjang atau tinggi badan.
if (input.measure === "berbaring") {
  hasil.zwfl = ambilZScore(() =>
    weightForLength(input.weight, input.height, input.sex)
  );
} else {
  hasil.zwfl = ambilZScore(() =>
    weightForHeight(input.weight, input.height, input.sex)
  );
}

// Lingkar kepala menurut umur.
if (input.head > 0) {
  hasil.zhc = ambilZScore(() =>
    headCircumferenceForAge(input.head, opsiUsia)
  );
}

process.stdout.write(JSON.stringify(hasil));
`

	cmd := exec.Command(
		"node",
		"--input-type=module",
		"-e",
		script,
	)

	cmd.Dir = folderFrontend
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		pesan := strings.TrimSpace(stderr.String())

		if pesan == "" {
			pesan = err.Error()
		}

		return hasilMentahWHO{}, fmt.Errorf(
			"Node.js gagal menjalankan perhitungan WHO: %s",
			pesan,
		)
	}

	var hasil hasilMentahWHO

	if err := json.Unmarshal(stdout.Bytes(), &hasil); err != nil {
		return hasilMentahWHO{}, fmt.Errorf(
			"hasil WHO bukan JSON valid: %s",
			strings.TrimSpace(stdout.String()),
		)
	}

	if hasil.Zwei == nil || hasil.Zlen == nil || hasil.Zwfl == nil {
		return hasilMentahWHO{}, fmt.Errorf(
			"paket WHO tidak menghasilkan seluruh indikator utama. Periksa rentang pengukuran dan data masukan",
		)
	}

	return hasil, nil
}

// =====================================================
// INTERPRETASI Z-SCORE
// =====================================================

func interpretasiZScore(nilai *float64, indikator string) string {
	if nilai == nil || !angkaValid(*nilai) {
		return "Data tidak tersedia atau indikator tidak dapat dihitung."
	}

	z := *nilai

	switch indikator {

	case "pb_tb_u":
		if z < -3 {
			return "Sangat pendek (di bawah -3 SD)."
		}
		if z < -2 {
			return "Stunting atau pendek (di bawah -2 SD)."
		}
		if z > 3 {
			return "Sangat tinggi (di atas +3 SD)."
		}
		if z > 2 {
			return "Tinggi (di atas +2 SD)."
		}
		return "Normal."

	case "bb_u":
		if z < -3 {
			return "Berat badan sangat kurang."
		}
		if z < -2 {
			return "Berat badan kurang."
		}
		if z > 2 {
			return "Berat badan tinggi menurut umur; periksa bersama indikator BB/PB atau BB/TB."
		}
		return "Normal."

	case "bb_pb_tb":
		if z < -3 {
			return "Sangat kurus (di bawah -3 SD)."
		}
		if z < -2 {
			return "Kurus atau wasting (di bawah -2 SD)."
		}
		if z > 3 {
			return "Obesitas (di atas +3 SD)."
		}
		if z > 2 {
			return "Gemuk atau berat badan berlebih (di atas +2 SD)."
		}
		if z > 1 {
			return "Berisiko gemuk (di atas +1 SD)."
		}
		return "Normal."

	case "lk_u":
		if z < -2 {
			return "Lingkar kepala di bawah -2 SD; perlu evaluasi."
		}
		if z > 2 {
			return "Lingkar kepala di atas +2 SD; perlu evaluasi."
		}
		return "Normal."
	}

	return "Indikator berhasil dihitung."
}

// =====================================================
// KESIMPULAN GABUNGAN
// =====================================================

func buatKesimpulan(
	zPB_TB_U *float64,
	zBB_PB_TB *float64,
	zBB_U *float64,
) string {

	if zPB_TB_U == nil || zBB_PB_TB == nil {
		return "Data belum cukup untuk menentukan kesimpulan pertumbuhan anak."
	}

	stunting := *zPB_TB_U < -2
	sangatPendek := *zPB_TB_U < -3

	gemuk := *zBB_PB_TB > 2
	obesitas := *zBB_PB_TB > 3

	kurus := *zBB_PB_TB < -2
	sangatKurus := *zBB_PB_TB < -3

	if stunting && obesitas {
		return "Anak terindikasi stunting dan obesitas berdasarkan indikator WHO. Disarankan konsultasi dengan tenaga kesehatan."
	}

	if stunting && gemuk {
		return "Anak terindikasi stunting dan memiliki berat badan berlebih (gemuk). Disarankan konsultasi dengan tenaga kesehatan."
	}

	if stunting && sangatKurus {
		return "Anak terindikasi stunting dan sangat kurus. Disarankan pemeriksaan tenaga kesehatan."
	}

	if stunting && kurus {
		return "Anak terindikasi stunting dan kurus (wasting). Disarankan pemeriksaan tenaga kesehatan."
	}

	if sangatPendek {
		return "Anak termasuk kategori sangat pendek berdasarkan panjang atau tinggi badan menurut umur."
	}

	if stunting {
		return "Anak terindikasi stunting berdasarkan panjang atau tinggi badan menurut umur."
	}

	if obesitas {
		return "Anak termasuk kategori obesitas berdasarkan berat badan menurut panjang atau tinggi badan."
	}

	if gemuk {
		return "Anak memiliki berat badan berlebih (gemuk) berdasarkan berat badan menurut panjang atau tinggi badan."
	}

	if sangatKurus {
		return "Anak termasuk kategori sangat kurus berdasarkan berat badan menurut panjang atau tinggi badan."
	}

	if kurus {
		return "Anak terindikasi kurus (wasting) berdasarkan berat badan menurut panjang atau tinggi badan."
	}

	if zBB_U != nil && *zBB_U < -3 {
		return "Berat badan anak sangat kurang menurut umur. Disarankan pemeriksaan tenaga kesehatan."
	}

	if zBB_U != nil && *zBB_U < -2 {
		return "Berat badan anak kurang menurut umur. Perlu pemantauan lebih lanjut."
	}

	return "Pertumbuhan anak berada dalam rentang normal berdasarkan indikator WHO yang tersedia."
}

// =====================================================
// RESPONSE JSON
// =====================================================

func kirimJSONPertumbuhan(
	w http.ResponseWriter,
	status int,
	data interface{},
) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		return
	}
}
