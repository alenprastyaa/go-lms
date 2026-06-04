package controllers

import (
	"math"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/utils"
)

type payrollSettingsRow struct {
	ID                      uint    `json:"id" gorm:"column:id"`
	SchoolID                uint    `json:"school_id" gorm:"column:school_id"`
	HourlyRate              int64   `json:"hourly_rate" gorm:"column:hourly_rate"`
	LessonMinutes           int     `json:"lesson_minutes" gorm:"column:lesson_minutes"`
	TeachingHoursMultiplier float64 `json:"teaching_hours_multiplier" gorm:"column:teaching_hours_multiplier"`
	Notes                   string  `json:"notes" gorm:"column:notes"`
}

type payrollComponentRow struct {
	ID              uint    `json:"id" gorm:"column:id"`
	Name            string  `json:"name" gorm:"column:name"`
	ComponentType   string  `json:"component_type" gorm:"column:component_type"`
	CalculationType string  `json:"calculation_type" gorm:"column:calculation_type"`
	DefaultAmount   int64   `json:"default_amount" gorm:"column:default_amount"`
	DefaultQuantity float64 `json:"default_quantity" gorm:"column:default_quantity"`
	AppliesToAll    bool    `json:"applies_to_all" gorm:"column:applies_to_all"`
	IsActive        bool    `json:"is_active" gorm:"column:is_active"`
}

type payrollTeacherRow struct {
	TeacherID           uint       `json:"teacher_id" gorm:"column:teacher_id"`
	TeacherName         string     `json:"teacher_name" gorm:"column:teacher_name"`
	Username            string     `json:"username" gorm:"column:username"`
	WeeklyHours         float64    `json:"weekly_hours" gorm:"column:weekly_hours"`
	SuggestedMonthHours float64    `json:"suggested_month_hours" gorm:"column:suggested_month_hours"`
	ValidAttendanceDays int        `json:"valid_attendance_days" gorm:"column:valid_attendance_days"`
	PayrollID           *uint      `json:"payroll_id" gorm:"column:payroll_id"`
	PreviousPayrollID   *uint      `json:"previous_payroll_id" gorm:"column:previous_payroll_id"`
	HourlyRate          *int64     `json:"hourly_rate" gorm:"column:hourly_rate"`
	TeachingHours       *float64   `json:"teaching_hours" gorm:"column:teaching_hours"`
	BaseAmount          *int64     `json:"base_amount" gorm:"column:base_amount"`
	AllowancesAmount    *int64     `json:"allowances_amount" gorm:"column:allowances_amount"`
	DeductionsAmount    *int64     `json:"deductions_amount" gorm:"column:deductions_amount"`
	TotalAmount         *int64     `json:"total_amount" gorm:"column:total_amount"`
	Status              *string    `json:"status" gorm:"column:status"`
	Note                *string    `json:"note" gorm:"column:note"`
	PaidAt              *time.Time `json:"paid_at" gorm:"column:paid_at"`
}

type payrollItemRow struct {
	ID              uint    `json:"id" gorm:"column:id"`
	PayrollID       uint    `json:"payroll_id" gorm:"column:payroll_id"`
	ComponentID     *uint   `json:"component_id" gorm:"column:component_id"`
	Name            string  `json:"name" gorm:"column:name"`
	ComponentType   string  `json:"component_type" gorm:"column:component_type"`
	CalculationType string  `json:"calculation_type" gorm:"column:calculation_type"`
	Quantity        float64 `json:"quantity" gorm:"column:quantity"`
	UnitAmount      int64   `json:"unit_amount" gorm:"column:unit_amount"`
	Amount          int64   `json:"amount" gorm:"column:amount"`
}

type payrollItemInput struct {
	ComponentID     *uint   `json:"component_id"`
	Name            string  `json:"name"`
	ComponentType   string  `json:"component_type"`
	CalculationType string  `json:"calculation_type"`
	Quantity        float64 `json:"quantity"`
	UnitAmount      int64   `json:"unit_amount"`
	Amount          int64   `json:"amount"`
}

type payrollSlipInput struct {
	TeacherID     uint               `json:"teacher_id"`
	Period        string             `json:"period"`
	HourlyRate    int64              `json:"hourly_rate"`
	TeachingHours float64            `json:"teaching_hours"`
	Note          string             `json:"note"`
	Status        string             `json:"status"`
	Items         []payrollItemInput `json:"items"`
}

const defaultPayrollPageSize = 10
const maxPayrollPageSize = 100

