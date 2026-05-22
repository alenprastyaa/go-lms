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
	ID              uint       `json:"id"`
	SchoolID        uint       `json:"school_id"`
	Title           string     `json:"title"`
	Content         string     `json:"content"`
	TargetAudience  string     `json:"target_audience"`
	TargetAudiences []string   `json:"target_audiences"`
	TargetLabel     string     `json:"target_label"`
	Status          string     `json:"status"`
	StatusLabel     string     `json:"status_label"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	PublishedAt     *time.Time `json:"published_at"`
	DeactivatedAt   *time.Time `json:"deactivated_at"`
	CreatedBy       *uint      `json:"created_by"`
	UpdatedBy       *uint      `json:"updated_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func announcementTargetSingleLabel(value string) string {
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

func announcementSplitTargets(value string) []string {
	cleaned := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(value)), ";", ",")
	parts := strings.Split(cleaned, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		target := strings.TrimSpace(part)
		if target != "" {
			values = append(values, target)
		}
	}
	return values
}

func announcementValidTarget(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case announcementTargetAll, announcementTargetSuperAdmin, announcementTargetAdmin, announcementTargetGuru, announcementTargetSiswa, announcementTargetSarpras, announcementTargetKoperasi:
		return true
	default:
		return false
	}
}

func announcementTargetValues(value string) []string {
	seen := map[string]bool{}
	targets := []string{}
	for _, target := range announcementSplitTargets(value) {
		if !announcementValidTarget(target) {
			continue
		}
		if target == announcementTargetAll {
			return []string{announcementTargetAll}
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		targets = append(targets, target)
	}
	return targets
}

func announcementTargetLabel(value string) string {
	targets := announcementTargetValues(value)
	if len(targets) == 0 {
		return "Tidak Dikenal"
	}
	if len(targets) == 1 {
		return announcementTargetSingleLabel(targets[0])
	}

	labels := make([]string, 0, len(targets))
	for _, target := range targets {
		labels = append(labels, announcementTargetSingleLabel(target))
	}
	return strings.Join(labels, ", ")
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

func announcementNormalizeTargets(values []string) (string, error) {
	seen := map[string]bool{}
	targets := []string{}
	for _, value := range values {
		for _, target := range announcementSplitTargets(value) {
			if !announcementValidTarget(target) {
				return "", fmt.Errorf("Target pengumuman tidak valid")
			}
			if target == announcementTargetAll {
				return announcementTargetAll, nil
			}
			if seen[target] {
				continue
			}
			seen[target] = true
			targets = append(targets, target)
		}
	}

	if len(targets) == 0 {
		return "", fmt.Errorf("Target pengumuman wajib dipilih")
	}

	return strings.Join(targets, ","), nil
}

func announcementNormalizeTargetPayload(targetAudience string, targetAudiences []string) (string, error) {
	values := []string{}
	if strings.TrimSpace(targetAudience) != "" {
		values = append(values, targetAudience)
	}
	values = append(values, targetAudiences...)
	return announcementNormalizeTargets(values)
}

func announcementVisibleToRole(targetAudience, role string) bool {
	role = strings.ToUpper(strings.TrimSpace(role))
	if role == "" {
		return false
	}

	for _, target := range announcementTargetValues(targetAudience) {
		if target == announcementTargetAll || target == role {
			return true
		}
	}
	return false
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

func parseAnnouncementTime(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	location := jakartaLocation()
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, layout := range layouts {
		var parsed time.Time
		var err error
		if layout == time.RFC3339 || layout == time.RFC3339Nano {
			parsed, err = time.Parse(layout, trimmed)
		} else {
			parsed, err = time.ParseInLocation(layout, trimmed, location)
		}
		if err != nil {
			continue
		}

		if layout == "2006-01-02" {
			parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, location)
		}

		return &parsed, nil
	}

	return nil, fmt.Errorf("Format tanggal expired tidak valid")
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
		ID:              item.ID,
		SchoolID:        item.SchoolID,
		Title:           item.Title,
		Content:         item.Content,
		TargetAudience:  item.TargetAudience,
		TargetAudiences: announcementTargetValues(item.TargetAudience),
		TargetLabel:     announcementTargetLabel(item.TargetAudience),
		Status:          item.Status,
		StatusLabel:     announcementStatusLabel(item.Status),
		ReviewedAt:      item.ReviewedAt,
		PublishedAt:     item.PublishedAt,
		DeactivatedAt:   item.DeactivatedAt,
		CreatedBy:       item.CreatedBy,
		UpdatedBy:       item.UpdatedBy,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
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

	hasRoleFilter := strings.TrimSpace(role) != ""
	if limit > 0 && !hasRoleFilter {
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
		if hasRoleFilter && !announcementVisibleToRole(row.TargetAudience, role) {
			continue
		}
		items = append(items, announcementToResponse(row))
		if hasRoleFilter && limit > 0 && len(items) >= limit {
			break
		}
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
		Title           string   `json:"title"`
		Content         string   `json:"content"`
		TargetAudience  string   `json:"target_audience"`
		TargetAudiences []string `json:"target_audiences"`
		DeactivatedAt   string   `json:"deactivated_at"`
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

	targetAudience, err := announcementNormalizeTargetPayload(body.TargetAudience, body.TargetAudiences)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	deactivatedAt, err := parseAnnouncementTime(body.DeactivatedAt)
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
		DeactivatedAt:  deactivatedAt,
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
		Title           string   `json:"title"`
		Content         string   `json:"content"`
		TargetAudience  string   `json:"target_audience"`
		TargetAudiences []string `json:"target_audiences"`
		DeactivatedAt   string   `json:"deactivated_at"`
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

	targetAudience, err := announcementNormalizeTargetPayload(body.TargetAudience, body.TargetAudiences)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}
	deactivatedAt, err := parseAnnouncementTime(body.DeactivatedAt)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	update := map[string]interface{}{
		"title":           title,
		"content":         content,
		"target_audience": targetAudience,
		"deactivated_at":  deactivatedAt,
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
