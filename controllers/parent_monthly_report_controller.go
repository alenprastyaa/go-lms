package controllers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/services"
	"lms/utils"
)

type parentMonthlyReportStudent struct {
	ID                uint    `json:"id" gorm:"column:id"`
	StudentName       string  `json:"student_name" gorm:"column:student_name"`
	Username          string  `json:"username" gorm:"column:username"`
	ClassName         string  `json:"class_name" gorm:"column:class_name"`
	SchoolName        string  `json:"school_name" gorm:"column:school_name"`
	StudentPhone      *string `json:"student_phone" gorm:"column:student_phone"`
	LinkedParentPhone *string `json:"linked_parent_phone" gorm:"column:linked_parent_phone"`
}

type parentMonthlyAttendanceSummary struct {
	PresentCount  int `json:"present_count" gorm:"column:present_count"`
	LateCount     int `json:"late_count" gorm:"column:late_count"`
	AbsentCount   int `json:"absent_count" gorm:"column:absent_count"`
	RecordedCount int `json:"recorded_count" gorm:"column:recorded_count"`
}

type parentMonthlyGradeRow struct {
	AssignmentID   uint     `json:"assignment_id" gorm:"column:assignment_id"`
	Title          string   `json:"title" gorm:"column:title"`
	AssignmentType string   `json:"assignment_type" gorm:"column:assignment_type"`
	SubjectName    string   `json:"subject_name" gorm:"column:subject_name"`
	Score          *float64 `json:"score" gorm:"column:score"`
	IsSubmitted    bool     `json:"is_submitted" gorm:"column:is_submitted"`
}

type parentMonthlyGradeSummary struct {
	TotalAssignments int      `json:"total_assignments"`
	SubmittedCount   int      `json:"submitted_count"`
	PendingCount     int      `json:"pending_count"`
	GradedCount      int      `json:"graded_count"`
	AverageScore     *float64 `json:"average_score"`
}

type parentMonthlyGeneratedReport struct {
	Student      parentMonthlyReportStudent
	Attendance   parentMonthlyAttendanceSummary
	Grades       []parentMonthlyGradeRow
	GradeSummary parentMonthlyGradeSummary
	MonthStart   time.Time
	MonthLabel   string
	PDFBytes     []byte
}

type parentWhatsAppReportScheduleSettings struct {
	Enabled        bool    `json:"enabled" gorm:"column:enabled"`
	ScheduleType   string  `json:"schedule_type" gorm:"column:schedule_type"`
	SendTime       string  `json:"send_time" gorm:"column:send_time"`
	DayOfWeek      int     `json:"day_of_week" gorm:"column:day_of_week"`
	DayOfMonth     int     `json:"day_of_month" gorm:"column:day_of_month"`
	ClassID        *uint   `json:"class_id" gorm:"column:class_id"`
	ClassName      *string `json:"class_name" gorm:"column:class_name"`
	LastSentPeriod *string `json:"last_sent_period" gorm:"column:last_sent_period"`
	LastSentAt     *string `json:"last_sent_at" gorm:"column:last_sent_at"`
	UpdatedAt      *string `json:"updated_at" gorm:"column:updated_at"`
}

type parentMonthlyReportProcessResult struct {
	MonthStart    time.Time
	MonthLabel    string
	TotalStudents int
	SuccessCount  int
	FailedCount   int
	Results       []fiber.Map
}

func (a *AppContext) SendParentMonthlyReportWhatsApp(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Month     string `json:"month"`
		Date      string `json:"date"`
		ClassID   uint   `json:"class_id"`
		StudentID uint   `json:"student_id"`
	}
	_ = c.BodyParser(&body)

	reportMonth, err := parseParentReportMonth(body.Month, body.Date)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	result, processErr := a.processParentMonthlyReportWhatsApp(c.Context(), schoolID, body.ClassID, body.StudentID, reportMonth)
	if processErr != nil {
		if processErr.Error() == "no_students" {
			return utils.Error(c, 404, "Tidak ada siswa yang sesuai dengan filter laporan")
		}
		return utils.Error(c, 500, "Gagal memproses laporan WhatsApp orang tua", processErr.Error())
	}

	return utils.Success(c, 200, "Laporan WhatsApp orang tua selesai diproses", fiber.Map{
		"month":          result.MonthStart.Format("2006-01"),
		"month_label":    result.MonthLabel,
		"class_id":       body.ClassID,
		"student_id":     body.StudentID,
		"total_students": result.TotalStudents,
		"success_count":  result.SuccessCount,
		"failed_count":   result.FailedCount,
		"results":        result.Results,
		"generated_at":   jakartaNow().Format(time.RFC3339),
	})
}

