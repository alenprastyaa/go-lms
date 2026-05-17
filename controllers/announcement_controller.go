package controllers

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

const (
	announcementStatusDraft      = "DRAFT"
	announcementStatusActive     = "ACTIVE"
	announcementStatusInactive   = "INACTIVE"
	announcementTargetAll        = "ALL"
	announcementTargetSuperAdmin = "SUPER_ADMIN"
	announcementTargetAdmin      = "ADMIN"
	announcementTargetGuru       = "GURU"
	announcementTargetSiswa      = "SISWA"
	announcementTargetSarpras    = "SARPRAS"
	announcementTargetKoperasi   = "KOPERASI"
)

type announcementItem struct {
	ID             uint       `json:"id"`
	SchoolID       uint       `json:"school_id"`
	Title          string     `json:"title"`
	Content        string     `json:"content"`
	TargetAudience string     `json:"target_audience"`
	TargetLabel    string     `json:"target_label"`
	Status         string     `json:"status"`
	StatusLabel    string     `json:"status_label"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	PublishedAt    *time.Time `json:"published_at"`
	DeactivatedAt  *time.Time `json:"deactivated_at"`
	CreatedBy      *uint      `json:"created_by"`
	UpdatedBy      *uint      `json:"updated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func announcementTargetLabel(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case announcementTargetAll:
		return "Semua Warga Sekolah"
	case announcementTargetSuperAdmin:
		return "Super Admin"
	case announcementTargetAdmin:
		return "Admin"
	case announcementTargetGuru:
		return "Guru"
	case announcementTargetSiswa:
		return "Siswa"
	case announcementTargetSarpras:
		return "Sarpras"
	case announcementTargetKoperasi:
		return "Koperasi"
	default:
		return "Tidak Dikenal"
	}
}

func announcementStatusLabel(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case announcementStatusDraft:
		return "Draft"
	case announcementStatusActive:
		return "Aktif"
	case announcementStatusInactive:
		return "Nonaktif"
	default:
		return "Tidak Dikenal"
	}
}

func announcementNormalizeTarget(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case announcementTargetAll, announcementTargetSuperAdmin, announcementTargetAdmin, announcementTargetGuru, announcementTargetSiswa, announcementTargetSarpras, announcementTargetKoperasi:
		return normalized, nil
	default:
		return "", fmt.Errorf("Target pengumuman tidak valid")
	}
}

func announcementNormalizeStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case announcementStatusActive:
		return announcementStatusActive
	case announcementStatusInactive:
		return announcementStatusInactive
	default:
		return announcementStatusDraft
	}
}

func announcementVisibleToRole(targetAudience, role string) bool {
	target := strings.ToUpper(strings.TrimSpace(targetAudience))
	role = strings.ToUpper(strings.TrimSpace(role))
	return target == announcementTargetAll || target == role
}

func normalizeAnnouncementItem(item *announcementItem) {
	if item == nil {
		return
	}

	if item.ReviewedAt != nil {
		converted := reinterpretAsJakartaClock(*item.ReviewedAt)
		item.ReviewedAt = &converted
	}
	if item.PublishedAt != nil {
		converted := reinterpretAsJakartaClock(*item.PublishedAt)
		item.PublishedAt = &converted
	}
	if item.DeactivatedAt != nil {
		converted := reinterpretAsJakartaClock(*item.DeactivatedAt)
		item.DeactivatedAt = &converted
	}
	item.CreatedAt = reinterpretAsJakartaClock(item.CreatedAt)
	item.UpdatedAt = reinterpretAsJakartaClock(item.UpdatedAt)
}

func normalizeAnnouncementItems(items []announcementItem) {
	for index := range items {
		normalizeAnnouncementItem(&items[index])
	}
}

func announcementToResponse(item models.SchoolAnnouncement) announcementItem {
	response := announcementItem{
		ID:             item.ID,
		SchoolID:       item.SchoolID,
		Title:          item.Title,
		Content:        item.Content,
		TargetAudience: item.TargetAudience,
		TargetLabel:    announcementTargetLabel(item.TargetAudience),
		Status:         item.Status,
		StatusLabel:    announcementStatusLabel(item.Status),
		ReviewedAt:     item.ReviewedAt,
		PublishedAt:    item.PublishedAt,
		DeactivatedAt:  item.DeactivatedAt,
		CreatedBy:      item.CreatedBy,
		UpdatedBy:      item.UpdatedBy,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
	normalizeAnnouncementItem(&response)
	return response
}

func announcementListOrder() string {
	return `CASE
		WHEN status = 'ACTIVE' THEN 0
		WHEN status = 'DRAFT' THEN 1
		ELSE 2
	END, COALESCE(published_at, updated_at, created_at) DESC, id DESC`
}

func (a *AppContext) fetchAnnouncementsForSchool(schoolID uint, role string, includeInactive bool, limit int) ([]announcementItem, error) {
	if schoolID == 0 {
		return []announcementItem{}, nil
	}

	query := a.DB.Table("school_announcements sa").
		Select("sa.id, sa.school_id, sa.title, sa.content, sa.target_audience, sa.status, sa.reviewed_at, sa.published_at, sa.deactivated_at, sa.created_by, sa.updated_by, sa.created_at, sa.updated_at").
		Where("sa.school_id = ?", schoolID)

	if !includeInactive {
		query = query.Where("sa.status = ?", announcementStatusActive)
	}

	if strings.TrimSpace(role) != "" {
		query = query.Where("(sa.target_audience = ? OR sa.target_audience = ?)", announcementTargetAll, strings.ToUpper(strings.TrimSpace(role)))
	}

	if limit > 0 {
		query = query.Order(announcementListOrder()).Limit(limit)
	} else {
		query = query.Order(announcementListOrder())
	}

	var rows []models.SchoolAnnouncement
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]announcementItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, announcementToResponse(row))
	}
	normalizeAnnouncementItems(items)
	return items, nil
}

