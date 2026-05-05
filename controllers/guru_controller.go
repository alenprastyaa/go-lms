package controllers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/services"
	"lms/utils"
)

func (a *AppContext) GetGuruDashboard(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	teacherID := c.Locals("userID").(uint)

	var homeroom map[string]interface{}
	a.DB.Raw(`
		SELECT c.id, c.class_name, c.school_id, u.username AS wali_guru_name
		FROM class c LEFT JOIN users u ON u.id = c.wali_guru_id
		WHERE c.school_id = ? AND c.wali_guru_id = ? LIMIT 1
	`, schoolID, teacherID).Scan(&homeroom)

	var overviewRows []struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	}
	a.DB.Raw(`
		SELECT 'students' AS key, COUNT(*)::int AS value
		FROM users u INNER JOIN class c ON c.id = u.class_id
		WHERE u.role='SISWA' AND u.school_id=? AND c.wali_guru_id=?
		UNION ALL
		SELECT 'attendance_today' AS key, COUNT(*)::int AS value
		FROM attendance a INNER JOIN users u ON u.id=a.user_id INNER JOIN class c ON c.id=u.class_id
		WHERE u.school_id=? AND c.wali_guru_id=? AND a.attendance_date=CURRENT_DATE
		UNION ALL
		SELECT 'receipts_this_month' AS key, COUNT(*)::int AS value
		FROM payment_receipt pr INNER JOIN users u ON u.id=pr.user_id INNER JOIN class c ON c.id=u.class_id
		WHERE u.school_id=? AND c.wali_guru_id=? AND DATE_TRUNC('month', pr.created_at)=DATE_TRUNC('month', CURRENT_DATE)
	`, schoolID, teacherID, schoolID, teacherID, schoolID, teacherID).Scan(&overviewRows)
	overview := map[string]int{}
	for _, r := range overviewRows {
		overview[r.Key] = r.Value
	}
	overview["absent_today"] = max(0, overview["students"]-overview["attendance_today"])

	var students []map[string]interface{}
	a.DB.Raw(`
		SELECT u.id,u.username,u.parent_email,u.phone_number,
		       CASE WHEN a.user_id IS NULL THEN false ELSE true END AS checked_in_today
		FROM users u
		INNER JOIN class c ON c.id=u.class_id
		LEFT JOIN attendance a ON a.user_id=u.id AND a.attendance_date=CURRENT_DATE
		WHERE u.role='SISWA' AND u.school_id=? AND c.wali_guru_id=?
		ORDER BY checked_in_today DESC, u.username ASC
		LIMIT 8
	`, schoolID, teacherID).Scan(&students)

	var recentAttendance []map[string]interface{}
	a.DB.Raw(`
		SELECT u.id AS student_id,u.username,a.attendance_date,a.clock_in,a.clock_out,a.status,a.image
		FROM attendance a
		INNER JOIN users u ON u.id=a.user_id
		INNER JOIN class c ON c.id=u.class_id
		WHERE u.school_id=? AND c.wali_guru_id=?
		ORDER BY a.clock_in DESC NULLS LAST LIMIT 8
	`, schoolID, teacherID).Scan(&recentAttendance)

	var recentReceipts []map[string]interface{}
	a.DB.Raw(`
		SELECT u.id AS student_id,u.username,pr.periode,pr.description,pr.created_at,pr.image_path
		FROM payment_receipt pr
		INNER JOIN users u ON u.id=pr.user_id
		INNER JOIN class c ON c.id=u.class_id
		WHERE u.school_id=? AND c.wali_guru_id=?
		ORDER BY pr.created_at DESC LIMIT 8
	`, schoolID, teacherID).Scan(&recentReceipts)

	return utils.Success(c, 200, "Success Get Guru Dashboard", fiber.Map{
		"generatedAt":      time.Now().UTC().Format(time.RFC3339),
		"homeroom":         homeroom,
		"overview":         overview,
		"students":         students,
		"recentAttendance": recentAttendance,
		"recentReceipts":   recentReceipts,
	})
}

func (a *AppContext) GetMyClassStudents(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	teacherID := c.Locals("userID").(uint)
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "10"), 10)
	offset := (page - 1) * limit

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT u.id,u.username,u.class_id,u.parent_email,u.phone_number,cn.class_name,
		       CASE WHEN a.user_id IS NULL THEN false ELSE true END AS checked_in_today,
		       a.clock_in,a.clock_out,a.status AS attendance_status
		FROM users u
		INNER JOIN class cn ON u.class_id=cn.id
		LEFT JOIN attendance a ON a.user_id=u.id AND a.attendance_date=CURRENT_DATE
		WHERE u.role='SISWA' AND u.school_id=? AND cn.wali_guru_id=?
		ORDER BY u.username ASC
		LIMIT ? OFFSET ?
	`, schoolID, teacherID, limit, offset).Scan(&rows)
	return utils.Success(c, 200, "Success Get My Class Students", fiber.Map{"page": page, "limit": limit, "data": rows})
}

func (a *AppContext) GetStudentAttendanceForTeacher(c *fiber.Ctx) error {
	studentID := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	teacherID := c.Locals("userID").(uint)
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "10"), 10)
	offset := (page - 1) * limit

	var allowed int64
	a.DB.Raw(`
		SELECT COUNT(*)
		FROM users u INNER JOIN class c ON c.id=u.class_id
		WHERE u.id=? AND u.school_id=? AND c.wali_guru_id=?
	`, studentID, schoolID, teacherID).Scan(&allowed)
	if allowed == 0 {
		return utils.Error(c, 403, "Forbidden: student is not in your homeroom class")
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT u.username, a.attendance_date, a.image, a.clock_in, a.clock_out, a.status
		FROM attendance a LEFT JOIN users u ON a.user_id=u.id
		WHERE a.user_id=?
		ORDER BY a.attendance_date DESC, a.clock_in DESC
		LIMIT ? OFFSET ?
	`, studentID, limit, offset).Scan(&rows)
	return utils.Success(c, 200, "Success Get Student Attendance", fiber.Map{"page": page, "limit": limit, "data": rows})
}

