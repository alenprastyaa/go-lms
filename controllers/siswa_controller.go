package controllers

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

func (a *AppContext) GetSiswaDashboard(c *fiber.Ctx) error {
	studentID := c.Locals("userID").(uint)

	var student map[string]interface{}
	a.DB.Raw(`
		SELECT u.id,u.username,u.parent_email,u.phone_number,c.class_name,s.name AS school_name
		FROM users u
		LEFT JOIN class c ON c.id=u.class_id
		LEFT JOIN schools s ON s.id=u.school_id
		WHERE u.id=?
	`, studentID).Scan(&student)

	var today map[string]interface{}
	a.DB.Raw(`SELECT attendance_date,clock_in,clock_out,status,image FROM attendance WHERE user_id=? AND attendance_date=CURRENT_DATE LIMIT 1`, studentID).Scan(&today)

	var overviewRows []struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	}
	a.DB.Raw(`
		SELECT 'attendance_total' AS key, COUNT(*)::int AS value FROM attendance WHERE user_id=?
		UNION ALL
		SELECT 'receipts_total' AS key, COUNT(*)::int AS value FROM payment_receipt WHERE user_id=?
		UNION ALL
		SELECT 'receipts_this_month' AS key, COUNT(*)::int AS value
		FROM payment_receipt WHERE user_id=? AND DATE_TRUNC('month', created_at)=DATE_TRUNC('month', CURRENT_DATE)
	`, studentID, studentID, studentID).Scan(&overviewRows)
	overview := map[string]int{}
	for _, r := range overviewRows {
		overview[r.Key] = r.Value
	}

	var recentAttendance []map[string]interface{}
	a.DB.Raw(`SELECT attendance_date,clock_in,clock_out,status,image FROM attendance WHERE user_id=? ORDER BY attendance_date DESC, clock_in DESC LIMIT 8`, studentID).Scan(&recentAttendance)
	var recentReceipts []map[string]interface{}
	a.DB.Raw(`SELECT id,periode,description,created_at,image_path FROM payment_receipt WHERE user_id=? ORDER BY created_at DESC LIMIT 8`, studentID).Scan(&recentReceipts)
	var pendingAssignments []map[string]interface{}
	a.DB.Raw(`
		SELECT la.id,la.title,la.due_date,ls.name AS subject_name,c.class_name,sub.id AS submission_id,sub.score
		FROM users u
		INNER JOIN class c ON c.id=u.class_id
		INNER JOIN learning_subjects ls ON ls.class_id=c.id
		INNER JOIN learning_assignments la ON la.subject_id=ls.id
		LEFT JOIN learning_submissions sub ON sub.assignment_id=la.id AND sub.student_id=u.id
		WHERE u.id=?
		ORDER BY la.due_date ASC NULLS LAST, la.created_at DESC
		LIMIT 12
	`, studentID).Scan(&pendingAssignments)
	pendingCount := 0
	gradedCount := 0
	filteredPending := make([]map[string]interface{}, 0)
	for _, item := range pendingAssignments {
		if item["submission_id"] == nil {
			pendingCount++
			filteredPending = append(filteredPending, item)
		}
		if item["score"] != nil {
			gradedCount++
		}
	}
	overview["pending_assignments"] = pendingCount
	overview["graded_assignments"] = gradedCount

	return utils.Success(c, 200, "Success Get Siswa Dashboard", fiber.Map{
		"generatedAt":        time.Now().UTC().Format(time.RFC3339),
		"student":            student,
		"todayAttendance":    nilIfEmptyMap(today),
		"overview":           overview,
		"recentAttendance":   recentAttendance,
		"recentReceipts":     recentReceipts,
		"pendingAssignments": filteredPending,
	})
}

