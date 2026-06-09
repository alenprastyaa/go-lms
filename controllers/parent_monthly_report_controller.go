package controllers

import (
	"fmt"
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

func (a *AppContext) SendParentMonthlyReportWhatsApp(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Month   string `json:"month"`
		Date    string `json:"date"`
		ClassID uint   `json:"class_id"`
	}
	_ = c.BodyParser(&body)

	reportMonth, err := parseParentReportMonth(body.Month, body.Date)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	monthStart := time.Date(reportMonth.Year(), reportMonth.Month(), 1, 0, 0, 0, 0, jakartaLocation())
	monthEnd := monthStart.AddDate(0, 1, 0)
	monthLabel := indonesianMonthYear(monthStart)

	students, err := a.fetchParentMonthlyReportStudents(schoolID, body.ClassID)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat data siswa", err.Error())
	}
	if len(students) == 0 {
		return utils.Error(c, 404, "Tidak ada siswa yang sesuai dengan filter laporan")
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
		pdfURL, uploadErr := utils.UploadBytesToR2(c.Context(), pdfBytes, fileName, "application/pdf")
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

	return utils.Success(c, 200, "Laporan WhatsApp orang tua selesai diproses", fiber.Map{
		"month":          monthStart.Format("2006-01"),
		"month_label":    monthLabel,
		"class_id":       body.ClassID,
		"total_students": len(students),
		"success_count":  successCount,
		"failed_count":   failedCount,
		"results":        results,
		"generated_at":   jakartaNow().Format(time.RFC3339),
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

func (a *AppContext) fetchParentMonthlyReportStudents(schoolID, classID uint) ([]parentMonthlyReportStudent, error) {
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
	query += ` ORDER BY COALESCE(c.class_name, ''), COALESCE(NULLIF(u.full_name, ''), u.username)`

	var students []parentMonthlyReportStudent
	err := a.DB.Raw(query, args...).Scan(&students).Error
	return students, err
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
