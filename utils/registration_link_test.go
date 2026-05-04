package utils

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGenerateAndParseSchoolRegistrationToken(t *testing.T) {
	t.Setenv("REGISTRATION_LINK_SECRET", "unit-test-secret")
	token, err := GenerateSchoolRegistrationToken(99, 2*time.Minute)
	if err != nil {
		t.Fatalf("GenerateSchoolRegistrationToken error: %v", err)
	}
	if strings.TrimSpace(token) == "" {
		t.Fatalf("token should not be empty")
	}
	schoolID, err := ParseSchoolRegistrationToken(token)
	if err != nil {
		t.Fatalf("ParseSchoolRegistrationToken error: %v", err)
	}
	if schoolID != 99 {
		t.Fatalf("schoolID = %d, want 99", schoolID)
	}
}

func TestGenerateSchoolRegistrationTokenInvalidSchoolID(t *testing.T) {
	t.Setenv("REGISTRATION_LINK_SECRET", "unit-test-secret")
	if _, err := GenerateSchoolRegistrationToken(0, time.Minute); err == nil {
		t.Fatalf("expected error for schoolID=0")
	}
}

func TestParseSchoolRegistrationToken_InvalidCases(t *testing.T) {
	t.Setenv("REGISTRATION_LINK_SECRET", "unit-test-secret")

	if _, err := ParseSchoolRegistrationToken("abc"); err == nil {
		t.Fatalf("expected error for malformed token")
	}

	expired, err := GenerateSchoolRegistrationToken(7, -1*time.Minute)
	if err != nil {
		t.Fatalf("unexpected generate error: %v", err)
	}
	// ttl <= 0 uses default ttl, so build a real expired token by mutating payload/signature mismatch
	if _, err := ParseSchoolRegistrationToken(expired + "x"); err == nil {
		t.Fatalf("expected signature error")
	}
}

func TestRegistrationLinkSecretFallback(t *testing.T) {
	_ = os.Unsetenv("REGISTRATION_LINK_SECRET")
	_ = os.Unsetenv("JWT_SECRET")
	if got := registrationLinkSecret(); got != "school-registration-secret-default" {
		t.Fatalf("default secret mismatch: %q", got)
	}

	t.Setenv("JWT_SECRET", "jwt-fallback")
	if got := registrationLinkSecret(); got != "jwt-fallback" {
		t.Fatalf("jwt fallback mismatch: %q", got)
	}
}