func (a *AppContext) GetPayrollOverview(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	period, err := parsePayrollPeriod(c.Query("period", ""))
	if err != nil {
		return utils.Error(c, 400, "Periode payroll tidak valid")
	}
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", ""), defaultPayrollPageSize)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultPayrollPageSize
	}
	if limit > maxPayrollPageSize {
		limit = maxPayrollPageSize
	}
	search := strings.TrimSpace(c.Query("search"))
	searchLike := "%" + search + "%"

	settings, err := a.ensurePayrollSettings(schoolID)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat setting payroll", err.Error())
	}

	components, err := a.loadPayrollComponents(schoolID, false)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat komponen payroll", err.Error())
	}

	var summary struct {
		TotalTeachers      int64 `json:"total_teachers" gorm:"column:total_teachers"`
		PaidCount          int64 `json:"paid_count" gorm:"column:paid_count"`
		UnpaidCount        int64 `json:"unpaid_count" gorm:"column:unpaid_count"`
		PaidAmount         int64 `json:"paid_amount" gorm:"column:paid_amount"`
		UnpaidAmount       int64 `json:"unpaid_amount" gorm:"column:unpaid_amount"`
		TotalPayrollAmount int64 `json:"total_payroll_amount" gorm:"column:total_payroll_amount"`
	}
	if err := a.DB.Raw(`
		WITH teacher_hours AS (
			SELECT
				u.id AS teacher_id,
				COALESCE(NULLIF(u.full_name, ''), u.username) AS teacher_name,
				u.username,
				COALESCE(SUM(ccd.weekly_hours), 0)::numeric(10,2) AS weekly_hours
			FROM users u
			LEFT JOIN curriculum_teacher_loads ctl
				ON ctl.school_id = u.school_id AND ctl.teacher_id = u.id
			LEFT JOIN curriculum_class_distributions ccd
				ON ccd.school_id = ctl.school_id AND ccd.curriculum_teacher_load_id = ctl.id
			WHERE u.school_id = ? AND u.role = 'GURU'
			GROUP BY u.id, u.full_name, u.username
		),
		payroll_rows AS (
			SELECT
				th.teacher_id,
				th.teacher_name,
				th.username,
				COALESCE(tp.status, 'BELUM_DIBUAT') AS status,
				CASE
					WHEN tp.id IS NOT NULL THEN COALESCE(tp.total_amount, ROUND((COALESCE(tp.hourly_rate, ?)::numeric * COALESCE(tp.teaching_hours, th.weekly_hours * ?))::numeric)::bigint)
					ELSE ROUND((?::numeric * (th.weekly_hours * ?))::numeric)::bigint
				END AS effective_total_amount
			FROM teacher_hours th
			LEFT JOIN teacher_payrolls tp
				ON tp.school_id = ? AND tp.teacher_id = th.teacher_id AND tp.period_month = ?
		)
		SELECT
			COUNT(*)::bigint AS total_teachers,
			COUNT(*) FILTER (WHERE status = 'PAID')::bigint AS paid_count,
			COUNT(*) FILTER (WHERE status <> 'PAID')::bigint AS unpaid_count,
			COALESCE(SUM(CASE WHEN status = 'PAID' THEN effective_total_amount ELSE 0 END), 0)::bigint AS paid_amount,
			COALESCE(SUM(CASE WHEN status <> 'PAID' THEN effective_total_amount ELSE 0 END), 0)::bigint AS unpaid_amount,
			COALESCE(SUM(effective_total_amount), 0)::bigint AS total_payroll_amount
		FROM payroll_rows
		WHERE (
			? = ''
			OR teacher_name ILIKE ?
			OR username ILIKE ?
			OR status ILIKE ?
			OR CASE
				WHEN status = 'PAID' THEN 'Lunas'
				WHEN status = 'DRAFT' THEN 'Draft'
				ELSE 'Belum dibuat'
			END ILIKE ?
		)
	`, schoolID, settings.HourlyRate, settings.TeachingHoursMultiplier, settings.HourlyRate, settings.TeachingHoursMultiplier, schoolID, period, search, searchLike, searchLike, searchLike, searchLike).Scan(&summary).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat ringkasan payroll", err.Error())
	}

	totalPages := totalPagesFromCount(summary.TotalTeachers, limit)
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * limit

	teachers := make([]payrollTeacherRow, 0)
	if err := a.DB.Raw(`
		WITH teacher_hours AS (
			SELECT
				u.id AS teacher_id,
				COALESCE(NULLIF(u.full_name, ''), u.username) AS teacher_name,
				u.username,
				COALESCE(SUM(ccd.weekly_hours), 0)::numeric(10,2) AS weekly_hours
			FROM users u
			LEFT JOIN curriculum_teacher_loads ctl
				ON ctl.school_id = u.school_id AND ctl.teacher_id = u.id
			LEFT JOIN curriculum_class_distributions ccd
				ON ccd.school_id = ctl.school_id AND ccd.curriculum_teacher_load_id = ctl.id
			WHERE u.school_id = ? AND u.role = 'GURU'
			GROUP BY u.id, u.full_name, u.username
		),
		valid_attendance AS (
			SELECT
				a.user_id AS teacher_id,
				COUNT(DISTINCT a.attendance_date)::int AS valid_attendance_days
			FROM attendance a
			INNER JOIN users u ON u.id = a.user_id AND u.school_id = ? AND u.role = 'GURU'
			WHERE a.attendance_date >= ? AND a.attendance_date < (?::date + INTERVAL '1 month')
			  AND a.clock_in IS NOT NULL
			  AND a.clock_out IS NOT NULL
			GROUP BY a.user_id
		),
		previous_payroll AS (
			SELECT DISTINCT ON (teacher_id)
				teacher_id,
				id AS previous_payroll_id
			FROM teacher_payrolls
			WHERE school_id = ? AND period_month < ?
			ORDER BY teacher_id, period_month DESC, id DESC
		)
		SELECT
			th.teacher_id,
			th.teacher_name,
			th.username,
			th.weekly_hours,
			(th.weekly_hours * ?)::numeric(10,2) AS suggested_month_hours,
			COALESCE(va.valid_attendance_days, 0) AS valid_attendance_days,
			tp.id AS payroll_id,
			pp.previous_payroll_id,
			tp.hourly_rate,
			tp.teaching_hours,
			tp.base_amount,
			tp.allowances_amount,
			tp.deductions_amount,
			tp.total_amount,
			tp.status,
			tp.note,
			tp.paid_at
		FROM teacher_hours th
		LEFT JOIN valid_attendance va ON va.teacher_id = th.teacher_id
		LEFT JOIN teacher_payrolls tp
			ON tp.school_id = ? AND tp.teacher_id = th.teacher_id AND tp.period_month = ?
		LEFT JOIN previous_payroll pp ON pp.teacher_id = th.teacher_id
		WHERE (
			? = ''
			OR th.teacher_name ILIKE ?
			OR th.username ILIKE ?
			OR COALESCE(tp.status, 'BELUM_DIBUAT') ILIKE ?
			OR CASE
				WHEN COALESCE(tp.status, 'BELUM_DIBUAT') = 'PAID' THEN 'Lunas'
				WHEN COALESCE(tp.status, 'BELUM_DIBUAT') = 'DRAFT' THEN 'Draft'
				ELSE 'Belum dibuat'
			END ILIKE ?
		)
		ORDER BY th.teacher_name ASC, th.username ASC
		LIMIT ? OFFSET ?
	`, schoolID, schoolID, period, period, schoolID, period, settings.TeachingHoursMultiplier, schoolID, period, search, searchLike, searchLike, searchLike, searchLike, limit, offset).Scan(&teachers).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat data guru payroll", err.Error())
	}

	payrollIDs := make([]uint, 0)
	previousPayrollIDs := make([]uint, 0)
	for _, teacher := range teachers {
		if teacher.PayrollID != nil && *teacher.PayrollID > 0 {
			payrollIDs = append(payrollIDs, *teacher.PayrollID)
		}
		if teacher.PreviousPayrollID != nil && *teacher.PreviousPayrollID > 0 {
			previousPayrollIDs = append(previousPayrollIDs, *teacher.PreviousPayrollID)
		}
	}
	itemsByPayroll, err := a.loadPayrollItemsByPayrollIDs(payrollIDs)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat item payroll", err.Error())
	}
	previousItemsByPayroll, err := a.loadPayrollItemsByPayrollIDs(previousPayrollIDs)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat item payroll bulan sebelumnya", err.Error())
	}

	rows := make([]fiber.Map, 0, len(teachers))
	for _, teacher := range teachers {
		hourlyRate := settings.HourlyRate
		if teacher.HourlyRate != nil {
			hourlyRate = *teacher.HourlyRate
		}
		teachingHours := teacher.SuggestedMonthHours
		if teacher.TeachingHours != nil {
			teachingHours = *teacher.TeachingHours
		}
		payrollID := uint(0)
		if teacher.PayrollID != nil {
			payrollID = *teacher.PayrollID
		}
		items := itemsByPayroll[payrollID]
		itemsSource := "current"
		status := stringPtrValue(teacher.Status, "BELUM_DIBUAT")
		if payrollID == 0 && teacher.PreviousPayrollID != nil && *teacher.PreviousPayrollID > 0 {
			items = clonePayrollItemRows(previousItemsByPayroll[*teacher.PreviousPayrollID])
			applyTransportAttendanceToPayrollItemRows(items, teacher.ValidAttendanceDays)
			itemsSource = "previous"
		} else if payrollID > 0 {
			items = clonePayrollItemRows(items)
		}
		baseAmount := int64PtrValue(teacher.BaseAmount)
		allowancesAmount := int64PtrValue(teacher.AllowancesAmount)
		deductionsAmount := int64PtrValue(teacher.DeductionsAmount)
		totalAmount := int64PtrValue(teacher.TotalAmount)
		if status != "PAID" {
			applyTransportAttendanceToPayrollItemRows(items, teacher.ValidAttendanceDays)
			baseAmount, allowancesAmount, deductionsAmount, totalAmount = calculatePayrollTotals(hourlyRate, teachingHours, payrollItemRowsToInputs(items))
		}
		rows = append(rows, fiber.Map{
			"teacher_id":            teacher.TeacherID,
			"teacher_name":          teacher.TeacherName,
			"username":              teacher.Username,
			"weekly_hours":          teacher.WeeklyHours,
			"suggested_month_hours": teacher.SuggestedMonthHours,
			"valid_attendance_days": teacher.ValidAttendanceDays,
			"previous_payroll_id":   teacher.PreviousPayrollID,
			"payroll_id":            teacher.PayrollID,
			"hourly_rate":           hourlyRate,
			"teaching_hours":        teachingHours,
			"base_amount":           baseAmount,
			"allowances_amount":     allowancesAmount,
			"deductions_amount":     deductionsAmount,
			"total_amount":          totalAmount,
			"status":                status,
			"note":                  stringPtrValue(teacher.Note, ""),
			"paid_at":               teacher.PaidAt,
			"items":                 items,
			"items_source":          itemsSource,
		})
	}

	return utils.Success(c, 200, "Success Get Payroll Overview", fiber.Map{
		"period":      period.Format("2006-01"),
		"settings":    settings,
		"components":  components,
		"teachers":    rows,
		"summary":     summary,
		"page":        page,
		"limit":       limit,
		"search":      search,
		"total":       summary.TotalTeachers,
		"total_pages": totalPages,
	})
}

