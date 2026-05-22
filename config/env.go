package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadEnv loads env files with priority:
// 1) Existing OS env (never overridden)
// 2) BE/.env
// 3) project root .env
// 4) Go/.env (overrides loaded file values, but not existing OS env)
func LoadEnv() {
	protected := existingEnvKeys()

	for _, path := range []string{
		"../BE/.env",
		"BE/.env",
		"../.env",
		".env",
	} {
		_ = godotenv.Load(path)
	}

	for _, path := range goEnvOverrideCandidates() {
		loadEnvOverridePreservingOS(path, protected)
	}
}

func existingEnvKeys() map[string]bool {
	keys := map[string]bool{}
	for _, entry := range os.Environ() {
		key, _, ok := splitEnvEntry(entry)
		if ok {
			keys[key] = true
		}
	}
	return keys
}

func splitEnvEntry(entry string) (string, string, bool) {
	for index, char := range entry {
		if char == '=' {
			return entry[:index], entry[index+1:], true
		}
	}
	return "", "", false
}

func goEnvOverrideCandidates() []string {
	candidates := []string{"Go/.env"}
	if cwd, err := os.Getwd(); err == nil && filepath.Base(cwd) == "Go" {
		candidates = append(candidates, ".env")
	}
	return candidates
}

func loadEnvOverridePreservingOS(path string, protected map[string]bool) {
	values, err := godotenv.Read(path)
	if err != nil {
		return
	}

	for key, value := range values {
		if protected[key] {
			continue
		}
		_ = os.Setenv(key, value)
	}
}
