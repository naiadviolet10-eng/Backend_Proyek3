package main

import "net/http"

// daftarkanAPI memasang seluruh endpoint JSON di bawah /api.
// Semua endpoint kecuali health wajib menyertakan Bearer token Supabase.
func daftarkanAPI() {

	http.HandleFunc("GET /api/health", apiHealth)

	http.HandleFunc("GET /api/me", wajibLoginAPI(apiMe))
	http.HandleFunc("PUT /api/me", wajibLoginAPI(apiUpdateMe))
	http.HandleFunc("POST /api/logout", wajibLoginAPI(apiLogout))

	http.HandleFunc("GET /api/categories", wajibLoginAPI(apiKategori))
	http.HandleFunc("GET /api/ingredients", wajibLoginAPI(apiDaftarBahan))
	http.HandleFunc("GET /api/ingredients/image/{file}", apiGambarBahan)
	http.HandleFunc("GET /api/ingredients/{id}", wajibLoginAPI(apiDetailBahan))

	http.HandleFunc("GET /api/stocks", wajibLoginAPI(apiDaftarStok))
	http.HandleFunc("POST /api/stocks", wajibLoginAPI(apiTambahStok))
	http.HandleFunc("GET /api/stocks/{id}", wajibLoginAPI(apiDetailStok))
	http.HandleFunc("PUT /api/stocks/{id}", wajibLoginAPI(apiUbahStok))
	http.HandleFunc("DELETE /api/stocks/{id}", wajibLoginAPI(apiHapusStok))
	http.HandleFunc("GET /api/notifications", wajibLoginAPI(apiNotifikasi))
	http.HandleFunc("GET /api/notifications/stock", wajibLoginAPI(apiNotifikasiStok))
	http.HandleFunc("PUT /api/notifications/{id}/read", wajibLoginAPI(apiTandaiDibaca))

	http.HandleFunc("GET /api/recipes", wajibLoginAPI(apiDaftarResep))
	http.HandleFunc("POST /api/recipes/generate", wajibLoginAPI(apiGenerateResep))
	http.HandleFunc("GET /api/recipes/{id}", wajibLoginAPI(apiDetailResep))

	http.HandleFunc("GET /api/cooking-logs", wajibLoginAPI(apiRiwayatMasak))
	http.HandleFunc("POST /api/cooking-logs", wajibLoginAPI(apiCatatMasak))

	http.HandleFunc("GET /api/children", wajibLoginAPI(apiDaftarAnak))
	http.HandleFunc("POST /api/children", wajibLoginAPI(apiTambahAnak))
	http.HandleFunc("GET /api/children/{id}", wajibLoginAPI(apiDetailAnak))
	http.HandleFunc("PUT /api/children/{id}", wajibLoginAPI(apiUbahAnak))
	http.HandleFunc("DELETE /api/children/{id}", wajibLoginAPI(apiHapusAnak))
	http.HandleFunc("GET /api/children/{id}/measurements", wajibLoginAPI(apiDaftarPengukuran))
	http.HandleFunc("POST /api/children/{id}/measurements", wajibLoginAPI(apiTambahPengukuran))

	http.HandleFunc("GET /api/pregnancies/active", wajibLoginAPI(apiKehamilanAktif))
	http.HandleFunc("GET /api/pregnancies", wajibLoginAPI(apiDaftarKehamilan))
	http.HandleFunc("POST /api/pregnancies", wajibLoginAPI(apiTambahKehamilan))
	http.HandleFunc("GET /api/pregnancies/{id}", wajibLoginAPI(apiDetailKehamilan))
	http.HandleFunc("PUT /api/pregnancies/{id}", wajibLoginAPI(apiUbahKehamilan))
	http.HandleFunc("POST /api/pregnancies/{id}/delivery", wajibLoginAPI(apiLahirkan))
	http.HandleFunc("DELETE /api/pregnancies/{id}", wajibLoginAPI(apiHapusKehamilan))

	http.HandleFunc("GET /api/food-logs", wajibLoginAPI(apiDaftarFoodLogs))
	http.HandleFunc("POST /api/food-logs", wajibLoginAPI(apiTambahFoodLog))
	http.HandleFunc("PUT /api/food-logs/{id}", wajibLoginAPI(apiUbahFoodLog))
	http.HandleFunc("DELETE /api/food-logs/{id}", wajibLoginAPI(apiHapusFoodLog))

	http.HandleFunc("GET /api/maternal-food-logs", wajibLoginAPI(apiDaftarMaternalFoodLogs))
	http.HandleFunc("POST /api/maternal-food-logs", wajibLoginAPI(apiTambahMaternalFoodLog))
	http.HandleFunc("PUT /api/maternal-food-logs/{id}", wajibLoginAPI(apiUbahMaternalFoodLog))
	http.HandleFunc("DELETE /api/maternal-food-logs/{id}", wajibLoginAPI(apiHapusMaternalFoodLog))
}
