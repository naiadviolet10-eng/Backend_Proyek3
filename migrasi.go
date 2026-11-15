package main

import (
	"context"
	"log"
)

// jalankanMigrasi menyiapkan tabel dan kolom yang dipakai API. Setiap
// pernyataan dibuat idempoten karena fungsi ini berjalan di setiap start.
func jalankanMigrasi(ctx context.Context) {
	pernyataan := []string{
		`CREATE EXTENSION IF NOT EXISTS pgcrypto`,

		`CREATE TABLE IF NOT EXISTS cooking_logs (
			id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id          uuid NOT NULL,
			recipe_id        bigint NOT NULL,
			nama_resep       text,
			servings         numeric NOT NULL DEFAULT 1,
			duration_minutes integer,
			cooked_at        timestamptz NOT NULL DEFAULT now(),
			nutrition        jsonb NOT NULL DEFAULT '{}'::jsonb,
			rincian          jsonb NOT NULL DEFAULT '{}'::jsonb,
			created_at       timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS maternal_profiles (
			id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    uuid NOT NULL,
			nama_ibu   text,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS pregnancies (
			id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			mother_id          uuid NOT NULL REFERENCES maternal_profiles(id) ON DELETE CASCADE,
			hpht               date NOT NULL,
			hpl                date,
			status             text NOT NULL DEFAULT 'hamil',
			tanggal_persalinan date,
			catatan            text,
			created_at         timestamptz NOT NULL DEFAULT now(),
			updated_at         timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS children_profiles (
			id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id                 uuid NOT NULL,
			nama                    text NOT NULL,
			jenis_kelamin           text,
			tanggal_lahir           date,
			hpht                    date,
			hpl                     date,
			pregnancy_id            uuid,
			berat_lahir_kg          numeric(5,2),
			panjang_lahir_cm        numeric(5,1),
			lingkar_kepala_lahir_cm numeric(5,1),
			created_at              timestamptz NOT NULL DEFAULT now(),
			updated_at              timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS growth_records (
			id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			child_id          uuid NOT NULL REFERENCES children_profiles(id) ON DELETE CASCADE,
			berat_kg          numeric(5,2),
			panjang_tinggi_cm numeric(5,1),
			lingkar_kepala_cm numeric(5,1),
			catatan           text,
			tanggal_pengukuran date NOT NULL,
			created_at        timestamptz NOT NULL DEFAULT now()
		)`,

		`ALTER TABLE pregnancies ADD COLUMN IF NOT EXISTS catatan text`,
		`ALTER TABLE children_profiles ADD COLUMN IF NOT EXISTS hpht date`,
		`ALTER TABLE children_profiles ADD COLUMN IF NOT EXISTS hpl date`,
		`ALTER TABLE children_profiles ADD COLUMN IF NOT EXISTS pregnancy_id uuid`,
		`ALTER TABLE children_profiles ADD COLUMN IF NOT EXISTS berat_lahir_kg numeric(5,2)`,
		`ALTER TABLE children_profiles ADD COLUMN IF NOT EXISTS panjang_lahir_cm numeric(5,1)`,
		`ALTER TABLE children_profiles ADD COLUMN IF NOT EXISTS lingkar_kepala_lahir_cm numeric(5,1)`,
		`ALTER TABLE growth_records ADD COLUMN IF NOT EXISTS lingkar_kepala_cm numeric(5,1)`,
		`ALTER TABLE growth_records ADD COLUMN IF NOT EXISTS catatan text`,

		`CREATE TABLE IF NOT EXISTS food_logs (
			id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			child_id            uuid NOT NULL REFERENCES children_profiles(id) ON DELETE CASCADE,
			nama_makanan        text NOT NULL,
			tanggal_konsumsi    date NOT NULL,
			waktu_makan         text NOT NULL,
			porsi               numeric NOT NULL DEFAULT 1,
			kalori_kcal         numeric,
			protein_g           numeric,
			karbohidrat_g       numeric,
			lemak_g             numeric,
			serat_g             numeric,
			sumber              text DEFAULT 'manual',
			resep_id            bigint,
			created_at          timestamptz NOT NULL DEFAULT now(),
			updated_at          timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS maternal_food_logs (
			id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			pregnancy_id        uuid NOT NULL REFERENCES pregnancies(id) ON DELETE CASCADE,
			nama_makanan        text NOT NULL,
			tanggal_konsumsi    date NOT NULL,
			waktu_makan         text NOT NULL,
			porsi               numeric NOT NULL DEFAULT 1,
			kalori_kcal         numeric,
			protein_g           numeric,
			karbohidrat_g       numeric,
			lemak_g             numeric,
			serat_g             numeric,
			sumber              text DEFAULT 'manual',
			resep_id            bigint,
			created_at          timestamptz NOT NULL DEFAULT now(),
			updated_at          timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE INDEX IF NOT EXISTS idx_food_logs_child_date ON food_logs (child_id, tanggal_konsumsi DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_maternal_food_logs_pregnancy_date ON maternal_food_logs (pregnancy_id, tanggal_konsumsi DESC)`,

		`ALTER TABLE food_logs ADD COLUMN IF NOT EXISTS sumber text DEFAULT 'manual'`,
		`ALTER TABLE food_logs ADD COLUMN IF NOT EXISTS resep_id bigint`,
		`ALTER TABLE food_logs ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now()`,
		`ALTER TABLE food_logs ALTER COLUMN kalori_kcal DROP NOT NULL`,
		`ALTER TABLE food_logs ALTER COLUMN protein_g DROP NOT NULL`,
		`ALTER TABLE food_logs ALTER COLUMN karbohidrat_g DROP NOT NULL`,
		`ALTER TABLE food_logs ALTER COLUMN lemak_g DROP NOT NULL`,
		`ALTER TABLE food_logs ALTER COLUMN serat_g DROP NOT NULL`,

		`ALTER TABLE maternal_food_logs ADD COLUMN IF NOT EXISTS sumber text DEFAULT 'manual'`,
		`ALTER TABLE maternal_food_logs ADD COLUMN IF NOT EXISTS resep_id bigint`,
		`ALTER TABLE maternal_food_logs ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now()`,
		`ALTER TABLE maternal_food_logs ALTER COLUMN kalori_kcal DROP NOT NULL`,
		`ALTER TABLE maternal_food_logs ALTER COLUMN protein_g DROP NOT NULL`,
		`ALTER TABLE maternal_food_logs ALTER COLUMN karbohidrat_g DROP NOT NULL`,
		`ALTER TABLE maternal_food_logs ALTER COLUMN lemak_g DROP NOT NULL`,
		`ALTER TABLE maternal_food_logs ALTER COLUMN serat_g DROP NOT NULL`,

		`INSERT INTO food_logs (child_id, nama_makanan, tanggal_konsumsi, waktu_makan, porsi, kalori_kcal, protein_g, karbohidrat_g, lemak_g, serat_g, sumber, resep_id, created_at, updated_at)
		SELECT
			ch.id,
			COALESCE(NULLIF(BTRIM(cl.nama_resep), ''), 'Menu Masakan'),
			GREATEST((cl.cooked_at AT TIME ZONE 'Asia/Jakarta')::date, COALESCE(ch.tanggal_lahir, (cl.cooked_at AT TIME ZONE 'Asia/Jakarta')::date)),
			CASE
				WHEN EXTRACT(HOUR FROM (cl.cooked_at AT TIME ZONE 'Asia/Jakarta')) < 11 THEN 'Sarapan'
				WHEN EXTRACT(HOUR FROM (cl.cooked_at AT TIME ZONE 'Asia/Jakarta')) < 15 THEN 'Makan siang'
				WHEN EXTRACT(HOUR FROM (cl.cooked_at AT TIME ZONE 'Asia/Jakarta')) < 18 THEN 'Selingan'
				ELSE 'Makan malam'
			END,
			GREATEST(COALESCE(cl.servings, 1), 0.1),
			(cl.nutrition->'total'->>'calories')::numeric,
			(cl.nutrition->'total'->>'protein_g')::numeric,
			(cl.nutrition->'total'->>'carbs_g')::numeric,
			(cl.nutrition->'total'->>'fat_g')::numeric,
			NULL,
			'resep_ai',
			cl.recipe_id,
			cl.cooked_at,
			cl.cooked_at
		FROM cooking_logs cl
		JOIN LATERAL (
			SELECT c.id, c.tanggal_lahir
			FROM children_profiles c
			WHERE c.user_id = cl.user_id
			ORDER BY c.created_at ASC
			LIMIT 1
		) ch ON TRUE
		WHERE NOT EXISTS (
			SELECT 1 FROM food_logs fl
			WHERE fl.child_id = ch.id AND fl.resep_id = cl.recipe_id AND fl.created_at = cl.cooked_at
		)`,

		`CREATE INDEX IF NOT EXISTS idx_cooking_logs_user_cooked ON cooking_logs (user_id, cooked_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_maternal_profiles_user ON maternal_profiles (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pregnancies_mother_status ON pregnancies (mother_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_children_user ON children_profiles (user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_children_pregnancy ON children_profiles (pregnancy_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_children_pregnancy_unique
			ON children_profiles (pregnancy_id) WHERE pregnancy_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_growth_child_date ON growth_records (child_id, tanggal_pengukuran)`,

		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'children_profiles_pregnancy_id_fkey'
			) THEN
				ALTER TABLE children_profiles
					ADD CONSTRAINT children_profiles_pregnancy_id_fkey
					FOREIGN KEY (pregnancy_id) REFERENCES pregnancies(id) ON DELETE SET NULL;
			END IF;
		EXCEPTION
			WHEN duplicate_object THEN NULL;
			WHEN others THEN RAISE NOTICE 'constraint pregnancy_id dilewati: %', SQLERRM;
		END $$`,
	}

	for _, q := range pernyataan {
		if _, err := db.Exec(ctx, q); err != nil {
			log.Printf("migrasi gagal: %v", err)
		}
	}
}
