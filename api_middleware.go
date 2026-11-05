package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// Pengguna hasil verifikasi token Supabase, disimpan di context request.
type pengguna struct {
	ID    string
	Email string
	Nama  string
}

type kunciKonteks string

const konteksPengguna kunciKonteks = "pengguna"

func penggunaKonteks(ctx context.Context) (pengguna, bool) {
	u, ok := ctx.Value(konteksPengguna).(pengguna)
	return u, ok
}

func rahasiaJWT() ([]byte, error) {
	s := os.Getenv("SUPABASE_JWT_SECRET")
	if s == "" {
		return nil, errors.New("SUPABASE_JWT_SECRET belum diatur di .env")
	}
	return []byte(s), nil
}

// Nama diambil dari user_metadata token Supabase; OAuth Google mengisi full_name.
func namaDariKlaim(klaim jwt.MapClaims) string {
	if meta, ok := klaim["user_metadata"].(map[string]any); ok {
		for _, kunci := range []string{"name", "full_name", "nama"} {
			if v, ok := meta[kunci].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	if email, ok := klaim["email"].(string); ok {
		if i := strings.Index(email, "@"); i > 0 {
			return email[:i]
		}
	}
	return "Pengguna"
}

// wajibLoginAPI memverifikasi Bearer token Supabase (HS256) lalu menyiapkan
// baris pengguna di tabel public.users bila belum ada.
func wajibLoginAPI(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			kirimPesan(w, http.StatusUnauthorized, "Token tidak ditemukan")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))

		klaim := jwt.MapClaims{}
		var err error
		if strings.TrimSpace(os.Getenv("SUPABASE_JWT_SECRET")) != "" {
			// Proyek dengan JWT Secret bersama: verifikasi HS256.
			var kunci []byte
			kunci, err = rahasiaJWT()
			if err == nil {
				_, err = jwt.ParseWithClaims(token, klaim, func(t *jwt.Token) (any, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("metode tanda tangan token tidak didukung")
					}
					return kunci, nil
				})
			}
		} else {
			// Proyek dengan kunci asimetris: verifikasi lewat JWKS Supabase.
			var jwks *cacheJWKS
			jwks, err = kunciSupabase()
			if err == nil {
				_, err = jwt.ParseWithClaims(token, klaim, jwks.Keyfunc)
			}
		}
		if err != nil {
			kirimPesan(w, http.StatusUnauthorized, "Token tidak valid atau kedaluwarsa")
			return
		}

		id, _ := klaim["sub"].(string)
		email, _ := klaim["email"].(string)
		if id == "" {
			kirimPesan(w, http.StatusUnauthorized, "Token tidak memuat identitas pengguna")
			return
		}

		u := pengguna{ID: id, Email: email, Nama: namaDariKlaim(klaim)}
		if err := pastikanPengguna(r.Context(), u); err != nil {
			kirimPesan(w, http.StatusInternalServerError, "Gagal menyiapkan data pengguna")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), konteksPengguna, u)))
	}
}
