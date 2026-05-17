package controllers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

const maxGlobalQuizDurationMinutes = 180

func normalizeLearningQuestionDurationSeconds(mode string, secondsRaw string, minutesRaw string) (int, error) {
	secondsValue := utils.ToInt(secondsRaw, 0)
	if mode != "GLOBAL" {
		return secondsValue, nil
	}

	minutesValue := utils.ToInt(minutesRaw, 0)
	if minutesValue <= 0 {
		if secondsValue > maxGlobalQuizDurationMinutes*60 {
			return 0, fmt.Errorf("Durasi quiz global maksimal %d menit", maxGlobalQuizDurationMinutes)
		}
		return secondsValue, nil
	}

	if minutesValue > maxGlobalQuizDurationMinutes {
		// Guard old/stale clients that accidentally send seconds in the minutes field.
		if minutesValue <= maxGlobalQuizDurationMinutes*60 {
			return minutesValue, nil
		}
		return 0, fmt.Errorf("Durasi quiz global maksimal %d menit", maxGlobalQuizDurationMinutes)
	}

	return minutesValue * 60, nil
}

func (a *AppContext) GetAdminSubjects(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "10"), 10)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit
	usePagination := c.Query("paginate") == "1"
	search := strings.TrimSpace(c.Query("q"))

	whereClause := "WHERE ls.school_id = ?"
	args := []interface{}{schoolID}
	if search != "" {
		whereClause += " AND (LOWER(ls.name) LIKE LOWER(?) OR LOWER(c.class_name) LIKE LOWER(?) OR LOWER(t.username) LIKE LOWER(?))"
		keyword := "%" + search + "%"
		args = append(args, keyword, keyword, keyword)
	}

	if usePagination {
		var totalRow struct {
			Total int64 `json:"total"`
		}
		countQuery := `
			SELECT COUNT(*) AS total
			FROM learning_subjects ls
			LEFT JOIN class c ON c.id = ls.class_id
			LEFT JOIN users t ON t.id = ls.teacher_id
		` + whereClause
		_ = a.DB.Raw(countQuery, args...).Scan(&totalRow).Error

		var rows []map[string]interface{}
		listQuery := `
			SELECT ls.*, c.class_name, t.username AS teacher_name
			FROM learning_subjects ls
			LEFT JOIN class c ON c.id = ls.class_id
			LEFT JOIN users t ON t.id = ls.teacher_id
		` + whereClause + `
			ORDER BY ls.created_at DESC
			LIMIT ? OFFSET ?
		`
		listArgs := append(args, limit, offset)
		a.DB.Raw(listQuery, listArgs...).Scan(&rows)
		normalizeJakartaDateTimeRows(rows, "created_at", "updated_at")
		return utils.Success(c, 200, "Success Get Subjects", fiber.Map{
			"page":  page,
			"limit": limit,
			"total": totalRow.Total,
			"data":  rows,
		})
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT ls.*, c.class_name, t.username AS teacher_name
		FROM learning_subjects ls
		LEFT JOIN class c ON c.id = ls.class_id
		LEFT JOIN users t ON t.id = ls.teacher_id
	`+whereClause+`
		ORDER BY ls.created_at DESC
	`, args...).Scan(&rows)
	normalizeJakartaDateTimeRows(rows, "created_at", "updated_at")
	return utils.Success(c, 200, "Success Get Subjects", rows)
}

func (a *AppContext) CreateLearningSubject(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	classID := c.FormValue("class_id")
	teacherID := c.FormValue("teacher_id")
	name := c.FormValue("name")
	description := c.FormValue("description")
	if name == "" || classID == "" || teacherID == "" {
		return utils.Error(c, 400, "class_id, teacher_id, and name are required")
	}

	chatIconURL := ""
	if f, err := c.FormFile("chat_icon"); err == nil && f != nil {
		if saved, saveErr := utils.SaveUploadedFile(c, f); saveErr == nil {
			chatIconURL = saved
		}
	}

	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_subjects (school_id, class_id, teacher_id, name, description, chat_icon_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
		RETURNING *
	`, schoolID, classID, teacherID, name, description, nullIfEmpty(chatIconURL)).Scan(&row)
	normalizeJakartaDateTimeFields(row, "created_at", "updated_at")
	return utils.Success(c, 201, "Success Create Subject", row)
}