func (a *AppContext) GetStudentReceiptForTeacher(c *fiber.Ctx) error {
	studentID := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	teacherID := c.Locals("userID").(uint)

	var allowed int64
	a.DB.Raw(`
		SELECT COUNT(*) FROM users u INNER JOIN class c ON c.id=u.class_id
		WHERE u.id=? AND u.school_id=? AND c.wali_guru_id=?
	`, studentID, schoolID, teacherID).Scan(&allowed)
	if allowed == 0 {
		return utils.Error(c, 403, "Forbidden: student is not in your homeroom class")
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT id,image_path,description,payment_date,created_at
		FROM payment_receipt
		WHERE user_id=?
		ORDER BY payment_date DESC NULLS LAST, created_at DESC
	`, studentID).Scan(&rows)
	return utils.Success(c, 200, "Success Get Student Receipt", rows)
}

func (a *AppContext) GetTeacherSubjects(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	teacherID := c.Locals("userID").(uint)
	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT ls.*, c.class_name, t.username AS teacher_name
		FROM learning_subjects ls
		LEFT JOIN class c ON c.id=ls.class_id
		LEFT JOIN users t ON t.id=ls.teacher_id
		WHERE ls.school_id=? AND ls.teacher_id=?
		ORDER BY ls.created_at DESC
	`, schoolID, teacherID).Scan(&rows)
	return utils.Success(c, 200, "Success Get Teacher Subjects", rows)
}

func (a *AppContext) GetSubjectMaterials(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT m.*, u.username AS created_by_name
		FROM learning_materials m
		LEFT JOIN users u ON u.id = m.created_by
		WHERE m.subject_id = ?
		ORDER BY m.created_at DESC
	`, subjectID).Scan(&rows)
	return utils.Success(c, 200, "Success Get Materials", rows)
}

func (a *AppContext) CreateLearningMaterial(c *fiber.Ctx) error {
	subjectID := c.FormValue("subject_id")
	title := c.FormValue("title")
	content := c.FormValue("content")
	userID := c.Locals("userID").(uint)
	if subjectID == "" || strings.TrimSpace(title) == "" {
		return utils.Error(c, 400, "subject_id and title are required")
	}
	attachment := ""
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		u, upErr := utils.SaveUploadedFile(c, f)
		if upErr == nil {
			attachment = u
		}
	}
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_materials (subject_id,title,content,attachment_url,created_by,created_at)
		VALUES (?,?,?,?,?,NOW()) RETURNING *
	`, subjectID, title, content, nullIfEmpty(attachment), userID).Scan(&row)
	return utils.Success(c, 201, "Success Create Material", row)
}

func (a *AppContext) UpdateLearningMaterial(c *fiber.Ctx) error {
	id := c.Params("materialId")
	var cur map[string]interface{}
	a.DB.Raw(`SELECT * FROM learning_materials WHERE id=?`, id).Scan(&cur)
	if len(cur) == 0 {
		return utils.Error(c, 404, "Material not found")
	}
	title := c.FormValue("title", asString(cur["title"]))
	content := c.FormValue("content", asString(cur["content"]))
	attachment := asString(cur["attachment_url"])
	if strings.ToLower(c.FormValue("remove_attachment")) == "true" {
		attachment = ""
	}
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		u, upErr := utils.SaveUploadedFile(c, f)
		if upErr == nil {
			attachment = u
		}
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_materials SET title=?, content=?, attachment_url=? WHERE id=? RETURNING *
	`, title, content, nullIfEmpty(attachment), id).Scan(&row)
	return utils.Success(c, 200, "Success Update Material", row)
}

func (a *AppContext) DeleteLearningMaterial(c *fiber.Ctx) error {
	id := c.Params("materialId")
	var row map[string]interface{}
	a.DB.Raw(`DELETE FROM learning_materials WHERE id=? RETURNING *`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Material not found")
	}
	return utils.Success(c, 200, "Success Delete Material", row)
}

func (a *AppContext) GetAssignmentSubmissionsForTeacher(c *fiber.Ctx) error {
	assignmentID := c.Params("assignmentId")

	var assignment struct {
		ID             int    `gorm:"column:id"`
		AssignmentType string `gorm:"column:assignment_type"`
		ClassID        int    `gorm:"column:class_id"`
	}
	a.DB.Raw(`
		SELECT la.id, la.assignment_type, ls.class_id
		FROM learning_assignments la
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		WHERE la.id = ?
		LIMIT 1
	`, assignmentID).Scan(&assignment)
	if assignment.ID == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}

	if strings.ToUpper(strings.TrimSpace(assignment.AssignmentType)) == "MANUAL" {
		a.DB.Exec(`
			INSERT INTO learning_submissions (assignment_id, student_id, started_at, is_submitted, submitted_at)
			SELECT ?, u.id, NOW(), false, NULL
			FROM users u
			WHERE u.class_id = ? AND u.role = 'SISWA'
			  AND NOT EXISTS (
			    SELECT 1
			    FROM learning_submissions s
			    WHERE s.assignment_id = ? AND s.student_id = u.id
			  )
		`, assignment.ID, assignment.ClassID, assignment.ID)
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT s.*, u.username, u.parent_email, u.phone_number
		FROM learning_submissions s
		LEFT JOIN users u ON u.id=s.student_id
		WHERE s.assignment_id=?
		ORDER BY u.username ASC, s.id ASC
	`, assignmentID).Scan(&rows)
	return utils.Success(c, 200, "Success Get Assignment Submissions", rows)
}

