package controllers

import (
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

func (a *AppContext) SubmitFaceEnrollment(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)

	file, err := c.FormFile("face_reference_image")
	if err != nil || file == nil {
		return utils.Error(c, 400, "Foto wajah wajib dikirim")
	}
	descriptor := strings.TrimSpace(c.FormValue("face_reference_descriptor"))
	if descriptor == "" {
		return utils.Error(c, 400, "Descriptor wajah wajib dikirim")
	}

	saved, err := utils.SaveUploadedFile(c, file)
	if err != nil {
		return utils.Error(c, 500, "Gagal upload foto wajah", err.Error())
	}

	var current struct {
		Role                    string  `gorm:"column:role"`
		FaceReferenceImage      *string `gorm:"column:face_reference_image"`
		FaceReferenceDescriptor *string `gorm:"column:face_reference_descriptor"`
	}
	if err := a.DB.Table("users").Select("role, face_reference_image, face_reference_descriptor").Where("id = ?", userID).Scan(&current).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca data wajah", err.Error())
	}
	normalizedRole := utils.NormalizeRoleName(current.Role)
	if normalizedRole != "SISWA" && normalizedRole != "GURU" {
		return utils.Error(c, 403, "Enrol wajah hanya tersedia untuk siswa dan guru")
	}

	hasReference := strings.TrimSpace(ptrStringValue(current.FaceReferenceImage)) != "" ||
		strings.TrimSpace(ptrStringValue(current.FaceReferenceDescriptor)) != ""
	if !hasReference {
		if err := a.DB.Table("users").Where("id = ?", userID).Updates(map[string]interface{}{
			"face_reference_image":      saved,
			"face_reference_descriptor": descriptor,
		}).Error; err != nil {
			return utils.Error(c, 500, "Gagal menyimpan enrol wajah", err.Error())
		}
		return utils.Success(c, 200, "Enrol wajah berhasil disimpan", fiber.Map{
			"mode":                      "ENROLLED",
			"face_reference_image":      saved,
			"face_reference_descriptor": descriptor,
		})
	}

	var pendingCount int64
	a.DB.Table("face_reference_change_requests").
		Where("student_id = ? AND status = 'PENDING'", userID).
		Count(&pendingCount)
	if pendingCount > 0 {
		return utils.Error(c, 409, "Masih ada permintaan perubahan wajah yang menunggu persetujuan admin")
	}

	var requestID uint
	if err := a.DB.Raw(`
		INSERT INTO face_reference_change_requests (
			student_id, school_id, requested_image, requested_descriptor, status, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, 'PENDING', NOW(), NOW())
		RETURNING id
	`, userID, schoolID, saved, descriptor).Scan(&requestID).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat permintaan perubahan wajah", err.Error())
	}

	a.notifyFaceEnrollmentPendingChanged(schoolID, requestID, true)

	return utils.Success(c, 200, "Permintaan perubahan wajah dikirim. Tunggu persetujuan admin.", fiber.Map{
		"mode":       "PENDING_APPROVAL",
		"request_id": requestID,
	})
}

func (a *AppContext) GetMyFaceEnrollmentRequest(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var row map[string]interface{}
	if err := a.DB.Raw(`
		SELECT id, status, admin_note, requested_image, reviewed_at, created_at
		FROM face_reference_change_requests
		WHERE student_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca status permintaan wajah", err.Error())
	}
	return utils.Success(c, 200, "Success Get Face Enrollment Request", row)
}

func (a *AppContext) ListFaceEnrollmentChangeRequests(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	status := strings.ToUpper(strings.TrimSpace(c.Query("status", "PENDING")))
	if status == "" || status == "ALL" {
		status = "%"
	}

	var rows []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			r.id,
			r.student_id,
			COALESCE(NULLIF(u.full_name, ''), u.username) AS student_name,
			u.username,
			COALESCE(c.class_name, '') AS class_name,
			r.requested_image,
			r.status,
			r.admin_note,
			r.reviewed_at,
			COALESCE(NULLIF(admin.full_name, ''), admin.username) AS reviewed_by_name,
			r.created_at
		FROM face_reference_change_requests r
		INNER JOIN users u ON u.id = r.student_id
		LEFT JOIN class c ON c.id = u.class_id
		LEFT JOIN users admin ON admin.id = r.reviewed_by
		WHERE r.school_id = ?
			AND (? = '%' OR r.status = ?)
		ORDER BY
			CASE WHEN r.status = 'PENDING' THEN 0 ELSE 1 END,
			r.created_at DESC
	`, schoolID, status, status).Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca permintaan enrol wajah", err.Error())
	}

	return utils.Success(c, 200, "Success Get Face Enrollment Requests", rows)
}