func (a *AppContext) UpdateLearningSubject(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)

	var current map[string]interface{}
	a.DB.Raw(`SELECT * FROM learning_subjects WHERE id = ? AND school_id = ?`, id, schoolID).Scan(&current)
	if len(current) == 0 {
		return utils.Error(c, 404, "Subject not found")
	}

	classID := c.FormValue("class_id", asString(current["class_id"]))
	teacherID := c.FormValue("teacher_id", asString(current["teacher_id"]))
	name := c.FormValue("name", asString(current["name"]))
	description := c.FormValue("description", asString(current["description"]))
	chatIconURL := asString(current["chat_icon_url"])
	if f, err := c.FormFile("chat_icon"); err == nil && f != nil {
		if saved, saveErr := utils.SaveUploadedFile(c, f); saveErr == nil {
			chatIconURL = saved
		}
	}

	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_subjects
		SET class_id = ?, teacher_id = ?, name = ?, description = ?, chat_icon_url = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING *
	`, classID, teacherID, name, description, nullIfEmpty(chatIconURL), id, schoolID).Scan(&row)
	normalizeJakartaDateTimeFields(row, "created_at", "updated_at")
	return utils.Success(c, 200, "Success Update Subject", row)
}

func (a *AppContext) DeleteLearningSubject(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var row map[string]interface{}
	a.DB.Raw(`DELETE FROM learning_subjects WHERE id = ? AND school_id = ? RETURNING *`, id, schoolID).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Subject not found")
	}
	normalizeJakartaDateTimeFields(row, "created_at", "updated_at")
	return utils.Success(c, 200, "Success Delete Subject", row)
}

func (a *AppContext) GetSubjectAssignments(c *fiber.Ctx) error {
	subjectID := c.Params("subjectId")
	schoolID := c.Locals("schoolID").(uint)
	userRole := fmt.Sprint(c.Locals("userRole"))
	userID := c.Locals("userID").(uint)

	var subject struct {
		ID      int
		School  int `gorm:"column:school_id"`
		ClassID int `gorm:"column:class_id"`
	}
	a.DB.Raw(`SELECT id, school_id, class_id FROM learning_subjects WHERE id = ?`, subjectID).Scan(&subject)
	if subject.ID == 0 || uint(subject.School) != schoolID {
		return utils.Error(c, 404, "Subject not found")
	}

	if userRole == "SISWA" {
		var student struct {
			ID      int
			ClassID int `gorm:"column:class_id"`
		}
		a.DB.Raw(`SELECT id, class_id FROM users WHERE id = ?`, userID).Scan(&student)
		if student.ID == 0 || student.ClassID != subject.ClassID {
			return utils.Error(c, 403, "Forbidden assignment access")
		}

		var rows []map[string]interface{}
		a.DB.Raw(`
			SELECT
			  la.*,
			  TO_CHAR(la.start_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS start_at,
			  TO_CHAR(la.due_date, 'YYYY-MM-DD"T"HH24:MI:SS') AS due_date,
			  COALESCE(la.question_bank_ids::text, '[]') AS question_bank_ids,
			  COALESCE(la.quiz_payload::text, '[]') AS quiz_payload,
			  CASE
			    WHEN COALESCE(la.exam_status, '') IN ('SUBMITTED', 'PUBLISHED') THEN la.exam_status
			    WHEN la.exam_submitted_at IS NOT NULL OR jsonb_array_length(COALESCE(la.quiz_payload, '[]'::jsonb)) > 0 THEN 'SUBMITTED'
			    ELSE COALESCE(la.exam_status, 'REQUESTED')
			  END AS effective_exam_status,
			  ls.name AS subject_name,
			  c.class_name,
			  t.username AS teacher_name,
			  sub.id AS submission_id,
			  sub.score,
			  sub.auto_graded,
			  sub.graded_by,
			  sub.graded_at,
			  sub.feedback,
			  sub.submission_text,
			  sub.attachment_url AS submission_attachment_url,
			  COALESCE(sub.answer_payload::text, '[]') AS answer_payload,
			  COALESCE(sub.access_blocked, false) AS access_blocked,
			  sub.access_block_reason,
			  TO_CHAR(sub.started_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS attempt_started_at,
			  TO_CHAR(sub.submitted_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS submitted_at,
			  sub.is_submitted
			FROM learning_assignments la
			INNER JOIN learning_subjects ls ON ls.id = la.subject_id
			LEFT JOIN class c ON c.id = ls.class_id
			LEFT JOIN users t ON t.id = ls.teacher_id
			LEFT JOIN LATERAL (
			  SELECT
			    s.id,
			    s.score,
			    s.auto_graded,
			    s.graded_by,
			    s.graded_at,
			    s.feedback,
			    s.submission_text,
			    s.attachment_url,
			    s.answer_payload,
			    s.access_blocked,
			    s.access_block_reason,
			    s.started_at,
			    s.submitted_at,
			    s.is_submitted
			  FROM learning_submissions s
			  WHERE s.assignment_id = la.id
			    AND s.student_id = ?
			  ORDER BY COALESCE(s.is_submitted, false) DESC,
			           s.submitted_at DESC NULLS LAST,
			           s.started_at DESC NULLS LAST,
			           s.id DESC
			  LIMIT 1
			) sub ON true
			WHERE la.subject_id = ?
			  AND ls.school_id = ?
			  AND (
			    COALESCE(la.is_exam, false) = false
			    OR (COALESCE(la.is_exam, false) = true AND la.exam_status = 'PUBLISHED')
			  )
			ORDER BY la.created_at DESC
		`, userID, subjectID, schoolID).Scan(&rows)
		normalizeJakartaDateTimeRows(rows, "start_at", "due_date", "exam_submitted_at", "exam_published_at", "created_at", "updated_at", "attempt_started_at", "submitted_at", "graded_at")
		for idx := range rows {
			normalizedDuration := normalizeStudentAssignmentDurationSeconds(
				boolFromAny(rows[idx]["is_exam"]),
				fmt.Sprint(rows[idx]["question_duration_mode"]),
				utils.ToInt(fmt.Sprint(rows[idx]["question_duration_seconds"]), 0),
			)
			if normalizedDuration > 0 {
				rows[idx]["question_duration_seconds"] = normalizedDuration
			}
		}
		a.syncAutoGradedMcqScores(rows)
		return utils.Success(c, 200, "Success Get Assignments", rows)
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT
		  la.*,
		  TO_CHAR(la.start_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS start_at,
		  TO_CHAR(la.due_date, 'YYYY-MM-DD"T"HH24:MI:SS') AS due_date,
		  TO_CHAR(la.exam_submitted_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS exam_submitted_at,
		  TO_CHAR(la.exam_published_at, 'YYYY-MM-DD"T"HH24:MI:SS') AS exam_published_at,
		  COALESCE(la.question_bank_ids::text, '[]') AS question_bank_ids,
		  COALESCE(la.quiz_payload::text, '[]') AS quiz_payload,
		  CASE
		    WHEN COALESCE(la.exam_status, '') IN ('SUBMITTED', 'PUBLISHED') THEN la.exam_status
		    WHEN la.exam_submitted_at IS NOT NULL OR jsonb_array_length(COALESCE(la.quiz_payload, '[]'::jsonb)) > 0 THEN 'SUBMITTED'
		    ELSE COALESCE(la.exam_status, 'REQUESTED')
		  END AS effective_exam_status,
		  ls.name AS subject_name,
		  c.class_name,
		  t.username AS teacher_name
		FROM learning_assignments la
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		LEFT JOIN class c ON c.id = ls.class_id
		LEFT JOIN users t ON t.id = ls.teacher_id
		WHERE la.subject_id = ? AND ls.school_id = ?
		ORDER BY la.created_at DESC
	`, subjectID, schoolID).Scan(&rows)
	normalizeJakartaDateTimeRows(rows, "start_at", "due_date", "exam_submitted_at", "exam_published_at", "created_at", "updated_at", "attempt_started_at", "submitted_at", "graded_at")
	return utils.Success(c, 200, "Success Get Assignments", rows)
}

func (a *AppContext) CreateLearningAssignment(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	schoolID := c.Locals("schoolID").(uint)
	role := strings.ToUpper(strings.TrimSpace(fmt.Sprint(c.Locals("userRole"))))

	subjectID := c.FormValue("subject_id")
	title := c.FormValue("title")
	description := c.FormValue("description")
	dueDate := c.FormValue("due_date")
	assignmentType := strings.ToUpper(strings.TrimSpace(c.FormValue("assignment_type")))
	isExam := strings.ToLower(c.FormValue("is_exam")) == "true"
	examCategory := c.FormValue("exam_category")
	examCode := strings.ToUpper(strings.TrimSpace(c.FormValue("exam_code")))
	startAt := c.FormValue("start_at")
	qDur := c.FormValue("question_duration_seconds")
	qDurMinutes := c.FormValue("question_duration_minutes")
	qDurMode := strings.ToUpper(strings.TrimSpace(c.FormValue("question_duration_mode")))
	examCount := c.FormValue("exam_target_question_count")
	questionBankIDsRaw := strings.TrimSpace(c.FormValue("question_bank_ids"))
	shuffleQuestions := strings.ToLower(strings.TrimSpace(c.FormValue("shuffle_questions"))) == "true"

	if assignmentType == "" {
		assignmentType = "FILE"
	}
	if qDurMode == "" {
		qDurMode = "PER_QUESTION"
	}
	if qDurMode != "PER_QUESTION" && qDurMode != "GLOBAL" {
		qDurMode = "PER_QUESTION"
	}
	qDurValue, durationErr := normalizeLearningQuestionDurationSeconds(qDurMode, qDur, qDurMinutes)
	if durationErr != nil {
		return utils.Error(c, 400, durationErr.Error())
	}
	if title == "" || subjectID == "" {
		return utils.Error(c, 400, "subject_id and title are required")
	}

	questionBankIDs := []int{}
	if questionBankIDsRaw != "" {
		if err := json.Unmarshal([]byte(questionBankIDsRaw), &questionBankIDs); err != nil {
			return utils.Error(c, 400, "question_bank_ids harus berupa JSON array id soal")
		}
	}

	var subject struct {
		ID int `json:"id"`
	}
	if role == "ADMIN" {
		a.DB.Raw(`
			SELECT ls.id
			FROM learning_subjects ls
			WHERE ls.id = ? AND ls.school_id = ?
			LIMIT 1
		`, subjectID, schoolID).Scan(&subject)
	} else {
		a.DB.Raw(`
			SELECT ls.id
			FROM learning_subjects ls
			LEFT JOIN class c ON c.id = ls.class_id
			WHERE ls.id = ?
			  AND ls.school_id = ?
			  AND (ls.teacher_id = ? OR c.wali_guru_id = ?)
			LIMIT 1
		`, subjectID, schoolID, userID, userID).Scan(&subject)
	}
	if subject.ID == 0 {
		return utils.Error(c, 404, "Subject not found")
	}

	attachmentURL := ""
	if f, err := c.FormFile("attachment"); err == nil && f != nil {
		if saved, saveErr := utils.SaveUploadedFile(c, f); saveErr == nil {
			attachmentURL = saved
		}
	}

	academicYearID, semesterID := a.resolveActiveAcademicPeriod(int(schoolID))
	quizPayload := []map[string]interface{}{}
	if assignmentType == "MCQ" || assignmentType == "ESSAY" {
		if isExam && len(questionBankIDs) == 0 {
			quizPayload = []map[string]interface{}{}
		} else if len(questionBankIDs) == 0 {
			return utils.Error(c, 400, "question_bank_ids wajib diisi untuk quiz")
		} else {
			query := `
				SELECT id, question_type, question_text, options, correct_option, rubric
				FROM learning_question_bank
				WHERE subject_id = ? AND id IN ?
			`
			if assignmentType == "MCQ" || assignmentType == "ESSAY" {
				query += " AND question_type = ?"
			}

			var selectedRows []map[string]interface{}
			a.DB.Raw(query, subjectID, questionBankIDs, assignmentType).Scan(&selectedRows)
			if len(selectedRows) == 0 {
				return utils.Error(c, 400, "soal yang dipilih tidak valid untuk assignment ini")
			}

			rowByID := map[int]map[string]interface{}{}
			for _, row := range selectedRows {
				if idFloat, ok := row["id"].(float64); ok {
					rowByID[int(idFloat)] = row
					continue
				}
				idInt := utils.ToInt(fmt.Sprint(row["id"]), 0)
				if idInt > 0 {
					rowByID[idInt] = row
				}
			}

			for _, qid := range questionBankIDs {
				row, ok := rowByID[qid]
				if !ok {
					continue
				}
				item := map[string]interface{}{
					"question_id":   row["id"],
					"question_type": row["question_type"],
					"question":      row["question_text"],
				}
				if assignmentType == "MCQ" {
					item["options"] = row["options"]
					item["correct_option"] = row["correct_option"]
				} else {
					item["rubric"] = row["rubric"]
				}
				quizPayload = append(quizPayload, item)
			}

			if len(quizPayload) == 0 {
				return utils.Error(c, 400, "tidak ada soal valid yang bisa dipakai untuk quiz")
			}
		}
	}

	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO learning_assignments (
		  subject_id, title, description, assignment_type, is_exam, exam_category, exam_code, exam_status,
		  question_bank_ids, shuffle_questions, quiz_payload,
		  start_at, managed_by_admin, exam_target_question_count, academic_year_id, semester_id,
		  question_duration_mode, question_duration_seconds, attachment_url, due_date, created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?::jsonb, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		RETURNING *
	`, 
		subjectID, title, description, assignmentType, isExam, nullIfEmpty(examCategory), nullIfEmpty(examCode),
		ternaryString(isExam, "REQUESTED", ""), toJSONRaw(questionBankIDs), shuffleQuestions, toJSONRaw(quizPayload),
		normalizeDateTimeLocalToWIB(startAt), true, nullIfEmpty(examCount),
		nullIfZero(academicYearID), nullIfZero(semesterID), qDurMode, nullIfZero(qDurValue), nullIfEmpty(attachmentURL),
		normalizeDateTimeLocalToWIB(dueDate), userID,
	).Scan(&row)
	normalizeJakartaDateTimeFields(row, "start_at", "due_date", "exam_submitted_at", "exam_published_at", "created_at", "updated_at")

	return utils.Success(c, 201, "Success Create Assignment", row)
}

func (a *AppContext) UpdateExamRequestByAdmin(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	subjectID := c.FormValue("subject_id")
	title := c.FormValue("title")
	description := c.FormValue("description")
	dueDate := c.FormValue("due_date")
	assignmentType := strings.ToUpper(strings.TrimSpace(c.FormValue("assignment_type")))
	examCategory := c.FormValue("exam_category")
	examCode := strings.ToUpper(strings.TrimSpace(c.FormValue("exam_code")))
	startAt := c.FormValue("start_at")
	qDur := c.FormValue("question_duration_seconds")
	qDurMinutes := c.FormValue("question_duration_minutes")
	qDurMode := strings.ToUpper(strings.TrimSpace(c.FormValue("question_duration_mode")))
	examCount := c.FormValue("exam_target_question_count")
	if assignmentType == "" {
		assignmentType = "MCQ"
	}
	if qDurMode == "" {
		qDurMode = "PER_QUESTION"
	}
	if qDurMode != "PER_QUESTION" && qDurMode != "GLOBAL" {
		qDurMode = "PER_QUESTION"
	}
	qDurValue, durationErr := normalizeLearningQuestionDurationSeconds(qDurMode, qDur, qDurMinutes)
	if durationErr != nil {
		return utils.Error(c, 400, durationErr.Error())
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_assignments
		SET subject_id = ?, title = ?, description = ?, due_date = ?, assignment_type = ?,
		    exam_category = ?, exam_code = ?, start_at = ?, question_duration_mode = ?, question_duration_seconds = ?,
		    exam_target_question_count = ?
		WHERE id = ? AND is_exam = true
		RETURNING *
	`, subjectID, title, description, normalizeDateTimeLocalToWIB(dueDate), assignmentType, nullIfEmpty(examCategory),
		nullIfEmpty(examCode), normalizeDateTimeLocalToWIB(startAt), qDurMode, nullIfZero(qDurValue), nullIfEmpty(examCount), id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	normalizeJakartaDateTimeFields(row, "start_at", "due_date", "exam_submitted_at", "exam_published_at", "created_at", "updated_at")
	return utils.Success(c, 200, "Success Update Exam Request", row)
}

func (a *AppContext) DeleteExamRequestByAdmin(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	var row map[string]interface{}
	a.DB.Raw(`DELETE FROM learning_assignments WHERE id = ? AND is_exam = true AND COALESCE(exam_status,'') <> 'PUBLISHED' RETURNING *`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	normalizeJakartaDateTimeFields(row, "start_at", "due_date", "exam_submitted_at", "exam_published_at", "created_at", "updated_at")
	return utils.Success(c, 200, "Success Delete Exam Request", row)
}

func (a *AppContext) PublishExamByAdmin(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	var current struct {
		ID             int    `gorm:"column:id"`
		ExamStatus     string `gorm:"column:exam_status"`
		QuizPayloadRaw string `gorm:"column:quiz_payload_raw"`
	}
	a.DB.Raw(`
		SELECT id, COALESCE(exam_status, '') AS exam_status, COALESCE(quiz_payload::text, '[]') AS quiz_payload_raw
		FROM learning_assignments
		WHERE id = ? AND is_exam = true
		LIMIT 1
	`, id).Scan(&current)
	if current.ID == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	var quizPayload []map[string]interface{}
	_ = json.Unmarshal([]byte(current.QuizPayloadRaw), &quizPayload)
	if len(quizPayload) == 0 {
		return utils.Error(c, 400, "Paket soal dari guru belum tersedia")
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_assignments
		SET exam_status = 'PUBLISHED', exam_published_at = NOW()
		WHERE id = ? AND is_exam = true
		RETURNING *
	`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	normalizeJakartaDateTimeFields(row, "start_at", "due_date", "exam_submitted_at", "exam_published_at", "created_at", "updated_at")
	return utils.Success(c, 200, "Success Publish Exam", row)
}

func generateExamAccessCode() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	code := make([]byte, 8)
	for i := range code {
		code[i] = alphabet[rnd.Intn(len(alphabet))]
	}
	return string(code)
}

func (a *AppContext) GenerateStudentExamAccessCodeByAdmin(c *fiber.Ctx) error {
	submissionID := c.Params("submissionId")
	schoolID := c.Locals("schoolID").(uint)

	var row map[string]interface{}
	a.DB.Raw(`
		SELECT s.*, la.id AS assignment_id, la.title AS assignment_title, la.exam_category, la.is_exam,
		       u.username AS student_name
		FROM learning_submissions s
		INNER JOIN learning_assignments la ON la.id = s.assignment_id
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		INNER JOIN users u ON u.id = s.student_id
		WHERE s.id = ? AND ls.school_id = ? AND COALESCE(la.is_exam, false) = true
		LIMIT 1
	`, submissionID, schoolID).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Official exam submission not found")
	}
	if boolFromAny(row["is_submitted"]) {
		return utils.Error(c, 400, "Submission has already been submitted")
	}

	code := generateExamAccessCode()
	var updated map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_submissions
		SET access_blocked = true,
		    access_code = ?,
		    access_code_generated_at = NOW(),
		    access_block_reason = COALESCE(NULLIF(access_block_reason, ''), 'MAX_VIOLATIONS')
		WHERE id = ?
		RETURNING id, assignment_id, student_id, access_blocked, access_code, access_code_generated_at, access_block_reason
	`, code, submissionID).Scan(&updated)
	normalizeJakartaDateTimeFields(updated, "access_code_generated_at")
	return utils.Success(c, 200, "Success Generate Student Exam Access Code", updated)
}

func (a *AppContext) resolveActiveAcademicPeriod(schoolID int) (int, int) {
	var row struct {
		AcademicYearID int `json:"academic_year_id"`
		SemesterID     int `json:"semester_id"`
	}
	a.DB.Raw(`
		SELECT ay.id AS academic_year_id, sem.id AS semester_id
		FROM academic_years ay
		LEFT JOIN academic_semesters sem ON sem.academic_year_id = ay.id AND sem.is_active = true
		WHERE ay.school_id = ? AND ay.is_active = true
		LIMIT 1
	`, schoolID).Scan(&row)
	return row.AcademicYearID, row.SemesterID
}

func nullIfEmpty(v string) interface{} {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func normalizeDateTimeLocalToWIB(value string) interface{} {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil
	}

	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		location = time.FixedZone("WIB", 7*60*60)
	}

	for _, layout := range []string{
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		time.RFC3339,
		"2006-01-02 15:04:05",
	} {
		if layout == time.RFC3339 {
			parsed, parseErr := time.Parse(layout, raw)
			if parseErr == nil {
				return parsed.In(location).Format("2006-01-02 15:04:05")
			}
			continue
		}

		parsed, parseErr := time.ParseInLocation(layout, raw, location)
		if parseErr == nil {
			return parsed.Format("2006-01-02 15:04:05")
		}
	}

	return raw
}

func nullIfZero(v int) interface{} {
	if v <= 0 {
		return nil
	}
	return v
}

func ternaryString(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