func (a *AppContext) GradeLearningSubmission(c *fiber.Ctx) error {
	id := c.Params("submissionId")
	teacherID := c.Locals("userID").(uint)
	var body struct {
		Score    interface{} `json:"score"`
		Feedback string      `json:"feedback"`
	}
	_ = c.BodyParser(&body)
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_submissions
		SET score=?, feedback=?, graded_at=NOW(), graded_by=?
		WHERE id=?
		RETURNING *
	`, body.Score, body.Feedback, teacherID, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Submission not found")
	}
	return utils.Success(c, 200, "Success Grade Submission", row)
}

func (a *AppContext) GetLearningChatSummary(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	role := c.Locals("userRole").(string)
	schoolID := c.Locals("schoolID").(uint)
	var rows []map[string]interface{}
	if role == "GURU" {
		a.DB.Raw(`
			SELECT
				ls.id AS subject_id,
				ls.name AS subject_name,
				ls.chat_icon_url,
				c.class_name,
				COALESCE(lcr.last_read_message_id, 0)::int AS last_read_message_id,
				COUNT(m.id) FILTER (
					WHERE m.sender_id <> ?
					  AND m.id > COALESCE(lcr.last_read_message_id, 0)
				)::int AS unread_count,
				MAX(m.created_at) AS last_message_at
			FROM learning_subjects ls
			LEFT JOIN class c ON c.id=ls.class_id
			LEFT JOIN learning_chat_reads lcr ON lcr.subject_id=ls.id AND lcr.user_id=?
			LEFT JOIN learning_chat_messages m ON m.subject_id=ls.id
			WHERE ls.school_id=? AND ls.teacher_id=?
			GROUP BY ls.id, ls.name, ls.chat_icon_url, c.class_name, lcr.last_read_message_id
			ORDER BY last_message_at DESC NULLS LAST, ls.name ASC
		`, userID, userID, schoolID, userID).Scan(&rows)
	} else {
		// fallback for siswa
		a.DB.Raw(`
			SELECT
				ls.id AS subject_id,
				ls.name AS subject_name,
				ls.chat_icon_url,
				c.class_name,
				COALESCE(lcr.last_read_message_id, 0)::int AS last_read_message_id,
				COUNT(m.id) FILTER (
					WHERE m.sender_id <> ?
					  AND m.id > COALESCE(lcr.last_read_message_id, 0)
				)::int AS unread_count,
				MAX(m.created_at) AS last_message_at
			FROM users u
			INNER JOIN learning_subjects ls ON ls.class_id=u.class_id
			LEFT JOIN class c ON c.id=ls.class_id
			LEFT JOIN learning_chat_reads lcr ON lcr.subject_id=ls.id AND lcr.user_id=?
			LEFT JOIN learning_chat_messages m ON m.subject_id=ls.id
			WHERE u.id=?
			GROUP BY ls.id, ls.name, ls.chat_icon_url, c.class_name, lcr.last_read_message_id
			ORDER BY last_message_at DESC NULLS LAST, ls.name ASC
		`, userID, userID, userID).Scan(&rows)
	}
	return utils.Success(c, 200, "Success Get Learning Chat Summary", rows)
}

func (a *AppContext) GetSubjectChatMessages(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT m.*, u.username AS sender_name, u.role AS sender_role, u.profile_image AS sender_profile_image
		FROM learning_chat_messages m
		LEFT JOIN users u ON u.id=m.sender_id
		WHERE m.subject_id=?
		ORDER BY m.created_at ASC
	`, subjectID).Scan(&rows)
	return utils.Success(c, 200, "Success Get Subject Chat Messages", rows)
}