func (a *AppContext) ListFaceEnrollmentChangeHistory(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	page := parsePositiveInt(c.Query("page"), 1)
	limit := parsePositiveInt(c.Query("limit"), 10)
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	search := strings.TrimSpace(c.Query("search"))

	where := `r.school_id = ? AND r.status IN ('APPROVED', 'REJECTED')`
	args := []interface{}{schoolID}
	if search != "" {
		where += ` AND (
			LOWER(COALESCE(NULLIF(u.full_name, ''), u.username)) LIKE LOWER(?)
			OR LOWER(u.username) LIKE LOWER(?)
			OR LOWER(COALESCE(c.class_name, '')) LIKE LOWER(?)
		)`
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}

	var total int64
	countArgs := append([]interface{}{}, args...)
	if err := a.DB.Raw(`
		SELECT COUNT(*)
		FROM face_reference_change_requests r
		INNER JOIN users u ON u.id = r.student_id
		LEFT JOIN class c ON c.id = u.class_id
		WHERE `+where, countArgs...).Scan(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghitung history enrol wajah", err.Error())
	}

	var rows []map[string]interface{}
	rowArgs := append(append([]interface{}{}, args...), limit, offset)
	if err := a.DB.Raw(`
		SELECT
			r.id,
			r.student_id,
			COALESCE(NULLIF(u.full_name, ''), u.username) AS student_name,
			u.username,
			COALESCE(c.class_name, '') AS class_name,
			r.requested_image,
			r.status,
			r.admin_note,
			r.reviewed_at,
			COALESCE(NULLIF(admin.full_name, ''), admin.username) AS reviewed_by_name,
			r.created_at
		FROM face_reference_change_requests r
		INNER JOIN users u ON u.id = r.student_id
		LEFT JOIN class c ON c.id = u.class_id
		LEFT JOIN users admin ON admin.id = r.reviewed_by
		WHERE `+where+`
		ORDER BY COALESCE(r.reviewed_at, r.updated_at, r.created_at) DESC, r.id DESC
		LIMIT ? OFFSET ?
	`, rowArgs...).Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca history enrol wajah", err.Error())
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	return utils.Success(c, 200, "Success Get Face Enrollment History", fiber.Map{
		"data":        rows,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	})
}

