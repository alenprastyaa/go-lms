package controllers

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/utils"
)

func (a *AppContext) GetSuperAdminDashboard(c *fiber.Ctx) error {
	var overviewRows []struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	}
	a.DB.Raw(`
		SELECT 'schools' AS key, COUNT(*)::int AS value FROM schools
		UNION ALL
		SELECT 'users' AS key, COUNT(*)::int AS value FROM users
		UNION ALL
		SELECT 'admins' AS key, COUNT(*)::int AS value FROM users WHERE role = 'ADMIN'
		UNION ALL
		SELECT 'teachers' AS key, COUNT(*)::int AS value FROM users WHERE role = 'GURU'
		UNION ALL
		SELECT 'students' AS key, COUNT(*)::int AS value FROM users WHERE role = 'SISWA'
		UNION ALL
		SELECT 'schools_with_admin' AS key, COUNT(*)::int
		FROM (
			SELECT s.id
			FROM schools s
			INNER JOIN users u ON u.school_id = s.id AND u.role = 'ADMIN'
			GROUP BY s.id
		) x
		UNION ALL
		SELECT 'schools_without_admin' AS key, COUNT(*)::int
		FROM schools s
		WHERE NOT EXISTS (
			SELECT 1 FROM users u WHERE u.school_id = s.id AND u.role = 'ADMIN'
		)
		UNION ALL
		SELECT 'schools_with_classes' AS key, COUNT(*)::int
		FROM (
			SELECT school_id FROM class GROUP BY school_id
		) x
		UNION ALL
		SELECT 'schools_with_curriculum' AS key, COUNT(*)::int
		FROM (
			SELECT school_id FROM curriculum_subjects GROUP BY school_id
		) x
	`).Scan(&overviewRows)
	overview := map[string]int{}
	for _, row := range overviewRows {
		overview[row.Key] = row.Value
	}

	var schools []map[string]interface{}
	a.DB.Raw(`
		SELECT
		  s.id,
		  s.name,
		  COUNT(DISTINCT u.id)::int AS total_users,
		  COUNT(DISTINCT CASE WHEN u.role = 'ADMIN' THEN u.id END)::int AS total_admins,
		  COUNT(DISTINCT CASE WHEN u.role = 'GURU' THEN u.id END)::int AS total_teachers,
		  COUNT(DISTINCT CASE WHEN u.role = 'SISWA' THEN u.id END)::int AS total_students,
		  COUNT(DISTINCT c.id)::int AS total_classes,
		  COUNT(DISTINCT CASE WHEN ay.is_active = true THEN ay.id END)::int AS active_academic_years,
		  COUNT(DISTINCT CASE WHEN cs.id IS NOT NULL THEN cs.id END)::int AS curriculum_subjects,
		  COUNT(DISTINCT CASE WHEN a.attendance_date = CURRENT_DATE THEN a.id END)::int AS attendance_today,
		  COUNT(DISTINCT CASE WHEN DATE_TRUNC('month', pr.created_at) = DATE_TRUNC('month', CURRENT_DATE) THEN pr.id END)::int AS receipts_this_month
		FROM schools s
		LEFT JOIN users u ON u.school_id = s.id
		LEFT JOIN class c ON c.school_id = s.id
		LEFT JOIN academic_years ay ON ay.school_id = s.id
		LEFT JOIN curriculum_subjects cs ON cs.school_id = s.id
		LEFT JOIN attendance a ON a.user_id = u.id
		LEFT JOIN payment_receipt pr ON pr.user_id = u.id
		GROUP BY s.id, s.name
		ORDER BY total_students DESC, s.name ASC
		LIMIT 12
	`).Scan(&schools)

	var schoolAlerts []map[string]interface{}
	a.DB.Raw(`
		SELECT
		  s.id,
		  s.name,
		  CASE
			WHEN COUNT(DISTINCT CASE WHEN u.role = 'ADMIN' THEN u.id END) = 0 THEN 'Belum punya admin sekolah'
			WHEN COUNT(DISTINCT c.id) = 0 THEN 'Belum punya kelas'
			WHEN COUNT(DISTINCT CASE WHEN u.role = 'SISWA' THEN u.id END) = 0 THEN 'Belum punya siswa'
			WHEN COUNT(DISTINCT cs.id) = 0 THEN 'Modul kurikulum belum diisi'
			ELSE 'Perlu pemantauan'
		  END AS issue,
		  COUNT(DISTINCT CASE WHEN u.role = 'ADMIN' THEN u.id END)::int AS total_admins,
		  COUNT(DISTINCT c.id)::int AS total_classes,
		  COUNT(DISTINCT CASE WHEN u.role = 'SISWA' THEN u.id END)::int AS total_students,
		  COUNT(DISTINCT cs.id)::int AS curriculum_subjects
		FROM schools s
		LEFT JOIN users u ON u.school_id = s.id
		LEFT JOIN class c ON c.school_id = s.id
		LEFT JOIN curriculum_subjects cs ON cs.school_id = s.id
		GROUP BY s.id, s.name
		HAVING
		  COUNT(DISTINCT CASE WHEN u.role = 'ADMIN' THEN u.id END) = 0
		  OR COUNT(DISTINCT c.id) = 0
		  OR COUNT(DISTINCT CASE WHEN u.role = 'SISWA' THEN u.id END) = 0
		  OR COUNT(DISTINCT cs.id) = 0
		ORDER BY total_students ASC, s.name ASC
		LIMIT 8
	`).Scan(&schoolAlerts)

	var recentAdmins []map[string]interface{}
	a.DB.Raw(`
		SELECT
		  u.id,
		  COALESCE(u.full_name, u.username) AS admin_name,
		  u.username,
		  COALESCE(s.name, '-') AS school_name,
		  COALESCE(u.parent_email, '-') AS email,
		  COALESCE(u.phone_number, '-') AS phone_number
		FROM users u
		LEFT JOIN schools s ON s.id = u.school_id
		WHERE u.role = 'ADMIN'
		ORDER BY u.id DESC
		LIMIT 8
	`).Scan(&recentAdmins)

	var recentAttendance []map[string]interface{}
	a.DB.Raw(`
		SELECT u.username, s.name AS school_name, a.attendance_date, a.clock_in, a.clock_out, a.status
		FROM attendance a
		INNER JOIN users u ON u.id = a.user_id
		LEFT JOIN schools s ON s.id = u.school_id
		ORDER BY a.clock_in DESC NULLS LAST
		LIMIT 8
	`).Scan(&recentAttendance)
	normalizeAttendanceMaps(recentAttendance)

	var recentReceipts []map[string]interface{}
	a.DB.Raw(`
		SELECT u.username, s.name AS school_name, pr.periode, pr.description, pr.created_at
		FROM payment_receipt pr
		INNER JOIN users u ON u.id = pr.user_id
		LEFT JOIN schools s ON s.id = u.school_id
		ORDER BY pr.created_at DESC
		LIMIT 8
	`).Scan(&recentReceipts)
	normalizeReceiptMaps(recentReceipts)

	return utils.Success(c, 200, "Success Get Super Admin Dashboard", fiber.Map{
		"generatedAt":      time.Now().UTC().Format(time.RFC3339),
		"overview":         overview,
		"schools":          schools,
		"schoolAlerts":     recentOrEmpty(schoolAlerts),
		"recentAdmins":     recentOrEmpty(recentAdmins),
		"recentAttendance": recentAttendance,
		"recentReceipts":   recentReceipts,
	})
}