func (a *AppContext) GetParentWhatsAppReportSettings(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	settings, err := a.fetchParentWhatsAppReportSettings(schoolID)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat setting laporan WhatsApp", err.Error())
	}
	return utils.Success(c, 200, "Setting laporan WhatsApp berhasil dimuat", settings)
}

func (a *AppContext) UpdateParentWhatsAppReportSettings(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var body struct {
		Enabled      bool   `json:"enabled"`
		ScheduleType string `json:"schedule_type"`
		SendTime     string `json:"send_time"`
		DayOfWeek    int    `json:"day_of_week"`
		DayOfMonth   int    `json:"day_of_month"`
		ClassID      *uint  `json:"class_id"`
	}
	_ = c.BodyParser(&body)

	scheduleType := normalizeParentReportScheduleType(body.ScheduleType)
	sendTime, err := normalizeParentReportSendTime(body.SendTime)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	dayOfWeek := body.DayOfWeek
	if dayOfWeek < 1 || dayOfWeek > 7 {
		dayOfWeek = 1
	}
	dayOfMonth := body.DayOfMonth
	if dayOfMonth < 1 || dayOfMonth > 31 {
		dayOfMonth = 1
	}

	if body.ClassID != nil && *body.ClassID > 0 {
		var count int64
		if err := a.DB.Table("class").Where("id = ? AND school_id = ?", *body.ClassID, schoolID).Count(&count).Error; err != nil {
			return utils.Error(c, 500, "Gagal memeriksa kelas", err.Error())
		}
		if count == 0 {
			return utils.Error(c, 400, "Kelas tidak ditemukan")
		}
	}

	var classIDValue interface{}
	if body.ClassID != nil && *body.ClassID > 0 {
		classIDValue = *body.ClassID
	}

	if err := a.DB.Exec(`
		INSERT INTO parent_whatsapp_report_settings (
			school_id, enabled, schedule_type, send_time, day_of_week, day_of_month, class_id, updated_by, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (school_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			schedule_type = EXCLUDED.schedule_type,
			send_time = EXCLUDED.send_time,
			day_of_week = EXCLUDED.day_of_week,
			day_of_month = EXCLUDED.day_of_month,
			class_id = EXCLUDED.class_id,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()
	`, schoolID, body.Enabled, scheduleType, sendTime, dayOfWeek, dayOfMonth, classIDValue, userID).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan setting laporan WhatsApp", err.Error())
	}

	settings, err := a.fetchParentWhatsAppReportSettings(schoolID)
	if err != nil {
		return utils.Error(c, 500, "Setting tersimpan, tetapi gagal dimuat ulang", err.Error())
	}
	return utils.Success(c, 200, "Setting laporan WhatsApp berhasil disimpan", settings)
}

func (a *AppContext) PreviewParentMonthlyReportPDF(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	reportMonth, err := parseParentReportMonth(c.Query("month"), c.Query("date"))
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	classID := uint(c.QueryInt("class_id", 0))
	studentID := uint(c.QueryInt("student_id", 0))
	report, err := a.buildParentMonthlyGeneratedReport(schoolID, classID, studentID, reportMonth)
	if err != nil {
		return parentMonthlyGeneratedReportError(c, err)
	}

	fileName := fmt.Sprintf("template-laporan-orang-tua-%d-%s.pdf", report.Student.ID, report.MonthStart.Format("2006-01"))
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, fileName))
	return c.Send(report.PDFBytes)
}