func (a *AppContext) GetStudentSubjects(c *fiber.Ctx) error {
	studentID := c.Locals("userID").(uint)
	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT ls.*, c.class_name, t.username AS teacher_name
		FROM users u
		INNER JOIN learning_subjects ls ON ls.class_id = u.class_id
		LEFT JOIN class c ON c.id = ls.class_id
		LEFT JOIN users t ON t.id = ls.teacher_id
		WHERE u.id = ?
		ORDER BY ls.created_at DESC
	`, studentID).Scan(&rows)
	return utils.Success(c, 200, "Success Get Student Subjects", rows)
}

func (a *AppContext) StartLearningQuizAttempt(c *fiber.Ctx) error {
	assignmentID := c.Params("assignmentId")
	studentID := c.Locals("userID").(uint)
	var body struct {
		ExamCode string `json:"exam_code"`
	}
	_ = c.BodyParser(&body)

	var assignment struct {
		ID                      int        `gorm:"column:id"`
		SchoolID                int        `gorm:"column:school_id"`
		ClassID                 int        `gorm:"column:class_id"`
		AssignmentType          string     `gorm:"column:assignment_type"`
		IsExam                  bool       `gorm:"column:is_exam"`
		ExamStatus              *string    `gorm:"column:exam_status"`
		ExamCode                *string    `gorm:"column:exam_code"`
		StartAt                 *time.Time `gorm:"column:start_at"`
		DueDate                 *time.Time `gorm:"column:due_date"`
		QuestionDurationSeconds *int       `gorm:"column:question_duration_seconds"`
		QuizPayloadText         string     `gorm:"column:quiz_payload_text"`
	}
	a.DB.Raw(`
		SELECT la.id, ls.school_id, ls.class_id, la.assignment_type, la.is_exam, la.exam_status, la.exam_code, la.start_at, la.due_date, la.question_duration_seconds,
		       COALESCE(la.quiz_payload::text, '[]') AS quiz_payload_text
		FROM learning_assignments la
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		WHERE la.id = ?
	`, assignmentID).Scan(&assignment)
	if assignment.ID == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	if assignment.AssignmentType != "MCQ" && assignment.AssignmentType != "ESSAY" {
		return utils.Error(c, 400, "This assignment is not a quiz")
	}
	var student struct {
		ID      int `gorm:"column:id"`
		ClassID int `gorm:"column:class_id"`
		School  int `gorm:"column:school_id"`
	}
	a.DB.Raw(`SELECT id, class_id, school_id FROM users WHERE id = ?`, studentID).Scan(&student)
	if student.ID == 0 || student.ClassID != assignment.ClassID || student.School != assignment.SchoolID {
		return utils.Error(c, 403, "Forbidden assignment access")
	}
	now := time.Now()
	if assignment.IsExam {
		if assignment.ExamStatus == nil || strings.ToUpper(strings.TrimSpace(*assignment.ExamStatus)) != "PUBLISHED" {
			return utils.Error(c, 400, "Exam is not published yet")
		}
		if assignment.StartAt == nil || assignment.StartAt.After(now) {
			return utils.Error(c, 400, "Exam has not started yet")
		}
		if strings.TrimSpace(body.ExamCode) == "" || !strings.EqualFold(strings.TrimSpace(body.ExamCode), strings.TrimSpace(valueOrEmpty(assignment.ExamCode))) {
			return utils.Error(c, 400, "Exam code is invalid")
		}
	}
	if assignment.DueDate != nil && assignment.DueDate.Before(now) {
		return utils.Error(c, 400, "Quiz deadline has passed")
	}

	var existing map[string]interface{}
	a.DB.Raw(`SELECT * FROM learning_submissions WHERE assignment_id = ? AND student_id = ? LIMIT 1`, assignmentID, studentID).Scan(&existing)
	if isSubmitted(existing) {
		return utils.Error(c, 400, "Quiz has already been submitted")
	}

	var row map[string]interface{}
	if len(existing) == 0 {
		a.DB.Raw(`
			INSERT INTO learning_submissions (assignment_id,student_id,started_at,is_submitted)
			VALUES (?, ?, NOW(), false)
			RETURNING *
		`, assignmentID, studentID).Scan(&row)
	} else {
		row = existing
	}
	startedAt := parseTimeAny(row["started_at"])
	questionCount := countQuizQuestionsFromText(assignment.QuizPayloadText)
	expiresAt := interface{}(nil)
	if startedAt != nil && assignment.QuestionDurationSeconds != nil && *assignment.QuestionDurationSeconds > 0 {
		windowSeconds := *assignment.QuestionDurationSeconds
		if !assignment.IsExam {
			if questionCount > 0 {
				windowSeconds = windowSeconds * questionCount
			}
		}
		exp := startedAt.Add(time.Duration(windowSeconds) * time.Second)
		expiresAt = exp.UTC().Format(time.RFC3339)
	}
	return utils.Success(c, 200, "Success Start Quiz Attempt", fiber.Map{
		"assignment_id":             assignment.ID,
		"started_at":                startedAt.UTC().Format(time.RFC3339),
		"expires_at":                expiresAt,
		"question_duration_seconds": assignment.QuestionDurationSeconds,
		"question_count":            questionCount,
	})
}

func (a *AppContext) SubmitLearningAssignment(c *fiber.Ctx) error {
	assignmentID := c.Params("assignmentId")
	studentID := c.Locals("userID").(uint)
	submissionText := c.FormValue("submission_text")
	answerPayload := c.FormValue("answer_payload")
	rawAnswers := c.FormValue("answers")

	attachmentURL := ""
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		if u, upErr := utils.SaveUploadedFile(c, f); upErr == nil {
			attachmentURL = u
		}
	}

	var assignment struct {
		ID              int        `gorm:"column:id"`
		AssignmentType  string     `gorm:"column:assignment_type"`
		IsExam          bool       `gorm:"column:is_exam"`
		SchoolID        int        `gorm:"column:school_id"`
		ClassID         int        `gorm:"column:class_id"`
		DueDate         *time.Time `gorm:"column:due_date"`
		QuizPayloadText string     `gorm:"column:quiz_payload_text"`
	}
	a.DB.Raw(`
		SELECT la.id, la.assignment_type, la.is_exam, ls.school_id, ls.class_id, la.due_date, COALESCE(la.quiz_payload::text, '[]') AS quiz_payload_text
		FROM learning_assignments la
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		WHERE la.id = ?
	`, assignmentID).Scan(&assignment)
	if assignment.ID == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	var student struct {
		ID      int `gorm:"column:id"`
		ClassID int `gorm:"column:class_id"`
		School  int `gorm:"column:school_id"`
	}
	a.DB.Raw(`SELECT id, class_id, school_id FROM users WHERE id = ?`, studentID).Scan(&student)
	if student.ID == 0 || student.ClassID != assignment.ClassID || student.School != assignment.SchoolID {
		return utils.Error(c, 403, "Forbidden assignment access")
	}
	if assignment.AssignmentType == "MANUAL" {
		return utils.Error(c, 400, "Manual assessment is graded directly by the teacher")
	}
	if assignment.DueDate != nil && assignment.DueDate.Before(time.Now()) {
		return utils.Error(c, 400, "Quiz deadline has passed")
	}

	var existing map[string]interface{}
	a.DB.Raw(`SELECT * FROM learning_submissions WHERE assignment_id=? AND student_id=? LIMIT 1`, assignmentID, studentID).Scan(&existing)
	if assignment.AssignmentType != "FILE" {
		if isSubmitted(existing) {
			return utils.Error(c, 400, "Quiz has already been submitted")
		}
		if parseTimeAny(existing["started_at"]) == nil {
			return utils.Error(c, 400, "Quiz attempt has not been started")
		}
	}

	score := interface{}(nil)
	autoGraded := false
	parsedAnswerJSON := interface{}(nil)
	if assignment.AssignmentType == "MCQ" || assignment.AssignmentType == "ESSAY" {
		quizPayload, err := parseQuizPayloadText(assignment.QuizPayloadText)
		if err != nil || len(quizPayload) == 0 {
			return utils.Error(c, 400, "Quiz questions are invalid")
		}
		if strings.TrimSpace(rawAnswers) == "" {
			rawAnswers = answerPayload
		}
		normalizedAnswers, err := parseStudentAnswers(assignment.AssignmentType, rawAnswers, quizPayload)
		if err != nil {
			return utils.Error(c, 400, err.Error())
		}
		raw, _ := json.Marshal(normalizedAnswers)
		parsedAnswerJSON = string(raw)
		if assignment.AssignmentType == "MCQ" {
			score = calculateMcqScore(quizPayload, normalizedAnswers)
			autoGraded = true
		}
	}

	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_submissions (
		  assignment_id,student_id,started_at,submission_text,answer_payload,attachment_url,submitted_at,is_submitted,score,auto_graded
		) VALUES (?, ?, NOW(), ?, ?::jsonb, ?, NOW(), true, ?, ?)
		ON CONFLICT (assignment_id,student_id)
		DO UPDATE SET submission_text=EXCLUDED.submission_text,
		              answer_payload=EXCLUDED.answer_payload,
		              attachment_url=COALESCE(NULLIF(EXCLUDED.attachment_url,''), learning_submissions.attachment_url),
		              submitted_at=NOW(),
		              is_submitted=true,
		              score=EXCLUDED.score,
		              auto_graded=EXCLUDED.auto_graded
		RETURNING *
	`, assignmentID, studentID, nullIfEmpty(submissionText), parsedAnswerJSON, nullIfEmpty(attachmentURL), score, autoGraded).Scan(&row)
	return utils.Success(c, 201, "Success Submit Assignment", row)
}