func (a *AppContext) GetAdminDashboard(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	type KV struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	}
	var overviewRows []KV
	a.DB.Raw(`
		SELECT 'teachers' AS key, COUNT(*)::int AS value FROM users WHERE school_id = ? AND role = 'GURU'
		UNION ALL
		SELECT 'students' AS key, COUNT(*)::int AS value FROM users WHERE school_id = ? AND role = 'SISWA'
		UNION ALL
		SELECT 'admins' AS key, COUNT(*)::int AS value FROM users WHERE school_id = ? AND role = 'ADMIN'
		UNION ALL
		SELECT 'classes' AS key, COUNT(*)::int AS value FROM class WHERE school_id = ?
		UNION ALL
		SELECT 'attendance_today' AS key, COUNT(*)::int AS value
		FROM attendance a INNER JOIN users u ON u.id = a.user_id
		WHERE u.school_id = ? AND a.attendance_date = CURRENT_DATE
		UNION ALL
		SELECT 'receipts_this_month' AS key, COUNT(*)::int AS value
		FROM payment_receipt pr INNER JOIN users u ON u.id = pr.user_id
		WHERE u.school_id = ? AND DATE_TRUNC('month', pr.created_at) = DATE_TRUNC('month', CURRENT_DATE)
	`, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID).Scan(&overviewRows)

	overview := map[string]int{}
	for _, row := range overviewRows {
		overview[row.Key] = row.Value
	}

	var school map[string]interface{}
	a.DB.Raw(`SELECT id, name FROM schools WHERE id = ?`, schoolID).Scan(&school)

	var classes []map[string]interface{}
	a.DB.Raw(`
		SELECT c.id, c.class_name, COALESCE(w.username, '-') AS wali_guru_name,
		       w.parent_email AS wali_guru_email, w.phone_number AS wali_guru_phone_number,
		       COUNT(u.id)::int AS student_count
		FROM class c
		LEFT JOIN users w ON w.id = c.wali_guru_id
		LEFT JOIN users u ON u.class_id = c.id AND u.role = 'SISWA'
		WHERE c.school_id = ?
		GROUP BY c.id, c.class_name, w.username, w.parent_email, w.phone_number
		ORDER BY student_count DESC, c.class_name ASC LIMIT 8
	`, schoolID).Scan(&classes)

	var recentAttendance []map[string]interface{}
	a.DB.Raw(`
		SELECT u.username, c.class_name, a.attendance_date, a.clock_in, a.clock_out, a.status
		FROM attendance a
		INNER JOIN users u ON u.id = a.user_id
		LEFT JOIN class c ON c.id = u.class_id
		WHERE u.school_id = ?
		ORDER BY a.clock_in DESC NULLS LAST LIMIT 8
	`, schoolID).Scan(&recentAttendance)
	normalizeAttendanceMaps(recentAttendance)

	var recentReceipts []map[string]interface{}
	a.DB.Raw(`
		SELECT u.username, c.class_name, pr.periode, pr.description, pr.created_at, pr.image_path
		FROM payment_receipt pr
		INNER JOIN users u ON u.id = pr.user_id
		LEFT JOIN class c ON c.id = u.class_id
		WHERE u.school_id = ?
		ORDER BY pr.created_at DESC LIMIT 8
	`, schoolID).Scan(&recentReceipts)
	normalizeReceiptMaps(recentReceipts)

	return utils.Success(c, 200, "Success Get Admin Dashboard", fiber.Map{
		"generatedAt":      time.Now().UTC().Format(time.RFC3339),
		"school":           school,
		"overview":         overview,
		"classes":          recentOrEmpty(classes),
		"recentAttendance": recentOrEmpty(recentAttendance),
		"recentReceipts":   recentOrEmpty(recentReceipts),
	})
}