func (a *AppContext) SendParentMonthlyReportWhatsAppTest(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Month       string `json:"month"`
		Date        string `json:"date"`
		ClassID     uint   `json:"class_id"`
		StudentID   uint   `json:"student_id"`
		PhoneNumber string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)

	targetPhone, phoneErr := normalizeParentWhatsAppPhone(body.PhoneNumber)
	if phoneErr != nil {
		return utils.Error(c, 400, phoneErr.Error())
	}

	reportMonth, err := parseParentReportMonth(body.Month, body.Date)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	report, err := a.buildParentMonthlyGeneratedReport(schoolID, body.ClassID, body.StudentID, reportMonth)
	if err != nil {
		return parentMonthlyGeneratedReportError(c, err)
	}

	fileName := fmt.Sprintf("tes-laporan-orang-tua-%d-%s.pdf", report.Student.ID, report.MonthStart.Format("2006-01"))
	pdfURL, uploadErr := utils.UploadBytesToR2(c.Context(), report.PDFBytes, fileName, "application/pdf")
	if uploadErr != nil {
		return utils.Error(c, 500, "Gagal upload PDF", uploadErr.Error())
	}

	message := buildParentMonthlyReportWhatsAppMessage(report.Student, report.Attendance, report.GradeSummary, report.MonthLabel, pdfURL)
	if _, sendErr := services.SendWhatsAppMessage(targetPhone, message); sendErr != nil {
		return utils.Error(c, 500, "Gagal mengirim WhatsApp", sendErr.Error())
	}

	return utils.Success(c, 200, "Tes laporan WhatsApp orang tua berhasil dikirim", fiber.Map{
		"month":         report.MonthStart.Format("2006-01"),
		"month_label":   report.MonthLabel,
		"class_id":      body.ClassID,
		"student_id":    report.Student.ID,
		"student_name":  report.Student.StudentName,
		"class_name":    report.Student.ClassName,
		"is_dummy":      report.Student.ID == 0,
		"target":        targetPhone,
		"pdf_url":       pdfURL,
		"average_score": report.GradeSummary.AverageScore,
		"generated_at":  jakartaNow().Format(time.RFC3339),
	})
}

func parseParentReportMonth(monthValue, dateValue string) (time.Time, error) {
	raw := strings.TrimSpace(monthValue)
	if raw == "" && len(strings.TrimSpace(dateValue)) >= 7 {
		raw = strings.TrimSpace(dateValue)[:7]
	}
	if raw == "" {
		now := jakartaNow()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, jakartaLocation()), nil
	}
	parsed, err := time.ParseInLocation("2006-01", raw, jakartaLocation())
	if err != nil {
		return time.Time{}, fmt.Errorf("Format bulan laporan tidak valid")
	}
	return parsed, nil
}

func (a *AppContext) fetchParentMonthlyReportStudents(schoolID, classID, studentID uint) ([]parentMonthlyReportStudent, error) {
	query := `
		SELECT
			u.id,
			COALESCE(NULLIF(u.full_name, ''), u.username) AS student_name,
			u.username,
			COALESCE(c.class_name, '') AS class_name,
			COALESCE(s.name, '') AS school_name,
			u.phone_number AS student_phone,
			parent_user.phone_number AS linked_parent_phone
		FROM users u
		LEFT JOIN class c ON c.id = u.class_id
		LEFT JOIN schools s ON s.id = u.school_id
		LEFT JOIN LATERAL (
			SELECT p.phone_number
			FROM parent_students ps
			INNER JOIN users p ON p.id = ps.parent_user_id AND p.role = 'ORANG_TUA'
			WHERE ps.student_user_id = u.id
			ORDER BY ps.updated_at DESC NULLS LAST, ps.id DESC
			LIMIT 1
		) parent_user ON true
		WHERE u.role = 'SISWA'
			AND u.school_id = ?
	`
	args := []interface{}{schoolID}
	if classID > 0 {
		query += ` AND u.class_id = ?`
		args = append(args, classID)
	}
	if studentID > 0 {
		query += ` AND u.id = ?`
		args = append(args, studentID)
	}
	query += ` ORDER BY COALESCE(c.class_name, ''), COALESCE(NULLIF(u.full_name, ''), u.username)`

	var students []parentMonthlyReportStudent
	err := a.DB.Raw(query, args...).Scan(&students).Error
	return students, err
}

func (a *AppContext) fetchParentWhatsAppReportSettings(schoolID uint) (parentWhatsAppReportScheduleSettings, error) {
	settings := defaultParentWhatsAppReportSettings()
	var row parentWhatsAppReportScheduleSettings
	result := a.DB.Raw(`
		SELECT
			s.enabled,
			s.schedule_type,
			s.send_time,
			s.day_of_week,
			s.day_of_month,
			s.class_id,
			c.class_name,
			s.last_sent_period,
			TO_CHAR(s.last_sent_at, 'YYYY-MM-DD HH24:MI:SS') AS last_sent_at,
			TO_CHAR(s.updated_at, 'YYYY-MM-DD HH24:MI:SS') AS updated_at
		FROM parent_whatsapp_report_settings s
		LEFT JOIN class c ON c.id = s.class_id
		WHERE s.school_id = ?
	`, schoolID).Scan(&row)
	if result.Error != nil {
		return settings, result.Error
	}
	if result.RowsAffected == 0 {
		return settings, nil
	}
	return row, nil
}

