package controllers

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

const defaultInventoryPageSize = 10
const maxInventoryPageSize = 100

func parseInventoryPagination(c *fiber.Ctx) (int, int) {
	page := utils.ToInt(c.Query("page"), 1)
	limit := utils.ToInt(c.Query("limit"), defaultInventoryPageSize)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultInventoryPageSize
	}
	if limit > maxInventoryPageSize {
		limit = maxInventoryPageSize
	}
	return page, limit
}

func totalPagesFromCount(total int64, limit int) int {
	if total <= 0 {
		return 1
	}
	pages := int(math.Ceil(float64(total) / float64(limit)))
	if pages < 1 {
		return 1
	}
	return pages
}

func (a *AppContext) GetSarprasDashboard(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	type KV struct {
		Key   string `json:"key"`
		Value int    `json:"value"`
	}

	var overviewRows []KV
	if err := a.DB.Raw(`
		SELECT 'items_total' AS key, COUNT(*)::int AS value FROM inventory_items WHERE school_id = ?
		UNION ALL
		SELECT 'items_active' AS key, COUNT(*)::int AS value FROM inventory_items WHERE school_id = ? AND is_active = true
		UNION ALL
		SELECT 'stock_total' AS key, COALESCE(SUM(total_quantity), 0)::int AS value FROM inventory_items WHERE school_id = ?
		UNION ALL
		SELECT 'stock_available' AS key, COALESCE(SUM(available_quantity), 0)::int AS value FROM inventory_items WHERE school_id = ?
		UNION ALL
		SELECT 'stock_borrowed' AS key, COALESCE(SUM(total_quantity - available_quantity), 0)::int AS value FROM inventory_items WHERE school_id = ?
		UNION ALL
		SELECT 'active_loans' AS key, COUNT(*)::int AS value FROM inventory_loans WHERE school_id = ? AND status = 'BORROWED'
		UNION ALL
		SELECT 'overdue_loans' AS key, COUNT(*)::int AS value FROM inventory_loans WHERE school_id = ? AND status = 'BORROWED' AND due_date IS NOT NULL AND due_date::date < CURRENT_DATE
		UNION ALL
		SELECT 'returned_loans' AS key, COUNT(*)::int AS value FROM inventory_loans WHERE school_id = ? AND status = 'RETURNED'
	`, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID, schoolID).Scan(&overviewRows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat dashboard sarpras", err.Error())
	}

	overview := map[string]int{}
	for _, row := range overviewRows {
		overview[row.Key] = row.Value
	}

	var school map[string]interface{}
	if err := a.DB.Raw(`SELECT id, name FROM schools WHERE id = ?`, schoolID).Scan(&school).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat data sekolah", err.Error())
	}

	var lowStockItems []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			id,
			name,
			COALESCE(code, '-') AS code,
			COALESCE(category, '-') AS category,
			total_quantity,
			available_quantity,
			(total_quantity - available_quantity) AS borrowed_quantity,
			condition_status
		FROM inventory_items
		WHERE school_id = ?
		  AND is_active = true
		ORDER BY available_quantity ASC, name ASC
		LIMIT 8
	`, schoolID).Scan(&lowStockItems).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat item sarpras", err.Error())
	}

	var recentLoans []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			il.id,
			il.quantity,
			il.status,
			il.borrowed_at,
			il.due_date,
			il.returned_at,
			ii.name AS item_name,
			COALESCE(ii.code, '-') AS item_code,
			COALESCE(u.full_name, u.username) AS borrower_name,
			u.role AS borrower_role,
			COALESCE(cl.class_name, '-') AS borrower_class_name,
			COALESCE(t.full_name, t.username) AS teacher_name,
			COALESCE(h.full_name, h.username) AS handled_by_name
		FROM inventory_loans il
		INNER JOIN inventory_items ii ON ii.id = il.item_id
		INNER JOIN users u ON u.id = il.borrower_id
		LEFT JOIN class cl ON cl.id = u.class_id
		LEFT JOIN users t ON t.id = il.teacher_id
		LEFT JOIN users h ON h.id = il.handled_by
		WHERE il.school_id = ?
		ORDER BY il.borrowed_at DESC, il.id DESC
		LIMIT 10
	`, schoolID).Scan(&recentLoans).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat riwayat peminjaman", err.Error())
	}

	announcements, err := a.fetchAnnouncementsForSchool(schoolID, "SARPRAS", false, 3)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat pengumuman dashboard", err.Error())
	}

	return utils.Success(c, 200, "Success Get Sarpras Dashboard", fiber.Map{
		"generatedAt":   jakartaNow().Format(time.RFC3339),
		"school":        school,
		"overview":      overview,
		"announcements": announcements,
		"lowStockItems": recentOrEmpty(lowStockItems),
		"recentLoans":   recentOrEmpty(recentLoans),
	})
}