func (a *AppContext) GetSchoolAnnouncements(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var bodyRole = strings.ToUpper(strings.TrimSpace(fmt.Sprint(c.Locals("userRole"))))
	statusFilter := strings.ToUpper(strings.TrimSpace(c.Query("status")))

	query := a.DB.Table("school_announcements sa").
		Select("sa.id, sa.school_id, sa.title, sa.content, sa.target_audience, sa.status, sa.reviewed_at, sa.published_at, sa.deactivated_at, sa.created_by, sa.updated_by, sa.created_at, sa.updated_at").
		Where("sa.school_id = ?", schoolID)

	if statusFilter != "" {
		query = query.Where("sa.status = ?", statusFilter)
	}

	// Admins can inspect every announcement in their school, including drafts and inactive items.
	if bodyRole == "" {
		bodyRole = announcementTargetAdmin
	}

	query = query.Order(announcementListOrder())

	var rows []models.SchoolAnnouncement
	if err := query.Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pengumuman", err.Error())
	}

	items := make([]announcementItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, announcementToResponse(row))
	}
	normalizeAnnouncementItems(items)

	var summaryRows []struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}
	if err := a.DB.Raw(`
		SELECT status, COUNT(*)::int AS count
		FROM school_announcements
		WHERE school_id = ?
		GROUP BY status
	`, schoolID).Scan(&summaryRows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat ringkasan pengumuman", err.Error())
	}

	summary := map[string]int{
		"total":    0,
		"draft":    0,
		"active":   0,
		"inactive": 0,
	}
	for _, row := range summaryRows {
		summary["total"] += row.Count
		switch strings.ToUpper(row.Status) {
		case announcementStatusDraft:
			summary["draft"] = row.Count
		case announcementStatusActive:
			summary["active"] = row.Count
		case announcementStatusInactive:
			summary["inactive"] = row.Count
		}
	}

	return utils.Success(c, 200, "Success Get Announcements", fiber.Map{
		"items":   items,
		"summary": summary,
	})
}

func (a *AppContext) CreateSchoolAnnouncement(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	if schoolID == 0 {
		return utils.Error(c, 400, "School ID wajib tersedia")
	}

	var body struct {
		Title          string `json:"title"`
		Content        string `json:"content"`
		TargetAudience string `json:"target_audience"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Payload pengumuman tidak valid", err.Error())
	}

	title := strings.TrimSpace(body.Title)
	content := strings.TrimSpace(body.Content)
	if title == "" {
		return utils.Error(c, 400, "Judul pengumuman wajib diisi")
	}
	if content == "" {
		return utils.Error(c, 400, "Isi pengumuman wajib diisi")
	}

	targetAudience, err := announcementNormalizeTarget(body.TargetAudience)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	now := jakartaNow()
	item := models.SchoolAnnouncement{
		SchoolID:       schoolID,
		Title:          title,
		Content:        content,
		TargetAudience: targetAudience,
		Status:         announcementStatusDraft,
		CreatedBy:      &userID,
		UpdatedBy:      &userID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := a.DB.Create(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan pengumuman", err.Error())
	}

	return utils.Success(c, 201, "Draft pengumuman berhasil disimpan", fiber.Map{
		"item": announcementToResponse(item),
	})
}

func (a *AppContext) UpdateSchoolAnnouncement(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	announcementIDInt := utils.ToInt(c.Params("id"), 0)
	announcementID := uint(announcementIDInt)
	if schoolID == 0 || announcementIDInt <= 0 {
		return utils.Error(c, 400, "Permintaan pengumuman tidak valid")
	}

	var existing models.SchoolAnnouncement
	if err := a.DB.Where("id = ? AND school_id = ?", announcementID, schoolID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, 404, "Pengumuman tidak ditemukan")
		}
		return utils.Error(c, 500, "Gagal memuat pengumuman", err.Error())
	}

	var body struct {
		Title          string `json:"title"`
		Content        string `json:"content"`
		TargetAudience string `json:"target_audience"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Payload pengumuman tidak valid", err.Error())
	}

	title := strings.TrimSpace(body.Title)
	content := strings.TrimSpace(body.Content)
	if title == "" {
		return utils.Error(c, 400, "Judul pengumuman wajib diisi")
	}
	if content == "" {
		return utils.Error(c, 400, "Isi pengumuman wajib diisi")
	}

	targetAudience, err := announcementNormalizeTarget(body.TargetAudience)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	update := map[string]interface{}{
		"title":           title,
		"content":         content,
		"target_audience": targetAudience,
		"updated_by":      userID,
		"updated_at":      jakartaNow(),
	}
	if err := a.DB.Model(&models.SchoolAnnouncement{}).Where("id = ? AND school_id = ?", announcementID, schoolID).Updates(update).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui pengumuman", err.Error())
	}

	if err := a.DB.Where("id = ? AND school_id = ?", announcementID, schoolID).First(&existing).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pengumuman terbaru", err.Error())
	}

	return utils.Success(c, 200, "Pengumuman berhasil diperbarui", fiber.Map{
		"item": announcementToResponse(existing),
	})
}

