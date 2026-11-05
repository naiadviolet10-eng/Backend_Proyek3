package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Verifikasi token Supabase memakai JWKS ketika JWT Secret bersama tidak
// diatur. Proyek Supabase baru menandatangani token dengan kunci asimetris
// (RS256/ES256), jadi kunci publiknya diambil dari endpoint JWKS dan di-cache.
type cacheJWKS struct {
	url     string
	mu      sync.Mutex
	kunci   map[string]any
	diambil time.Time
}

const umurCacheJWKS = time.Hour

var (
	cacheSupabase        *cacheJWKS
	sekaliInisialisasi   sync.Once
	errJWKSTidakDikonfig = errors.New("SUPABASE_URL belum diatur untuk verifikasi token")
)

func kunciSupabase() (*cacheJWKS, error) {
	sekaliInisialisasi.Do(func() {
		base := strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/")
		if base != "" {
			cacheSupabase = &cacheJWKS{url: base + "/auth/v1/.well-known/jwks.json"}
		}
	})
	if cacheSupabase == nil {
		return nil, errJWKSTidakDikonfig
	}
	return cacheSupabase, nil
}

func (c *cacheJWKS) ambil(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	klien := &http.Client{Timeout: 10 * time.Second}
	resp, err := klien.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("endpoint JWKS membalas kode %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return err
	}

	kunci := map[string]any{}
	for _, k := range set.Keys {
		switch k.Kty {
		case "RSA":
			pub, err := rsaDariJWK(k.N, k.E)
			if err == nil {
				kunci[k.Kid] = pub
			}
		case "EC":
			pub, err := ecDariJWK(k.Crv, k.X, k.Y)
			if err == nil {
				kunci[k.Kid] = pub
			}
		}
	}
	if len(kunci) == 0 {
		return errors.New("tidak ada kunci publik yang bisa dipakai dari JWKS")
	}
	c.kunci = kunci
	c.diambil = time.Now()
	return nil
}

func (c *cacheJWKS) Keyfunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
		if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("metode tanda tangan %v tidak didukung", t.Header["alg"])
		}
	}
	kid, _ := t.Header["kid"].(string)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.kunci == nil || time.Since(c.diambil) > umurCacheJWKS {
		if err := c.ambil(context.Background()); err != nil {
			return nil, err
		}
	}
	if pub, ok := c.kunci[kid]; ok {
		return pub, nil
	}
	// Kunci mungkin sudah dirotasi; muat ulang sekali sebelum menyerah.
	if err := c.ambil(context.Background()); err != nil {
		return nil, err
	}
	if pub, ok := c.kunci[kid]; ok {
		return pub, nil
	}
	return nil, fmt.Errorf("kunci dengan kid %q tidak ditemukan", kid)
}

func rsaDariJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	e := new(big.Int).SetBytes(eBytes)
	if !e.IsInt64() {
		return nil, errors.New("eksponen RSA terlalu besar")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: int(e.Int64())}, nil
}

func ecDariJWK(crv, xB64, yB64 string) (*ecdsa.PublicKey, error) {
	var kurva elliptic.Curve
	switch crv {
	case "P-256":
		kurva = elliptic.P256()
	case "P-384":
		kurva = elliptic.P384()
	case "P-521":
		kurva = elliptic.P521()
	default:
		return nil, fmt.Errorf("kurva %q tidak didukung", crv)
	}
	x, err := base64.RawURLEncoding.DecodeString(xB64)
	if err != nil {
		return nil, err
	}
	y, err := base64.RawURLEncoding.DecodeString(yB64)
	if err != nil {
		return nil, err
	}
	return &ecdsa.PublicKey{Curve: kurva, X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
}