func (a *AppContext) RecordQuizViolation(c *fiber.Ctx) error {
	assignmentID := c.Params("assignmentId")
	studentID := c.Locals("userID").(uint)
	var body struct {
		SubmissionID     interface{} `json:"submission_id"`
		ViolationType    string      `json:"violation_type"`
		ViolationMessage string      `json:"violation_message"`
	}
	_ = c.BodyParser(&body)

	var assignment struct {
		ID             int    `gorm:"column:id"`
		AssignmentType string `gorm:"column:assignment_type"`
		SchoolID       int    `gorm:"column:school_id"`
		ClassID        int    `gorm:"column:class_id"`
	}
	a.DB.Raw(`
		SELECT la.id, la.assignment_type, ls.school_id, ls.class_id
		FROM learning_assignments la
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		WHERE la.id = ?
	`, assignmentID).Scan(&assignment)
	if assignment.ID == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	if assignment.AssignmentType != "MCQ" && assignment.AssignmentType != "ESSAY" {
		return utils.Error(c, 400, "Violation log only supported for quiz assignments")
	}
	var student struct {
		ID      int `gorm:"column:id"`
		ClassID int `gorm:"column:class_id"`
		School  int `gorm:"column:school_id"`
	}
	a.DB.Raw(`SELECT id, class_id, school_id FROM users WHERE id=?`, studentID).Scan(&student)
	if student.ID == 0 || student.ClassID != assignment.ClassID || student.School != assignment.SchoolID {
		return utils.Error(c, 403, "Forbidden assignment access")
	}

	submissionID := body.SubmissionID
	if submissionID == nil {
		var tmp struct{ ID int }
		a.DB.Raw(`SELECT id FROM learning_submissions WHERE assignment_id=? AND student_id=? LIMIT 1`, assignmentID, studentID).Scan(&tmp)
		submissionID = tmp.ID
	}
	if submissionID == nil || submissionID == 0 {
		return utils.Error(c, 404, "Quiz attempt not found")
	}
	a.DB.Exec(`
		INSERT INTO learning_quiz_violation_logs (submission_id,assignment_id,student_id,violation_type,violation_message,created_at)
		VALUES (?, ?, ?, ?, ?, NOW())
	`, submissionID, assignmentID, studentID, fallbackStr(body.ViolationType, "FOCUS_LOST"), nullIfEmpty(body.ViolationMessage))
	maxViolations := envInt("QUIZ_MAX_VIOLATIONS", 3)
	var violationCount int
	a.DB.Raw(`SELECT COUNT(*)::int FROM learning_quiz_violation_logs WHERE submission_id = ?`, submissionID).Scan(&violationCount)
	autoSubmitted := false
	if maxViolations > 0 && violationCount >= maxViolations {
		a.DB.Exec(`
			UPDATE learning_submissions
			SET submitted_at = COALESCE(submitted_at, NOW()),
			    is_submitted = true
			WHERE id = ? AND COALESCE(is_submitted, false) = false
		`, submissionID)
		autoSubmitted = true
	}
	return utils.Success(c, 201, "Success Record Quiz Violation", fiber.Map{
		"recorded":        true,
		"violation_count": violationCount,
		"auto_submitted":  autoSubmitted,
		"max_violations":  maxViolations,
	})
}