func (a *AppContext) SendHomeroomAttendanceReport(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Date string `json:"date"`
	}
	_ = c.BodyParser(&body)

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT c.id AS class_id, c.class_name, u.username AS wali_guru_name, u.parent_email AS target
		FROM class c
		LEFT JOIN users u ON u.id = c.wali_guru_id
		WHERE c.school_id = ?
		ORDER BY c.class_name ASC
	`, schoolID).Scan(&rows)

	results := make([]map[string]interface{}, 0, len(rows))
	successCount := 0
	failedCount := 0
	for _, r := range rows {
		target := fmt.Sprint(r["target"])
		ok := strings.TrimSpace(target) != "" && target != "<nil>"
		if ok {
			successCount++
		} else {
			failedCount++
		}
		results = append(results, map[string]interface{}{
			"class_id":        r["class_id"],
			"class_name":      r["class_name"],
			"wali_guru_name":  r["wali_guru_name"],
			"target":          target,
			"success":         ok,
			"error":           ternary(!ok, "Email wali kelas belum tersedia", nil),
			"attendance_date": time.Now().Format("2006-01-02"),
		})
	}

	return utils.Success(c, 200, "Success Send Homeroom Email Attendance Reports", fiber.Map{
		"total_classes": len(rows),
		"success_count": successCount,
		"failed_count":  failedCount,
		"results":       results,
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *AppContext) GetAdminSettingsSummary(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var summary struct {
		Teachers                 int `json:"teachers"`
		Students                 int `json:"students"`
		Classes                  int `json:"classes"`
		AcademicYears            int `json:"academic_years"`
		Semesters                int `json:"semesters"`
		CurriculumSubjects       int `json:"curriculum_subjects"`
		CurriculumTeacherLoads   int `json:"curriculum_teacher_loads"`
		CurriculumDistributions  int `json:"curriculum_class_distributions"`
		CurriculumScheduleSlots  int `json:"curriculum_schedule_slots"`
		CurriculumScheduleResult int `json:"curriculum_schedule_entries"`
		Subjects                 int `json:"subjects"`
		Materials                int `json:"materials"`
		Chats                    int `json:"chats"`
		QuestionBank             int `json:"question_bank"`
		FileTasks                int `json:"file_tasks"`
		ManualAssess             int `json:"manual_assessments"`
		Quizzes                  int `json:"quizzes"`
		OfficialExams            int `json:"official_exams"`
		Attendance               int `json:"attendance"`
		Receipts                 int `json:"receipts"`
	}
	if err := a.DB.Raw(`
		WITH subject_scope AS (SELECT id FROM learning_subjects WHERE school_id = ?)
		SELECT
		  (SELECT COUNT(*)::int FROM users WHERE school_id = ? AND role = 'GURU') AS teachers,
		  (SELECT COUNT(*)::int FROM users WHERE school_id = ? AND role = 'SISWA') AS students,
		  (SELECT COUNT(*)::int FROM class WHERE school_id = ?) AS classes,
		  (SELECT COUNT(*)::int FROM academic_years WHERE school_id = ?) AS academic_years,
		  (SELECT COUNT(*)::int FROM academic_semesters WHERE academic_year_id IN (SELECT id FROM academic_years WHERE school_id = ?)) AS semesters,
		  (SELECT COUNT(*)::int FROM curriculum_subjects WHERE school_id = ?) AS curriculum_subjects,
		  (SELECT COUNT(*)::int FROM curriculum_teacher_loads WHERE school_id = ?) AS curriculum_teacher_loads,
		  (SELECT COUNT(*)::int FROM curriculum_class_distributions WHERE school_id = ?) AS curriculum_class_distributions,
		  (SELECT COUNT(*)::int FROM curriculum_schedule_slots WHERE school_id = ?) AS curriculum_schedule_slots,
		  (SELECT COUNT(*)::int FROM curriculum_schedule_entries WHERE school_id = ?) AS curriculum_schedule_entries,
		  (SELECT COUNT(*)::int FROM learning_subjects WHERE school_id = ?) AS subjects,
		  (SELECT COUNT(*)::int FROM learning_materials WHERE subject_id IN (SELECT id FROM subject_scope)) AS materials,
		  (SELECT COUNT(*)::int FROM learning_chat_messages WHERE subject_id IN (SELECT id FROM subject_scope)) AS chats,
		  (SELECT COUNT(*)::int FROM learning_question_bank WHERE subject_id IN (SELECT id FROM subject_scope)) AS question_bank,
		  (SELECT COUNT(*)::int FROM learning_assignments WHERE subject_id IN (SELECT id FROM subject_scope) AND assignment_type = 'FILE' AND COALESCE(is_exam, false) = false) AS file_tasks,
		  (SELECT COUNT(*)::int FROM learning_assignments WHERE subject_id IN (SELECT id FROM subject_scope) AND assignment_type = 'MANUAL' AND COALESCE(is_exam, false) = false) AS manual_assessments,
		  (SELECT COUNT(*)::int FROM learning_assignments WHERE subject_id IN (SELECT id FROM subject_scope) AND COALESCE(is_exam, false) = false AND assignment_type IN ('MCQ', 'ESSAY')) AS quizzes,
		  (SELECT COUNT(*)::int FROM learning_assignments WHERE subject_id IN (SELECT id FROM subject_scope) AND COALESCE(is_exam, false) = true) AS official_exams,
		  (SELECT COUNT(*)::int FROM attendance WHERE user_id IN (SELECT id FROM users WHERE school_id = ? AND role = 'SISWA')) AS attendance,
		  (SELECT COUNT(*)::int FROM payment_receipt WHERE user_id IN (SELECT id FROM users WHERE school_id = ? AND role = 'SISWA')) AS receipts
	`, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID).Scan(&summary).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat ringkasan setting admin", err.Error())
	}

	items := []map[string]interface{}{
		{"key": "teachers", "label": "Guru", "description": "Menghapus semua akun guru pada sekolah ini.", "count": summary.Teachers},
		{"key": "students", "label": "Siswa", "description": "Menghapus semua akun siswa pada sekolah ini beserta data terkait siswa.", "count": summary.Students},
		{"key": "classes", "label": "Kelas", "description": "Menghapus semua kelas pada sekolah ini.", "count": summary.Classes},
		{"key": "academic_periods", "label": "Periode Akademik", "description": "Menghapus tahun ajaran dan semester pada sekolah ini.", "count": summary.AcademicYears + summary.Semesters},
		{"key": "curriculum_subjects", "label": "Kurikulum Mapel", "description": "Menghapus master mata pelajaran kurikulum beserta data generate terkait.", "count": summary.CurriculumSubjects},
		{"key": "curriculum_teacher_loads", "label": "Kurikulum Beban Guru", "description": "Menghapus seluruh beban guru pada modul kurikulum.", "count": summary.CurriculumTeacherLoads},
		{"key": "curriculum_class_distributions", "label": "Kurikulum Distribusi Kelas", "description": "Menghapus pembagian guru ke kelas pada modul kurikulum.", "count": summary.CurriculumDistributions},
		{"key": "curriculum_schedule_slots", "label": "Kurikulum Slot Jadwal", "description": "Menghapus template slot jadwal pada modul kurikulum.", "count": summary.CurriculumScheduleSlots},
		{"key": "curriculum_schedule_entries", "label": "Kurikulum Hasil Generate", "description": "Menghapus hasil generate jadwal dan subject LMS otomatis dari modul kurikulum.", "count": summary.CurriculumScheduleResult},
		{"key": "subjects", "label": "Mapel LMS", "description": "Menghapus semua mapel pembelajaran LMS beserta materi, chat, quiz, ujian, dan bank soal yang terkait.", "count": summary.Subjects},
		{"key": "materials", "label": "Materi Pembelajaran", "description": "Menghapus seluruh materi pembelajaran pada semua mapel sekolah ini.", "count": summary.Materials},
		{"key": "learning_chat", "label": "Chat Pembelajaran", "description": "Menghapus pesan chat pembelajaran dan status baca pada semua mapel.", "count": summary.Chats},
		{"key": "question_bank", "label": "Bank Soal", "description": "Menghapus seluruh bank soal pada sekolah ini.", "count": summary.QuestionBank},
		{"key": "file_tasks", "label": "Tugas File", "description": "Menghapus semua tugas file beserta submission siswa.", "count": summary.FileTasks},
		{"key": "manual_assessments", "label": "Penilaian Manual", "description": "Menghapus semua penilaian manual atau ujian luar LMS beserta submission siswa.", "count": summary.ManualAssess},
		{"key": "quizzes", "label": "Quiz", "description": "Menghapus semua quiz biasa beserta submission siswa.", "count": summary.Quizzes},
		{"key": "official_exams", "label": "Ujian Resmi", "description": "Menghapus semua ujian resmi beserta submission siswa.", "count": summary.OfficialExams},
		{"key": "attendance", "label": "Absensi", "description": "Menghapus seluruh riwayat absensi siswa pada sekolah ini.", "count": summary.Attendance},
		{"key": "receipts", "label": "Bukti Pembayaran", "description": "Menghapus seluruh bukti pembayaran siswa pada sekolah ini.", "count": summary.Receipts},
	}
	return utils.Success(c, 200, "Success Get Admin Settings Summary", fiber.Map{"items": items})
}

func (a *AppContext) GetPublicStudentRegistrationLink(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	token, err := utils.GenerateSchoolRegistrationToken(schoolID, 180*24*time.Hour)
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat link pendaftaran")
	}

	return utils.Success(c, 200, "Success Generate Public Registration Link", fiber.Map{
		"token": token,
		"path":  "/student-registration?token=" + token,
	})
}

func (a *AppContext) RunAdminLoadTest(c *fiber.Ctx) error {
	var body struct {
		HitCount int `json:"hit_count"`
	}
	_ = c.BodyParser(&body)

	if body.HitCount <= 0 {
		body.HitCount = 100
	}
	if body.HitCount > 2000 {
		return utils.Error(c, 400, "hit_count maksimal 2000 per eksekusi")
	}

	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if authHeader == "" {
		return utils.Error(c, 401, "Authorization header is required")
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "9900"
	}
	targetURL := fmt.Sprintf("http://127.0.0.1:%s/api/admin-settings/summary", port)
	client := &http.Client{Timeout: 60 * time.Second}

	type testResult struct {
		DurationMs float64 `json:"duration_ms"`
		StatusCode int     `json:"status_code"`
		Error      string  `json:"error,omitempty"`
	}

	results := make(chan testResult, body.HitCount)
	var successCount int64
	var failureCount int64
	startedAt := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < body.HitCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			start := time.Now()
			req, err := http.NewRequest(http.MethodGet, targetURL, nil)
			if err != nil {
				atomic.AddInt64(&failureCount, 1)
				results <- testResult{DurationMs: 0, StatusCode: 0, Error: err.Error()}
				return
			}
			req.Header.Set("Authorization", authHeader)

			resp, err := client.Do(req)
			durationMs := float64(time.Since(start).Milliseconds())
			if err != nil {
				atomic.AddInt64(&failureCount, 1)
				results <- testResult{DurationMs: durationMs, StatusCode: 0, Error: err.Error()}
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failureCount, 1)
			}
			results <- testResult{DurationMs: durationMs, StatusCode: resp.StatusCode}
		}()
	}

	wg.Wait()
	close(results)

	durations := make([]float64, 0, body.HitCount)
	errorSamples := make([]string, 0, 5)
	var totalDurationMs float64
	var maxDurationMs float64

	for result := range results {
		durations = append(durations, result.DurationMs)
		totalDurationMs += result.DurationMs
		if result.DurationMs > maxDurationMs {
			maxDurationMs = result.DurationMs
		}
		if result.Error != "" && len(errorSamples) < 5 {
			errorSamples = append(errorSamples, result.Error)
		}
	}

	sort.Float64s(durations)
	averageDurationMs := 0.0
	p95DurationMs := 0.0
	if len(durations) > 0 {
		averageDurationMs = totalDurationMs / float64(len(durations))
		p95Index := int(math.Ceil(float64(len(durations))*0.95)) - 1
		if p95Index < 0 {
			p95Index = 0
		}
		if p95Index >= len(durations) {
			p95Index = len(durations) - 1
		}
		p95DurationMs = durations[p95Index]
	}

	return utils.Success(c, 200, "Success Run Admin Load Test", fiber.Map{
		"target":              "/api/admin-settings/summary",
		"hit_count":           body.HitCount,
		"success_count":       successCount,
		"failure_count":       failureCount,
		"total_elapsed_ms":    time.Since(startedAt).Milliseconds(),
		"average_duration_ms": math.Round(averageDurationMs*100) / 100,
		"max_duration_ms":     math.Round(maxDurationMs*100) / 100,
		"p95_duration_ms":     math.Round(p95DurationMs*100) / 100,
		"error_samples":       errorSamples,
		"executed_at":         time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *AppContext) RunAdminLoginLoadTest(c *fiber.Ctx) error {
	var body struct {
		HitCount int    `json:"hit_count"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = c.BodyParser(&body)

	if strings.TrimSpace(body.Username) == "" || strings.TrimSpace(body.Password) == "" {
		return utils.Error(c, 400, "username and password are required")
	}
	if body.HitCount <= 0 {
		body.HitCount = 100
	}
	if body.HitCount > 2000 {
		return utils.Error(c, 400, "hit_count maksimal 2000 per eksekusi")
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "9900"
	}
	targetURL := fmt.Sprintf("http://127.0.0.1:%s/api/auth/login", port)
	client := &http.Client{Timeout: 60 * time.Second}

	type testResult struct {
		DurationMs float64 `json:"duration_ms"`
		StatusCode int     `json:"status_code"`
		Error      string  `json:"error,omitempty"`
	}

	payload := fmt.Sprintf(`{"username":%q,"password":%q}`, body.Username, body.Password)
	results := make(chan testResult, body.HitCount)
	var successCount int64
	var failureCount int64
	startedAt := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < body.HitCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			start := time.Now()
			req, err := http.NewRequest(http.MethodPost, targetURL, strings.NewReader(payload))
			if err != nil {
				atomic.AddInt64(&failureCount, 1)
				results <- testResult{DurationMs: 0, StatusCode: 0, Error: err.Error()}
				return
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			durationMs := float64(time.Since(start).Milliseconds())
			if err != nil {
				atomic.AddInt64(&failureCount, 1)
				results <- testResult{DurationMs: durationMs, StatusCode: 0, Error: err.Error()}
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&failureCount, 1)
			}
			results <- testResult{DurationMs: durationMs, StatusCode: resp.StatusCode}
		}()
	}

	wg.Wait()
	close(results)

	durations := make([]float64, 0, body.HitCount)
	errorSamples := make([]string, 0, 5)
	var totalDurationMs float64
	var maxDurationMs float64

	for result := range results {
		durations = append(durations, result.DurationMs)
		totalDurationMs += result.DurationMs
		if result.DurationMs > maxDurationMs {
			maxDurationMs = result.DurationMs
		}
		if result.Error != "" && len(errorSamples) < 5 {
			errorSamples = append(errorSamples, result.Error)
		}
	}

	sort.Float64s(durations)
	averageDurationMs := 0.0
	p95DurationMs := 0.0
	if len(durations) > 0 {
		averageDurationMs = totalDurationMs / float64(len(durations))
		p95Index := int(math.Ceil(float64(len(durations))*0.95)) - 1
		if p95Index < 0 {
			p95Index = 0
		}
		if p95Index >= len(durations) {
			p95Index = len(durations) - 1
		}
		p95DurationMs = durations[p95Index]
	}

	return utils.Success(c, 200, "Success Run Admin Login Load Test", fiber.Map{
		"target":              "/api/auth/login",
		"username":            body.Username,
		"hit_count":           body.HitCount,
		"success_count":       successCount,
		"failure_count":       failureCount,
		"total_elapsed_ms":    time.Since(startedAt).Milliseconds(),
		"average_duration_ms": math.Round(averageDurationMs*100) / 100,
		"max_duration_ms":     math.Round(maxDurationMs*100) / 100,
		"p95_duration_ms":     math.Round(p95DurationMs*100) / 100,
		"error_samples":       errorSamples,
		"executed_at":         time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *AppContext) ResetAdminScope(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Scope       string `json:"scope"`
		ConfirmText string `json:"confirm_text"`
	}
	_ = c.BodyParser(&body)
	scope := strings.ToLower(strings.TrimSpace(body.Scope))
	if strings.ToUpper(strings.TrimSpace(body.ConfirmText)) != "RESET" {
		return utils.Error(c, 400, "confirm_text must be RESET")
	}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		switch scope {
		case "teachers":
			tx.Exec(`UPDATE class SET wali_guru_id = NULL WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM users WHERE school_id = ? AND role = 'GURU'`, schoolID)
		case "students":
			tx.Exec(`DELETE FROM users WHERE school_id = ? AND role = 'SISWA'`, schoolID)
		case "classes":
			tx.Exec(`UPDATE users SET class_id = NULL WHERE school_id = ? AND role = 'SISWA'`, schoolID)
			tx.Exec(`UPDATE class SET wali_guru_id = NULL WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM class WHERE school_id = ?`, schoolID)
		case "academic_periods":
			tx.Exec(`DELETE FROM academic_semesters WHERE academic_year_id IN (SELECT id FROM academic_years WHERE school_id = ?)`, schoolID)
			tx.Exec(`DELETE FROM academic_years WHERE school_id = ?`, schoolID)
		case "curriculum_subjects":
			tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_class_distributions WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_teacher_loads WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_subjects WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ? AND COALESCE(curriculum_auto_generated, false) = true`, schoolID)
		case "curriculum_teacher_loads":
			tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_class_distributions WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_teacher_loads WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ? AND COALESCE(curriculum_auto_generated, false) = true`, schoolID)
		case "curriculum_class_distributions":
			tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_class_distributions WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ? AND COALESCE(curriculum_auto_generated, false) = true`, schoolID)
		case "curriculum_schedule_slots":
			tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM curriculum_schedule_slots WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ? AND COALESCE(curriculum_auto_generated, false) = true`, schoolID)
		case "curriculum_schedule_entries":
			tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, schoolID)
			tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ? AND COALESCE(curriculum_auto_generated, false) = true`, schoolID)
		case "subjects":
			tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ?`, schoolID)
		case "materials":
			tx.Exec(`DELETE FROM learning_materials WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, schoolID)
		case "learning_chat":
			tx.Exec(`DELETE FROM learning_chat_reads WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, schoolID)
			tx.Exec(`DELETE FROM learning_chat_messages WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, schoolID)
		case "question_bank":
			tx.Exec(`DELETE FROM learning_question_bank WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, schoolID)
		case "file_tasks":
			tx.Exec(`DELETE FROM learning_assignments WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?) AND assignment_type = 'FILE' AND COALESCE(is_exam, false) = false`, schoolID)
		case "manual_assessments":
			tx.Exec(`DELETE FROM learning_assignments WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?) AND assignment_type = 'MANUAL' AND COALESCE(is_exam, false) = false`, schoolID)
		case "quizzes":
			tx.Exec(`DELETE FROM learning_assignments WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?) AND COALESCE(is_exam, false) = false AND assignment_type IN ('MCQ','ESSAY')`, schoolID)
		case "official_exams":
			tx.Exec(`DELETE FROM learning_assignments WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?) AND COALESCE(is_exam, false) = true`, schoolID)
		case "attendance":
			tx.Exec(`DELETE FROM attendance WHERE user_id IN (SELECT id FROM users WHERE school_id = ? AND role = 'SISWA')`, schoolID)
		case "receipts":
			tx.Exec(`DELETE FROM payment_receipt WHERE user_id IN (SELECT id FROM users WHERE school_id = ? AND role = 'SISWA')`, schoolID)
		default:
			return fmt.Errorf("invalid scope")
		}
		return nil
	})
	if err != nil {
		return utils.Error(c, 400, "Invalid reset scope")
	}
	return utils.Success(c, 200, "Success Reset Admin Scope", fiber.Map{
		"scope": scope,
	})
}

func recentOrEmpty(v []map[string]interface{}) []map[string]interface{} {
	if v == nil {
		return []map[string]interface{}{}
	}
	return v
}

func ternary(cond bool, a interface{}, b interface{}) interface{} {
	if cond {
		return a
	}
	return b
}

func toIntAny(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}