func (a *AppContext) GetInventoryItems(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := strings.ToUpper(strings.TrimSpace(fmt.Sprint(c.Locals("userRole"))))
	includeInactive := c.Query("include_inactive") == "1"
	search := strings.TrimSpace(c.Query("search"))
	page, limit := parseInventoryPagination(c)
	offset := (page - 1) * limit

	query := a.DB.Table("inventory_items ii").
		Select(`
			ii.id,
			ii.school_id,
			ii.name,
			ii.code,
			ii.category,
			ii.description,
			ii.condition_status,
			ii.total_quantity,
			ii.available_quantity,
			(ii.total_quantity - ii.available_quantity) AS borrowed_quantity,
			ii.is_active,
			ii.created_by,
			ii.updated_by,
			ii.created_at,
			ii.updated_at
		`).
		Where("ii.school_id = ?", schoolID)
	if !(includeInactive && role != "SISWA") {
		query = query.Where("ii.is_active = true")
	}
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(`
			LOWER(COALESCE(ii.name, '')) LIKE ? OR
			LOWER(COALESCE(ii.code, '')) LIKE ? OR
			LOWER(COALESCE(ii.category, '')) LIKE ? OR
			LOWER(COALESCE(ii.description, '')) LIKE ? OR
			LOWER(COALESCE(ii.condition_status, '')) LIKE ?
		`, pattern, pattern, pattern, pattern, pattern)
	}

	var items []map[string]interface{}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat daftar barang", err.Error())
	}
	if err := query.Order("ii.is_active DESC, ii.name ASC").Limit(limit).Offset(offset).Scan(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat daftar barang", err.Error())
	}

	return utils.Success(c, 200, "Success Get Inventory Items", fiber.Map{
		"items":      recentOrEmpty(items),
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPagesFromCount(total, limit),
	})
}

