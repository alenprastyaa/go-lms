package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var userRoleValues = []string{"SUPER_ADMIN", "ADMIN", "ADMIN_SPMB", "SARPRAS", "KOPERASI", "BENDAHARA", "GURU", "SISWA", "ORANG_TUA"}

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
	sqlDB.SetMaxOpenConns(getEnvInt("DB_MAX_OPEN_CONNS", 15))
	sqlDB.SetMaxIdleConns(getEnvInt("DB_MAX_IDLE_CONNS", 5))
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
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INT NOT NULL DEFAULT 0`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_locked_until TIMESTAMP NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS logo_url TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS inventory_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_teacher_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_latitude DOUBLE PRECISION NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_longitude DOUBLE PRECISION NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_radius_meters INT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_late_after_time TEXT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_checkout_deadline TEXT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS attendance_seat_map_columns INT NOT NULL DEFAULT 4`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE attendance ADD COLUMN IF NOT EXISTS checkout_note TEXT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS official_exam_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS koperasi_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS private_chat_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS teaching_module_ai_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS payroll_module_enabled BOOLEAN NOT NULL DEFAULT TRUE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS spmb_module_enabled BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE schools ADD COLUMN IF NOT EXISTS personal_teacher_mode_enabled BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS face_reference_image TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS face_reference_descriptor TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS face_reference_change_requests (
		id BIGSERIAL PRIMARY KEY,
		student_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		school_id BIGINT REFERENCES schools(id) ON DELETE CASCADE,
		requested_image TEXT NOT NULL,
		requested_descriptor TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'PENDING',
		admin_note TEXT,
		reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
		reviewed_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_face_reference_change_requests_school_status ON face_reference_change_requests (school_id, status, created_at DESC)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_face_reference_change_requests_one_pending ON face_reference_change_requests (student_id) WHERE status = 'PENDING'`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS initial_password_ciphertext TEXT`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE IF EXISTS school_visit_targets ADD COLUMN IF NOT EXISTS email TEXT`).Error; err != nil {
		return nil, err
	}
	if err := ensurePostgresEnumValues(db, "user_role", userRoleValues); err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('SUPER_ADMIN', 'ADMIN', 'ADMIN_SPMB', 'SARPRAS', 'KOPERASI', 'BENDAHARA', 'GURU', 'SISWA', 'ORANG_TUA'))`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS majors (
		id SERIAL PRIMARY KEY,
		school_id INTEGER NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		code TEXT NOT NULL,
		quota INTEGER NULL,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE (school_id, code)
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS class_levels (
		id SERIAL PRIMARY KEY,
		school_id INTEGER NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
		name TEXT NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE (school_id, name)
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE class ADD COLUMN IF NOT EXISTS major_id INTEGER NULL REFERENCES majors(id) ON DELETE SET NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE class ADD COLUMN IF NOT EXISTS class_level_id INTEGER NULL REFERENCES class_levels(id) ON DELETE SET NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_majors_school_active ON majors (school_id, is_active, name)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_class_levels_school_order ON class_levels (school_id, sort_order, name)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_class_major_id ON class (major_id)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_class_level_id ON class (class_level_id)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS spmb_applicants (
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
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_spmb_applicants_school_status ON spmb_applicants (school_id, status, created_at DESC)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_spmb_applicants_school_major ON spmb_applicants (school_id, first_major_id, accepted_major_id)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_spmb_applicants_token_hash ON spmb_applicants (access_token_hash)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS parent_whatsapp_report_settings (
		school_id INTEGER PRIMARY KEY REFERENCES schools(id) ON DELETE CASCADE,
		enabled BOOLEAN NOT NULL DEFAULT FALSE,
		schedule_type TEXT NOT NULL DEFAULT 'MONTHLY_DATE',
		send_time TEXT NOT NULL DEFAULT '08:00',
		day_of_week INT NOT NULL DEFAULT 1,
		day_of_month INT NOT NULL DEFAULT 1,
		class_id INTEGER NULL REFERENCES class(id) ON DELETE SET NULL,
		last_sent_period TEXT NULL,
		last_sent_at TIMESTAMP NULL,
		updated_by INTEGER NULL REFERENCES users(id) ON DELETE SET NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_parent_wa_report_settings_enabled ON parent_whatsapp_report_settings (enabled, send_time)`).Error; err != nil {
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
	if err := db.Exec(`ALTER TABLE school_visit_targets ADD COLUMN IF NOT EXISTS is_visited BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE school_visit_targets ADD COLUMN IF NOT EXISTS visited_at TIMESTAMP NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE school_visit_targets ADD COLUMN IF NOT EXISTS wakur TEXT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE school_visit_targets ADD COLUMN IF NOT EXISTS kepsek TEXT NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE school_visit_targets ADD COLUMN IF NOT EXISTS is_planned BOOLEAN NOT NULL DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE school_visit_targets ALTER COLUMN is_planned SET DEFAULT FALSE`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE school_visit_targets ADD COLUMN IF NOT EXISTS planned_at TIMESTAMP NULL`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_assignments ADD COLUMN IF NOT EXISTS question_duration_mode TEXT NOT NULL DEFAULT 'PER_QUESTION'`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE learning_assignments ADD COLUMN IF NOT EXISTS max_violations INT NOT NULL DEFAULT 3`).Error; err != nil {
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
			required_room_type TEXT NOT NULL DEFAULT 'CLASSROOM',
			preferred_room_id BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS curriculum_rooms (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			code TEXT NOT NULL,
			name TEXT NOT NULL,
			room_type TEXT NOT NULL DEFAULT 'CLASSROOM',
			color TEXT NOT NULL DEFAULT '#E2E8F0',
			capacity INT NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			notes TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (school_id, code)
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
			room_id BIGINT NULL,
			learning_subject_id BIGINT NULL,
			generated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS private_chat_messages (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			sender_id BIGINT NOT NULL,
			recipient_id BIGINT NOT NULL,
			message TEXT NOT NULL,
			message_type TEXT NOT NULL DEFAULT 'TEXT',
			attachment_url TEXT NULL,
			attachment_name TEXT NULL,
			attachment_mime_type TEXT NULL,
			attachment_size BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS private_chat_reads (
			owner_user_id BIGINT NOT NULL,
			peer_user_id BIGINT NOT NULL,
			last_read_message_id BIGINT NULL,
			last_read_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			PRIMARY KEY (owner_user_id, peer_user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS school_announcements (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			target_audience TEXT NOT NULL DEFAULT 'ALL',
			status TEXT NOT NULL DEFAULT 'DRAFT',
			reviewed_at TIMESTAMP NULL,
			published_at TIMESTAMP NULL,
			deactivated_at TIMESTAMP NULL,
			created_by BIGINT NULL,
			updated_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS student_class_enrollments (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			student_id BIGINT NOT NULL,
			class_id BIGINT NOT NULL,
			academic_year_id BIGINT NULL,
			semester_id BIGINT NULL,
			start_date DATE NOT NULL DEFAULT CURRENT_DATE,
			end_date DATE NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			promotion_note TEXT NULL,
			created_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS parent_students (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			parent_user_id BIGINT NOT NULL,
			student_user_id BIGINT NOT NULL,
			relationship TEXT NOT NULL DEFAULT 'WALI',
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (parent_user_id, student_user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS parent_login_otps (
			id BIGSERIAL PRIMARY KEY,
			parent_user_id BIGINT NOT NULL,
			email TEXT NOT NULL,
			otp_hash TEXT NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			used_at TIMESTAMP NULL,
			attempts INT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS school_visit_targets (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NULL,
			wakur TEXT NULL,
			kepsek TEXT NULL,
			full_address TEXT NULL,
			province TEXT NULL,
			city TEXT NULL,
			district TEXT NULL,
			latitude DOUBLE PRECISION NULL,
			longitude DOUBLE PRECISION NULL,
			google_maps_url TEXT NULL,
			place_id TEXT NULL,
			is_planned BOOLEAN NOT NULL DEFAULT FALSE,
			planned_at TIMESTAMP NULL,
			is_visited BOOLEAN NOT NULL DEFAULT FALSE,
			visited_at TIMESTAMP NULL,
			created_by BIGINT NULL,
			updated_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS marketing_email_offer_logs (
			id BIGSERIAL PRIMARY KEY,
			email TEXT NOT NULL,
			school_name TEXT NULL,
			contact_name TEXT NULL,
			success BOOLEAN NOT NULL DEFAULT FALSE,
			source TEXT NOT NULL DEFAULT 'brevo',
			sent_at TIMESTAMP NOT NULL DEFAULT NOW(),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_items (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			name TEXT NOT NULL,
			code TEXT NULL,
			category TEXT NULL,
			description TEXT NULL,
			condition_status TEXT NOT NULL DEFAULT 'BAIK',
			total_quantity INT NOT NULL DEFAULT 1,
			available_quantity INT NOT NULL DEFAULT 1,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_by BIGINT NULL,
			updated_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS inventory_loans (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			item_id BIGINT NOT NULL,
			borrower_id BIGINT NOT NULL,
			teacher_id BIGINT NULL,
			quantity INT NOT NULL DEFAULT 1,
			borrowed_at TIMESTAMP NOT NULL DEFAULT NOW(),
			due_date TIMESTAMP NULL,
			returned_at TIMESTAMP NULL,
			status TEXT NOT NULL DEFAULT 'BORROWED',
			notes TEXT NULL,
			handled_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS koperasi_products (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			name TEXT NOT NULL,
			code TEXT NULL,
			category TEXT NULL,
			description TEXT NULL,
			image_url TEXT NULL,
			price BIGINT NOT NULL DEFAULT 0,
			stock INT NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_by BIGINT NULL,
			updated_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS koperasi_orders (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			order_number TEXT NOT NULL UNIQUE,
			buyer_id BIGINT NOT NULL,
			buyer_role TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'PENDING',
			payment_method TEXT NULL,
			payment_provider TEXT NULL,
			payment_status TEXT NOT NULL DEFAULT 'CASH_DUE',
			payment_request_id TEXT NULL,
			payment_qr_string TEXT NULL,
			payment_expires_at TIMESTAMP NULL,
			note TEXT NULL,
			total_amount BIGINT NOT NULL DEFAULT 0,
			handled_by BIGINT NULL,
			paid_at TIMESTAMP NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS koperasi_order_items (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			quantity INT NOT NULL DEFAULT 1,
			price BIGINT NOT NULL DEFAULT 0,
			subtotal BIGINT NOT NULL DEFAULT 0,
			product_name_snapshot TEXT NOT NULL,
			product_code_snapshot TEXT NULL,
			product_category_snapshot TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS koperasi_cart_items (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			buyer_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			quantity INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (school_id, buyer_id, product_id)
		)`,
		`CREATE TABLE IF NOT EXISTS koperasi_payment_logs (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			order_id BIGINT NOT NULL,
			event_type TEXT NOT NULL,
			status TEXT NOT NULL,
			payment_request_id TEXT NULL,
			note TEXT NULL,
			metadata TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS payroll_settings (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL UNIQUE,
			hourly_rate BIGINT NOT NULL DEFAULT 40000,
			lesson_minutes INT NOT NULL DEFAULT 45,
			teaching_hours_multiplier NUMERIC(10,2) NOT NULL DEFAULT 4,
			notes TEXT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS payroll_components (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			name TEXT NOT NULL,
			component_type TEXT NOT NULL DEFAULT 'ALLOWANCE',
			calculation_type TEXT NOT NULL DEFAULT 'FIXED',
			default_amount BIGINT NOT NULL DEFAULT 0,
			default_quantity NUMERIC(10,2) NOT NULL DEFAULT 1,
			applies_to_all BOOLEAN NOT NULL DEFAULT FALSE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_by BIGINT NULL,
			updated_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS teacher_payrolls (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			period_month DATE NOT NULL,
			hourly_rate BIGINT NOT NULL DEFAULT 0,
			teaching_hours NUMERIC(10,2) NOT NULL DEFAULT 0,
			base_amount BIGINT NOT NULL DEFAULT 0,
			allowances_amount BIGINT NOT NULL DEFAULT 0,
			deductions_amount BIGINT NOT NULL DEFAULT 0,
			total_amount BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'DRAFT',
			note TEXT NULL,
			paid_at TIMESTAMP NULL,
			created_by BIGINT NULL,
			updated_by BIGINT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE (school_id, teacher_id, period_month)
		)`,
		`CREATE TABLE IF NOT EXISTS teacher_payroll_items (
			id BIGSERIAL PRIMARY KEY,
			payroll_id BIGINT NOT NULL,
			component_id BIGINT NULL,
			name TEXT NOT NULL,
			component_type TEXT NOT NULL DEFAULT 'ALLOWANCE',
			calculation_type TEXT NOT NULL DEFAULT 'FIXED',
			quantity NUMERIC(10,2) NOT NULL DEFAULT 1,
			unit_amount BIGINT NOT NULL DEFAULT 0,
			amount BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
	}
	for _, stmt := range curriculumStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

	curriculumAlterStatements := []string{
		`ALTER TABLE curriculum_subjects ADD COLUMN IF NOT EXISTS required_room_type TEXT NOT NULL DEFAULT 'CLASSROOM'`,
		`ALTER TABLE curriculum_subjects ADD COLUMN IF NOT EXISTS preferred_room_id BIGINT NULL`,
		`ALTER TABLE curriculum_schedule_entries ADD COLUMN IF NOT EXISTS room_id BIGINT NULL`,
	}
	for _, stmt := range curriculumAlterStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

	chatAlterStatements := []string{
		`ALTER TABLE IF EXISTS private_chat_messages ADD COLUMN IF NOT EXISTS attachment_preview_url TEXT NULL`,
		`ALTER TABLE IF EXISTS learning_chat_messages ADD COLUMN IF NOT EXISTS attachment_preview_url TEXT NULL`,
		`ALTER TABLE IF EXISTS private_chat_messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMP NULL`,
		`ALTER TABLE IF EXISTS learning_chat_messages ADD COLUMN IF NOT EXISTS edited_at TIMESTAMP NULL`,
	}
	for _, stmt := range chatAlterStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

	koperasiAlterStatements := []string{
		`ALTER TABLE koperasi_orders ADD COLUMN IF NOT EXISTS payment_provider TEXT NULL`,
		`ALTER TABLE koperasi_orders ADD COLUMN IF NOT EXISTS payment_status TEXT NOT NULL DEFAULT 'CASH_DUE'`,
		`ALTER TABLE koperasi_orders ADD COLUMN IF NOT EXISTS payment_request_id TEXT NULL`,
		`ALTER TABLE koperasi_orders ADD COLUMN IF NOT EXISTS payment_qr_string TEXT NULL`,
		`ALTER TABLE koperasi_orders ADD COLUMN IF NOT EXISTS payment_expires_at TIMESTAMP NULL`,
		`ALTER TABLE koperasi_orders ADD COLUMN IF NOT EXISTS paid_at TIMESTAMP NULL`,
		`ALTER TABLE koperasi_cart_items ADD COLUMN IF NOT EXISTS school_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE koperasi_cart_items ADD COLUMN IF NOT EXISTS buyer_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE koperasi_cart_items ADD COLUMN IF NOT EXISTS product_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE koperasi_cart_items ADD COLUMN IF NOT EXISTS quantity INT NOT NULL DEFAULT 1`,
	}
	for _, stmt := range koperasiAlterStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

	if err := db.Exec(`ALTER TABLE inventory_loans ADD COLUMN IF NOT EXISTS teacher_id BIGINT NULL`).Error; err != nil {
		return nil, err
	}

	payrollAlterStatements := []string{
		`ALTER TABLE payroll_settings ADD COLUMN IF NOT EXISTS teaching_hours_multiplier NUMERIC(10,2) NOT NULL DEFAULT 4`,
		`ALTER TABLE payroll_components ADD COLUMN IF NOT EXISTS calculation_type TEXT NOT NULL DEFAULT 'FIXED'`,
		`ALTER TABLE payroll_components ADD COLUMN IF NOT EXISTS default_quantity NUMERIC(10,2) NOT NULL DEFAULT 1`,
		`ALTER TABLE payroll_components ADD COLUMN IF NOT EXISTS applies_to_all BOOLEAN NOT NULL DEFAULT FALSE`,
		`ALTER TABLE teacher_payroll_items ADD COLUMN IF NOT EXISTS calculation_type TEXT NOT NULL DEFAULT 'FIXED'`,
		`ALTER TABLE teacher_payroll_items ADD COLUMN IF NOT EXISTS quantity NUMERIC(10,2) NOT NULL DEFAULT 1`,
		`ALTER TABLE teacher_payroll_items ADD COLUMN IF NOT EXISTS unit_amount BIGINT NOT NULL DEFAULT 0`,
		`UPDATE teacher_payroll_items SET unit_amount = amount WHERE COALESCE(unit_amount, 0) = 0`,
	}
	for _, stmt := range payrollAlterStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

	indexStatements := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_school_role ON users (school_id, role)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users (username)`,
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
		`CREATE INDEX IF NOT EXISTS idx_private_chat_messages_school_pair_created ON private_chat_messages (school_id, sender_id, recipient_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_private_chat_messages_recipient_created ON private_chat_messages (recipient_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_private_chat_reads_owner_peer ON private_chat_reads (owner_user_id, peer_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_cart_items_school_buyer ON koperasi_cart_items (school_id, buyer_id)`,
		`CREATE INDEX IF NOT EXISTS idx_student_class_enrollments_school_class ON student_class_enrollments (school_id, class_id, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_student_class_enrollments_student ON student_class_enrollments (student_id, start_date, end_date)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_student_class_enrollments_one_active ON student_class_enrollments (student_id) WHERE is_active = true`,
		`CREATE INDEX IF NOT EXISTS idx_parent_students_parent ON parent_students (parent_user_id, school_id)`,
		`CREATE INDEX IF NOT EXISTS idx_parent_students_student ON parent_students (student_user_id, school_id)`,
		`CREATE INDEX IF NOT EXISTS idx_parent_login_otps_parent_created ON parent_login_otps (parent_user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_parent_login_otps_email_created ON parent_login_otps (email, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_school_visit_targets_created ON school_visit_targets (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_school_visit_targets_name ON school_visit_targets (name)`,
		`CREATE INDEX IF NOT EXISTS idx_school_visit_targets_planned ON school_visit_targets (is_planned, planned_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_school_visit_targets_visited ON school_visit_targets (is_visited, visited_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_marketing_email_offer_logs_email_success ON marketing_email_offer_logs (LOWER(email), success, sent_at DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_items_school_code ON inventory_items (school_id, code) WHERE code IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_items_school_active ON inventory_items (school_id, is_active, name)`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_loans_school_status ON inventory_loans (school_id, status, borrowed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_loans_item_status ON inventory_loans (item_id, status, borrowed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_loans_borrower ON inventory_loans (borrower_id, borrowed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_loans_teacher ON inventory_loans (teacher_id, borrowed_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_koperasi_products_school_code ON koperasi_products (school_id, code) WHERE code IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_products_school_active ON koperasi_products (school_id, is_active, name)`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_products_school_stock ON koperasi_products (school_id, stock, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_orders_school_status ON koperasi_orders (school_id, status, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_orders_buyer_created ON koperasi_orders (buyer_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_payment_logs_school_order_created ON koperasi_payment_logs (school_id, order_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_koperasi_order_items_order ON koperasi_order_items (order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payroll_components_school_active ON payroll_components (school_id, is_active, name)`,
		`CREATE INDEX IF NOT EXISTS idx_teacher_payrolls_school_period ON teacher_payrolls (school_id, period_month, status)`,
		`CREATE INDEX IF NOT EXISTS idx_teacher_payrolls_teacher_period ON teacher_payrolls (teacher_id, period_month)`,
		`CREATE INDEX IF NOT EXISTS idx_teacher_payroll_items_payroll ON teacher_payroll_items (payroll_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_user_date ON attendance (user_id, attendance_date)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_date_user ON attendance (attendance_date, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_receipt_user_created ON payment_receipt (user_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_receipt_created_user ON payment_receipt (created_at, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_subjects_school ON curriculum_subjects (school_id, name)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_rooms_school_type ON curriculum_rooms (school_id, room_type, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_teacher_loads_school_teacher ON curriculum_teacher_loads (school_id, teacher_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_curriculum_teacher_loads_unique_subject ON curriculum_teacher_loads (school_id, teacher_id, curriculum_subject_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_curriculum_class_distributions_unique ON curriculum_class_distributions (school_id, curriculum_teacher_load_id, class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_class_distributions_school_load ON curriculum_class_distributions (school_id, curriculum_teacher_load_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_schedule_slots_school_day ON curriculum_schedule_slots (school_id, day_order, session_order)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_schedule_entries_school_class ON curriculum_schedule_entries (school_id, class_id, schedule_slot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_curriculum_schedule_entries_school_room ON curriculum_schedule_entries (school_id, room_id, schedule_slot_id)`,
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
		`CREATE TABLE IF NOT EXISTS push_subscriptions (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			endpoint TEXT NOT NULL UNIQUE,
			p256dh TEXT NOT NULL,
			auth TEXT NOT NULL,
			expiration_time TIMESTAMP NULL,
			subscription_json TEXT NOT NULL,
			user_agent TEXT NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS learning_teaching_modules (
			id BIGSERIAL PRIMARY KEY,
			school_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			subject_id BIGINT NOT NULL,
			title TEXT NOT NULL,
			topic TEXT NOT NULL,
			grade_label TEXT NULL,
			phase_name TEXT NULL,
			curriculum_name TEXT NULL,
			time_allocation TEXT NOT NULL,
			meetings INT NOT NULL DEFAULT 1,
			learning_model TEXT NULL,
			status TEXT NOT NULL DEFAULT 'DRAFT',
			input_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			draft_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_school_invoices_school_status ON school_invoices (school_id, status, due_date)`,
		`CREATE INDEX IF NOT EXISTS idx_push_subscriptions_school_user ON push_subscriptions (school_id, user_id, is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_teaching_modules_subject_updated ON learning_teaching_modules (subject_id, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_teaching_modules_teacher_subject ON learning_teaching_modules (teacher_id, subject_id, created_at DESC)`,
	}
	for _, stmt := range indexStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Exec(`
		INSERT INTO student_class_enrollments (
			school_id, student_id, class_id, start_date, is_active, promotion_note, created_at, updated_at
		)
		SELECT u.school_id, u.id, u.class_id, CURRENT_DATE, true, 'Migrasi awal dari kelas aktif siswa', NOW(), NOW()
		FROM users u
		WHERE u.role = 'SISWA'
		  AND u.school_id IS NOT NULL
		  AND u.class_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1
			FROM student_class_enrollments sce
			WHERE sce.student_id = u.id
			  AND sce.is_active = true
		  )
	`).Error; err != nil {
		return nil, err
	}
	_ = db.Exec(`UPDATE school_billings SET due_date = (DATE_TRUNC('month', CURRENT_DATE) + ((due_day_of_month - 1) || ' days')::interval)::timestamp WHERE due_date IS NULL`).Error

	if err := db.Exec(`CREATE TABLE IF NOT EXISTS lms_packages (
		id BIGSERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		tagline TEXT NOT NULL DEFAULT '',
		price NUMERIC(14,2) NOT NULL DEFAULT 0,
		original_price NUMERIC(14,2) NULL,
		billing_period TEXT NOT NULL DEFAULT 'bulan',
		badge_text TEXT NOT NULL DEFAULT '',
		logo_url TEXT NOT NULL DEFAULT '',
		is_popular BOOLEAN NOT NULL DEFAULT FALSE,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		sort_order INTEGER NOT NULL DEFAULT 0,
		cta_label TEXT NOT NULL DEFAULT 'Pilih Paket',
		cta_url TEXT NOT NULL DEFAULT '',
		modules JSONB NOT NULL DEFAULT '[]'::jsonb,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_lms_packages_active_order ON lms_packages (is_active, sort_order, id)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`ALTER TABLE lms_packages ADD COLUMN IF NOT EXISTS logo_url TEXT NOT NULL DEFAULT ''`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS package_checkout_orders (
		id BIGSERIAL PRIMARY KEY,
		reference_id TEXT NOT NULL UNIQUE,
		package_id BIGINT NOT NULL REFERENCES lms_packages(id) ON DELETE RESTRICT,
		package_name TEXT NOT NULL,
		school_name TEXT NOT NULL,
		email TEXT NOT NULL,
		amount BIGINT NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'PENDING',
		payment_method TEXT NULL,
		transaction_id TEXT NULL,
		payment_url TEXT NULL,
		invoice_sent_at TIMESTAMP NULL,
		credential_sent_at TIMESTAMP NULL,
		school_id BIGINT NULL REFERENCES schools(id) ON DELETE SET NULL,
		admin_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
		paid_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_package_checkout_orders_status ON package_checkout_orders (status, created_at DESC)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_package_checkout_orders_email ON package_checkout_orders (LOWER(email), created_at DESC)`).Error; err != nil {
		return nil, err
	}
	if err := seedDefaultPackages(db); err != nil {
		return nil, err
	}
	if err := ensureLandingTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func ensureLandingTables(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS landing_sections (
		id BIGSERIAL PRIMARY KEY,
		section_key TEXT NOT NULL UNIQUE,
		label TEXT NOT NULL DEFAULT '',
		content JSONB NOT NULL DEFAULT '{}'::jsonb,
		is_active BOOLEAN NOT NULL DEFAULT TRUE,
		sort_order INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_landing_sections_active_order ON landing_sections (is_active, sort_order, section_key)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS landing_blog_posts (
		id BIGSERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		slug TEXT NOT NULL UNIQUE,
		excerpt TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		cover_image_url TEXT NOT NULL DEFAULT '',
		author_name TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL DEFAULT '',
		tags TEXT NOT NULL DEFAULT '',
		meta_title TEXT NOT NULL DEFAULT '',
		meta_description TEXT NOT NULL DEFAULT '',
		is_published BOOLEAN NOT NULL DEFAULT FALSE,
		is_featured BOOLEAN NOT NULL DEFAULT FALSE,
		sort_order INTEGER NOT NULL DEFAULT 0,
		published_at TIMESTAMP NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_landing_blog_posts_public ON landing_blog_posts (is_published, is_featured, sort_order, published_at DESC, id DESC)`).Error; err != nil {
		return err
	}
	return seedLandingSections(db)
}

func seedLandingSections(db *gorm.DB) error {
	type seedSection struct {
		key, label, content string
		sort                int
	}
	seeds := []seedSection{
		{"seo", "SEO & Metadata", `{"title":"School System LMS - Bitwize Digital Platform","description":"Platform LMS dan operasional sekolah untuk pembelajaran, absensi, ujian, billing, SPMB, koperasi, payroll, dan komunikasi."}`, 1},
		{"brand", "Brand", `{"name":"Bitwize Digital Platform","shortName":"Bitwize","logoUrl":"/logob.png","tagline":"Digital Platform"}`, 2},
		{"hero", "Hero / Jumbotron", `{"eyebrow":"","title":"Rapikan operasional sekolah dalam satu sistem yang bisa langsung dipakai.","subtitle":"Bitwize membantu sekolah mengelola pembelajaran, absensi, ujian, tagihan, SPMB, koperasi, payroll, dan komunikasi tanpa rekap manual yang berulang.","primaryLabel":"Konsultasi Paket","primaryHref":"#harga","secondaryLabel":"Cek Modul","secondaryHref":"#fitur"}`, 3},
		{"metrics", "Hero Metrics", `{"items":[{"value":"10+","label":"Modul operasional"},{"value":"4","label":"Role pengguna utama"},{"value":"1","label":"Sumber data sekolah"},{"value":"24/7","label":"Akses online"}]}`, 4},
		{"dashboard", "Dashboard Preview", `{"schoolName":"SMA Nusantara","summaryCards":[{"icon":"ph:buildings","label":"Total Kelas","value":"36","caption":"Rombel aktif","cardClass":"bg-sky-600"},{"icon":"ph:student","label":"Total Siswa","value":"1.248","caption":"Siswa terdaftar","cardClass":"bg-amber-500"},{"icon":"ph:chalkboard-teacher","label":"Total Guru","value":"84","caption":"Pengajar aktif","cardClass":"bg-emerald-600"},{"icon":"ph:clipboard-text","label":"Tugas Aktif","value":"36","caption":"Berjalan minggu ini","cardClass":"bg-indigo-600"}]}`, 5},
		{"pain", "Masalah yang Diselesaikan", `{"eyebrow":"Masalah yang diselesaikan","title":"Stop rekap manual. Satukan data sekolah sebelum pekerjaan makin menumpuk.","items":[{"icon":"ph:files","title":"Rekap manual menghabiskan jam kerja","desc":"Absensi, nilai, tagihan, dan laporan tidak perlu dipindahkan berkali-kali ke file berbeda."},{"icon":"ph:chat-circle-dots","title":"Informasi sekolah harus mudah dicari","desc":"Pengumuman, tugas, dan status pembayaran punya tempat yang jelas, bukan tercecer di chat."},{"icon":"ph:chart-line-down","title":"Manajemen butuh data hari ini","desc":"Pimpinan bisa membaca kondisi sekolah tanpa menunggu laporan manual selesai dibuat."}]}`, 6},
		{"features", "Fitur / Modul", `{"eyebrow":"Modul produk","title":"Modul penting untuk menjalankan sekolah modern tanpa alat yang terpisah-pisah.","ctaLabel":"Bandingkan paket","ctaHref":"#harga","items":[{"icon":"ph:warehouse","title":"Sarpras","desc":"Catat aset sekolah, kondisi barang, dan kebutuhan fasilitas tanpa spreadsheet terpisah."},{"icon":"ph:calendar-check","title":"Absensi Siswa & Guru","desc":"Kehadiran harian, keterlambatan, dan rekap kelas otomatis masuk laporan."},{"icon":"ph:exam","title":"Ujian Resmi","desc":"Bank soal, jadwal ujian, pengerjaan online, dan hasil evaluasi dalam satu alur."},{"icon":"ph:notebook","title":"Modul Ajar AI","desc":"Bantu guru menyiapkan rancangan modul ajar lebih cepat dan tetap bisa diedit."},{"icon":"ph:storefront","title":"Koperasi","desc":"Transaksi koperasi sekolah tercatat rapi untuk dipantau admin."},{"icon":"ph:money","title":"Payroll","desc":"Komponen gaji, slip payroll, dan pembayaran pegawai dikelola lebih tertib."}]}`, 7},
		{"workflow", "Alur Kerja", `{"eyebrow":"Cara kerja harian","title":"Alur kerja dibuat jelas: input sekali, dipakai banyak bagian.","description":"Data yang dimasukkan admin, guru, dan siswa bergerak ke rekap yang bisa dibaca manajemen.","items":[{"title":"Admin input data inti","desc":"Kelas, siswa, guru, jurusan, tahun ajaran, dan modul aktif disiapkan sebagai dasar sistem."},{"title":"Guru menjalankan kelas","desc":"Materi, tugas, ujian, nilai, dan absensi dikelola dari tampilan guru."},{"title":"Siswa mengikuti instruksi","desc":"Tugas, pengumuman, hasil belajar, dan informasi sekolah lebih mudah ditemukan."},{"title":"Manajemen membaca angka","desc":"Data operasional terkumpul untuk evaluasi harian, mingguan, atau bulanan."}]}`, 8},
		{"roles", "Target Role", `{"eyebrow":"Untuk semua peran","title":"Setiap peran mendapat menu yang tepat, tanpa akses yang membingungkan.","items":[{"icon":"ph:briefcase","title":"Admin Sekolah","desc":"Mengendalikan data akademik, absensi, pembayaran, dan laporan operasional."},{"icon":"ph:chalkboard-teacher","title":"Guru","desc":"Mengajar, memberi tugas, membuat ujian, menilai, dan memantau kelas."},{"icon":"ph:student","title":"Siswa & Orang Tua","desc":"Melihat tugas, informasi sekolah, hasil belajar, dan riwayat pembayaran."}]}`, 9},
		{"pricing", "Pricing Intro", `{"eyebrow":"Paket berlangganan","title":"Pilih paket yang jelas. Aktifkan modul sesuai kebutuhan sekolah.","description":"Paket dapat disesuaikan dari modul yang tersedia, sehingga sekolah tidak membayar fitur yang belum dipakai."}`, 10},
		{"blog", "Blog Section", `{"eyebrow":"Blog","title":"Insight terbaru untuk operasional sekolah digital.","description":"Artikel praktis tentang LMS, absensi, pembayaran, SPMB, dan manajemen sekolah.","ctaLabel":"Lihat semua artikel","ctaHref":"/blog"}`, 11},
		{"faq", "FAQ", `{"eyebrow":"FAQ","title":"Pertanyaan umum","items":[{"q":"Apakah modul bisa dipilih sesuai kebutuhan sekolah?","a":"Bisa. Sekolah dapat mulai dari modul utama lalu menambah fitur ketika operasional sudah membutuhkan."},{"q":"Apakah sistem ini hanya untuk pembelajaran online?","a":"Tidak. Bitwize mencakup LMS, absensi, ujian, SPMB, koperasi, payroll, billing, dan komunikasi."},{"q":"Apakah tersedia demo sebelum berlangganan?","a":"Ya. Sekolah dapat menghubungi tim Bitwize melalui WhatsApp atau email untuk menjadwalkan demo singkat."}]}`, 12},
		{"contact", "Kontak", `{"eyebrow":"Kontak","title":"Siap melihat sistemnya berjalan?","description":"Hubungi Bitwize Digital Platform untuk konsultasi paket, demo singkat, atau penyesuaian modul sekolah.","phone":"085719578195","email":"bitwizedigitalplatform@gmail.com","address":"Jalan Harun II No.126 A, Palmerah, Jakarta Barat","whatsappUrl":"https://wa.me/6285719578195","mapUrl":"https://www.google.com/maps/search/?api=1&query=Jalan%20Harun%20II%20No.126%20A%2C%20Palmerah%2C%20Jakarta%20Barat"}`, 13},
		{"cta", "CTA Penutup", `{"eyebrow":"Siap dipakai sekolah Anda","title":"Mulai dari modul paling mendesak. Kembangkan saat sekolah siap.","description":"Kami bantu sekolah masuk ke sistem digital secara bertahap, tanpa mengganggu operasional harian.","primaryLabel":"Pilih Paket","primaryHref":"#harga","secondaryLabel":"Login","secondaryHref":"/auth/login"}`, 14},
		{"footer", "Footer", `{"description":"Platform LMS dan administrasi untuk membantu sekolah mengelola pembelajaran, absensi, tagihan, SPMB, payroll, dan komunikasi dalam satu sistem.","badges":["LMS","Absensi","SPMB","Billing","Payroll"]}`, 15},
	}
	for _, seed := range seeds {
		if err := db.Exec(`INSERT INTO landing_sections (section_key, label, content, sort_order, created_at, updated_at)
			VALUES (?, ?, ?::jsonb, ?, NOW(), NOW())
			ON CONFLICT (section_key) DO NOTHING`, seed.key, seed.label, seed.content, seed.sort).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedDefaultPackages(db *gorm.DB) error {
	var count int64
	if err := db.Table("lms_packages").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	type seedPkg struct {
		name, tagline, badge, ctaLabel, ctaURL string
		price, original                        float64
		popular                                bool
		sort                                   int
		modules                                string
	}
	wa := "https://wa.me/6281234567890?text=Halo%2C%20saya%20tertarik%20paket%20"
	seeds := []seedPkg{
		{
			name: "Starter", tagline: "Untuk sekolah / guru yang baru mulai digitalisasi.",
			price: 149000, original: 249000, sort: 1, ctaLabel: "Pilih Starter", ctaURL: wa + "Starter",
			modules: `[
				{"label":"Manajemen Kelas & Siswa","icon":"ph:users-three","included":true},
				{"label":"Absensi Digital","icon":"ph:calendar-check","included":true},
				{"label":"Materi & Tugas","icon":"ph:book-open","included":true},
				{"label":"Ujian & Bank Soal","icon":"ph:exam","included":false},
				{"label":"Modul Ajar AI","icon":"ph:robot","included":false},
				{"label":"Payroll & Keuangan","icon":"ph:wallet","included":false}
			]`,
		},
		{
			name: "Pro", tagline: "Paket terlengkap untuk sekolah yang serius bertumbuh.",
			price: 349000, original: 499000, badge: "Paling Populer", popular: true, sort: 2,
			ctaLabel: "Pilih Pro", ctaURL: wa + "Pro",
			modules: `[
				{"label":"Manajemen Kelas & Siswa","icon":"ph:users-three","included":true},
				{"label":"Absensi Digital","icon":"ph:calendar-check","included":true},
				{"label":"Materi & Tugas","icon":"ph:book-open","included":true},
				{"label":"Ujian & Bank Soal","icon":"ph:exam","included":true},
				{"label":"Modul Ajar AI","icon":"ph:robot","included":true},
				{"label":"Payroll & Keuangan","icon":"ph:wallet","included":false}
			]`,
		},
		{
			name: "Enterprise", tagline: "Skala yayasan / multi-sekolah dengan dukungan penuh.",
			price: 749000, original: 999000, badge: "Skala Besar", sort: 3,
			ctaLabel: "Hubungi Sales", ctaURL: wa + "Enterprise",
			modules: `[
				{"label":"Semua fitur paket Pro","icon":"ph:check-circle","included":true},
				{"label":"Payroll & Keuangan","icon":"ph:wallet","included":true},
				{"label":"Multi-Sekolah / Yayasan","icon":"ph:buildings","included":true},
				{"label":"SPMB / PPDB Online","icon":"ph:identification-card","included":true},
				{"label":"Prioritas Support 24/7","icon":"ph:headset","included":true},
				{"label":"Onboarding & Pelatihan","icon":"ph:graduation-cap","included":true}
			]`,
		},
	}

	for _, s := range seeds {
		if err := db.Exec(`INSERT INTO lms_packages
			(name, tagline, price, original_price, billing_period, badge_text, is_popular, is_active, sort_order, cta_label, cta_url, modules)
			VALUES (?, ?, ?, ?, 'bulan', ?, ?, TRUE, ?, ?, ?, ?::jsonb)`,
			s.name, s.tagline, s.price, s.original, s.badge, s.popular, s.sort, s.ctaLabel, s.ctaURL, s.modules,
		).Error; err != nil {
			return err
		}
	}
	return nil
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

func ensurePostgresEnumValues(db *gorm.DB, enumName string, values []string) error {
	var exists bool
	if err := db.Raw(`SELECT to_regtype(?) IS NOT NULL`, enumName).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return nil
	}

	quotedEnumName := quotePostgresIdentifier(enumName)
	for _, value := range values {
		stmt := fmt.Sprintf(
			`ALTER TYPE %s ADD VALUE IF NOT EXISTS %s`,
			quotedEnumName,
			quotePostgresLiteral(value),
		)
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

func quotePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quotePostgresLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}
