package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type registrationPayload struct {
	SchoolID uint  `json:"school_id"`
	Exp      int64 `json:"exp"`
}

func GenerateSchoolRegistrationToken(schoolID uint, ttl time.Duration) (string, error) {
	if schoolID == 0 {
		return "", fmt.Errorf("invalid school id")
	}
	if ttl <= 0 {
		ttl = 180 * 24 * time.Hour
	}

	payload := registrationPayload{
		SchoolID: schoolID,
		Exp:      time.Now().Add(ttl).Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(raw)
	signature := signRegistrationPayload(payloadPart)
	return payloadPart + "." + signature, nil
}

func ParseSchoolRegistrationToken(token string) (uint, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid registration token")
	}

	payloadPart := parts[0]
	signature := parts[1]
	expected := signRegistrationPayload(payloadPart)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return 0, fmt.Errorf("invalid registration token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil {
		return 0, fmt.Errorf("invalid registration token payload")
	}

	var payload registrationPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return 0, fmt.Errorf("invalid registration token data")
	}
	if payload.SchoolID == 0 {
		return 0, fmt.Errorf("invalid school in registration token")
	}
	if payload.Exp <= time.Now().Unix() {
		return 0, fmt.Errorf("registration token expired")
	}
	return payload.SchoolID, nil
}

func signRegistrationPayload(payloadPart string) string {
	mac := hmac.New(sha256.New, []byte(registrationLinkSecret()))
	_, _ = mac.Write([]byte(payloadPart))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func registrationLinkSecret() string {
	secret := strings.TrimSpace(os.Getenv("REGISTRATION_LINK_SECRET"))
	if secret != "" {
		return secret
	}
	secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret != "" {
		return secret
	}
	return "school-registration-secret-default"
}