func (a *AppContext) CreateSubjectChatMessage(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	userID := c.Locals("userID").(uint)
	var body struct {
		Message            string  `json:"message"`
		Text               string  `json:"text"`
		MessageType        string  `json:"message_type"`
		AttachmentURL      string  `json:"attachment_url"`
		AttachmentName     string  `json:"attachment_name"`
		AttachmentMimeType string  `json:"attachment_mime_type"`
		AttachmentSize     float64 `json:"attachment_size"`
		ClientID           string  `json:"client_id"`
	}
	_ = c.BodyParser(&body)

	msg := strings.TrimSpace(body.Message)
	if msg == "" {
		msg = strings.TrimSpace(body.Text)
	}
	if msg == "" {
		msg = strings.TrimSpace(c.FormValue("message"))
	}
	if msg == "" {
		msg = strings.TrimSpace(c.FormValue("text"))
	}

	attachment := strings.TrimSpace(body.AttachmentURL)
	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(c.FormValue("client_id"))
	}
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		u, upErr := utils.SaveUploadedFile(c, f)
		if upErr == nil {
			attachment = u
		}
	}
	if msg == "" && attachment == "" {
		return utils.Error(c, 400, "message is required")
	}

	messageType := detectChatMessageType(body.MessageType, body.AttachmentMimeType, body.AttachmentName, attachment)
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_chat_messages (
			subject_id,
			sender_id,
			message,
			message_type,
			attachment_url,
			attachment_name,
			attachment_mime_type,
			attachment_size,
			created_at
		)
		VALUES (?,?,?,?,?,?,?,?,NOW()) RETURNING *
	`, subjectID, userID, msg, messageType, nullIfEmpty(attachment), nullIfEmpty(body.AttachmentName), nullIfEmpty(body.AttachmentMimeType), nullIfZero(int(body.AttachmentSize))).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 500, "Failed to create chat message")
	}

	var fullRow map[string]interface{}
	a.DB.Raw(`
		SELECT m.*, u.username AS sender_name, u.role AS sender_role, u.profile_image AS sender_profile_image
		FROM learning_chat_messages m
		LEFT JOIN users u ON u.id=m.sender_id
		WHERE m.id=?
		LIMIT 1
	`, row["id"]).Scan(&fullRow)
	if len(fullRow) == 0 {
		fullRow = row
	}
	fullRow["origin_client_id"] = clientID

	if a.Realtime != nil {
		a.Realtime.BroadcastSubjectChatMessage(subjectID, fullRow)
	}

	return utils.Success(c, 201, "Success Create Subject Chat Message", fullRow)
}

func (a *AppContext) MarkSubjectChatAsRead(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	userID := c.Locals("userID").(uint)
	var body struct {
		ClientID string `json:"client_id"`
	}
	_ = c.BodyParser(&body)
	var last struct {
		ID int `json:"id"`
	}
	a.DB.Raw(`SELECT id FROM learning_chat_messages WHERE subject_id=? ORDER BY id DESC LIMIT 1`, subjectID).Scan(&last)
	if last.ID == 0 {
		return utils.Success(c, 200, "Success Mark Subject Chat As Read", fiber.Map{"subject_id": subjectID, "last_read_message_id": nil})
	}
	a.DB.Exec(`
		INSERT INTO learning_chat_reads (subject_id,user_id,last_read_message_id,last_read_at)
		VALUES (?,?,?,NOW())
		ON CONFLICT (subject_id,user_id)
		DO UPDATE SET
			last_read_message_id = GREATEST(
				COALESCE(learning_chat_reads.last_read_message_id, 0),
				COALESCE(EXCLUDED.last_read_message_id, 0)
			),
			last_read_at = NOW()
	`, subjectID, userID, last.ID)
	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(c.FormValue("client_id"))
	}
	payload := fiber.Map{"subject_id": subjectID, "user_id": userID, "last_read_message_id": last.ID, "origin_client_id": clientID}
	if a.Realtime != nil {
		a.Realtime.BroadcastSubjectReadUpdated(subjectID, payload)
	}
	return utils.Success(c, 200, "Success Mark Subject Chat As Read", payload)
}

func (a *AppContext) BroadcastSubjectTyping(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	userID := c.Locals("userID").(uint)
	var body struct {
		IsTyping bool   `json:"is_typing"`
		ClientID string `json:"client_id"`
	}
	_ = c.BodyParser(&body)

	clientID := strings.TrimSpace(body.ClientID)
	if clientID == "" {
		clientID = strings.TrimSpace(c.FormValue("client_id"))
	}

	var sender struct {
		Username string `json:"username"`
	}
	_ = a.DB.Raw(`SELECT username FROM users WHERE id = ?`, userID).Scan(&sender).Error

	payload := fiber.Map{
		"subject_id":        subjectID,
		"user_id":           userID,
		"sender_name":       sender.Username,
		"is_typing":         body.IsTyping,
		"origin_client_id":  clientID,
		"updated_at_unixms": time.Now().UnixMilli(),
	}
	if a.Realtime != nil {
		a.Realtime.BroadcastSubjectTyping(subjectID, payload)
	}
	return utils.Success(c, 200, "Success Broadcast Subject Typing", payload)
}

func (a *AppContext) GetSubjectOnlineUsers(c *fiber.Ctx) error {
	subjectID := uint(utils.ToInt(c.Params("subjectId"), 0))
	schoolID := c.Locals("schoolID").(uint)
	if subjectID == 0 {
		return utils.Error(c, 400, "subject id tidak valid")
	}

	if a.Realtime == nil {
		return utils.Success(c, 200, "Success Get Subject Online Users", fiber.Map{
			"subject_id":   subjectID,
			"online_count": 0,
			"users":        []fiber.Map{},
		})
	}

	userIDs := a.Realtime.SubjectOnlineUsers(schoolID, subjectID)
	if len(userIDs) == 0 {
		return utils.Success(c, 200, "Success Get Subject Online Users", fiber.Map{
			"subject_id":   subjectID,
			"online_count": 0,
			"users":        []fiber.Map{},
		})
	}

	type onlineUserRow struct {
		ID           uint   `json:"id"`
		Username     string `json:"username"`
		Role         string `json:"role"`
		FullName     string `json:"full_name"`
		ProfileImage string `json:"profile_image"`
	}
	var rows []onlineUserRow
	_ = a.DB.Table("users").
		Select("id, username, role, full_name, profile_image").
		Where("id IN ? AND school_id = ?", userIDs, schoolID).
		Order("username ASC").
		Scan(&rows).Error

	users := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		users = append(users, fiber.Map{
			"id":            row.ID,
			"username":      row.Username,
			"role":          row.Role,
			"full_name":     row.FullName,
			"profile_image": row.ProfileImage,
		})
	}

	return utils.Success(c, 200, "Success Get Subject Online Users", fiber.Map{
		"subject_id":   subjectID,
		"online_count": len(users),
		"users":        users,
	})
}

func detectChatMessageType(explicitType, mimeType, attachmentName, attachmentURL string) string {
	normalized := strings.ToUpper(strings.TrimSpace(explicitType))
	if normalized != "" {
		return normalized
	}

	if strings.TrimSpace(attachmentURL) != "" || strings.TrimSpace(attachmentName) != "" {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType)), "image/") {
			return "IMAGE"
		}
		return "FILE"
	}

	return "TEXT"
}

func (a *AppContext) UpdateLearningSubjectChatIconByTeacher(c *fiber.Ctx) error {
	id := c.Params("subjectId")
	f, err := c.FormFile("chat_icon")
	if err != nil || f == nil {
		return utils.Error(c, 400, "chat_icon is required")
	}
	u, upErr := utils.SaveUploadedFile(c, f)
	if upErr != nil {
		return utils.Error(c, 500, "Failed Update Subject Chat Icon", upErr.Error())
	}
	var row map[string]interface{}
	a.DB.Raw(`UPDATE learning_subjects SET chat_icon_url=?, updated_at=NOW() WHERE id=? RETURNING *`, u, id).Scan(&row)
	return utils.Success(c, 200, "Success Update Subject Chat Icon", row)
}

func (a *AppContext) SubmitExamPackageByTeacher(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	var body struct {
		QuestionBankIDs         interface{} `json:"question_bank_ids"`
		ShuffleQuestions        bool        `json:"shuffle_questions"`
		QuestionDurationSeconds int         `json:"question_duration_seconds"`
	}
	_ = c.BodyParser(&body)
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_assignments
		SET exam_status='SUBMITTED', question_bank_ids=?, shuffle_questions=?, question_duration_seconds=?, exam_submitted_at=NOW(), updated_at=NOW()
		WHERE id=? RETURNING *
	`, body.QuestionBankIDs, body.ShuffleQuestions, body.QuestionDurationSeconds, id).Scan(&row)
	return utils.Success(c, 200, "Success Submit Exam Package", row)
}