func (a *AppContext) CreateInventoryItem(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)

	var body struct {
		Name            string `json:"name"`
		Code            string `json:"code"`
		Category        string `json:"category"`
		Description     string `json:"description"`
		ConditionStatus string `json:"condition_status"`
		TotalQuantity   int    `json:"total_quantity"`
		IsActive        *bool  `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama barang wajib diisi")
	}
	totalQuantity := body.TotalQuantity
	if totalQuantity <= 0 {
		return utils.Error(c, 400, "Jumlah stok minimal 1")
	}

	item := models.InventoryItem{
		SchoolID:          schoolID,
		Name:              name,
		Code:              stringPtrIfNotBlank(body.Code),
		Category:          stringPtrIfNotBlank(body.Category),
		Description:       stringPtrIfNotBlank(body.Description),
		ConditionStatus:   normalizeConditionStatus(body.ConditionStatus),
		TotalQuantity:     totalQuantity,
		AvailableQuantity: totalQuantity,
		IsActive:          body.IsActive == nil || *body.IsActive,
		CreatedBy:         &userID,
		UpdatedBy:         &userID,
	}

	if err := a.DB.Create(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat barang", err.Error())
	}

	return a.respondInventoryItem(c, item.ID, 201, "Barang berhasil dibuat")
}

func (a *AppContext) UpdateInventoryItem(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	id := c.Params("id")

	var current models.InventoryItem
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Barang tidak ditemukan")
	}

	var body struct {
		Name            *string `json:"name"`
		Code            *string `json:"code"`
		Category        *string `json:"category"`
		Description     *string `json:"description"`
		ConditionStatus *string `json:"condition_status"`
		TotalQuantity   *int    `json:"total_quantity"`
		IsActive        *bool   `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	updates := map[string]interface{}{
		"updated_by": userID,
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return utils.Error(c, 400, "Nama barang wajib diisi")
		}
		updates["name"] = name
	}
	if body.Code != nil {
		updates["code"] = valueIfNotBlank(*body.Code)
	}
	if body.Category != nil {
		updates["category"] = valueIfNotBlank(*body.Category)
	}
	if body.Description != nil {
		updates["description"] = valueIfNotBlank(*body.Description)
	}
	if body.ConditionStatus != nil {
		updates["condition_status"] = normalizeConditionStatus(*body.ConditionStatus)
	}
	if body.IsActive != nil {
		updates["is_active"] = *body.IsActive
	}
	if body.TotalQuantity != nil {
		nextTotal := *body.TotalQuantity
		if nextTotal <= 0 {
			return utils.Error(c, 400, "Jumlah stok minimal 1")
		}
		borrowed := current.TotalQuantity - current.AvailableQuantity
		if nextTotal < borrowed {
			return utils.Error(c, 400, "Jumlah stok tidak boleh lebih kecil dari jumlah yang sedang dipinjam")
		}
		updates["total_quantity"] = nextTotal
		updates["available_quantity"] = nextTotal - borrowed
	}

	if len(updates) == 1 {
		return utils.Error(c, 400, "Tidak ada perubahan barang")
	}

	if err := a.DB.Model(&models.InventoryItem{}).Where("id = ? AND school_id = ?", current.ID, schoolID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui barang", err.Error())
	}

	return a.respondInventoryItem(c, current.ID, 200, "Barang berhasil diperbarui")
}

func (a *AppContext) DeleteInventoryItem(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")

	var current models.InventoryItem
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Barang tidak ditemukan")
	}

	var activeLoans int64
	if err := a.DB.Table("inventory_loans").
		Where("school_id = ? AND item_id = ? AND status = 'BORROWED'", schoolID, current.ID).
		Count(&activeLoans).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa peminjaman", err.Error())
	}
	if activeLoans > 0 {
		return utils.Error(c, 400, "Barang masih sedang dipinjam")
	}

	if err := a.DB.Model(&models.InventoryItem{}).
		Where("id = ? AND school_id = ?", current.ID, schoolID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_by": c.Locals("userID").(uint),
		}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menonaktifkan barang", err.Error())
	}

	return utils.Success(c, 200, "Barang berhasil dinonaktifkan", fiber.Map{
		"id": current.ID,
	})
}