func defaultParentWhatsAppReportSettings() parentWhatsAppReportScheduleSettings {
	return parentWhatsAppReportScheduleSettings{
		Enabled:      false,
		ScheduleType: "MONTHLY_DATE",
		SendTime:     "08:00",
		DayOfWeek:    1,
		DayOfMonth:   1,
	}
}

func normalizeParentReportScheduleType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "WEEKLY_DAY":
		return "WEEKLY_DAY"
	default:
		return "MONTHLY_DATE"
	}
}

func normalizeParentReportSendTime(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		raw = "08:00"
	}
	parsed, err := time.Parse("15:04", raw)
	if err != nil {
		return "", fmt.Errorf("Jam kirim wajib format HH:MM")
	}
	return parsed.Format("15:04"), nil
}

func (a *AppContext) processParentMonthlyReportWhatsApp(ctx context.Context, schoolID, classID, studentID uint, reportMonth time.Time) (parentMonthlyReportProcessResult, error) {
	monthStart := time.Date(reportMonth.Year(), reportMonth.Month(), 1, 0, 0, 0, 0, jakartaLocation())
	monthEnd := monthStart.AddDate(0, 1, 0)
	monthLabel := indonesianMonthYear(monthStart)

	students, err := a.fetchParentMonthlyReportStudents(schoolID, classID, studentID)
	if err != nil {
		return parentMonthlyReportProcessResult{}, err
	}
	if len(students) == 0 {
		return parentMonthlyReportProcessResult{}, fmt.Errorf("no_students")
	}

	results := make([]fiber.Map, 0, len(students))
	successCount := 0
	failedCount := 0
	for _, student := range students {
		result := fiber.Map{
			"student_id":   student.ID,
			"student_name": student.StudentName,
			"class_name":   student.ClassName,
			"success":      false,
		}

		targetPhone, phoneErr := parentMonthlyReportPhone(student)
		if phoneErr != nil {
			result["error"] = phoneErr.Error()
			failedCount++
			results = append(results, result)
			continue
		}
		result["target"] = targetPhone

		attendance, attendanceErr := a.fetchParentMonthlyAttendanceSummary(student.ID, monthStart, monthEnd)
		if attendanceErr != nil {
			result["error"] = "Gagal memuat kehadiran: " + attendanceErr.Error()
			failedCount++
			results = append(results, result)
			continue
		}

		grades, gradeErr := a.fetchParentMonthlyGrades(student.ID, schoolID, monthStart, monthEnd)
		if gradeErr != nil {
			result["error"] = "Gagal memuat nilai: " + gradeErr.Error()
			failedCount++
			results = append(results, result)
			continue
		}
		gradeSummary := summarizeParentMonthlyGrades(grades)

		pdfBytes := services.BuildMonthlyStudentReportPDF(buildParentMonthlyReportPDFInput(student, attendance, gradeSummary, grades, monthLabel))
		fileName := fmt.Sprintf("laporan-orang-tua-%d-%s.pdf", student.ID, monthStart.Format("2006-01"))
		pdfURL, uploadErr := utils.UploadBytesToR2(ctx, pdfBytes, fileName, "application/pdf")
		if uploadErr != nil {
			result["error"] = "Gagal upload PDF: " + uploadErr.Error()
			failedCount++
			results = append(results, result)
			continue
		}

		message := buildParentMonthlyReportWhatsAppMessage(student, attendance, gradeSummary, monthLabel, pdfURL)
		if _, sendErr := services.SendWhatsAppMessage(targetPhone, message); sendErr != nil {
			result["pdf_url"] = pdfURL
			result["error"] = "Gagal mengirim WhatsApp: " + sendErr.Error()
			failedCount++
			results = append(results, result)
			continue
		}

		result["success"] = true
		result["pdf_url"] = pdfURL
		result["present_count"] = attendance.PresentCount
		result["late_count"] = attendance.LateCount
		result["absent_count"] = attendance.AbsentCount
		result["recorded_count"] = attendance.RecordedCount
		result["average_score"] = gradeSummary.AverageScore
		successCount++
		results = append(results, result)
	}

	return parentMonthlyReportProcessResult{
		MonthStart:    monthStart,
		MonthLabel:    monthLabel,
		TotalStudents: len(students),
		SuccessCount:  successCount,
		FailedCount:   failedCount,
		Results:       results,
	}, nil
}

