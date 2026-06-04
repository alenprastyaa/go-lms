package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"lms/models"
	"lms/services"
	"lms/utils"
)

const (
	parentOTPValidity      = 10 * time.Minute
	parentOTPMaxAttempts   = 5
	parentOTPRequestWindow = time.Minute
	parentSessionValidity  = 365 * 24 * time.Hour
)

func normalizeParentWhatsAppPhone(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", fmt.Errorf("Nomor WhatsApp orang tua wajib diisi")
	}

	replacer := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "", "+", "")
	normalized := replacer.Replace(raw)
	switch {
	case strings.HasPrefix(normalized, "62"):
	case strings.HasPrefix(normalized, "0"):
		normalized = "62" + normalized[1:]
	default:
		return "", fmt.Errorf("Format nomor WhatsApp orang tua tidak valid")
	}

	for _, char := range normalized {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("Nomor WhatsApp orang tua hanya boleh berisi angka")
		}
	}

	if len(normalized) < 10 {
		return "", fmt.Errorf("Nomor WhatsApp orang tua terlalu pendek")
	}

	return normalized, nil
}

func hashParentOTP(phoneNumber, otp string) string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	sum := sha256.Sum256([]byte(secret + ":" + strings.TrimSpace(phoneNumber) + ":" + strings.TrimSpace(otp)))
	return hex.EncodeToString(sum[:])
}

func generateParentOTP() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func randomParentPassword() string {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("parent-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func ensureParentAccountLinkedTx(tx *gorm.DB, schoolID uint, studentID uint, parentPhone *string) error {
	phone := ""
	if parentPhone != nil {
		var err error
		phone, err = normalizeParentWhatsAppPhone(*parentPhone)
		if err != nil {
			return err
		}
	}
	if phone == "" {
		return tx.Exec(`DELETE FROM parent_students WHERE school_id = ? AND student_user_id = ?`, schoolID, studentID).Error
	}

	var parent models.User
	err := tx.Where("role = ? AND (phone_number = ? OR username = ?)", "ORANG_TUA", phone, phone).First(&parent).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(randomParentPassword()), 8)
		if hashErr != nil {
			return hashErr
		}
		fullName := fmt.Sprintf("Orang Tua %s", phone)
		parent = models.User{
			FullName:    utils.StringPtr(fullName),
			Username:    phone,
			Password:    string(hash),
			Role:        "ORANG_TUA",
			SchoolID:    &schoolID,
			PhoneNumber: utils.StringPtr(phone),
		}
		if err := tx.Create(&parent).Error; err != nil {
			return err
		}
	} else if stringPointerValue(parent.PhoneNumber) != phone {
		if err := tx.Model(&models.User{}).Where("id = ?", parent.ID).Update("phone_number", phone).Error; err != nil {
			return err
		}
	}

	if parent.SchoolID == nil {
		if err := tx.Model(&models.User{}).Where("id = ?", parent.ID).Update("school_id", schoolID).Error; err != nil {
			return err
		}
	}

	if err := tx.Exec(`DELETE FROM parent_students WHERE school_id = ? AND student_user_id = ? AND parent_user_id <> ?`, schoolID, studentID, parent.ID).Error; err != nil {
		return err
	}

	return tx.Exec(`
		INSERT INTO parent_students (school_id, parent_user_id, student_user_id, relationship, created_at, updated_at)
		VALUES (?, ?, ?, 'WALI', NOW(), NOW())
		ON CONFLICT (parent_user_id, student_user_id)
		DO UPDATE SET school_id = EXCLUDED.school_id, updated_at = NOW()
`, schoolID, parent.ID, studentID).Error
}

func sendParentOTPWhatsApp(phoneNumber, otp string) error {
	message := fmt.Sprintf("Kode OTP login orang tua Anda adalah *%s*. Kode berlaku selama 10 menit.", otp)
	_, err := services.SendWhatsAppMessage(phoneNumber, message)
	return err
}