func (a *AppContext) UpdatePayrollSettings(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		HourlyRate              int64   `json:"hourly_rate"`
		LessonMinutes           int     `json:"lesson_minutes"`
		TeachingHoursMultiplier float64 `json:"teaching_hours_multiplier"`
		Notes                   string  `json:"notes"`
	}
	_ = c.BodyParser(&body)
	if body.HourlyRate <= 0 {
		return utils.Error(c, 400, "Tarif per jam wajib lebih dari 0")
	}
	if body.LessonMinutes <= 0 {
		body.LessonMinutes = 45
	}
	if body.TeachingHoursMultiplier <= 0 {
		body.TeachingHoursMultiplier = 4
	}
	if err := a.DB.Exec(`
		INSERT INTO payroll_settings (school_id, hourly_rate, lesson_minutes, teaching_hours_multiplier, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (school_id)
		DO UPDATE SET hourly_rate = EXCLUDED.hourly_rate, lesson_minutes = EXCLUDED.lesson_minutes, teaching_hours_multiplier = EXCLUDED.teaching_hours_multiplier, notes = EXCLUDED.notes, updated_at = NOW()
	`, schoolID, body.HourlyRate, body.LessonMinutes, body.TeachingHoursMultiplier, strings.TrimSpace(body.Notes)).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan setting payroll", err.Error())
	}
	settings, err := a.ensurePayrollSettings(schoolID)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat setting payroll", err.Error())
	}
	return utils.Success(c, 200, "Setting payroll berhasil disimpan", settings)
}

