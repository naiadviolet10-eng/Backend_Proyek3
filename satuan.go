package main

import "strings"

// Konversi satuan dapur. Resep dan stok bisa memakai satuan berbeda untuk
// bahan yang sama; tanpa konversi, pengurangan stok akan salah.
var massa = map[string]float64{
	"mg": 0.001, "g": 1, "gram": 1, "gr": 1, "ons": 100, "kg": 1000, "kilogram": 1000,
}

var volume = map[string]float64{
	"ml": 1, "mililiter": 1, "cc": 1, "sdt": 5, "tehpoon": 5,
	"sdm": 15, "tablespoon": 15, "gelas": 240, "cup": 240, "l": 1000, "liter": 1000,
}

var satuanHitung = map[string]float64{
	"butir": 1, "buah": 1, "biji": 1, "siung": 1, "ruas": 1, "potong": 1,
	"iris": 1, "batang": 1, "lembar": 1, "helai": 1, "daun": 1,
}

var kelompokSatuan = []map[string]float64{massa, volume, satuanHitung}

func normalSatuan(satuan string) string {
	return strings.ToLower(strings.TrimSpace(satuan))
}

// ubahSatuan mengonversi jumlah antar satuan dalam kelompok yang sama.
// Satuan tak dikenal mengembalikan nil supaya stok tidak dikurangi dengan asumsi.
func ubahSatuan(jumlah float64, dari, ke string) (float64, bool) {
	dari = normalSatuan(dari)
	ke = normalSatuan(ke)
	if dari == ke {
		return jumlah, true
	}
	for _, kel := range kelompokSatuan {
		a, okA := kel[dari]
		b, okB := kel[ke]
		if okA && okB {
			return (jumlah * a) / b, true
		}
	}
	return 0, false
}

// keGram mengubah takaran massa menjadi gram; nil bila bukan satuan massa.
func keGram(jumlah float64, satuan string) (float64, bool) {
	faktor, ok := massa[normalSatuan(satuan)]
	if !ok {
		return 0, false
	}
	return jumlah * faktor, true
}

// keGramViaMl memakai perbandingan dapur 1 ml = 1 g untuk satuan volume.
func keGramViaMl(jumlah float64, satuan string) (float64, bool) {
	faktor, ok := volume[normalSatuan(satuan)]
	if !ok {
		return 0, false
	}
	return jumlah * faktor, true
}

// gramUntukGizi: massa dipakai langsung, volume lewat ml; satuan hitung
// (butir, siung) tidak punya bobot tetap sehingga dianggap tidak terhitung.
func gramUntukGizi(jumlah float64, satuan string) (float64, bool) {
	if g, ok := keGram(jumlah, satuan); ok {
		return g, true
	}
	return keGramViaMl(jumlah, satuan)
}