func (a *AppContext) PublishSchoolAnnouncement(c *fiber.Ctx) error {
	return a.updateAnnouncementStatus(c, announcementStatusActive)
}

func (a *AppContext) ToggleSchoolAnnouncementStatus(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	announcementIDInt := utils.ToInt(c.Params("id"), 0)
	announcementID := uint(announcementIDInt)
	if schoolID == 0 || announcementIDInt <= 0 {
		return utils.Error(c, 400, "Permintaan pengumuman tidak valid")
	}

	var existing models.SchoolAnnouncement
	if err := a.DB.Where("id = ? AND school_id = ?", announcementID, schoolID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Error(c, 404, "Pengumuman tidak ditemukan")
		}
		return utils.Error(c, 500, "Gagal memuat pengumuman", err.Error())
	}

	if existing.Status == announcementStatusDraft {
		return utils.Error(c, 400, "Draft harus direview dan diposting terlebih dahulu")
	}

	nextStatus := announcementStatusInactive
	if existing.Status == announcementStatusInactive {
		nextStatus = announcementStatusActive
	}

	return a.persistAnnouncementStatus(c, schoolID, announcementID, userID, nextStatus)
}

func (a *AppContext) DeleteSchoolAnnouncement(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	announcementIDInt := utils.ToInt(c.Params("id"), 0)
	announcementID := uint(announcementIDInt)
	if schoolID == 0 || announcementIDInt <= 0 {
		return utils.Error(c, 400, "Permintaan pengumuman tidak valid")
	}

	if err := a.DB.Where("id = ? AND school_id = ?", announcementID, schoolID).Delete(&models.SchoolAnnouncement{}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus pengumuman", err.Error())
	}

	return utils.Success(c, 200, "Pengumuman berhasil dihapus", nil)
}

func (a *AppContext) updateAnnouncementStatus(c *fiber.Ctx, status string) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	announcementIDInt := utils.ToInt(c.Params("id"), 0)
	announcementID := uint(announcementIDInt)
	if schoolID == 0 || announcementIDInt <= 0 {
		return utils.Error(c, 400, "Permintaan pengumuman tidak valid")
	}

	return a.persistAnnouncementStatus(c, schoolID, announcementID, userID, status)
}

func (a *AppContext) persistAnnouncementStatus(c *fiber.Ctx, schoolID, announcementID, userID uint, status string) error {
	now := jakartaNow()
	update := map[string]interface{}{
		"status":     status,
		"updated_by": userID,
		"updated_at": now,
	}
	switch status {
	case announcementStatusActive:
		update["reviewed_at"] = now
		update["published_at"] = now
		update["deactivated_at"] = nil
	case announcementStatusInactive:
		update["deactivated_at"] = now
	}

	if err := a.DB.Model(&models.SchoolAnnouncement{}).
		Where("id = ? AND school_id = ?", announcementID, schoolID).
		Updates(update).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui status pengumuman", err.Error())
	}

	var latest models.SchoolAnnouncement
	if err := a.DB.Where("id = ? AND school_id = ?", announcementID, schoolID).First(&latest).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pengumuman terbaru", err.Error())
	}

	if status == announcementStatusActive {
		go func(item models.SchoolAnnouncement) {
			_ = a.notifyAnnouncementIfActive(item)
		}(latest)
	}

	message := "Status pengumuman berhasil diperbarui"
	if status == announcementStatusActive {
		message = "Pengumuman berhasil diposting"
	}

	return utils.Success(c, 200, message, fiber.Map{
		"item": announcementToResponse(latest),
	})
}

func (a *AppContext) GetDashboardAnnouncements(c *fiber.Ctx) error {
	var schoolID uint
	if value := c.Locals("schoolID"); value != nil {
		if typed, ok := value.(uint); ok {
			schoolID = typed
		}
	}
	role := strings.ToUpper(strings.TrimSpace(fmt.Sprint(c.Locals("userRole"))))
	items, err := a.fetchAnnouncementsForSchool(schoolID, role, false, 3)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat pengumuman dashboard", err.Error())
	}
	return utils.Success(c, 200, "Success Get Dashboard Announcements", fiber.Map{
		"items": items,
	})
}