func (a *AppContext) CreatePayrollComponent(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var body payrollComponentRow
	_ = c.BodyParser(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama komponen wajib diisi")
	}
	componentType := normalizePayrollComponentType(body.ComponentType)
	calculationType := normalizePayrollCalculationType(body.CalculationType)
	if body.DefaultQuantity <= 0 {
		body.DefaultQuantity = 1
	}
	var row payrollComponentRow
	if err := a.DB.Raw(`
		INSERT INTO payroll_components (school_id, name, component_type, calculation_type, default_amount, default_quantity, applies_to_all, is_active, created_by, updated_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		RETURNING id, name, component_type, calculation_type, default_amount, default_quantity, applies_to_all, is_active
	`, schoolID, name, componentType, calculationType, body.DefaultAmount, body.DefaultQuantity, body.AppliesToAll, body.IsActive, userID, userID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat komponen payroll", err.Error())
	}
	return utils.Success(c, 201, "Komponen payroll berhasil dibuat", row)
}

func (a *AppContext) UpdatePayrollComponent(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	id := c.Params("id")
	var body payrollComponentRow
	_ = c.BodyParser(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama komponen wajib diisi")
	}
	componentType := normalizePayrollComponentType(body.ComponentType)
	calculationType := normalizePayrollCalculationType(body.CalculationType)
	if body.DefaultQuantity <= 0 {
		body.DefaultQuantity = 1
	}
	var row payrollComponentRow
	if err := a.DB.Raw(`
		UPDATE payroll_components
		SET name = ?, component_type = ?, calculation_type = ?, default_amount = ?, default_quantity = ?, applies_to_all = ?, is_active = ?, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, name, component_type, calculation_type, default_amount, default_quantity, applies_to_all, is_active
	`, name, componentType, calculationType, body.DefaultAmount, body.DefaultQuantity, body.AppliesToAll, body.IsActive, userID, id, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui komponen payroll", err.Error())
	}
	if row.ID == 0 {
		return utils.Error(c, 404, "Komponen payroll tidak ditemukan")
	}
	return utils.Success(c, 200, "Komponen payroll berhasil diperbarui", row)
}

func (a *AppContext) DeletePayrollComponent(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")
	var row struct {
		ID uint `gorm:"column:id"`
	}
	if err := a.DB.Raw(`
		DELETE FROM payroll_components
		WHERE id = ? AND school_id = ?
		RETURNING id
	`, id, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus komponen payroll", err.Error())
	}
	if row.ID == 0 {
		return utils.Error(c, 404, "Komponen payroll tidak ditemukan")
	}
	return utils.Success(c, 200, "Komponen payroll berhasil dihapus", fiber.Map{"id": row.ID})
}

