package controllers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

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
			  ls.name AS subject_name,
			  c.class_name,
			  t.username AS teacher_name,
			  sub.id AS submission_id,
			  sub.score,
			  sub.feedback,
			  sub.submission_text,
			  sub.attachment_url AS submission_attachment_url,
			  sub.started_at AS attempt_started_at,
			  sub.submitted_at,
			  sub.is_submitted
			FROM learning_assignments la
			INNER JOIN learning_subjects ls ON ls.id = la.subject_id
			LEFT JOIN class c ON c.id = ls.class_id
			LEFT JOIN users t ON t.id = ls.teacher_id
			LEFT JOIN learning_submissions sub
			  ON sub.assignment_id = la.id
			 AND sub.student_id = ?
			WHERE la.subject_id = ?
			  AND ls.school_id = ?
			  AND (
			    COALESCE(la.is_exam, false) = false
			    OR (COALESCE(la.is_exam, false) = true AND la.exam_status = 'PUBLISHED')
			  )
			ORDER BY la.created_at DESC
		`, userID, subjectID, schoolID).Scan(&rows)
		return utils.Success(c, 200, "Success Get Assignments", rows)
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT la.*, ls.name AS subject_name, c.class_name, t.username AS teacher_name
		FROM learning_assignments la
		INNER JOIN learning_subjects ls ON ls.id = la.subject_id
		LEFT JOIN class c ON c.id = ls.class_id
		LEFT JOIN users t ON t.id = ls.teacher_id
		WHERE la.subject_id = ? AND ls.school_id = ?
		ORDER BY la.created_at DESC
	`, subjectID, schoolID).Scan(&rows)
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
	examCount := c.FormValue("exam_target_question_count")
	questionBankIDsRaw := strings.TrimSpace(c.FormValue("question_bank_ids"))
	shuffleQuestions := strings.ToLower(strings.TrimSpace(c.FormValue("shuffle_questions"))) == "true"

	if assignmentType == "" {
		assignmentType = "FILE"
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
		  question_duration_seconds, attachment_url, due_date, created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?::jsonb, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		RETURNING *
	`,
		subjectID, title, description, assignmentType, isExam, nullIfEmpty(examCategory), nullIfEmpty(examCode),
		ternaryString(isExam, "REQUESTED", ""), toJSONRaw(questionBankIDs), shuffleQuestions, toJSONRaw(quizPayload),
		nullIfEmpty(startAt), true, nullIfEmpty(examCount),
		nullIfZero(academicYearID), nullIfZero(semesterID), nullIfEmpty(qDur), nullIfEmpty(attachmentURL),
		nullIfEmpty(dueDate), userID,
	).Scan(&row)

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
	examCount := c.FormValue("exam_target_question_count")
	if assignmentType == "" {
		assignmentType = "MCQ"
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_assignments
		SET subject_id = ?, title = ?, description = ?, due_date = ?, assignment_type = ?,
		    exam_category = ?, exam_code = ?, start_at = ?, question_duration_seconds = ?,
		    exam_target_question_count = ?, updated_at = NOW()
		WHERE id = ? AND is_exam = true
		RETURNING *
	`, subjectID, title, description, nullIfEmpty(dueDate), assignmentType, nullIfEmpty(examCategory),
		nullIfEmpty(examCode), nullIfEmpty(startAt), nullIfEmpty(qDur), nullIfEmpty(examCount), id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	return utils.Success(c, 200, "Success Update Exam Request", row)
}

func (a *AppContext) DeleteExamRequestByAdmin(c *fiber.Ctx) error {
	id := c.Params("assignmentId")
	var row map[string]interface{}
	a.DB.Raw(`DELETE FROM learning_assignments WHERE id = ? AND is_exam = true AND COALESCE(exam_status,'') <> 'PUBLISHED' RETURNING *`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
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
	if strings.ToUpper(strings.TrimSpace(current.ExamStatus)) != "SUBMITTED" {
		return utils.Error(c, 400, "Ujian belum siap diterbitkan")
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE learning_assignments
		SET exam_status = 'PUBLISHED', exam_published_at = NOW(), updated_at = NOW()
		WHERE id = ? AND is_exam = true
		RETURNING *
	`, id).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Assignment not found")
	}
	return utils.Success(c, 200, "Success Publish Exam", row)
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
