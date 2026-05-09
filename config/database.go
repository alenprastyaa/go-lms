package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDatabase() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(getEnvInt("DB_MAX_OPEN_CONNS", 60))
	sqlDB.SetMaxIdleConns(getEnvInt("DB_MAX_IDLE_CONNS", 30))
	sqlDB.SetConnMaxLifetime(time.Duration(getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(getEnvInt("DB_CONN_MAX_IDLE_MINUTES", 10)) * time.Minute)

	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS session_version BIGINT NOT NULL DEFAULT 0`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS current_session_device TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS current_session_user_agent TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS current_session_ip TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS current_session_login_at TIMESTAMP NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS logo_url TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS face_reference_image TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS face_reference_descriptor TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_submissions ADD COLUMN IF NOT EXISTS access_blocked BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_submissions ADD COLUMN IF NOT EXISTS access_code TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_submissions ADD COLUMN IF NOT EXISTS access_code_generated_at TIMESTAMP NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_submissions ADD COLUMN IF NOT EXISTS access_block_reason TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_subjects ADD COLUMN IF NOT EXISTS curriculum_subject_id BIGINT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_subjects ADD COLUMN IF NOT EXISTS curriculum_auto_generated BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	curriculumStatements := []string{
		`CREATE TABLE IF NOT EXISTS curriculum_subjects (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			code TEXT NULL,
			name TEXT NOT NULL,
			description TEXT NULL,
			weekly_hours INT NOT NULL DEFAULT 2,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS curriculum_teacher_loads (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			curriculum_subject_id BIGINT NOT NULL,
			max_weekly_hours INT NOT NULL DEFAULT 0,
			notes TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS curriculum_class_distributions (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			curriculum_teacher_load_id BIGINT NOT NULL,
			class_id BIGINT NOT NULL,
			weekly_hours INT NOT NULL DEFAULT 0,
			notes TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS curriculum_schedule_slots (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			day_name TEXT NOT NULL,
			day_order INT NOT NULL,
			session_order INT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			label TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (school_id, day_order, session_order)
		)`,
		`CREATE TABLE IF NOT EXISTS curriculum_schedule_entries (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			class_id BIGINT NOT NULL,
			curriculum_subject_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			schedule_slot_id BIGINT NOT NULL,
			learning_subject_id BIGINT NULL,
			generated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
	}
	for _, stmt := range curriculumStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

	indexStatements := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_school_role ON users (school_id, role)`,
		`CREATE INDEX IF NOT EXISTS idx_users_school_class_role ON users (school_id, class_id, role)`,
		`CREATE INDEX IF NOT EXISTS idx_users_school_username ON users (school_id, username)`,
		`CREATE INDEX IF NOT EXISTS idx_class_school_wali_guru ON class (school_id, wali_guru_id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_subjects_school_class ON learning_subjects (school_id, class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_subjects_class ON learning_subjects (class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_assignments_subject_due_created ON learning_assignments (subject_id, due_date, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_assignments_subject_semester_created ON learning_assignments (subject_id, semester_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_submissions_assignment_student ON learning_submissions (assignment_id, student_id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_submissions_student_assignment_latest ON learning_submissions (student_id, assignment_id, submitted_at, started_at, id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_subjects_curriculum_subject ON learning_subjects (school_id, curriculum_subject_id, class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_chat_messages_subject_created ON learning_chat_messages (subject_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_chat_reads_subject_user ON learning_chat_reads (subject_id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_user_date ON attendance (user_id, attendance_date)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_date_user ON attendance (attendance_date, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_receipt_user_created ON payment_receipt (user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_receipt_created_user ON payment_receipt (created_at, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_subjects_school ON curriculum_subjects (school_id, name)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_teacher_loads_school_teacher ON curriculum_teacher_loads (school_id, teacher_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_curriculum_teacher_loads_unique_subject ON curriculum_teacher_loads (school_id, teacher_id, curriculum_subject_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_curriculum_class_distributions_unique ON curriculum_class_distributions (school_id, curriculum_teacher_load_id, class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_class_distributions_school_load ON curriculum_class_distributions (school_id, curriculum_teacher_load_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_schedule_slots_school_day ON curriculum_schedule_slots (school_id, day_order, session_order)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_schedule_entries_school_class ON curriculum_schedule_entries (school_id, class_id, schedule_slot_id)`,
		`CREATE TABLE IF NOT EXISTS school_billings (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL UNIQUE,
			billing_name TEXT NOT NULL,
			amount BIGINT NOT NULL DEFAULT 0,
			currency TEXT NOT NULL DEFAULT 'IDR',
			due_date TIMESTAMP NULL,
			due_day_of_month INT NOT NULL DEFAULT 1,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			notes TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE school_billings ADD COLUMN IF NOT EXISTS due_date TIMESTAMP NULL`,
		`CREATE TABLE IF NOT EXISTS school_invoices (
			id BIGSERIAL PRIMARY KEY,
			school_billing_id BIGINT NOT NULL,
			school_id BIGINT NOT NULL,
			invoice_number TEXT NOT NULL UNIQUE,
			amount BIGINT NOT NULL DEFAULT 0,
			due_date TIMESTAMP NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			payment_method TEXT NULL,
			gross_amount BIGINT NULL,
			transaction_id TEXT NULL,
			snap_token TEXT NULL,
			snap_redirect_url TEXT NULL,
			paid_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_school_invoices_school_status ON school_invoices (school_id, status, due_date)`,
	}
	for _, stmt := range indexStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}
	_ = db.Exec(`UPDATE school_billings SET due_date = (DATE_TRUNC('month', CURRENT_DATE) + ((due_day_of_month - 1) || ' days')::interval)::timestamp WHERE due_date IS NULL`).Error

	return db, nil
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