func (a *AppContext) UpsertTeacherPayroll(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var body payrollSlipInput
	_ = c.BodyParser(&body)
	if body.TeacherID == 0 {
		return utils.Error(c, 400, "Guru wajib dipilih")
	}
	period, err := parsePayrollPeriod(body.Period)
	if err != nil {
		return utils.Error(c, 400, "Periode payroll tidak valid")
	}
	if body.HourlyRate <= 0 {
		return utils.Error(c, 400, "Tarif per jam wajib lebih dari 0")
	}
	if body.TeachingHours < 0 {
		return utils.Error(c, 400, "Jam mengajar tidak valid")
	}
	var teacher struct {
		ID uint `gorm:"column:id"`
	}
	a.DB.Raw(`SELECT id FROM users WHERE id = ? AND school_id = ? AND role = 'GURU'`, body.TeacherID, schoolID).Scan(&teacher)
	if teacher.ID == 0 {
		return utils.Error(c, 404, "Guru tidak ditemukan")
	}

	validAttendanceDays, err := a.countValidTeacherAttendanceDays(schoolID, body.TeacherID, period)
	if err != nil {
		return utils.Error(c, 500, "Gagal menghitung absensi valid", err.Error())
	}
	applyTransportAttendanceToPayrollItemInputs(body.Items, validAttendanceDays)
	baseAmount, allowancesAmount, deductionsAmount, totalAmount := calculatePayrollTotals(body.HourlyRate, body.TeachingHours, body.Items)
	status := normalizePayrollStatus(body.Status)
	var savedID uint
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		var row struct {
			ID uint `gorm:"column:id"`
		}
		if err := tx.Raw(`
			INSERT INTO teacher_payrolls (
				school_id, teacher_id, period_month, hourly_rate, teaching_hours,
				base_amount, allowances_amount, deductions_amount, total_amount,
				status, note, paid_at, created_by, updated_by, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CASE WHEN ? = 'PAID' THEN NOW() ELSE NULL END, ?, ?, NOW(), NOW())
			ON CONFLICT (school_id, teacher_id, period_month)
			DO UPDATE SET
				hourly_rate = EXCLUDED.hourly_rate,
				teaching_hours = EXCLUDED.teaching_hours,
				base_amount = EXCLUDED.base_amount,
				allowances_amount = EXCLUDED.allowances_amount,
				deductions_amount = EXCLUDED.deductions_amount,
				total_amount = EXCLUDED.total_amount,
				status = EXCLUDED.status,
				note = EXCLUDED.note,
				paid_at = CASE WHEN EXCLUDED.status = 'PAID' THEN COALESCE(teacher_payrolls.paid_at, NOW()) ELSE NULL END,
				updated_by = EXCLUDED.updated_by,
				updated_at = NOW()
			RETURNING id
		`, schoolID, body.TeacherID, period, body.HourlyRate, body.TeachingHours, baseAmount, allowancesAmount, deductionsAmount, totalAmount, status, strings.TrimSpace(body.Note), status, userID, userID).Scan(&row).Error; err != nil {
			return err
		}
		savedID = row.ID
		if err := tx.Exec(`DELETE FROM teacher_payroll_items WHERE payroll_id = ?`, savedID).Error; err != nil {
			return err
		}
		for _, item := range body.Items {
			normalizedItem, ok := normalizePayrollItemInput(item)
			if !ok {
				continue
			}
			name := normalizedItem.Name
			if err := tx.Exec(`
				INSERT INTO teacher_payroll_items (payroll_id, component_id, name, component_type, calculation_type, quantity, unit_amount, amount, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
			`, savedID, normalizedItem.ComponentID, name, normalizedItem.ComponentType, normalizedItem.CalculationType, normalizedItem.Quantity, normalizedItem.UnitAmount, normalizedItem.Amount).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return utils.Error(c, 500, "Gagal menyimpan slip payroll", err.Error())
	}

	return utils.Success(c, 200, "Slip payroll berhasil disimpan", fiber.Map{
		"id":                savedID,
		"base_amount":       baseAmount,
		"allowances_amount": allowancesAmount,
		"deductions_amount": deductionsAmount,
		"total_amount":      totalAmount,
		"status":            status,
	})
}