func (a *AppContext) buildParentMonthlyGeneratedReport(schoolID, classID, studentID uint, reportMonth time.Time) (parentMonthlyGeneratedReport, error) {
	monthStart := time.Date(reportMonth.Year(), reportMonth.Month(), 1, 0, 0, 0, 0, jakartaLocation())
	monthEnd := monthStart.AddDate(0, 1, 0)
	monthLabel := indonesianMonthYear(monthStart)

	students, err := a.fetchParentMonthlyReportStudents(schoolID, classID, studentID)
	if err != nil {
		return parentMonthlyGeneratedReport{}, fmt.Errorf("load_students: %w", err)
	}
	if len(students) == 0 {
		return buildDummyParentMonthlyGeneratedReport(monthStart, monthLabel), nil
	}

	student := students[0]
	attendance, err := a.fetchParentMonthlyAttendanceSummary(student.ID, monthStart, monthEnd)
	if err != nil {
		return parentMonthlyGeneratedReport{}, fmt.Errorf("load_attendance: %w", err)
	}
	grades, err := a.fetchParentMonthlyGrades(student.ID, schoolID, monthStart, monthEnd)
	if err != nil {
		return parentMonthlyGeneratedReport{}, fmt.Errorf("load_grades: %w", err)
	}
	gradeSummary := summarizeParentMonthlyGrades(grades)
	pdfBytes := services.BuildMonthlyStudentReportPDF(buildParentMonthlyReportPDFInput(student, attendance, gradeSummary, grades, monthLabel))

	return parentMonthlyGeneratedReport{
		Student:      student,
		Attendance:   attendance,
		Grades:       grades,
		GradeSummary: gradeSummary,
		MonthStart:   monthStart,
		MonthLabel:   monthLabel,
		PDFBytes:     pdfBytes,
	}, nil
}

func buildDummyParentMonthlyGeneratedReport(monthStart time.Time, monthLabel string) parentMonthlyGeneratedReport {
	scoreA := 88.0
	scoreB := 92.5
	scoreC := 79.0
	student := parentMonthlyReportStudent{
		ID:          0,
		StudentName: "Contoh Siswa",
		Username:    "contoh.siswa",
		ClassName:   "X IPA 1",
		SchoolName:  "Sekolah Contoh",
	}
	attendance := parentMonthlyAttendanceSummary{
		PresentCount:  18,
		LateCount:     2,
		AbsentCount:   1,
		RecordedCount: 21,
	}
	grades := []parentMonthlyGradeRow{
		{AssignmentID: 1, Title: "Tugas Persamaan Linear", AssignmentType: "MANUAL", SubjectName: "Matematika", Score: &scoreA, IsSubmitted: true},
		{AssignmentID: 2, Title: "Quiz Pemahaman Teks", AssignmentType: "MCQ", SubjectName: "Bahasa Indonesia", Score: &scoreB, IsSubmitted: true},
		{AssignmentID: 3, Title: "Praktik Observasi Lingkungan", AssignmentType: "FILE", SubjectName: "IPA", Score: &scoreC, IsSubmitted: true},
		{AssignmentID: 4, Title: "Refleksi Pembelajaran", AssignmentType: "ESSAY", SubjectName: "PPKn", Score: nil, IsSubmitted: false},
	}
	gradeSummary := summarizeParentMonthlyGrades(grades)

	return parentMonthlyGeneratedReport{
		Student:      student,
		Attendance:   attendance,
		Grades:       grades,
		GradeSummary: gradeSummary,
		MonthStart:   monthStart,
		MonthLabel:   monthLabel,
		PDFBytes:     services.BuildMonthlyStudentReportPDF(buildParentMonthlyReportPDFInput(student, attendance, gradeSummary, grades, monthLabel)),
	}
}

