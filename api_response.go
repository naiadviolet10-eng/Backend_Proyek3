package main

import (
	"encoding/json"
	"net/http"
)

// Amplop respons mengikuti API lama: {success, message, data}.
// Aplikasi Flutter membaca data dan message, bukan success.
func kirimJSON(w http.ResponseWriter, status int, isi any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(isi); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func kirimData(w http.ResponseWriter, data any) {
	kirimDataPesan(w, http.StatusOK, "OK", data)
}

// kirimDataPesan membalas sukses dengan pesan khusus beserta data,
// mis. "Stok berhasil ditambahkan".
func kirimDataPesan(w http.ResponseWriter, status int, pesan string, data any) {
	kirimJSON(w, status, map[string]any{
		"success": true,
		"message": pesan,
		"data":    data,
	})
}

// kirimPesanSukses untuk aksi tanpa data balik (mis. hapus) yang tetap
// mengembalikan success=true beserta pesan.
func kirimPesanSukses(w http.ResponseWriter, status int, pesan string) {
	kirimJSON(w, status, map[string]any{
		"success": true,
		"message": pesan,
		"data":    nil,
	})
}

func kirimPesan(w http.ResponseWriter, status int, pesan string) {
	kirimJSON(w, status, map[string]any{
		"success": false,
		"message": pesan,
		"data":    nil,
	})
}

// Kesalahan validasi memakai bentuk Laravel {message, errors:{field:[pesan]}}
// karena bagian auth dan stok di Flutter membaca errors.
func kirimValidasi(w http.ResponseWriter, pesan string, field string, rincian string) {
	kirimJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"success": false,
		"message": pesan,
		"data":    nil,
		"errors":  map[string][]string{field: {rincian}},
	})
}

// maksBacaBody membatasi byte yang dibaca dari body permintaan maupun
// respons luar supaya respons raksasa tidak menghabiskan memori.
const maksBacaBody = 1 << 20

// bacaJSON mengabaikan field yang tidak dikenal, sama seperti validasi Laravel,
// supaya perubahan di aplikasi tidak langsung mematahkan permintaan.
func bacaJSON(r *http.Request, tujuan any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(tujuan)
}