func (a *AppContext) GetFaceEnrollmentPendingCount(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	pendingCount, err := a.countPendingFaceEnrollmentRequests(schoolID)
	if err != nil {
		return utils.Error(c, 500, "Gagal menghitung permintaan enrol wajah", err.Error())
	}

	return utils.Success(c, 200, "Success Get Face Enrollment Pending Count", fiber.Map{
		"pending_count": pendingCount,
	})
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func (a *AppContext) ReviewFaceEnrollmentChangeRequest(c *fiber.Ctx) error {
	adminID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	requestID, err := c.ParamsInt("id")
	if err != nil || requestID <= 0 {
		return utils.Error(c, 400, "ID permintaan tidak valid")
	}

	var body struct {
		Action    string `json:"action"`
		AdminNote string `json:"admin_note"`
	}
	_ = c.BodyParser(&body)
	action := strings.ToUpper(strings.TrimSpace(body.Action))
	if action != "APPROVE" && action != "REJECT" {
		return utils.Error(c, 400, "Action harus APPROVE atau REJECT")
	}

	var row struct {
		ID                  uint   `gorm:"column:id"`
		StudentID           uint   `gorm:"column:student_id"`
		RequestedImage      string `gorm:"column:requested_image"`
		RequestedDescriptor string `gorm:"column:requested_descriptor"`
		Status              string `gorm:"column:status"`
	}
	if err := a.DB.Raw(`
		SELECT id, student_id, requested_image, requested_descriptor, status
		FROM face_reference_change_requests
		WHERE id = ? AND school_id = ?
		LIMIT 1
	`, requestID, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca permintaan", err.Error())
	}
	if row.ID == 0 {
		return utils.Error(c, 404, "Permintaan tidak ditemukan")
	}
	if row.Status != "PENDING" {
		return utils.Error(c, 400, "Permintaan ini sudah diproses")
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		return utils.Error(c, 500, "Gagal memulai transaksi", tx.Error.Error())
	}

	nextStatus := "REJECTED"
	if action == "APPROVE" {
		nextStatus = "APPROVED"
		if err := tx.Table("users").Where("id = ?", row.StudentID).Updates(map[string]interface{}{
			"face_reference_image":      row.RequestedImage,
			"face_reference_descriptor": row.RequestedDescriptor,
		}).Error; err != nil {
			tx.Rollback()
			return utils.Error(c, 500, "Gagal memperbarui wajah siswa", err.Error())
		}
	}

	if err := tx.Table("face_reference_change_requests").Where("id = ?", row.ID).Updates(map[string]interface{}{
		"status":      nextStatus,
		"admin_note":  strings.TrimSpace(body.AdminNote),
		"reviewed_by": adminID,
		"reviewed_at": time.Now(),
		"updated_at":  time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		return utils.Error(c, 500, "Gagal menyimpan review", err.Error())
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menyelesaikan review", err.Error())
	}

	a.notifyFaceEnrollmentPendingChanged(schoolID, row.ID, false)

	return utils.Success(c, 200, "Permintaan enrol wajah berhasil diproses", fiber.Map{
		"id":     row.ID,
		"status": nextStatus,
	})
}

func (a *AppContext) countPendingFaceEnrollmentRequests(schoolID uint) (int64, error) {
	var pendingCount int64
	err := a.DB.Table("face_reference_change_requests").
		Where("school_id = ? AND status = 'PENDING'", schoolID).
		Count(&pendingCount).Error
	return pendingCount, err
}

func (a *AppContext) notifyFaceEnrollmentPendingChanged(schoolID uint, requestID uint, isNewRequest bool) {
	pendingCount, err := a.countPendingFaceEnrollmentRequests(schoolID)
	if err != nil {
		return
	}

	payload := fiber.Map{
		"request_id":     requestID,
		"pending_count":  pendingCount,
		"route":          "/attendance-admin",
		"sound_url":      "announcement",
		"notificationAt": time.Now().UTC().Format(time.RFC3339),
	}
	if a.Realtime != nil {
		a.Realtime.BroadcastSchoolRoleEvent("face-enrollment:pending-count", schoolID, []string{"ADMIN"}, payload)
		if isNewRequest && pendingCount > 0 {
			a.Realtime.BroadcastSchoolRoleEvent("face-enrollment:new-request", schoolID, []string{"ADMIN"}, payload)
		}
	}

	if !isNewRequest || pendingCount <= 0 {
		return
	}

	go func() {
		var recipients []pushNotificationRecipient
		if err := a.DB.Table("users").
			Select("id AS user_id, role").
			Where("school_id = ? AND role = 'ADMIN'", schoolID).
			Scan(&recipients).Error; err != nil {
			return
		}

		targets := make([]pushNotificationTarget, 0, len(recipients))
		for _, recipient := range recipients {
			targets = append(targets, pushNotificationTarget{
				UserID: recipient.UserID,
				Message: pushNotificationMessage{
					Title:    "Approval Enrol Wajah",
					Body:     "Ada permintaan perubahan wajah siswa yang menunggu persetujuan.",
					Kind:     "face_enrollment",
					URL:      "/attendance-admin",
					Icon:     "/pwa-icon.svg",
					Badge:    "/logo.png",
					Tag:      "face-enrollment-approval",
					Renotify: true,
				},
			})
		}
		_ = a.sendPushTargets(targets)
	}()
}