func (a *AppContext) GenerateTeacherPayrolls(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var body struct {
		Period string `json:"period"`
	}
	_ = c.BodyParser(&body)
	period, err := parsePayrollPeriod(body.Period)
	if err != nil {
		return utils.Error(c, 400, "Periode payroll tidak valid")
	}
	settings, err := a.ensurePayrollSettings(schoolID)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat setting payroll", err.Error())
	}
	components, err := a.loadPayrollComponents(schoolID, true)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat komponen payroll", err.Error())
	}
	globalItems := make([]payrollItemInput, 0)
	for _, component := range components {
		if component.AppliesToAll {
			globalItems = append(globalItems, payrollItemFromComponent(component))
		}
	}

	type teacherHoursRow struct {
		TeacherID           uint    `gorm:"column:teacher_id"`
		TeachingHours       float64 `gorm:"column:teaching_hours"`
		ValidAttendanceDays int     `gorm:"column:valid_attendance_days"`
		PreviousPayrollID   *uint   `gorm:"column:previous_payroll_id"`
	}
	teachers := make([]teacherHoursRow, 0)
	if err := a.DB.Raw(`
		WITH teacher_hours AS (
			SELECT
				u.id AS teacher_id,
				COALESCE(SUM(ccd.weekly_hours), 0)::numeric(10,2) AS weekly_hours
			FROM users u
			LEFT JOIN curriculum_teacher_loads ctl
				ON ctl.school_id = u.school_id AND ctl.teacher_id = u.id
			LEFT JOIN curriculum_class_distributions ccd
				ON ccd.school_id = ctl.school_id AND ccd.curriculum_teacher_load_id = ctl.id
			WHERE u.school_id = ? AND u.role = 'GURU'
			GROUP BY u.id
		),
		valid_attendance AS (
			SELECT
				a.user_id AS teacher_id,
				COUNT(DISTINCT a.attendance_date)::int AS valid_attendance_days
			FROM attendance a
			INNER JOIN users u ON u.id = a.user_id AND u.school_id = ? AND u.role = 'GURU'
			WHERE a.attendance_date >= ? AND a.attendance_date < (?::date + INTERVAL '1 month')
			  AND a.clock_in IS NOT NULL
			  AND a.clock_out IS NOT NULL
			GROUP BY a.user_id
		),
		previous_payroll AS (
			SELECT DISTINCT ON (teacher_id)
				teacher_id,
				id AS previous_payroll_id
			FROM teacher_payrolls
			WHERE school_id = ? AND period_month < ?
			ORDER BY teacher_id, period_month DESC, id DESC
		)
		SELECT
			th.teacher_id,
			(th.weekly_hours * ?)::numeric(10,2) AS teaching_hours,
			COALESCE(va.valid_attendance_days, 0) AS valid_attendance_days,
			pp.previous_payroll_id
		FROM teacher_hours th
		LEFT JOIN valid_attendance va ON va.teacher_id = th.teacher_id
		LEFT JOIN previous_payroll pp ON pp.teacher_id = th.teacher_id
		ORDER BY th.teacher_id ASC
	`, schoolID, schoolID, period, period, schoolID, period, settings.TeachingHoursMultiplier).Scan(&teachers).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat jam guru", err.Error())
	}

	previousPayrollIDs := make([]uint, 0)
	for _, teacher := range teachers {
		if teacher.PreviousPayrollID != nil && *teacher.PreviousPayrollID > 0 {
			previousPayrollIDs = append(previousPayrollIDs, *teacher.PreviousPayrollID)
		}
	}
	previousItemsByPayroll, err := a.loadPayrollItemsByPayrollIDs(previousPayrollIDs)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat item payroll bulan sebelumnya", err.Error())
	}

	generated := 0
	skippedPaid := 0
	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		for _, teacher := range teachers {
			items := clonePayrollItemInputs(globalItems)
			if teacher.PreviousPayrollID != nil && *teacher.PreviousPayrollID > 0 {
				previousItems := payrollItemRowsToInputs(previousItemsByPayroll[*teacher.PreviousPayrollID])
				if len(previousItems) > 0 {
					items = previousItems
				}
			}
			applyTransportAttendanceToPayrollItemInputs(items, teacher.ValidAttendanceDays)
			baseAmount, allowancesAmount, deductionsAmount, totalAmount := calculatePayrollTotals(settings.HourlyRate, teacher.TeachingHours, items)
			var row struct {
				ID uint `gorm:"column:id"`
			}
			if err := tx.Raw(`
				INSERT INTO teacher_payrolls (
					school_id, teacher_id, period_month, hourly_rate, teaching_hours,
					base_amount, allowances_amount, deductions_amount, total_amount,
					status, note, created_by, updated_by, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'DRAFT', 'Generated dari komponen global', ?, ?, NOW(), NOW())
				ON CONFLICT (school_id, teacher_id, period_month)
				DO UPDATE SET
					hourly_rate = EXCLUDED.hourly_rate,
					teaching_hours = EXCLUDED.teaching_hours,
					base_amount = EXCLUDED.base_amount,
					allowances_amount = EXCLUDED.allowances_amount,
					deductions_amount = EXCLUDED.deductions_amount,
					total_amount = EXCLUDED.total_amount,
					status = EXCLUDED.status,
					note = EXCLUDED.note,
					paid_at = NULL,
					updated_by = EXCLUDED.updated_by,
					updated_at = NOW()
				WHERE teacher_payrolls.status <> 'PAID'
				RETURNING id
			`, schoolID, teacher.TeacherID, period, settings.HourlyRate, teacher.TeachingHours, baseAmount, allowancesAmount, deductionsAmount, totalAmount, userID, userID).Scan(&row).Error; err != nil {
				return err
			}
			if row.ID == 0 {
				skippedPaid++
				continue
			}
			if err := tx.Exec(`DELETE FROM teacher_payroll_items WHERE payroll_id = ?`, row.ID).Error; err != nil {
				return err
			}
			for _, item := range items {
				normalizedItem, ok := normalizePayrollItemInput(item)
				if !ok {
					continue
				}
				if err := tx.Exec(`
					INSERT INTO teacher_payroll_items (payroll_id, component_id, name, component_type, calculation_type, quantity, unit_amount, amount, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
				`, row.ID, normalizedItem.ComponentID, normalizedItem.Name, normalizedItem.ComponentType, normalizedItem.CalculationType, normalizedItem.Quantity, normalizedItem.UnitAmount, normalizedItem.Amount).Error; err != nil {
					return err
				}
			}
			generated++
		}
		return nil
	}); err != nil {
		return utils.Error(c, 500, "Gagal generate slip payroll", err.Error())
	}

	return utils.Success(c, 200, "Slip payroll berhasil digenerate", fiber.Map{
		"generated":    generated,
		"skipped_paid": skippedPaid,
	})
}

