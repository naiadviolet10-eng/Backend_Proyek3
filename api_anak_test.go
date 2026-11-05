package main

import (
	"testing"
	"time"
)

func TestHpl(t *testing.T) {
	if got := hpl("2026-01-01"); got != "2026-10-08" {
		t.Fatalf("hpl = %q, want 2026-10-08", got)
	}
	if got := hpl("bukan-tanggal"); got != "" {
		t.Fatalf("hpl invalid = %q, want empty", got)
	}
}

func TestStatusMapping(t *testing.T) {
	kasus := map[string]string{"active": "hamil", "delivered": "lahir", "archived": "selesai"}
	for app, db := range kasus {
		got, ok := statusKeDB(app)
		if !ok || got != db {
			t.Fatalf("statusKeDB(%q) = %q, %v; want %q, true", app, got, ok, db)
		}
		if balik := statusKeApp(db); balik != app {
			t.Fatalf("statusKeApp(%q) = %q; want %q", db, balik, app)
		}
	}
	if _, ok := statusKeDB("unknown"); ok {
		t.Fatal("statusKeDB unknown should fail")
	}
}

func TestUmurHariPada(t *testing.T) {
	sekarang := time.Date(2026, 10, 27, 23, 59, 0, 0, time.UTC)
	kasus := []struct {
		lahir string
		want  int
	}{
		{"2026-10-27", 0},
		{"2026-10-26", 1},
		{"2025-10-27", 365},
		{"2026-10-28", -1},
	}
	for _, k := range kasus {
		got, ok := umurHariPada(k.lahir, sekarang)
		if !ok || got != k.want {
			t.Fatalf("umurHariPada(%q) = %d, %v; want %d, true", k.lahir, got, ok, k.want)
		}
	}
}

func TestValidasiAnakHpht(t *testing.T) {
	if errs := validasiAnak("Aisyah", "female", "2026-10-01", true, "2026-01-01", "2026-10-08"); errs != nil {
		t.Fatalf("validasiAnak valid = %v", errs)
	}
	if errs := validasiAnak("Aisyah", "female", "2026-10-01", true, "2026-10-02", "2026-10-08"); errs == nil {
		t.Fatal("HPHT setelah tanggal lahir harus ditolak")
	}
	if errs := validasiAnak("Aisyah", "female", "2026-10-01", true, "2026-01-01", "2025-12-31"); errs == nil {
		t.Fatal("HPL sebelum HPHT harus ditolak")
	}
	if errs := validasiAnak("Aisyah", "female", "2026-10-01", true, "2025-01-01", "2025-10-08"); errs == nil {
		t.Fatal("HPHT terlalu jauh dari tanggal lahir harus ditolak")
	}
}

func TestValidasiLahir(t *testing.T) {
	berat, panjang, kepala := 3.1, 49.0, 34.0
	if errs := validasiLahir(&berat, &panjang, &kepala); errs != nil {
		t.Fatalf("validasiLahir valid = %v", errs)
	}
	berat = 20
	if errs := validasiLahir(&berat, &panjang, &kepala); errs == nil {
		t.Fatal("berat lahir di luar batas harus ditolak")
	}
}

func TestGenderMapping(t *testing.T) {
	keDB := map[string]string{
		"male":      "Laki-laki",
		"female":    "Perempuan",
		"Laki-laki": "Laki-laki",
		"Perempuan": "Perempuan",
		"male ":     "Laki-laki",
	}
	for masuk, mau := range keDB {
		if got := genderKeDB(masuk); got != mau {
			t.Fatalf("genderKeDB(%q) = %q, want %q", masuk, got, mau)
		}
	}
	dariDB := map[string]string{
		"Laki-laki": "male",
		"Perempuan": "female",
		"male":      "male",
		"female":    "female",
	}
	for masuk, mau := range dariDB {
		if got := genderDariDB(masuk); got != mau {
			t.Fatalf("genderDariDB(%q) = %q, want %q", masuk, got, mau)
		}
	}
}