func (a *AppContext) CheckIn(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	file, err := c.FormFile("image")
	if err != nil || file == nil {
		return utils.Error(c, 400, "Image is required")
	}
	var exists int64
	a.DB.Raw(`SELECT COUNT(*) FROM attendance WHERE user_id=? AND attendance_date=CURRENT_DATE`, userID).Scan(&exists)
	if exists > 0 {
		return utils.Error(c, 400, "Anda sudah melakukan absensi masuk hari ini.")
	}
	url, upErr := utils.SaveUploadedFile(c, file)
	if upErr != nil {
		return utils.Error(c, 500, "Check-in failed", upErr.Error())
	}
	a.DB.Exec(`INSERT INTO attendance (user_id,attendance_date,image,clock_in,status) VALUES (?, CURRENT_DATE, ?, NOW(), 'hadir')`, userID, url)
	return c.JSON(fiber.Map{"success": true, "message": "Check-in successful."})
}

func (a *AppContext) CheckOut(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE attendance SET clock_out = NOW()
		WHERE user_id=? AND attendance_date=CURRENT_DATE AND clock_out IS NULL
		RETURNING *
	`, userID).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "You have not checked in today.")
	}
	return c.JSON(fiber.Map{"success": true, "message": "Check-out successful."})
}

func (a *AppContext) GetListAttendance(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "10"), 10)
	offset := (page - 1) * limit
	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT u.username,a.attendance_date,a.image,a.clock_in,a.clock_out,a.status
		FROM attendance a
		LEFT JOIN users u ON a.user_id=u.id
		WHERE a.user_id=?
		ORDER BY a.attendance_date DESC, a.clock_in DESC
		LIMIT ? OFFSET ?
	`, userID, limit, offset).Scan(&rows)
	return utils.Success(c, 200, "Success Get Attendance Data", fiber.Map{"page": page, "limit": limit, "data": rows})
}