func (a *AppContext) MarkTeacherPayrollPaid(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	id := c.Params("id")
	var row struct {
		ID            uint      `gorm:"column:id"`
		TeacherID     uint      `gorm:"column:teacher_id"`
		PeriodMonth   time.Time `gorm:"column:period_month"`
		HourlyRate    int64     `gorm:"column:hourly_rate"`
		TeachingHours float64   `gorm:"column:teaching_hours"`
		Status        string    `gorm:"column:status"`
	}
	if err := a.DB.Raw(`
		SELECT id, teacher_id, period_month, hourly_rate, teaching_hours, status
		FROM teacher_payrolls
		WHERE id = ? AND school_id = ?
	`, id, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat slip payroll", err.Error())
	}
	if row.ID == 0 {
		return utils.Error(c, 404, "Slip payroll tidak ditemukan")
	}
	if row.Status != "PAID" {
		itemsByPayroll, err := a.loadPayrollItemsByPayrollIDs([]uint{row.ID})
		if err != nil {
			return utils.Error(c, 500, "Gagal memuat item payroll", err.Error())
		}
		items := payrollItemRowsToInputs(clonePayrollItemRows(itemsByPayroll[row.ID]))
		validAttendanceDays, err := a.countValidTeacherAttendanceDays(schoolID, row.TeacherID, row.PeriodMonth)
		if err != nil {
			return utils.Error(c, 500, "Gagal menghitung absensi valid", err.Error())
		}
		applyTransportAttendanceToPayrollItemInputs(items, validAttendanceDays)
		baseAmount, allowancesAmount, deductionsAmount, totalAmount := calculatePayrollTotals(row.HourlyRate, row.TeachingHours, items)
		if err := a.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`
				UPDATE teacher_payrolls
				SET base_amount = ?, allowances_amount = ?, deductions_amount = ?, total_amount = ?,
					status = 'PAID', paid_at = COALESCE(paid_at, NOW()), updated_by = ?, updated_at = NOW()
				WHERE id = ? AND school_id = ?
			`, baseAmount, allowancesAmount, deductionsAmount, totalAmount, userID, row.ID, schoolID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`DELETE FROM teacher_payroll_items WHERE payroll_id = ?`, row.ID).Error; err != nil {
				return err
			}
			for _, item := range items {
				normalizedItem, ok := normalizePayrollItemInput(item)
				if !ok {
					continue
				}
				if err := tx.Exec(`
					INSERT INTO teacher_payroll_items (payroll_id, component_id, name, component_type, calculation_type, quantity, unit_amount, amount, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
				`, row.ID, normalizedItem.ComponentID, normalizedItem.Name, normalizedItem.ComponentType, normalizedItem.CalculationType, normalizedItem.Quantity, normalizedItem.UnitAmount, normalizedItem.Amount).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return utils.Error(c, 500, "Gagal menandai payroll dibayar", err.Error())
		}
		return utils.Success(c, 200, "Payroll berhasil ditandai dibayar", fiber.Map{"id": row.ID})
	}
	if err := a.DB.Exec(`
		UPDATE teacher_payrolls
		SET status = 'PAID', paid_at = COALESCE(paid_at, NOW()), updated_by = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
	`, userID, row.ID, schoolID).Error; err != nil {
		return utils.Error(c, 500, "Gagal menandai payroll dibayar", err.Error())
	}
	return utils.Success(c, 200, "Payroll berhasil ditandai dibayar", fiber.Map{"id": row.ID})
}

func (a *AppContext) MarkTeacherPayrollUnpaid(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	id := c.Params("id")
	var row struct {
		ID uint `gorm:"column:id"`
	}
	if err := a.DB.Raw(`
		SELECT id
		FROM teacher_payrolls
		WHERE id = ? AND school_id = ?
	`, id, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat slip payroll", err.Error())
	}
	if row.ID == 0 {
		return utils.Error(c, 404, "Slip payroll tidak ditemukan")
	}
	if err := a.DB.Exec(`
		UPDATE teacher_payrolls
		SET status = 'DRAFT', paid_at = NULL, updated_by = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
	`, userID, row.ID, schoolID).Error; err != nil {
		return utils.Error(c, 500, "Gagal membatalkan status lunas", err.Error())
	}
	return utils.Success(c, 200, "Payroll berhasil ditandai belum lunas", fiber.Map{"id": row.ID})
}

func (a *AppContext) ensurePayrollSettings(schoolID uint) (payrollSettingsRow, error) {
	if err := a.DB.Exec(`
		INSERT INTO payroll_settings (school_id, hourly_rate, lesson_minutes, teaching_hours_multiplier, created_at, updated_at)
		VALUES (?, 40000, 45, 4, NOW(), NOW())
		ON CONFLICT (school_id) DO NOTHING
	`, schoolID).Error; err != nil {
		return payrollSettingsRow{}, err
	}
	var settings payrollSettingsRow
	err := a.DB.Raw(`
		SELECT id, school_id, hourly_rate, lesson_minutes, teaching_hours_multiplier, COALESCE(notes, '') AS notes
		FROM payroll_settings
		WHERE school_id = ?
	`, schoolID).Scan(&settings).Error
	return settings, err
}

