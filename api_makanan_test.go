package main

import (
	"encoding/json"
	"testing"
)

func TestValidasiEntriMakan(t *testing.T) {
	kasus := []struct {
		nama       string
		isian      isianMakan
		mauPesan   bool
		mauTanggal string
	}{
		{
			nama: "lengkap",
			isian: isianMakan{
				NamaMakanan:     "  Nasi uduk  ",
				TanggalKonsumsi: "2026-01-10",
				WaktuMakan:      "Sarapan",
				Porsi:           1,
			},
			mauPesan:   false,
			mauTanggal: "2026-01-10",
		},
		{
			nama: "nama kosong",
			isian: isianMakan{
				TanggalKonsumsi: "2026-01-10",
				WaktuMakan:      "Sarapan",
				Porsi:           1,
			},
			mauPesan: true,
		},
		{
			nama: "nama terlalu panjang",
			isian: isianMakan{
				NamaMakanan:     string(make([]rune, 121)),
				TanggalKonsumsi: "2026-01-10",
				WaktuMakan:      "Sarapan",
				Porsi:           1,
			},
			mauPesan: true,
		},
		{
			nama: "format tanggal salah",
			isian: isianMakan{
				NamaMakanan:     "Bubur",
				TanggalKonsumsi: "10/01/2026",
				WaktuMakan:      "Sarapan",
				Porsi:           1,
			},
			mauPesan: true,
		},
		{
			nama: "tanggal masa depan",
			isian: isianMakan{
				NamaMakanan:     "Bubur",
				TanggalKonsumsi: "2099-01-01",
				WaktuMakan:      "Sarapan",
				Porsi:           1,
			},
			mauPesan: true,
		},
		{
			nama: "waktu makan tidak dikenal",
			isian: isianMakan{
				NamaMakanan:     "Bubur",
				TanggalKonsumsi: "2026-01-10",
				WaktuMakan:      "Tengah malam",
				Porsi:           1,
			},
			mauPesan: true,
		},
		{
			nama: "porsi nol",
			isian: isianMakan{
				NamaMakanan:     "Bubur",
				TanggalKonsumsi: "2026-01-10",
				WaktuMakan:      "Sarapan",
				Porsi:           0,
			},
			mauPesan: true,
		},
		{
			nama: "gizi negatif",
			isian: isianMakan{
				NamaMakanan:     "Bubur",
				TanggalKonsumsi: "2026-01-10",
				WaktuMakan:      "Sarapan",
				Porsi:           1,
				ProteinG:        f64(-1),
			},
			mauPesan: true,
		},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			isian := k.isian
			tgl, pesan := validasiEntriMakan(&isian)
			if k.mauPesan {
				if pesan == "" {
					t.Fatal("pesan kosong, mau ada galat")
				}
				return
			}
			if pesan != "" {
				t.Fatalf("pesan = %q, mau kosong", pesan)
			}
			if got := tgl.Format("2006-01-02"); got != k.mauTanggal {
				t.Fatalf("tanggal = %q, mau %q", got, k.mauTanggal)
			}
		})
	}
}

// Nama dipangkas di tempat supaya nilai yang disimpan ke database ikut bersih.
func TestValidasiEntriMakanMemangkasNama(t *testing.T) {
	isian := isianMakan{
		NamaMakanan:     "   Nasi uduk   ",
		TanggalKonsumsi: "2026-01-10",
		WaktuMakan:      "Sarapan",
		Porsi:           1,
	}
	if _, pesan := validasiEntriMakan(&isian); pesan != "" {
		t.Fatalf("pesan = %q, mau kosong", pesan)
	}
	if isian.NamaMakanan != "Nasi uduk" {
		t.Fatalf("nama = %q, mau %q", isian.NamaMakanan, "Nasi uduk")
	}
}

// Field isianTambahMakan harus terbaca dari JSON walau tertanam (embedded).
func TestIsianTambahMakanJSON(t *testing.T) {
	var isian struct {
		isianTambahMakan
		ChildID string `json:"child_id"`
	}
	mentah := `{"child_id":"c1","nama_makanan":"Bubur","tanggal_konsumsi":"2026-01-10",` +
		`"waktu_makan":"Sarapan","porsi":1.5,"sumber":"manual","resep_id":7}`
	if err := json.Unmarshal([]byte(mentah), &isian); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if isian.ChildID != "c1" || isian.NamaMakanan != "Bubur" || isian.Porsi != 1.5 ||
		isian.Sumber != "manual" || isian.ResepID == nil || *isian.ResepID != 7 {
		t.Fatalf("field tidak terbaca dengan benar: %+v", isian)
	}
}

func TestValidasiRentangTanggal(t *testing.T) {
	kasus := []struct {
		nama, mulai, akhir, mauPesan string
	}{
		{"lengkap", "2026-01-01", "2026-01-31", ""},
		{"kosong", "", "", "start_date dan end_date wajib"},
		{"mulai salah", "besok", "2026-01-31", "Format tanggal tidak valid (YYYY-MM-DD)"},
		{"akhir salah", "2026-01-01", "31-01-2026", "Format tanggal tidak valid (YYYY-MM-DD)"},
	}
	for _, k := range kasus {
		if got := validasiRentangTanggal(k.mulai, k.akhir); got != k.mauPesan {
			t.Errorf("%s: pesan = %q, mau %q", k.nama, got, k.mauPesan)
		}
	}
}

func f64(v float64) *float64 { return &v }