func (a *AppContext) GetInventoryLoans(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := strings.ToUpper(strings.TrimSpace(fmt.Sprint(c.Locals("userRole"))))
	currentUserID := c.Locals("userID").(uint)
	page, limit := parseInventoryPagination(c)
	offset := (page - 1) * limit

	query := a.DB.Table("inventory_loans il").
		Select(`
			il.id,
			il.school_id,
			il.item_id,
			il.borrower_id,
			il.quantity,
			il.borrowed_at,
			il.due_date,
			il.returned_at,
			il.status,
			il.notes,
			il.teacher_id,
			il.handled_by,
			il.created_at,
			il.updated_at,
			ii.name AS item_name,
			COALESCE(ii.code, '-') AS item_code,
			COALESCE(b.full_name, b.username) AS borrower_name,
			COALESCE(b.class_id::text, '-') AS borrower_class_id,
			b.role AS borrower_role,
			COALESCE(t.full_name, t.username) AS teacher_name,
			COALESCE(h.full_name, h.username) AS handled_by_name
		`).
		Joins("INNER JOIN inventory_items ii ON ii.id = il.item_id").
		Joins("INNER JOIN users b ON b.id = il.borrower_id").
		Joins("LEFT JOIN users t ON t.id = il.teacher_id").
		Joins("LEFT JOIN users h ON h.id = il.handled_by").
		Where("il.school_id = ?", schoolID)

	if role == "SISWA" {
		query = query.Where("il.borrower_id = ?", currentUserID)
	} else if role != "ADMIN" && role != "SARPRAS" && role != "SUPER_ADMIN" {
		query = query.Where("1 = 0")
	}

	var loans []map[string]interface{}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat riwayat peminjaman", err.Error())
	}
	if err := query.Order("il.borrowed_at DESC, il.id DESC").Limit(limit).Offset(offset).Scan(&loans).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat riwayat peminjaman", err.Error())
	}

	return utils.Success(c, 200, "Success Get Inventory Loans", fiber.Map{
		"items":      recentOrEmpty(loans),
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPagesFromCount(total, limit),
	})
}

func (a *AppContext) GetInventoryActiveLoans(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	page, limit := parseInventoryPagination(c)
	offset := (page - 1) * limit

	var total int64
	countQuery := a.DB.Table("inventory_loans il").Where("il.school_id = ? AND il.status = 'BORROWED'", schoolID)
	if err := countQuery.Count(&total).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pinjaman aktif", err.Error())
	}

	var loans []map[string]interface{}
	if err := a.DB.Table("inventory_loans il").
		Select(`
			il.id,
			il.quantity,
			il.borrowed_at,
			il.due_date,
			il.status,
			il.teacher_id,
			ii.name AS item_name,
			COALESCE(ii.code, '-') AS item_code,
			COALESCE(b.full_name, b.username) AS borrower_name,
			COALESCE(t.full_name, t.username) AS teacher_name
		`).
		Joins("INNER JOIN inventory_items ii ON ii.id = il.item_id").
		Joins("INNER JOIN users b ON b.id = il.borrower_id").
		Joins("LEFT JOIN users t ON t.id = il.teacher_id").
		Where("il.school_id = ? AND il.status = 'BORROWED'", schoolID).
		Order("il.borrowed_at DESC, il.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&loans).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pinjaman aktif", err.Error())
	}

	return utils.Success(c, 200, "Success Get Inventory Active Loans", fiber.Map{
		"items":      recentOrEmpty(loans),
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": totalPagesFromCount(total, limit),
	})
}