func (a *AppContext) GetQuizAssignmentOverviewForTeacher(c *fiber.Ctx) error {
	assignmentID := c.Params("assignmentId")
	var submitted []map[string]interface{}
	var pending []map[string]interface{}
	a.DB.Raw(`
		SELECT s.id, s.student_id, u.username, u.parent_email, s.submitted_at,
		       COALESCE((SELECT COUNT(*) FROM learning_quiz_violation_logs v WHERE v.submission_id=s.id),0)::int AS violation_count
		FROM learning_submissions s
		INNER JOIN users u ON u.id=s.student_id
		WHERE s.assignment_id=? AND s.is_submitted=true
		ORDER BY s.submitted_at DESC NULLS LAST
	`, assignmentID).Scan(&submitted)
	a.DB.Raw(`
		SELECT u.id AS student_id, u.username, u.parent_email
		FROM learning_assignments a
		INNER JOIN learning_subjects ls ON ls.id=a.subject_id
		INNER JOIN users u ON u.class_id=ls.class_id AND u.role='SISWA'
		WHERE a.id=? AND NOT EXISTS (
			SELECT 1 FROM learning_submissions s WHERE s.assignment_id=a.id AND s.student_id=u.id AND s.is_submitted=true
		)
		ORDER BY u.username ASC
	`, assignmentID).Scan(&pending)
	return utils.Success(c, 200, "Success Get Quiz Assignment Overview", fiber.Map{
		"submitted_students": submitted,
		"pending_students":   pending,
	})
}

func (a *AppContext) GetFinalGradeReportForTeacher(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	semesterID := c.Query("semester_id")
	_ = semesterID
	var assignments []map[string]interface{}
	a.DB.Raw(`
		SELECT id,title,assignment_type,is_exam,semester_id
		FROM learning_assignments
		WHERE subject_id=?
		ORDER BY created_at ASC
	`, subjectID).Scan(&assignments)

	var students []map[string]interface{}
	a.DB.Raw(`
		SELECT u.id, u.username, u.class_id
		FROM learning_subjects ls
		INNER JOIN users u ON u.class_id=ls.class_id AND u.role='SISWA'
		WHERE ls.id=?
		ORDER BY u.username ASC
	`, subjectID).Scan(&students)

	var rows []map[string]interface{}
	for _, s := range students {
		studentID := fmt.Sprint(s["id"])
		scoreMap := map[string]interface{}{}
		var scoredCount int
		var avg float64
		var n int
		for _, aItem := range assignments {
			aid := fmt.Sprint(aItem["id"])
			var sc struct{ Score *float64 }
			a.DB.Raw(`SELECT score FROM learning_submissions WHERE assignment_id=? AND student_id=? LIMIT 1`, aid, studentID).Scan(&sc)
			if sc.Score != nil {
				scoreMap[aid] = *sc.Score
				scoredCount++
				avg += *sc.Score
				n++
			} else {
				scoreMap[aid] = nil
			}
		}
		finalAvg := interface{}(nil)
		if n > 0 {
			finalAvg = float64(int((avg/float64(n))*100)) / 100
		}
		rows = append(rows, map[string]interface{}{
			"student_id":    s["id"],
			"student_name":  s["username"],
			"class_id":      s["class_id"],
			"scores":        scoreMap,
			"scored_count":  scoredCount,
			"final_average": finalAvg,
		})
	}
	return utils.Success(c, 200, "Success Get Final Grade Report", fiber.Map{
		"assignments": assignments,
		"students":    rows,
	})
}