func parentMonthlyGeneratedReportError(c *fiber.Ctx, err error) error {
	message := err.Error()
	switch {
	case strings.HasPrefix(message, "load_students:"):
		return utils.Error(c, 500, "Gagal memuat data siswa", strings.TrimPrefix(message, "load_students: "))
	case strings.HasPrefix(message, "load_attendance:"):
		return utils.Error(c, 500, "Gagal memuat kehadiran", strings.TrimPrefix(message, "load_attendance: "))
	case strings.HasPrefix(message, "load_grades:"):
		return utils.Error(c, 500, "Gagal memuat nilai", strings.TrimPrefix(message, "load_grades: "))
	default:
		return utils.Error(c, 500, "Gagal membuat laporan", message)
	}
}

func (a *AppContext) fetchParentMonthlyAttendanceSummary(studentID uint, startDate, endDate time.Time) (parentMonthlyAttendanceSummary, error) {
	var summary parentMonthlyAttendanceSummary
	err := a.DB.Raw(`
		SELECT
			COUNT(*) FILTER (WHERE LOWER(COALESCE(status, '')) = 'hadir')::int AS present_count,
			COUNT(*) FILTER (WHERE LOWER(COALESCE(status, '')) = 'terlambat')::int AS late_count,
			COUNT(*) FILTER (WHERE LOWER(COALESCE(status, '')) NOT IN ('hadir', 'terlambat'))::int AS absent_count,
			COUNT(*)::int AS recorded_count
		FROM attendance
		WHERE user_id = ?
			AND attendance_date >= ?
			AND attendance_date < ?
	`, studentID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).Scan(&summary).Error
	return summary, err
}

func (a *AppContext) fetchParentMonthlyGrades(studentID, schoolID uint, startDate, endDate time.Time) ([]parentMonthlyGradeRow, error) {
	var grades []parentMonthlyGradeRow
	err := a.DB.Raw(`
		SELECT
			la.id AS assignment_id,
			la.title,
			la.assignment_type,
			ls.name AS subject_name,
			sub.score,
			COALESCE(sub.is_submitted, false) AS is_submitted
		FROM users stu
		INNER JOIN learning_subjects ls ON ls.class_id = stu.class_id
		INNER JOIN learning_assignments la ON la.subject_id = ls.id
		LEFT JOIN LATERAL (
			SELECT s.score, s.is_submitted, s.submitted_at, s.started_at
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
			AND la.created_at >= ?
			AND la.created_at < ?
		ORDER BY ls.name ASC, la.created_at DESC
		LIMIT 12
	`, studentID, schoolID, schoolID, startDate, endDate).Scan(&grades).Error
	return grades, err
}

func parentMonthlyReportPhone(student parentMonthlyReportStudent) (string, error) {
	raw := stringPointerValue(student.LinkedParentPhone)
	if raw == "" {
		raw = stringPointerValue(student.StudentPhone)
	}
	if raw == "" {
		return "", fmt.Errorf("Nomor WhatsApp orang tua belum tersedia")
	}
	return normalizeParentWhatsAppPhone(raw)
}

func summarizeParentMonthlyGrades(grades []parentMonthlyGradeRow) parentMonthlyGradeSummary {
	summary := parentMonthlyGradeSummary{TotalAssignments: len(grades)}
	var totalScore float64
	for _, row := range grades {
		if row.IsSubmitted {
			summary.SubmittedCount++
		} else {
			summary.PendingCount++
		}
		if row.Score == nil {
			continue
		}
		summary.GradedCount++
		totalScore += *row.Score
	}
	if summary.GradedCount > 0 {
		average := float64(int((totalScore/float64(summary.GradedCount))*100)) / 100
		summary.AverageScore = &average
	}
	return summary
}