func nilIfEmptyMap(v map[string]interface{}) interface{} {
	if len(v) == 0 {
		return nil
	}
	return v
}

func fallbackStr(v, def string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	return v
}

func nullIfEmptyJSON(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func isSubmitted(row map[string]interface{}) bool {
	if len(row) == 0 {
		return false
	}
	switch t := row["is_submitted"].(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

func parseTimeAny(v interface{}) *time.Time {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		return &t
	case *time.Time:
		return t
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return &parsed
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", t); err == nil {
			return &parsed
		}
	}
	return nil
}

func countQuizQuestionsFromText(raw string) int {
	payload, err := parseQuizPayloadText(raw)
	if err != nil {
		return 0
	}
	return len(payload)
}

func parseQuizPayloadText(raw string) ([]map[string]interface{}, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []map[string]interface{}{}, nil
	}
	var payload []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func parseStudentAnswers(assignmentType, rawAnswers string, quizPayload []map[string]interface{}) ([]map[string]interface{}, error) {
	var answers []map[string]interface{}
	if err := json.Unmarshal([]byte(rawAnswers), &answers); err != nil {
		return nil, fmt.Errorf("Answers must be an array")
	}
	if len(answers) != len(quizPayload) {
		return nil, fmt.Errorf("Answers count does not match questions")
	}
	result := make([]map[string]interface{}, 0, len(answers))
	for idx, answer := range answers {
		if assignmentType == "MCQ" {
			selected := answer["selected_option"]
			if selected == nil || fmt.Sprint(selected) == "" {
				result = append(result, map[string]interface{}{"selected_option": nil})
				continue
			}
			selectedIndex, err := toInt(selected)
			if err != nil {
				return nil, fmt.Errorf("Answer for question %d is invalid", idx+1)
			}
			options, _ := quizPayload[idx]["options"].([]interface{})
			if selectedIndex < 0 || selectedIndex >= len(options) {
				return nil, fmt.Errorf("Answer for question %d is invalid", idx+1)
			}
			result = append(result, map[string]interface{}{"selected_option": selectedIndex})
			continue
		}
		result = append(result, map[string]interface{}{"answer_text": strings.TrimSpace(fmt.Sprint(answer["answer_text"]))})
	}
	return result, nil
}

func calculateMcqScore(quizPayload []map[string]interface{}, answers []map[string]interface{}) float64 {
	if len(quizPayload) == 0 {
		return 0
	}
	correct := 0
	for i := range quizPayload {
		sel, _ := toInt(answers[i]["selected_option"])
		correctOption, _ := toInt(quizPayload[i]["correct_option"])
		if sel >= 0 && sel == correctOption {
			correct++
		}
	}
	value := (float64(correct) / float64(len(quizPayload))) * 100
	return float64(int(value*100)) / 100
}

func toInt(v interface{}) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int32:
		return int(t), nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			return 0, err
		}
		return i, nil
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		i, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return i, nil
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