func (a *AppContext) GetLearningQuestionBank(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	keyword := strings.TrimSpace(c.Query("keyword"))
	qType := strings.TrimSpace(c.Query("question_type"))
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "20"), 20)
	offset := (page - 1) * limit

	q := a.DB.Table("learning_question_bank").Where("subject_id = ?", subjectID)
	if keyword != "" {
		q = q.Where("question_text ILIKE ?", "%"+keyword+"%")
	}
	if qType != "" {
		q = q.Where("question_type = ?", strings.ToUpper(qType))
	}
	var total int64
	q.Count(&total)
	var rows []map[string]interface{}
	q.Order("id DESC").Limit(limit).Offset(offset).Scan(&rows)
	return utils.Success(c, 200, "Success Get Question Bank", fiber.Map{
		"data":  rows,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (a *AppContext) CreateLearningQuestionBankItem(c *fiber.Ctx) error {
	var body struct {
		SubjectID     int         `json:"subject_id"`
		QuestionType  string      `json:"question_type"`
		QuestionText  string      `json:"question_text"`
		Options       interface{} `json:"options"`
		CorrectOption interface{} `json:"correct_option"`
		Rubric        string      `json:"rubric"`
	}
	_ = c.BodyParser(&body)
	userID := c.Locals("userID").(uint)
	if body.SubjectID == 0 || strings.TrimSpace(body.QuestionText) == "" || strings.TrimSpace(body.QuestionType) == "" {
		return utils.Error(c, 400, "subject_id, question_type and question_text are required")
	}
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_question_bank (subject_id, question_type, question_text, options, correct_option, rubric, created_by, created_at)
		VALUES (?, ?, ?, ?::jsonb, ?, ?, ?, NOW())
		RETURNING *
	`, body.SubjectID, strings.ToUpper(body.QuestionType), body.QuestionText, toJSONRaw(body.Options), body.CorrectOption, nullIfEmpty(body.Rubric), userID).Scan(&row)
	return utils.Success(c, 201, "Success Create Question Bank Item", row)
}

func (a *AppContext) UpdateLearningQuestionBankItem(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		QuestionText  string      `json:"question_text"`
		Options       interface{} `json:"options"`
		CorrectOption interface{} `json:"correct_option"`
		Rubric        string      `json:"rubric"`
	}
	_ = c.BodyParser(&body)
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_question_bank
		SET question_text = COALESCE(NULLIF(?, ''), question_text),
		    options = COALESCE(?::jsonb, options),
		    correct_option = COALESCE(?, correct_option),
		    rubric = COALESCE(?, rubric)
		WHERE id = ?
		RETURNING *
	`, body.QuestionText, toJSONRaw(body.Options), body.CorrectOption, nullIfEmpty(body.Rubric), id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Question bank item not found")
	}
	return utils.Success(c, 200, "Success Update Question Bank Item", row)
}

func (a *AppContext) DeleteLearningQuestionBankItem(c *fiber.Ctx) error {
	id := c.Params("id")
	var row map[string]interface{}
	a.DB.Raw(`DELETE FROM learning_question_bank WHERE id=? RETURNING *`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Question bank item not found")
	}
	return utils.Success(c, 200, "Success Delete Question Bank Item", row)
}

func (a *AppContext) DeleteLearningQuestionBankItemsBulk(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	var body struct {
		IDs []int `json:"ids"`
	}
	_ = c.BodyParser(&body)
	if len(body.IDs) == 0 {
		return utils.Error(c, 400, "ids are required")
	}
	a.DB.Exec(`DELETE FROM learning_question_bank WHERE subject_id=? AND id IN ?`, subjectID, body.IDs)
	return utils.Success(c, 200, "Success Delete Question Bank Items Bulk", fiber.Map{"deleted_ids": body.IDs})
}

func (a *AppContext) GenerateLearningQuestionBankWithAI(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	var body struct {
		QuestionType           string `json:"question_type"`
		Topic                  string `json:"topic"`
		Count                  int    `json:"question_count"`
		LegacyCount            int    `json:"count"`
		Difficulty             string `json:"difficulty"`
		GradeLabel             string `json:"grade_label"`
		PhaseName              string `json:"phase_name"`
		CurriculumName         string `json:"curriculum_name"`
		AdditionalInstructions string `json:"additional_instructions"`
	}
	_ = c.BodyParser(&body)
	if body.Count <= 0 {
		body.Count = body.LegacyCount
	}
	if body.Count <= 0 {
		body.Count = 5
	}

	qType := strings.ToUpper(strings.TrimSpace(body.QuestionType))
	if qType == "" {
		qType = "MCQ"
	}
	if qType != "MCQ" && qType != "ESSAY" {
		return utils.Error(c, 400, "question_type must be MCQ atau ESSAY")
	}

	topic := strings.TrimSpace(body.Topic)
	if topic == "" {
		return utils.Error(c, 400, "topic is required")
	}

	difficulty := strings.ToUpper(strings.TrimSpace(body.Difficulty))
	if difficulty == "" {
		difficulty = "MENENGAH"
	}
	if difficulty != "MUDAH" && difficulty != "MENENGAH" && difficulty != "SULIT" {
		return utils.Error(c, 400, "difficulty must be MUDAH, MENENGAH, atau SULIT")
	}

	subject, code, message := a.loadGuruSubjectAccess(c, subjectID)
	if code != 0 {
		return utils.Error(c, code, message)
	}

	items, err := services.GenerateQuestionBankItemsWithOpenRouter(services.QuestionBankAIInput{
		SubjectName:            subject.Name,
		ClassName:              subject.ClassName,
		GradeLabel:             strings.TrimSpace(body.GradeLabel),
		PhaseName:              strings.TrimSpace(body.PhaseName),
		CurriculumName:         strings.TrimSpace(body.CurriculumName),
		Topic:                  topic,
		QuestionType:           qType,
		QuestionCount:          body.Count,
		Difficulty:             difficulty,
		AdditionalInstructions: strings.TrimSpace(body.AdditionalInstructions),
	})
	if err != nil {
		return utils.Error(c, 500, "Failed Generate Question Bank With AI", err.Error())
	}
	if len(items) == 0 {
		return utils.Error(c, 500, "Failed Generate Question Bank With AI", "Hasil OpenRouter tidak valid untuk dijadikan bank soal")
	}

	return utils.Success(c, 200, "Success Generate Question Bank Preview", fiber.Map{
		"total": len(items),
		"items": items,
	})
}

func (a *AppContext) SaveGeneratedLearningQuestionBankItems(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	userID := c.Locals("userID").(uint)
	var body struct {
		Items []map[string]interface{} `json:"items"`
	}
	_ = c.BodyParser(&body)
	if len(body.Items) == 0 {
		return utils.Error(c, 400, "items are required")
	}
	saved := make([]map[string]interface{}, 0, len(body.Items))
	for _, item := range body.Items {
		var row map[string]interface{}
		a.DB.Raw(`
			INSERT INTO learning_question_bank (subject_id, question_type, question_text, options, correct_option, rubric, created_by, created_at)
			VALUES (?, ?, ?, ?::jsonb, ?, ?, ?, NOW())
			RETURNING *
		`, subjectID, strings.ToUpper(fmt.Sprint(item["question_type"])), fmt.Sprint(item["question_text"]),
			toJSONRaw(item["options"]), item["correct_option"], nullIfEmpty(fmt.Sprint(item["rubric"])), userID).Scan(&row)
		if len(row) > 0 {
			saved = append(saved, row)
		}
	}
	return utils.Success(c, 201, "Success Save Generated Question Bank Items", saved)
}

func (a *AppContext) DownloadLearningQuestionBankTemplate(c *fiber.Ctx) error {
	qType := strings.ToUpper(strings.TrimSpace(c.Query("question_type", "MCQ")))
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", `attachment; filename="question-bank-template.csv"`)
	records := [][]string{}
	if qType == "ESSAY" {
		records = append(records, []string{"question_text", "rubric"})
		records = append(records, []string{
			"Jelaskan perbedaan antara simbiosis mutualisme dan parasitisme.",
			"Skor 0-100: ketepatan konsep, contoh yang relevan, dan kejelasan penjelasan.",
		})
		records = append(records, []string{
			"Mengapa menjaga kebersihan lingkungan sekolah itu penting?",
			"Skor 0-100: alasan logis, dampak, dan solusi yang disampaikan.",
		})
	} else {
		records = append(records, []string{"question_text", "option_a", "option_b", "option_c", "option_d", "correct_option_index"})
		records = append(records, []string{
			"Hasil dari 12 + 8 adalah ...",
			"18", "20", "22", "24", "1",
		})
		records = append(records, []string{
			"Ibu kota Indonesia adalah ...",
			"Bandung", "Surabaya", "Jakarta", "Medan", "2",
		})
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.WriteAll(records)
	w.Flush()
	return c.SendString(b.String())
}

func (a *AppContext) ImportLearningQuestionBankFromDocument(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	userID := c.Locals("userID").(uint)
	f, err := c.FormFile("document")
	if err != nil || f == nil {
		return utils.Error(c, 400, "document is required")
	}
	fileReader, openErr := f.Open()
	if openErr != nil {
		return utils.Error(c, 400, "failed to read uploaded document")
	}
	defer fileReader.Close()

	reader := csv.NewReader(fileReader)
	reader.TrimLeadingSpace = true
	rows, readErr := reader.ReadAll()
	if readErr != nil {
		return utils.Error(c, 400, "format file tidak valid, gunakan template CSV yang diunduh")
	}
	if len(rows) < 2 {
		return utils.Error(c, 400, "file tidak berisi data soal")
	}

	normalizeHeader := func(value string) string {
		next := strings.TrimSpace(strings.ToLower(value))
		next = strings.ReplaceAll(next, " ", "_")
		next = strings.ReplaceAll(next, "-", "_")
		return next
	}

	headers := make([]string, 0, len(rows[0]))
	for _, item := range rows[0] {
		headers = append(headers, normalizeHeader(item))
	}
	headerIndex := map[string]int{}
	for index, key := range headers {
		headerIndex[key] = index
	}

	getValue := func(record []string, key string) string {
		idx, ok := headerIndex[key]
		if !ok || idx < 0 || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	isEssayTemplate := headerIndex["rubric"] >= 0
	isMcqTemplate := headerIndex["option_a"] >= 0 && headerIndex["option_b"] >= 0 && headerIndex["option_c"] >= 0 && headerIndex["option_d"] >= 0

	if !isEssayTemplate && !isMcqTemplate {
		return utils.Error(c, 400, "header template tidak dikenali, unduh ulang template terbaru")
	}

	importedItems := make([]map[string]interface{}, 0)
	mcqCount := 0
	essayCount := 0

	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		record := rows[rowIndex]
		questionText := getValue(record, "question_text")
		if questionText == "" {
			continue
		}

		if isEssayTemplate && !isMcqTemplate {
			rubric := getValue(record, "rubric")
			var inserted map[string]interface{}
			a.DB.Raw(`
				INSERT INTO learning_question_bank (subject_id, question_type, question_text, rubric, created_by, created_at)
				VALUES (?, 'ESSAY', ?, ?, ?, NOW())
				RETURNING *
			`, subjectID, questionText, nullIfEmpty(rubric), userID).Scan(&inserted)
			if len(inserted) > 0 {
				importedItems = append(importedItems, inserted)
				essayCount++
			}
			continue
		}

		optionA := getValue(record, "option_a")
		optionB := getValue(record, "option_b")
		optionC := getValue(record, "option_c")
		optionD := getValue(record, "option_d")
		correctRaw := getValue(record, "correct_option_index")
		if optionA == "" || optionB == "" || optionC == "" || optionD == "" {
			continue
		}

		correct := 0
		if correctRaw == "1" || strings.EqualFold(correctRaw, "b") {
			correct = 1
		} else if correctRaw == "2" || strings.EqualFold(correctRaw, "c") {
			correct = 2
		} else if correctRaw == "3" || strings.EqualFold(correctRaw, "d") {
			correct = 3
		}

		var inserted map[string]interface{}
		a.DB.Raw(`
			INSERT INTO learning_question_bank (subject_id, question_type, question_text, options, correct_option, created_by, created_at)
			VALUES (?, 'MCQ', ?, ?::jsonb, ?, ?, NOW())
			RETURNING *
		`, subjectID, questionText, toJSONRaw([]string{optionA, optionB, optionC, optionD}), correct, userID).Scan(&inserted)
		if len(inserted) > 0 {
			importedItems = append(importedItems, inserted)
			mcqCount++
		}
	}

	if len(importedItems) == 0 {
		return utils.Error(c, 400, "tidak ada baris valid untuk diimpor, periksa isi template")
	}

	return utils.Success(c, 201, "Success Import Question Bank From Document", fiber.Map{
		"total": len(importedItems),
		"mcq":   mcqCount,
		"essay": essayCount,
		"items": importedItems,
	})
}

func (a *AppContext) GenerateLearningMaterialPptWithAI(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	var body struct {
		Topic                  string `json:"topic"`
		Title                  string `json:"title"`
		Content                string `json:"content"`
		LearningGoals          string `json:"learning_goals"`
		AdditionalInstructions string `json:"additional_instructions"`
		SlideCount             int    `json:"slide_count"`
	}
	_ = c.BodyParser(&body)
	subject, code, message := a.loadGuruSubjectAccess(c, subjectID)
	if code != 0 {
		return utils.Error(c, code, message)
	}

	title := strings.TrimSpace(body.Title)
	topic := strings.TrimSpace(body.Topic)
	if title == "" {
		return utils.Error(c, 400, "title is required")
	}
	if topic == "" {
		return utils.Error(c, 400, "topic is required")
	}
	if body.SlideCount < 3 || body.SlideCount > 15 {
		return utils.Error(c, 400, "slide_count must be between 3 and 15")
	}

	outline, err := services.GeneratePowerPointOutlineWithOpenRouter(services.PowerPointAIInput{
		SubjectName:            subject.Name,
		ClassName:              subject.ClassName,
		Topic:                  topic,
		MaterialTitle:          title,
		SlideCount:             body.SlideCount,
		TeacherSummary:         strings.TrimSpace(body.Content),
		LearningGoals:          strings.TrimSpace(body.LearningGoals),
		AdditionalInstructions: strings.TrimSpace(body.AdditionalInstructions),
	})
	if err != nil {
		return utils.Error(c, 500, "Failed Generate AI PowerPoint Preview", err.Error())
	}

	return utils.Success(c, 200, "Success Generate AI PowerPoint Preview", fiber.Map{
		"presentation_title": outline.PresentationTitle,
		"summary":            outline.Summary,
		"slides_total":       len(outline.Slides),
		"slides":             outline.Slides,
	})
}

func (a *AppContext) PublishLearningMaterialPptWithAI(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	userID := c.Locals("userID").(uint)
	var body struct {
		Title             string                   `json:"title"`
		Content           string                   `json:"content"`
		PresentationTitle string                   `json:"presentation_title"`
		Slides            []map[string]interface{} `json:"slides"`
	}
	_ = c.BodyParser(&body)

	subject, code, message := a.loadGuruSubjectAccess(c, subjectID)
	if code != 0 {
		return utils.Error(c, code, message)
	}

	title := strings.TrimSpace(body.Title)
	if title == "" {
		return utils.Error(c, 400, "title is required")
	}

	slides := normalizePowerPointSlidesForGo(body.Slides, fallbackTitle(body.PresentationTitle, title))
	if len(slides) == 0 {
		return utils.Error(c, 400, "slides preview is required")
	}

	pptFile, err := services.BuildPowerPointMaterialFile(
		fallbackTitle(body.PresentationTitle, title),
		fmt.Sprintf("%s • %s", subject.Name, subject.ClassName),
		subject.Name,
		subject.ClassName,
		slides,
		"uploads",
	)
	if err != nil {
		return utils.Error(c, 500, "Failed Publish Learning Material AI PPTX", err.Error())
	}
	defer func() {
		_ = os.Remove(pptFile.OutputPath)
	}()

	attachmentURL, err := utils.UploadLocalFileToAlentest(pptFile.OutputPath, pptFile.FileName, pptFile.MimeType)
	if err != nil {
		return utils.Error(c, 500, "Failed Upload Generated PPTX", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_materials (subject_id, title, content, attachment_url, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, NOW()) RETURNING *
	`, subjectID, title, strings.TrimSpace(body.Content), attachmentURL, userID).Scan(&row)

	return utils.Success(c, 201, "Success Publish AI PowerPoint Material", fiber.Map{
		"material":       row,
		"slides_total":   len(slides),
		"attachment_url": attachmentURL,
	})
}

func toJSONRaw(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return string(raw)
}

func fallbackTopic(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "materi"
	}
	return v
}

func fallbackTitle(title, topic string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return "Materi Pembelajaran"
	}
	return "Materi " + topic
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type guruSubjectAccess struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	ClassName string `json:"class_name"`
	SchoolID  uint   `json:"school_id"`
	TeacherID uint   `json:"teacher_id"`
}

func (a *AppContext) loadGuruSubjectAccess(c *fiber.Ctx, subjectID string) (*guruSubjectAccess, int, string) {
	schoolID := c.Locals("schoolID").(uint)
	teacherID := c.Locals("userID").(uint)

	var subject guruSubjectAccess
	a.DB.Raw(`
		SELECT ls.id, ls.name, ls.school_id, ls.teacher_id, COALESCE(c.class_name, '') AS class_name
		FROM learning_subjects ls
		LEFT JOIN class c ON c.id = ls.class_id
		WHERE ls.id = ?
	`, subjectID).Scan(&subject)

	if subject.ID == 0 || subject.SchoolID != schoolID {
		return nil, 404, "Subject not found"
	}

	if subject.TeacherID != teacherID {
		return nil, 403, "Forbidden subject access"
	}

	return &subject, 0, ""
}

func normalizePowerPointSlidesForGo(raw []map[string]interface{}, fallbackTitle string) []services.PowerPointSlide {
	slides := make([]services.PowerPointSlide, 0, len(raw))
	for index, slide := range raw {
		title := cleanStringValue(slide["title"])
		if title == "" {
			title = fmt.Sprintf("%s %d", fallbackTitle, index+1)
		}

		bullets := make([]string, 0, 5)
		switch value := slide["bullets"].(type) {
		case []interface{}:
			for _, item := range value {
				text := strings.TrimSpace(fmt.Sprint(item))
				if text == "" {
					continue
				}
				bullets = append(bullets, text)
				if len(bullets) >= 5 {
					break
				}
			}
		case []string:
			for _, item := range value {
				text := strings.TrimSpace(item)
				if text == "" {
					continue
				}
				bullets = append(bullets, text)
				if len(bullets) >= 5 {
					break
				}
			}
		}

		if len(bullets) == 0 {
			continue
		}

		slides = append(slides, services.PowerPointSlide{
			Title:        title,
			Bullets:      bullets,
			SpeakerNotes: cleanStringValue(slide["speaker_notes"]),
		})
	}

	return slides
}

func cleanStringValue(value interface{}) string {
	if value == nil {
		return ""
	}

	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func (a *AppContext) UpdateLearningAssignmentByTeacher(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	var cur map[string]interface{}
	a.DB.Raw(`SELECT * FROM learning_assignments WHERE id=?`, id).Scan(&cur)
	if len(cur) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	title := c.FormValue("title", asString(cur["title"]))
	description := c.FormValue("description", asString(cur["description"]))
	dueDate := c.FormValue("due_date", asString(cur["due_date"]))
	assignmentType := strings.ToUpper(strings.TrimSpace(c.FormValue("assignment_type", asString(cur["assignment_type"]))))
	if assignmentType == "" {
		assignmentType = asString(cur["assignment_type"])
	}
	attachment := asString(cur["attachment_url"])
	if strings.ToLower(c.FormValue("remove_attachment")) == "true" || assignmentType == "MANUAL" {
		attachment = ""
	}
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		u, upErr := utils.SaveUploadedFile(c, f)
		if upErr == nil {
			attachment = u
		}
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_assignments
		SET title=?, description=?, due_date=?, assignment_type=?, attachment_url=?
		WHERE id=? RETURNING *
	`, title, description, nullIfEmpty(dueDate), assignmentType, nullIfEmpty(attachment), id).Scan(&row)
	return utils.Success(c, 200, "Success Update Assignment", row)
}

func (a *AppContext) DeleteLearningAssignmentByTeacher(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	var row map[string]interface{}
	a.DB.Raw(`
		DELETE FROM learning_assignments
		WHERE id=? AND COALESCE(is_exam,false)=false AND assignment_type IN ('FILE','MANUAL')
		RETURNING *
	`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	return utils.Success(c, 200, "Success Delete Assignment", row)
}
