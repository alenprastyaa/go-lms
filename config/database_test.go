package config

import "testing"

func TestGetEnvInt(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "120")
	if got := getEnvInt("DB_MAX_OPEN_CONNS", 10); got != 120 {
		t.Fatalf("getEnvInt valid = %d", got)
	}

	t.Setenv("DB_MAX_OPEN_CONNS", "invalid")
	if got := getEnvInt("DB_MAX_OPEN_CONNS", 10); got != 10 {
		t.Fatalf("getEnvInt invalid = %d", got)
	}

	t.Setenv("DB_MAX_OPEN_CONNS", "-1")
	if got := getEnvInt("DB_MAX_OPEN_CONNS", 10); got != 10 {
		t.Fatalf("getEnvInt negative = %d", got)
	}
}

