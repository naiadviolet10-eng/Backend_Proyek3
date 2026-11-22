-- =====================================================================
-- Migrasi Supabase: Tabel Admin Users
-- Dapur Cerdas / Nutripath
-- =====================================================================

-- Pastikan extension pgcrypto aktif untuk fungsi hashing (crypt / gen_salt)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 1. Buat Tabel admin_users
CREATE TABLE IF NOT EXISTS public.admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    nama TEXT,
    role TEXT NOT NULL DEFAULT 'admin',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 2. Index untuk pencarian username cepat (case-insensitive)
CREATE INDEX IF NOT EXISTS idx_admin_users_username ON public.admin_users (LOWER(username));

-- 3. RLS (Row Level Security)
ALTER TABLE public.admin_users ENABLE ROW LEVEL SECURITY;

-- Policy agar backend service_role atau akses terautentikasi dapat membaca admin
CREATE POLICY "Akses internal service role admin_users"
    ON public.admin_users
    FOR ALL
    USING (true);

-- 4. Contoh Menambahkan User Admin Baru (Password: admin123)
-- Password di-hash menggunakan algoritma Blowfish / Bcrypt standar (crypt + gen_salt('bf'))
-- Ganti 'admin' dan 'admin123' sesuai kredensial yang diinginkan.
INSERT INTO public.admin_users (username, password_hash, nama, role)
VALUES (
    'admin',
    crypt('admin123', gen_salt('bf')),
    'Administrator Utama',
    'admin'
)
ON CONFLICT (username) DO NOTHING;