func buildParentMonthlyReportPDFLines(student parentMonthlyReportStudent, attendance parentMonthlyAttendanceSummary, grades parentMonthlyGradeSummary, gradeRows []parentMonthlyGradeRow, monthLabel string) []string {
	averageScore := "Belum tersedia"
	if grades.AverageScore != nil {
		averageScore = fmt.Sprintf("%.2f", *grades.AverageScore)
	}

	lines := []string{
		"Sekolah: " + fallbackParentReportText(student.SchoolName, "-"),
		"Periode: " + monthLabel,
		"Nama siswa: " + fallbackParentReportText(student.StudentName, "-"),
		"Kelas: " + fallbackParentReportText(student.ClassName, "-"),
		"",
		"Ringkasan Kehadiran",
		fmt.Sprintf("- Hadir: %d", attendance.PresentCount),
		fmt.Sprintf("- Terlambat: %d", attendance.LateCount),
		fmt.Sprintf("- Catatan selain hadir/terlambat: %d", attendance.AbsentCount),
		fmt.Sprintf("- Total data kehadiran tercatat: %d", attendance.RecordedCount),
		"",
		"Ringkasan Nilai",
		fmt.Sprintf("- Total tugas/ujian bulan ini: %d", grades.TotalAssignments),
		fmt.Sprintf("- Sudah dikumpulkan: %d", grades.SubmittedCount),
		fmt.Sprintf("- Belum dikumpulkan: %d", grades.PendingCount),
		fmt.Sprintf("- Sudah dinilai: %d", grades.GradedCount),
		"- Rata-rata nilai: " + averageScore,
		"",
		"Nilai Terbaru",
	}
	if len(gradeRows) == 0 {
		lines = append(lines, "- Belum ada data nilai pada periode ini.")
		return lines
	}
	for index, row := range gradeRows {
		score := "Belum dinilai"
		if row.Score != nil {
			score = fmt.Sprintf("%.2f", *row.Score)
		}
		status := "Belum dikumpulkan"
		if row.IsSubmitted {
			status = "Sudah dikumpulkan"
		}
		lines = append(lines, fmt.Sprintf("%d. %s - %s - %s - %s", index+1, fallbackParentReportText(row.SubjectName, "-"), fallbackParentReportText(row.Title, "-"), score, status))
	}
	return lines
}

func buildParentMonthlyReportPDFInput(student parentMonthlyReportStudent, attendance parentMonthlyAttendanceSummary, grades parentMonthlyGradeSummary, gradeRows []parentMonthlyGradeRow, monthLabel string) services.MonthlyStudentReportPDFInput {
	averageScore := "Belum tersedia"
	if grades.AverageScore != nil {
		averageScore = fmt.Sprintf("%.2f", *grades.AverageScore)
	}

	rows := make([]services.MonthlyStudentReportPDFGradeRow, 0, len(gradeRows))
	for _, row := range gradeRows {
		score := "Belum dinilai"
		if row.Score != nil {
			score = fmt.Sprintf("%.2f", *row.Score)
		}
		status := "Belum dikumpulkan"
		if row.IsSubmitted {
			status = "Sudah dikumpulkan"
		}
		rows = append(rows, services.MonthlyStudentReportPDFGradeRow{
			Subject: fallbackParentReportText(row.SubjectName, "-"),
			Title:   fallbackParentReportText(row.Title, "-"),
			Score:   score,
			Status:  status,
		})
	}

	return services.MonthlyStudentReportPDFInput{
		SchoolName:       fallbackParentReportText(student.SchoolName, "-"),
		StudentName:      fallbackParentReportText(student.StudentName, "-"),
		ClassName:        fallbackParentReportText(student.ClassName, "-"),
		MonthLabel:       monthLabel,
		PresentCount:     attendance.PresentCount,
		LateCount:        attendance.LateCount,
		AbsentCount:      attendance.AbsentCount,
		RecordedCount:    attendance.RecordedCount,
		TotalAssignments: grades.TotalAssignments,
		SubmittedCount:   grades.SubmittedCount,
		PendingCount:     grades.PendingCount,
		GradedCount:      grades.GradedCount,
		AverageScore:     averageScore,
		GradeRows:        rows,
	}
}

func buildParentMonthlyReportWhatsAppMessage(student parentMonthlyReportStudent, attendance parentMonthlyAttendanceSummary, grades parentMonthlyGradeSummary, monthLabel, pdfURL string) string {
	averageScore := "Belum tersedia"
	if grades.AverageScore != nil {
		averageScore = fmt.Sprintf("%.2f", *grades.AverageScore)
	}
	className := fallbackParentReportText(student.ClassName, "-")
	studentName := fallbackParentReportText(student.StudentName, "siswa")

	return strings.TrimSpace(fmt.Sprintf(`Yth. Bapak/Ibu Orang Tua/Wali %s,

Berikut kami sampaikan laporan perkembangan siswa untuk bulan *%s*.

Nama siswa: *%s*
Kelas: *%s*

Ringkasan kehadiran:
- Hadir: *%d*
- Terlambat: *%d*
- Total catatan: *%d*

Ringkasan nilai:
- Tugas/ujian bulan ini: *%d*
- Sudah dinilai: *%d*
- Rata-rata nilai: *%s*

Silakan membuka laporan PDF melalui link berikut:
%s

Mohon Bapak/Ibu dapat meninjau laporan ini. Apabila ada pertanyaan, silakan menghubungi pihak sekolah.

Hormat kami,
Admin Sekolah`, studentName, monthLabel, studentName, className, attendance.PresentCount, attendance.LateCount, attendance.RecordedCount, grades.TotalAssignments, grades.GradedCount, averageScore, pdfURL))
}

