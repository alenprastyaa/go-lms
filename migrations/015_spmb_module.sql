ALTER TABLE schools ADD COLUMN IF NOT EXISTS spmb_module_enabled BOOLEAN NOT NULL DEFAULT FALSE;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
    IF NOT EXISTS (
      SELECT 1
      FROM pg_enum e
      JOIN pg_type t ON t.oid = e.enumtypid
      WHERE t.typname = 'user_role' AND e.enumlabel = 'ADMIN_SPMB'
    ) THEN
      ALTER TYPE user_role ADD VALUE 'ADMIN_SPMB';
    END IF;
  END IF;
END $$;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('SUPER_ADMIN', 'ADMIN', 'ADMIN_SPMB', 'SARPRAS', 'KOPERASI', 'BENDAHARA', 'GURU', 'SISWA', 'ORANG_TUA'));

CREATE TABLE IF NOT EXISTS majors (
  id SERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  code TEXT NOT NULL,
  quota INTEGER NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (school_id, code)
);

ALTER TABLE class ADD COLUMN IF NOT EXISTS major_id INTEGER NULL REFERENCES majors(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_majors_school_active ON majors (school_id, is_active, name);
CREATE INDEX IF NOT EXISTS idx_class_major_id ON class (major_id);

CREATE TABLE IF NOT EXISTS spmb_applicants (
  id SERIAL PRIMARY KEY,
  school_id INTEGER NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
  registration_number TEXT NOT NULL UNIQUE,
  access_token_hash TEXT NULL,
  access_token_expires_at TIMESTAMP NULL,
  full_name TEXT NOT NULL,
  birth_place TEXT NULL,
  birth_date DATE NULL,
  gender TEXT NULL,
  nisn TEXT NULL,
  origin_school TEXT NULL,
  parent_name TEXT NULL,
  phone_number TEXT NOT NULL,
  email TEXT NULL,
  address TEXT NULL,
  first_major_id INTEGER NULL REFERENCES majors(id) ON DELETE SET NULL,
  second_major_id INTEGER NULL REFERENCES majors(id) ON DELETE SET NULL,
  third_major_id INTEGER NULL REFERENCES majors(id) ON DELETE SET NULL,
  accepted_major_id INTEGER NULL REFERENCES majors(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'SUBMITTED',
  notes TEXT NULL,
  revision_note TEXT NULL,
  last_link_sent_at TIMESTAMP NULL,
  converted_user_id INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  created_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_spmb_applicants_school_status ON spmb_applicants (school_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_spmb_applicants_school_major ON spmb_applicants (school_id, first_major_id, accepted_major_id);
CREATE INDEX IF NOT EXISTS idx_spmb_applicants_token_hash ON spmb_applicants (access_token_hash);