func (a *AppContext) loadPayrollComponents(schoolID uint, activeOnly bool) ([]payrollComponentRow, error) {
	rows := make([]payrollComponentRow, 0)
	query := `
		SELECT id, name, component_type, calculation_type, default_amount, default_quantity, applies_to_all, is_active
		FROM payroll_components
		WHERE school_id = ?
	`
	args := []interface{}{schoolID}
	if activeOnly {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY is_active DESC, component_type ASC, name ASC`
	err := a.DB.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (a *AppContext) loadPayrollItemsByPayrollIDs(payrollIDs []uint) (map[uint][]payrollItemRow, error) {
	result := map[uint][]payrollItemRow{}
	if len(payrollIDs) == 0 {
		return result, nil
	}
	rows := make([]payrollItemRow, 0)
	if err := a.DB.Raw(`
		SELECT id, payroll_id, component_id, name, component_type, calculation_type, quantity, unit_amount, amount
		FROM teacher_payroll_items
		WHERE payroll_id IN ?
		ORDER BY id ASC
	`, payrollIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.PayrollID] = append(result[row.PayrollID], row)
	}
	return result, nil
}

func (a *AppContext) countValidTeacherAttendanceDays(schoolID uint, teacherID uint, period time.Time) (int, error) {
	var row struct {
		ValidAttendanceDays int `gorm:"column:valid_attendance_days"`
	}
	err := a.DB.Raw(`
		SELECT COUNT(DISTINCT a.attendance_date)::int AS valid_attendance_days
		FROM attendance a
		INNER JOIN users u ON u.id = a.user_id AND u.school_id = ? AND u.role = 'GURU'
		WHERE a.user_id = ?
		  AND a.attendance_date >= ?
		  AND a.attendance_date < (?::date + INTERVAL '1 month')
		  AND a.clock_in IS NOT NULL
		  AND a.clock_out IS NOT NULL
	`, schoolID, teacherID, period, period).Scan(&row).Error
	return row.ValidAttendanceDays, err
}

func parsePayrollPeriod(value string) (time.Time, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local), nil
	}
	if parsed, err := time.Parse("2006-01", raw); err == nil {
		return time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.Local), nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.Local), nil
}

func normalizePayrollComponentType(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "DEDUCTION" || normalized == "POTONGAN" {
		return "DEDUCTION"
	}
	return "ALLOWANCE"
}

func normalizePayrollCalculationType(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "DAILY", "HARIAN":
		return "DAILY"
	default:
		return "FIXED"
	}
}

func normalizePayrollStatus(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "PAID" || normalized == "DIBAYAR" {
		return "PAID"
	}
	return "DRAFT"
}

func calculatePayrollTotals(hourlyRate int64, teachingHours float64, items []payrollItemInput) (int64, int64, int64, int64) {
	baseAmount := int64(math.Round(float64(hourlyRate) * teachingHours))
	var allowancesAmount int64
	var deductionsAmount int64
	for _, item := range items {
		normalizedItem, ok := normalizePayrollItemInput(item)
		if !ok {
			continue
		}
		if normalizedItem.ComponentType == "DEDUCTION" {
			deductionsAmount += normalizedItem.Amount
		} else {
			allowancesAmount += normalizedItem.Amount
		}
	}
	totalAmount := baseAmount + allowancesAmount - deductionsAmount
	return baseAmount, allowancesAmount, deductionsAmount, totalAmount
}

func normalizePayrollItemInput(item payrollItemInput) (payrollItemInput, bool) {
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		return item, false
	}
	item.ComponentType = normalizePayrollComponentType(item.ComponentType)
	item.CalculationType = normalizePayrollCalculationType(item.CalculationType)
	if item.Quantity < 0 {
		item.Quantity = 0
	}
	if item.UnitAmount <= 0 {
		item.UnitAmount = item.Amount
	}
	if item.UnitAmount < 0 {
		item.UnitAmount = -item.UnitAmount
	}
	if item.Amount <= 0 {
		item.Amount = int64(math.Round(float64(item.UnitAmount) * item.Quantity))
	}
	if item.Amount < 0 {
		item.Amount = -item.Amount
	}
	return item, true
}

func payrollItemFromComponent(component payrollComponentRow) payrollItemInput {
	quantity := component.DefaultQuantity
	if quantity <= 0 {
		quantity = 1
	}
	amount := int64(math.Round(float64(component.DefaultAmount) * quantity))
	return payrollItemInput{
		ComponentID:     &component.ID,
		Name:            component.Name,
		ComponentType:   component.ComponentType,
		CalculationType: component.CalculationType,
		Quantity:        quantity,
		UnitAmount:      component.DefaultAmount,
		Amount:          amount,
	}
}

func clonePayrollItemInputs(items []payrollItemInput) []payrollItemInput {
	result := make([]payrollItemInput, len(items))
	copy(result, items)
	return result
}

func clonePayrollItemRows(items []payrollItemRow) []payrollItemRow {
	result := make([]payrollItemRow, len(items))
	copy(result, items)
	return result
}

func payrollItemRowsToInputs(items []payrollItemRow) []payrollItemInput {
	result := make([]payrollItemInput, 0, len(items))
	for _, item := range items {
		result = append(result, payrollItemInput{
			ComponentID:     item.ComponentID,
			Name:            item.Name,
			ComponentType:   item.ComponentType,
			CalculationType: item.CalculationType,
			Quantity:        item.Quantity,
			UnitAmount:      item.UnitAmount,
			Amount:          item.Amount,
		})
	}
	return result
}

func isTransportPayrollItem(name, calculationType string) bool {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	return normalizePayrollCalculationType(calculationType) == "DAILY" &&
		(strings.Contains(normalizedName, "transport") || strings.Contains(normalizedName, "transpor"))
}

func transportAmount(unitAmount int64, quantity float64) int64 {
	if quantity < 0 {
		quantity = 0
	}
	return int64(math.Round(float64(unitAmount) * quantity))
}

func applyTransportAttendanceToPayrollItemInputs(items []payrollItemInput, validAttendanceDays int) {
	if validAttendanceDays < 0 {
		validAttendanceDays = 0
	}
	for index := range items {
		if !isTransportPayrollItem(items[index].Name, items[index].CalculationType) {
			continue
		}
		items[index].CalculationType = "DAILY"
		items[index].Quantity = float64(validAttendanceDays)
		items[index].Amount = transportAmount(items[index].UnitAmount, items[index].Quantity)
	}
}

func applyTransportAttendanceToPayrollItemRows(items []payrollItemRow, validAttendanceDays int) {
	if validAttendanceDays < 0 {
		validAttendanceDays = 0
	}
	for index := range items {
		if !isTransportPayrollItem(items[index].Name, items[index].CalculationType) {
			continue
		}
		items[index].CalculationType = "DAILY"
		items[index].Quantity = float64(validAttendanceDays)
		items[index].Amount = transportAmount(items[index].UnitAmount, items[index].Quantity)
	}
}

func int64PtrValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func stringPtrValue(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}