func fallbackParentReportText(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func indonesianMonthYear(value time.Time) string {
	months := []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	monthName := months[int(value.In(jakartaLocation()).Month())-1]
	return fmt.Sprintf("%s %d", monthName, value.In(jakartaLocation()).Year())
}

func StartParentWhatsAppReportScheduler(app *AppContext) {
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		for range ticker.C {
			app.runDueParentWhatsAppReportSchedules()
		}
	}()
}

func (a *AppContext) runDueParentWhatsAppReportSchedules() {
	now := jakartaNow().Truncate(time.Minute)
	var rows []struct {
		SchoolID       uint    `gorm:"column:school_id"`
		ScheduleType   string  `gorm:"column:schedule_type"`
		SendTime       string  `gorm:"column:send_time"`
		DayOfWeek      int     `gorm:"column:day_of_week"`
		DayOfMonth     int     `gorm:"column:day_of_month"`
		ClassID        *uint   `gorm:"column:class_id"`
		LastSentPeriod *string `gorm:"column:last_sent_period"`
	}
	if err := a.DB.Raw(`
		SELECT school_id, schedule_type, send_time, day_of_week, day_of_month, class_id, last_sent_period
		FROM parent_whatsapp_report_settings
		WHERE enabled = TRUE
	`).Scan(&rows).Error; err != nil {
		log.Printf("parent whatsapp scheduler: load settings failed: %v", err)
		return
	}

	for _, row := range rows {
		due, periodKey := isParentWhatsAppReportScheduleDue(row.ScheduleType, row.SendTime, row.DayOfWeek, row.DayOfMonth, row.LastSentPeriod, now)
		if !due {
			continue
		}

		classID := uint(0)
		if row.ClassID != nil {
			classID = *row.ClassID
		}
		result, err := a.processParentMonthlyReportWhatsApp(context.Background(), row.SchoolID, classID, 0, now)
		if err != nil {
			log.Printf("parent whatsapp scheduler: school=%d failed: %v", row.SchoolID, err)
			continue
		}
		if err := a.DB.Exec(`
			UPDATE parent_whatsapp_report_settings
			SET last_sent_period = ?, last_sent_at = NOW(), updated_at = NOW()
			WHERE school_id = ?
		`, periodKey, row.SchoolID).Error; err != nil {
			log.Printf("parent whatsapp scheduler: update last sent school=%d failed: %v", row.SchoolID, err)
		}
		log.Printf("parent whatsapp scheduler: school=%d sent=%d failed=%d period=%s", row.SchoolID, result.SuccessCount, result.FailedCount, periodKey)
	}
}

func isParentWhatsAppReportScheduleDue(scheduleType, sendTime string, dayOfWeek, dayOfMonth int, lastSentPeriod *string, now time.Time) (bool, string) {
	if now.Format("15:04") != strings.TrimSpace(sendTime) {
		return false, ""
	}

	periodKey := ""
	switch normalizeParentReportScheduleType(scheduleType) {
	case "WEEKLY_DAY":
		currentDay := int(now.Weekday())
		if currentDay == 0 {
			currentDay = 7
		}
		if currentDay != dayOfWeek {
			return false, ""
		}
		periodKey = fmt.Sprintf("weekly:%s:%s", now.Format("2006-01-02"), sendTime)
	default:
		targetDay := dayOfMonth
		if targetDay < 1 {
			targetDay = 1
		}
		lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
		if targetDay > lastDay {
			targetDay = lastDay
		}
		if now.Day() != targetDay {
			return false, ""
		}
		periodKey = fmt.Sprintf("monthly:%s:%02d:%s", now.Format("2006-01"), targetDay, sendTime)
	}

	return lastSentPeriod == nil || strings.TrimSpace(*lastSentPeriod) != periodKey, periodKey
}