func (a *AppContext) CreateInventoryLoan(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	borrowerID := c.Locals("userID").(uint)

	var body struct {
		ItemID         uint   `json:"item_id"`
		Quantity       int    `json:"quantity"`
		TeacherID      *uint  `json:"teacher_id"`
		ScheduleSlotID *uint  `json:"schedule_slot_id"`
		Notes          string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	if body.ItemID == 0 {
		return utils.Error(c, 400, "Barang wajib dipilih")
	}

	quantity := body.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	teacherID := body.TeacherID
	if teacherID != nil && *teacherID == 0 {
		teacherID = nil
	}
	if teacherID == nil {
		return utils.Error(c, 400, "Guru yang mengajar wajib dipilih")
	}
	if body.ScheduleSlotID == nil || *body.ScheduleSlotID == 0 {
		return utils.Error(c, 400, "Jam kembali wajib dipilih")
	}

	var created models.InventoryLoan
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		var borrower struct {
			ClassID *uint `gorm:"column:class_id"`
		}
		if err := tx.Raw(`SELECT class_id FROM users WHERE id = ? AND school_id = ? AND role = 'SISWA'`, borrowerID, schoolID).Scan(&borrower).Error; err != nil {
			return err
		}
		dueDate, err := a.resolveInventoryLoanDueDateTx(tx, schoolID, borrower.ClassID, teacherID, body.ScheduleSlotID)
		if err != nil {
			return err
		}
		var teacher models.User
		if err := tx.Where("id = ? AND school_id = ? AND role = 'GURU'", *teacherID, schoolID).First(&teacher).Error; err != nil {
			return fiber.NewError(400, "Guru yang dipilih tidak valid")
		}
		var item models.InventoryItem
		if err := tx.Where("id = ? AND school_id = ? AND is_active = true", body.ItemID, schoolID).First(&item).Error; err != nil {
			return err
		}
		if item.AvailableQuantity < quantity {
			return fiber.NewError(400, "Stok barang tidak cukup")
		}

		loan := models.InventoryLoan{
			SchoolID:   schoolID,
			ItemID:     item.ID,
			BorrowerID: borrowerID,
			TeacherID:  teacherID,
			Quantity:   quantity,
			BorrowedAt: time.Now().UTC(),
			DueDate:    dueDate,
			Status:     "BORROWED",
			Notes:      stringPtrIfNotBlank(body.Notes),
		}

		if err := tx.Create(&loan).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.InventoryItem{}).
			Where("id = ? AND school_id = ?", item.ID, schoolID).
			Update("available_quantity", gorm.Expr("available_quantity - ?", quantity)).Error; err != nil {
			return err
		}

		created = loan
		return nil
	})
	if err != nil {
		return respondInventoryError(c, err, "Gagal meminjam barang")
	}

	return a.respondInventoryLoan(c, created.ID, 201, "Barang berhasil dipinjam")
}

