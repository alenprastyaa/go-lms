package config

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadEnv loads env files with priority:
// 1) Existing OS env (never overridden)
// 2) BE/.env
// 3) ../.env (project root)
// 4) Go/.env (overrides loaded file values)
func LoadEnv() {
	_ = godotenv.Load("../BE/.env")
	_ = godotenv.Load("../.env")

	// Optional local override for Go-only env file
	if _, err := os.Stat(".env"); err == nil {
		_ = godotenv.Overload(".env")
	}
}
