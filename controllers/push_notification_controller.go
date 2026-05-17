package controllers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lms/models"
	"lms/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
)

type pushSubscriptionKeys struct {
	Auth   string `json:"auth"`
	P256DH string `json:"p256dh"`
}

type pushSubscriptionPayload struct {
	Endpoint       string               `json:"endpoint"`
	ExpirationTime *float64             `json:"expirationTime"`
	Keys           pushSubscriptionKeys `json:"keys"`
}

func (a *AppContext) UpsertPushSubscription(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)

	var payload pushSubscriptionPayload
	if err := c.BodyParser(&payload); err != nil {
		return utils.Error(c, 400, "Payload subscription tidak valid")
	}

	endpoint := strings.TrimSpace(payload.Endpoint)
	auth := strings.TrimSpace(payload.Keys.Auth)
	p256dh := strings.TrimSpace(payload.Keys.P256DH)
	if endpoint == "" || auth == "" || p256dh == "" {
		return utils.Error(c, 400, "Data subscription tidak lengkap")
	}

	var expirationTime *time.Time
	if payload.ExpirationTime != nil && *payload.ExpirationTime > 0 {
		t := time.UnixMilli(int64(*payload.ExpirationTime))
		expirationTime = &t
	}

	rawBody := strings.TrimSpace(string(c.Body()))
	if rawBody == "" {
		rawBody = "{}"
	}
	if _, err := json.Marshal(payload); err != nil {
		return utils.Error(c, 400, "Subscription tidak valid")
	}

	userAgent := strings.TrimSpace(c.Get("User-Agent"))
	record := models.PushSubscription{
		SchoolID:         schoolID,
		UserID:           userID,
		Endpoint:         endpoint,
		P256DH:           p256dh,
		Auth:             auth,
		ExpirationTime:   expirationTime,
		SubscriptionJSON: rawBody,
		IsActive:         true,
	}
	if userAgent != "" {
		record.UserAgent = &userAgent
	}

	if err := a.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"school_id",
			"user_id",
			"p256dh",
			"auth",
			"expiration_time",
			"subscription_json",
			"user_agent",
			"is_active",
			"updated_at",
		}),
	}).Create(&record).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan subscription push")
	}

	return utils.Success(c, 200, "Subscription push tersimpan", fiber.Map{
		"endpoint": endpoint,
	})
}

func (a *AppContext) GetVapidPublicKey(c *fiber.Ctx) error {
	publicKey, _, _, err := loadVapidConfig()
	if err != nil {
		return utils.Error(c, 500, err.Error())
	}

	return utils.Success(c, 200, "Success Get VAPID Public Key", fiber.Map{
		"public_key": publicKey,
	})
}

func (a *AppContext) DeletePushSubscription(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)

	var payload struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.BodyParser(&payload); err != nil {
		return utils.Error(c, 400, "Payload tidak valid")
	}

	endpoint := strings.TrimSpace(payload.Endpoint)
	if endpoint == "" {
		return utils.Error(c, 400, "Endpoint subscription wajib diisi")
	}

	if err := a.DB.Where("endpoint = ? AND user_id = ? AND school_id = ?", endpoint, userID, schoolID).
		Delete(&models.PushSubscription{}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus subscription push")
	}

	return utils.Success(c, 200, "Subscription push dihapus", fiber.Map{
		"endpoint": endpoint,
	})
}

func announcementPushRoles(targetAudience string) []string {
	switch strings.ToUpper(strings.TrimSpace(targetAudience)) {
	case announcementTargetAll:
		return []string{"ADMIN", "GURU", "SISWA", "SARPRAS", "KOPERASI"}
	case announcementTargetAdmin:
		return []string{"ADMIN"}
	case announcementTargetGuru:
		return []string{"GURU"}
	case announcementTargetSiswa:
		return []string{"SISWA"}
	case announcementTargetSarpras:
		return []string{"SARPRAS"}
	case announcementTargetKoperasi:
		return []string{"KOPERASI"}
	default:
		return []string{}
	}
}

func assignmentKindLabel(assignmentType string) string {
	switch strings.ToUpper(strings.TrimSpace(assignmentType)) {
	case "MCQ", "ESSAY":
		return "Quiz"
	case "FILE", "MANUAL":
		return "Tugas"
	default:
		return "Pembelajaran"
	}
}

func previewPushText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.EqualFold(trimmed, "<nil>") {
		return "Pesan baru"
	}
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	if limit < 4 {
		return trimmed[:limit]
	}

	cutoff := limit - 3
	if cutoff > len(trimmed) {
		cutoff = len(trimmed)
	}
	return strings.TrimSpace(trimmed[:cutoff]) + "..."
}

func sanitizePushText(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.EqualFold(trimmed, "<nil>") {
		return fallback
	}
	return trimmed
}

func subjectChatRoute(role string, subjectID interface{}) string {
	subject := fmt.Sprint(subjectID)
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "GURU":
		return "/learning-chat-teacher?subject=" + subject
	case "SISWA":
		return "/learning-chat-student?subject=" + subject
	default:
		return "/dashboard"
	}
}

func assignmentRoute(assignmentType string, subjectID interface{}, assignmentID interface{}) string {
	subject := fmt.Sprint(subjectID)
	id := fmt.Sprint(assignmentID)
	switch strings.ToUpper(strings.TrimSpace(assignmentType)) {
	case "MCQ", "ESSAY":
		return "/learning-quiz-student?subject=" + subject + "&assignment=" + id
	default:
		return "/learning-student?subject=" + subject + "&assignment=" + id
	}
}

func announcementRouteForRole(role string, announcementID interface{}) string {
	id := fmt.Sprint(announcementID)
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "ADMIN":
		return "/announcements?announcement=" + id
	default:
		return "/dashboard?announcement=" + id
	}
}