func (a *AppContext) RequestParentLoginOTP(c *fiber.Ctx) error {
	var body struct {
		PhoneNumber string `json:"phone_number"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}
	phoneNumber, err := normalizeParentWhatsAppPhone(body.PhoneNumber)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	var parent models.User
	if err := a.DB.Where("role = ? AND (phone_number = ? OR username = ?)", "ORANG_TUA", phoneNumber, phoneNumber).First(&parent).Error; err != nil {
		return utils.Error(c, 404, "Nomor WhatsApp orang tua belum terhubung ke siswa")
	}

	var linkedCount int64
	if err := a.DB.Table("parent_students").Where("parent_user_id = ?", parent.ID).Count(&linkedCount).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa relasi siswa", err.Error())
	}
	if linkedCount == 0 {
		return utils.Error(c, 404, "Nomor WhatsApp orang tua belum terhubung ke siswa")
	}

	var recentCount int64
	if err := a.DB.Table("parent_login_otps").
		Where("parent_user_id = ? AND created_at >= ?", parent.ID, jakartaNow().Add(-parentOTPRequestWindow)).
		Count(&recentCount).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa OTP", err.Error())
	}
	if recentCount > 0 {
		nextSendAt := jakartaNow().Add(parentOTPRequestWindow)

		return c.Status(429).JSON(fiber.Map{
			"success": false,
			"message": "OTP sudah dikirim. Silakan coba lagi pada waktu yang tertera.",
			"data": fiber.Map{
				"next_send_at": nextSendAt.Format(time.RFC3339),
			},
		})
	}
	if strings.TrimSpace(os.Getenv("NGIRIMWA_API_KEY")) == "" {
		return utils.Error(c, 500, "NGIRIMWA_API_KEY belum diatur di server")
	}

	otp, err := generateParentOTP()
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat OTP", err.Error())
	}

	row := models.ParentLoginOTP{
		ParentUserID: parent.ID,
		Identifier:   phoneNumber,
		OTPHash:      hashParentOTP(phoneNumber, otp),
		ExpiresAt:    jakartaNow().Add(parentOTPValidity),
	}
	if err := a.DB.Create(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan OTP", err.Error())
	}
	if err := sendParentOTPWhatsApp(phoneNumber, otp); err != nil {
		_ = a.DB.Where("id = ?", row.ID).Delete(&models.ParentLoginOTP{}).Error
		return utils.Error(c, 500, "Gagal mengirim OTP", err.Error())
	}

	return utils.Success(c, 200, "Kode OTP telah dikirim ke WhatsApp orang tua", fiber.Map{
		"phone_number": phoneNumber,
		"expires_in":   int(parentOTPValidity.Seconds()),
	})
}

func (a *AppContext) VerifyParentLoginOTP(c *fiber.Ctx) error {
	var body struct {
		PhoneNumber string `json:"phone_number"`
		OTP         string `json:"otp"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}
	phoneNumber, err := normalizeParentWhatsAppPhone(body.PhoneNumber)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	otp := strings.TrimSpace(body.OTP)
	if phoneNumber == "" || otp == "" {
		return utils.Error(c, 400, "Nomor WhatsApp dan OTP wajib diisi")
	}

	var parent models.User
	if err := a.DB.Where("role = ? AND (phone_number = ? OR username = ?)", "ORANG_TUA", phoneNumber, phoneNumber).First(&parent).Error; err != nil {
		return utils.Error(c, 404, "Nomor WhatsApp orang tua belum terhubung ke siswa")
	}

	var row models.ParentLoginOTP
	if err := a.DB.Where("parent_user_id = ? AND email = ? AND used_at IS NULL", parent.ID, phoneNumber).
		Order("created_at DESC").
		First(&row).Error; err != nil {
		return utils.Error(c, 400, "OTP tidak tersedia atau sudah digunakan")
	}
	if row.ExpiresAt.Before(jakartaNow()) {
		return utils.Error(c, 400, "OTP sudah kedaluwarsa")
	}
	if row.Attempts >= parentOTPMaxAttempts {
		return utils.Error(c, 400, "OTP sudah melebihi batas percobaan")
	}

	if row.OTPHash != hashParentOTP(phoneNumber, otp) {
		_ = a.DB.Model(&models.ParentLoginOTP{}).Where("id = ?", row.ID).Update("attempts", gorm.Expr("attempts + 1")).Error
		return utils.Error(c, 400, "Kode OTP tidak sesuai")
	}

	now := jakartaNow()
	if err := a.DB.Model(&models.ParentLoginOTP{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
		"used_at":  now,
		"attempts": gorm.Expr("attempts + 1"),
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui OTP", err.Error())
	}

	return a.issueParentSession(c, &parent)
}

func (a *AppContext) issueParentSession(c *fiber.Ctx, parent *models.User) error {
	sessionDevice := detectLoginDevice(c.Get("User-Agent"))
	sessionIP := strings.TrimSpace(c.IP())
	loginAt := time.Now().UTC()

	var sessionRow struct {
		SessionVersion int64 `json:"session_version"`
	}
	if err := a.DB.Raw(`
		UPDATE users
		SET
			session_version = COALESCE(session_version, 0) + 1,
			current_session_device = ?,
			current_session_user_agent = ?,
			current_session_ip = ?,
			current_session_login_at = ?
		WHERE id = ?
		RETURNING session_version
	`, sessionDevice, nullIfSessionValueEmpty(c.Get("User-Agent")), nullIfSessionValueEmpty(sessionIP), loginAt, parent.ID).Scan(&sessionRow).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat sesi login", err.Error())
	}
	parent.SessionVersion = sessionRow.SessionVersion

	normalizedRole := "ORANG_TUA"
	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": parent.ID, "role": normalizedRole, "schoolId": parent.SchoolID, "username": parent.Username, "sessionVersion": parent.SessionVersion, "exp": time.Now().Add(parentSessionValidity).Unix(),
	}).SignedString([]byte(os.Getenv("JWT_SECRET")))

	var school models.School
	var schoolName interface{} = nil
	var schoolLogo interface{} = nil
	if parent.SchoolID != nil {
		_ = a.DB.Select("name", "logo_url", "inventory_module_enabled", "attendance_module_enabled", "official_exam_module_enabled", "koperasi_module_enabled", "private_chat_module_enabled", "teaching_module_ai_enabled", "payroll_module_enabled", "personal_teacher_mode_enabled").Where("id = ?", *parent.SchoolID).First(&school).Error
		schoolName = school.Name
		schoolLogo = school.LogoURL
	}

	return utils.Success(c, 200, "Login orang tua berhasil", fiber.Map{
		"role": normalizedRole, "username": parent.Username, "school_id": parent.SchoolID, "school_name": schoolName, "school_logo": schoolLogo, "school_features": fiber.Map{
			"inventory_module_enabled":      school.InventoryModuleEnabled,
			"attendance_module_enabled":     school.AttendanceModuleEnabled,
			"official_exam_module_enabled":  school.OfficialExamModuleEnabled,
			"koperasi_module_enabled":       school.KoperasiModuleEnabled,
			"private_chat_module_enabled":   school.PrivateChatModuleEnabled,
			"teaching_module_ai_enabled":    school.TeachingModuleAIEnabled,
			"payroll_module_enabled":        school.PayrollModuleEnabled,
			"personal_teacher_mode_enabled": school.PersonalTeacherModeEnabled,
		}, "profile_image": parent.ProfileImage, "token": token,
	})
}