func (a *AppContext) resolveInventoryLoanDueDateTx(tx *gorm.DB, schoolID uint, classID *uint, teacherID *uint, scheduleSlotID *uint) (*time.Time, error) {
	location := jakartaLocation()
	now := time.Now().In(location)
	weekdayToOrder := func(day time.Weekday) int {
		switch day {
		case time.Monday:
			return 1
		case time.Tuesday:
			return 2
		case time.Wednesday:
			return 3
		case time.Thursday:
			return 4
		case time.Friday:
			return 5
		case time.Saturday:
			return 6
		default:
			return 7
		}
	}
	parseClock := func(value string) (int, int, bool) {
		value = strings.TrimSpace(value)
		if value == "" {
			return 0, 0, false
		}
		parts := strings.Split(value, ":")
		if len(parts) < 2 {
			return 0, 0, false
		}
		hour := utils.ToInt(parts[0], -1)
		minute := utils.ToInt(parts[1], -1)
		if hour < 0 || minute < 0 {
			return 0, 0, false
		}
		return hour, minute, true
	}
	if classID == nil || *classID == 0 {
		due := now.UTC().Add(7 * 24 * time.Hour)
		return &due, nil
	}

	type scheduleRow struct {
		ID           uint   `gorm:"column:id"`
		DayOrder     int    `gorm:"column:day_order"`
		SessionOrder int    `gorm:"column:session_order"`
		StartTime    string `gorm:"column:start_time"`
		EndTime      string `gorm:"column:end_time"`
	}

	var rows []scheduleRow
	query := tx.Table("curriculum_schedule_entries cse").
		Select(`
			slot.id,
			slot.day_order,
			slot.session_order,
			slot.start_time,
			slot.end_time
		`).
		Joins("INNER JOIN curriculum_schedule_slots slot ON slot.id = cse.schedule_slot_id").
		Where("cse.school_id = ? AND cse.class_id = ?", schoolID, *classID)
	if teacherID != nil && *teacherID != 0 {
		query = query.Where("cse.teacher_id = ?", *teacherID)
	}
	if err := query.Order("slot.day_order ASC, slot.session_order ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 && teacherID != nil && *teacherID != 0 {
		if err := tx.Table("curriculum_schedule_entries cse").
			Select(`
				slot.id,
				slot.day_order,
				slot.session_order,
				slot.start_time,
				slot.end_time
			`).
			Joins("INNER JOIN curriculum_schedule_slots slot ON slot.id = cse.schedule_slot_id").
			Where("cse.school_id = ? AND cse.class_id = ?", schoolID, *classID).
			Order("slot.day_order ASC, slot.session_order ASC").
			Scan(&rows).Error; err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		due := now.UTC().Add(7 * 24 * time.Hour)
		return &due, nil
	}

	if scheduleSlotID != nil && *scheduleSlotID != 0 {
		for _, row := range rows {
			if row.ID == *scheduleSlotID {
				if endHour, endMinute, ok := parseClock(row.EndTime); ok {
					dueLocal := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMinute, 0, 0, location)
					dayDiff := row.DayOrder - weekdayToOrder(now.Weekday())
					if dayDiff < 0 {
						dayDiff += 7
					}
					dueLocal = dueLocal.AddDate(0, 0, dayDiff)
					due := dueLocal.UTC()
					return &due, nil
				}
			}
		}
	}

	todayOrder := weekdayToOrder(now.Weekday())
	nowMinute := now.Hour()*60 + now.Minute()
	type candidate struct {
		due   time.Time
		score int
	}
	candidates := make([]candidate, 0, len(rows))
	for _, row := range rows {
		dayOrder := row.DayOrder
		endHour, endMinute, ok := parseClock(row.EndTime)
		if !ok {
			continue
		}

		dayDiff := dayOrder - todayOrder
		if dayDiff < 0 {
			dayDiff += 7
		}
		if dayDiff == 0 {
			startHour, startMinute, okStart := parseClock(row.StartTime)
			if okStart {
				startMinuteTotal := startHour*60 + startMinute
				endMinuteTotal := endHour*60 + endMinute
				if nowMinute > endMinuteTotal {
					dayDiff = 7
				} else if nowMinute < startMinuteTotal {
					dayDiff = 0
				}
			}
		}

		dueLocal := time.Date(now.Year(), now.Month(), now.Day(), endHour, endMinute, 0, 0, location).AddDate(0, 0, dayDiff)
		candidates = append(candidates, candidate{due: dueLocal, score: dayDiff*1000 + row.DayOrder*10 + row.SessionOrder})
	}
	if len(candidates) == 0 {
		due := now.UTC().Add(7 * 24 * time.Hour)
		return &due, nil
	}

	best := candidates[0]
	for _, cand := range candidates[1:] {
		if cand.score < best.score {
			best = cand
		}
	}
	due := best.due.UTC()
	return &due, nil
}

func (a *AppContext) ReturnInventoryLoan(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	role := strings.ToUpper(strings.TrimSpace(fmt.Sprint(c.Locals("userRole"))))
	id := c.Params("id")

	var current models.InventoryLoan
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Data peminjaman tidak ditemukan")
	}
	status := strings.ToUpper(strings.TrimSpace(current.Status))
	if role == "SISWA" {
		if current.BorrowerID != userID {
			return utils.Error(c, 403, "Forbidden: hanya bisa mengajukan pengembalian pinjaman sendiri")
		}
		if status != "BORROWED" {
			return utils.Error(c, 400, "Pengembalian sudah diajukan atau barang sudah selesai diproses")
		}
		if err := a.DB.Model(&models.InventoryLoan{}).
			Where("id = ? AND school_id = ?", current.ID, schoolID).
			Updates(map[string]interface{}{
				"status":     "RETURN_REQUESTED",
				"handled_by": nil,
			}).Error; err != nil {
			return utils.Error(c, 500, "Gagal mengajukan pengembalian", err.Error())
		}
		return a.respondInventoryLoan(c, current.ID, 200, "Pengembalian berhasil diajukan")
	}

	if role != "ADMIN" && role != "SARPRAS" {
		return utils.Error(c, 403, "Forbidden: insufficient privileges")
	}
	if status != "RETURN_REQUESTED" {
		return utils.Error(c, 400, "Pengembalian belum diajukan siswa")
	}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		if err := tx.Model(&models.InventoryLoan{}).
			Where("id = ? AND school_id = ?", current.ID, schoolID).
			Updates(map[string]interface{}{
				"status":      "RETURNED",
				"returned_at": &now,
				"handled_by":  userID,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.InventoryItem{}).
			Where("id = ? AND school_id = ?", current.ItemID, schoolID).
			Update("available_quantity", gorm.Expr("available_quantity + ?", current.Quantity)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal mengembalikan barang", err.Error())
	}

	return a.respondInventoryLoan(c, current.ID, 200, "Barang berhasil dikembalikan")
}

