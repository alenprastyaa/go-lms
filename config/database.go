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

	indexStatements := []string{
		`CREATE INDEX IF NOT EXISTS idx_users_school_role ON users (school_id, role)`,
		`CREATE INDEX IF NOT EXISTS idx_users_school_class_role ON users (school_id, class_id, role)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_subjects_school_class ON learning_subjects (school_id, class_id)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_chat_messages_subject_created ON learning_chat_messages (subject_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_chat_reads_subject_user ON learning_chat_reads (subject_id, user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_user_date ON attendance (user_id, attendance_date)`,
		`CREATE INDEX IF NOT EXISTS idx_payment_receipt_user_created ON payment_receipt (user_id, created_at)`,
	}
	for _, stmt := range indexStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return nil, err
		}
	}

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