func (a *AppContext) GetParentDashboard(c *fiber.Ctx) error {
	parentID := c.Locals("userID").(uint)
	childIDRaw := strings.TrimSpace(c.Query("child_id"))

	var children []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			u.id,
			COALESCE(NULLIF(u.full_name, ''), u.username) AS full_name,
			u.username,
			u.parent_email,
			u.phone_number,
			u.school_id,
			c.class_name,
			s.name AS school_name
		FROM parent_students ps
		INNER JOIN users u ON u.id = ps.student_user_id AND u.role = 'SISWA'
		LEFT JOIN class c ON c.id = u.class_id
		LEFT JOIN schools s ON s.id = u.school_id
		WHERE ps.parent_user_id = ?
		ORDER BY s.name ASC, c.class_name ASC, u.full_name ASC, u.username ASC
	`, parentID).Scan(&children).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat daftar anak", err.Error())
	}
	if len(children) == 0 {
		return utils.Success(c, 200, "Dashboard orang tua belum memiliki siswa", fiber.Map{
			"generatedAt": jakartaNow().Format(time.RFC3339),
			"children":    []map[string]interface{}{},
			"overview":    fiber.Map{},
		})
	}

	selectedChildID := uint(utils.ToInt(fmt.Sprint(children[0]["id"]), 0))
	if childIDRaw != "" {
		if parsed, err := strconv.ParseUint(childIDRaw, 10, 64); err == nil {
			for _, child := range children {
				if uint(utils.ToInt(fmt.Sprint(child["id"]), 0)) == uint(parsed) {
					selectedChildID = uint(parsed)
					break
				}
			}
		}
	}

	var selectedChild map[string]interface{}
	for _, child := range children {
		if uint(utils.ToInt(fmt.Sprint(child["id"]), 0)) == selectedChildID {
			selectedChild = child
			break
		}
	}

	var today map[string]interface{}
	a.DB.Raw(`SELECT attendance_date, clock_in, clock_out, status, checkout_note, image FROM attendance WHERE user_id = ? AND attendance_date = CURRENT_DATE LIMIT 1`, selectedChildID).Scan(&today)
	normalizeAttendanceMap(today)

	var overviewSummary struct {
		AttendanceTotal   int `json:"attendance_total"`
		ReceiptsTotal     int `json:"receipts_total"`
		ReceiptsThisMonth int `json:"receipts_this_month"`
	}
	a.DB.Raw(`
		SELECT
			(SELECT COUNT(*)::int FROM attendance WHERE user_id = ?) AS attendance_total,
			(SELECT COUNT(*)::int FROM payment_receipt WHERE user_id = ?) AS receipts_total,
			(SELECT COUNT(*)::int FROM payment_receipt WHERE user_id = ? AND DATE_TRUNC('month', created_at) = DATE_TRUNC('month', CURRENT_DATE)) AS receipts_this_month
	`, selectedChildID, selectedChildID, selectedChildID).Scan(&overviewSummary)
	overview := map[string]int{
		"attendance_total":    overviewSummary.AttendanceTotal,
		"receipts_total":      overviewSummary.ReceiptsTotal,
		"receipts_this_month": overviewSummary.ReceiptsThisMonth,
	}

	var recentAttendance []map[string]interface{}
	a.DB.Raw(`SELECT attendance_date, clock_in, clock_out, status, checkout_note, image FROM attendance WHERE user_id = ? ORDER BY attendance_date DESC, clock_in DESC LIMIT 8`, selectedChildID).Scan(&recentAttendance)
	normalizeAttendanceMaps(recentAttendance)

	var recentReceipts []map[string]interface{}
	a.DB.Raw(`SELECT id, periode, description, created_at, image_path FROM payment_receipt WHERE user_id = ? ORDER BY created_at DESC LIMIT 8`, selectedChildID).Scan(&recentReceipts)
	normalizeReceiptMaps(recentReceipts)

	var pendingAssignments []map[string]interface{}
	a.DB.Raw(`
		SELECT la.id, la.title, la.due_date, ls.name AS subject_name, c.class_name, sub.id AS submission_id, sub.score
		FROM users u
		INNER JOIN class c ON c.id = u.class_id
		INNER JOIN learning_subjects ls ON ls.class_id = c.id
		INNER JOIN learning_assignments la ON la.subject_id = ls.id
		LEFT JOIN learning_submissions sub ON sub.assignment_id = la.id AND sub.student_id = u.id
		WHERE u.id = ?
		ORDER BY la.due_date ASC NULLS LAST, la.created_at DESC
		LIMIT 12
	`, selectedChildID).Scan(&pendingAssignments)
	normalizeJakartaDateTimeRows(pendingAssignments, "due_date", "created_at", "updated_at", "submitted_at", "started_at", "assignment_created_at")
	filteredPending := make([]map[string]interface{}, 0)
	gradedCount := 0
	for _, item := range pendingAssignments {
		if item["submission_id"] == nil {
			filteredPending = append(filteredPending, item)
		}
		if item["score"] != nil {
			gradedCount++
		}
	}
	overview["pending_assignments"] = len(filteredPending)
	overview["graded_assignments"] = gradedCount

	var grades []map[string]interface{}
	selectedSchoolID := uint(utils.ToInt(fmt.Sprint(selectedChild["school_id"]), 0))
	if selectedSchoolID > 0 {
		a.DB.Raw(`
			SELECT
				la.id AS assignment_id,
				la.title,
				la.description,
				la.assignment_type,
				COALESCE(la.is_exam, false) AS is_exam,
				la.due_date,
				la.created_at AS assignment_created_at,
				ls.id AS subject_id,
				ls.name AS subject_name,
				COALESCE(cls.class_name, '') AS class_name,
				sub.id AS submission_id,
				sub.started_at,
				sub.submitted_at,
				sub.score,
				sub.feedback,
				COALESCE(sub.is_submitted, false) AS is_submitted
			FROM users stu
			INNER JOIN learning_subjects ls ON ls.class_id = stu.class_id
			LEFT JOIN class cls ON cls.id = ls.class_id
			INNER JOIN learning_assignments la ON la.subject_id = ls.id
			LEFT JOIN LATERAL (
				SELECT s.id, s.started_at, s.submitted_at, s.score, s.feedback, s.is_submitted
				FROM learning_submissions s
				WHERE s.assignment_id = la.id
				  AND s.student_id = stu.id
				ORDER BY COALESCE(s.is_submitted, false) DESC,
				         s.submitted_at DESC NULLS LAST,
				         s.started_at DESC NULLS LAST,
				         s.id DESC
				LIMIT 1
			) sub ON true
			WHERE stu.id = ?
				AND stu.school_id = ?
				AND ls.school_id = ?
				AND (
					COALESCE(la.is_exam, false) = false
					OR COALESCE(la.exam_status, '') = 'PUBLISHED'
				)
				AND la.assignment_type IN ('FILE', 'MANUAL', 'MCQ', 'ESSAY')
			ORDER BY ls.name ASC, la.due_date DESC NULLS LAST, la.created_at DESC
			LIMIT 12
		`, selectedChildID, selectedSchoolID, selectedSchoolID).Scan(&grades)
		normalizeJakartaDateTimeRows(grades, "due_date", "assignment_created_at", "started_at", "submitted_at", "graded_at", "created_at", "updated_at")
	}

	submittedCount := 0
	pendingCount := 0
	gradedScoreCount := 0
	var totalGradeScore float64
	for _, row := range grades {
		submitted := isSubmitted(row)
		if submitted {
			submittedCount++
		} else {
			pendingCount++
		}
		scoreValue, ok := row["score"]
		if !ok || scoreValue == nil {
			continue
		}
		scoreText := strings.TrimSpace(fmt.Sprint(scoreValue))
		if scoreText == "" || scoreText == "<nil>" {
			continue
		}
		totalGradeScore += floatFromAny(scoreValue)
		gradedScoreCount++
	}
	var averageScore interface{} = nil
	if gradedScoreCount > 0 {
		averageScore = float64(int((totalGradeScore/float64(gradedScoreCount))*100)) / 100
	}
	gradesSummary := map[string]interface{}{
		"total_assignments": len(grades),
		"submitted_count":   submittedCount,
		"pending_count":     pendingCount,
		"graded_count":      gradedScoreCount,
		"average_score":     averageScore,
	}

	var announcements []announcementItem
	if selectedChild != nil {
		schoolID := uint(utils.ToInt(fmt.Sprint(selectedChild["school_id"]), 0))
		if schoolID > 0 {
			items, err := a.fetchAnnouncementsForSchool(schoolID, "SISWA", false, 3)
			if err != nil {
				return utils.Error(c, 500, "Gagal memuat pengumuman dashboard", err.Error())
			}
			announcements = items
		}
	}

	return utils.Success(c, 200, "Success Get Parent Dashboard", fiber.Map{
		"generatedAt":        jakartaNow().Format(time.RFC3339),
		"children":           children,
		"student":            selectedChild,
		"selected_child_id":  selectedChildID,
		"todayAttendance":    nilIfEmptyMap(today),
		"overview":           overview,
		"announcements":      announcements,
		"recentAttendance":   recentAttendance,
		"recentReceipts":     recentReceipts,
		"pendingAssignments": filteredPending,
		"grades":             grades,
		"gradesSummary":      gradesSummary,
	})
}