func (a *AppContext) respondInventoryItem(c *fiber.Ctx, itemID uint, code int, message string) error {
	var item map[string]interface{}
	if err := a.DB.Table("inventory_items").
		Select(`
			id,
			school_id,
			name,
			code,
			category,
			description,
			condition_status,
			total_quantity,
			available_quantity,
			(total_quantity - available_quantity) AS borrowed_quantity,
			is_active,
			created_by,
			updated_by,
			created_at,
			updated_at
		`).
		Where("id = ?", itemID).
		Scan(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat data barang", err.Error())
	}
	normalizeJakartaDateTimeFields(item, "created_at", "updated_at")

	return utils.Success(c, code, message, item)
}

func (a *AppContext) respondInventoryLoan(c *fiber.Ctx, loanID uint, code int, message string) error {
	var loan map[string]interface{}
	if err := a.DB.Table("inventory_loans il").
		Select(`
			il.id,
			il.school_id,
			il.item_id,
			il.borrower_id,
			il.quantity,
			il.borrowed_at,
			il.due_date,
			il.returned_at,
			il.status,
			il.notes,
			il.teacher_id,
			il.handled_by,
			il.created_at,
			il.updated_at,
			ii.name AS item_name,
			COALESCE(ii.code, '-') AS item_code,
			COALESCE(b.full_name, b.username) AS borrower_name,
			COALESCE(t.full_name, t.username) AS teacher_name,
			COALESCE(h.full_name, h.username) AS handled_by_name
		`).
		Joins("INNER JOIN inventory_items ii ON ii.id = il.item_id").
		Joins("INNER JOIN users b ON b.id = il.borrower_id").
		Joins("LEFT JOIN users t ON t.id = il.teacher_id").
		Joins("LEFT JOIN users h ON h.id = il.handled_by").
		Where("il.id = ?", loanID).
		Scan(&loan).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat data peminjaman", err.Error())
	}
	normalizeJakartaDateTimeFields(loan, "borrowed_at", "due_date", "returned_at", "created_at", "updated_at")

	return utils.Success(c, code, message, loan)
}

func respondInventoryError(c *fiber.Ctx, err error, fallback string) error {
	if err == nil {
		return nil
	}
	if fiberErr, ok := err.(*fiber.Error); ok {
		return utils.Error(c, fiberErr.Code, fiberErr.Message)
	}
	if err == gorm.ErrRecordNotFound {
		return utils.Error(c, 404, fallback)
	}
	return utils.Error(c, 500, fallback, err.Error())
}

func normalizeConditionStatus(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "BARU", "NEW":
		return "BARU"
	case "BAIK", "GOOD":
		return "BAIK"
	case "RUSAK", "DAMAGED":
		return "RUSAK"
	case "PERBAIKAN", "REPAIR", "PERLU PERBAIKAN":
		return "PERBAIKAN"
	default:
		return "BAIK"
	}
}

func stringPtrIfNotBlank(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func valueIfNotBlank(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
