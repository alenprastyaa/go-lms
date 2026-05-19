package utils

import "strings"

func NormalizeRoleName(role string) string {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	switch normalized {
	case "SUPERADMIN", "SUPER_ADMINISTRATOR":
		return "SUPER_ADMIN"
	default:
		return normalized
	}
}
