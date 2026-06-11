package controllers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

type curriculumSubjectRow struct {
	ID                 uint   `gorm:"column:id" json:"id"`
	SchoolID           uint   `gorm:"column:school_id" json:"school_id"`
	Code               string `gorm:"column:code" json:"code"`
	Name               string `gorm:"column:name" json:"name"`
	Description        string `gorm:"column:description" json:"description"`
	WeeklyHours        int    `gorm:"column:weekly_hours" json:"weekly_hours"`
	RequiredRoomType   string `gorm:"column:required_room_type" json:"required_room_type"`
	PreferredRoomID    uint   `gorm:"column:preferred_room_id" json:"preferred_room_id"`
	PreferredRoomCode  string `gorm:"column:preferred_room_code" json:"preferred_room_code"`
	PreferredRoomName  string `gorm:"column:preferred_room_name" json:"preferred_room_name"`
	PreferredRoomColor string `gorm:"column:preferred_room_color" json:"preferred_room_color"`
	AssignedTeacherIDs []uint `gorm:"-" json:"assigned_teacher_ids"`
}

type curriculumRoomRow struct {
	ID       uint   `gorm:"column:id" json:"id"`
	SchoolID uint   `gorm:"column:school_id" json:"school_id"`
	Code     string `gorm:"column:code" json:"code"`
	Name     string `gorm:"column:name" json:"name"`
	RoomType string `gorm:"column:room_type" json:"room_type"`
	Color    string `gorm:"column:color" json:"color"`
	Capacity int    `gorm:"column:capacity" json:"capacity"`
	IsActive bool   `gorm:"column:is_active" json:"is_active"`
	Notes    string `gorm:"column:notes" json:"notes"`
}

type curriculumTeacherLoadRow struct {
	ID                uint   `gorm:"column:id" json:"id"`
	SchoolID          uint   `gorm:"column:school_id" json:"school_id"`
	TeacherID         uint   `gorm:"column:teacher_id" json:"teacher_id"`
	CurriculumSubject uint   `gorm:"column:curriculum_subject_id" json:"curriculum_subject_id"`
	MaxWeeklyHours    int    `gorm:"column:max_weekly_hours" json:"max_weekly_hours"`
	Notes             string `gorm:"column:notes" json:"notes"`
	TeacherName       string `gorm:"column:teacher_name" json:"teacher_name"`
	SubjectName       string `gorm:"column:subject_name" json:"subject_name"`
	SubjectCode       string `gorm:"column:subject_code" json:"subject_code"`
	DistributedHours  int    `gorm:"column:distributed_hours" json:"distributed_hours"`
	RemainingHours    int    `gorm:"column:remaining_hours" json:"remaining_hours"`
}

type curriculumClassDistributionRow struct {
	ID                    uint   `gorm:"column:id" json:"id"`
	SchoolID              uint   `gorm:"column:school_id" json:"school_id"`
	CurriculumTeacherLoad uint   `gorm:"column:curriculum_teacher_load_id" json:"curriculum_teacher_load_id"`
	ClassID               uint   `gorm:"column:class_id" json:"class_id"`
	WeeklyHours           int    `gorm:"column:weekly_hours" json:"weekly_hours"`
	Notes                 string `gorm:"column:notes" json:"notes"`
	TeacherID             uint   `gorm:"column:teacher_id" json:"teacher_id"`
	TeacherName           string `gorm:"column:teacher_name" json:"teacher_name"`
	ClassName             string `gorm:"column:class_name" json:"class_name"`
	SubjectID             uint   `gorm:"column:curriculum_subject_id" json:"curriculum_subject_id"`
	SubjectName           string `gorm:"column:subject_name" json:"subject_name"`
	SubjectCode           string `gorm:"column:subject_code" json:"subject_code"`
	LoadCapacity          int    `gorm:"column:load_capacity" json:"load_capacity"`
}

type curriculumScheduleSlotRow struct {
	ID           uint   `gorm:"column:id" json:"id"`
	SchoolID     uint   `gorm:"column:school_id" json:"school_id"`
	DayName      string `gorm:"column:day_name" json:"day_name"`
	DayOrder     int    `gorm:"column:day_order" json:"day_order"`
	SessionOrder int    `gorm:"column:session_order" json:"session_order"`
	StartTime    string `gorm:"column:start_time" json:"start_time"`
	EndTime      string `gorm:"column:end_time" json:"end_time"`
	Label        string `gorm:"column:label" json:"label"`
}

type curriculumScheduleEntryRow struct {
	ID                uint   `gorm:"column:id" json:"id"`
	SchoolID          uint   `gorm:"column:school_id" json:"school_id"`
	ClassID           uint   `gorm:"column:class_id" json:"class_id"`
	CurriculumSubject uint   `gorm:"column:curriculum_subject_id" json:"curriculum_subject_id"`
	TeacherID         uint   `gorm:"column:teacher_id" json:"teacher_id"`
	ScheduleSlotID    uint   `gorm:"column:schedule_slot_id" json:"schedule_slot_id"`
	RoomID            uint   `gorm:"column:room_id" json:"room_id"`
	LearningSubjectID uint   `gorm:"column:learning_subject_id" json:"learning_subject_id"`
	GeneratedAt       string `gorm:"column:generated_at" json:"generated_at"`
	ClassName         string `gorm:"column:class_name" json:"class_name"`
	TeacherName       string `gorm:"column:teacher_name" json:"teacher_name"`
	SubjectName       string `gorm:"column:subject_name" json:"subject_name"`
	SubjectCode       string `gorm:"column:subject_code" json:"subject_code"`
	RoomName          string `gorm:"column:room_name" json:"room_name"`
	RoomCode          string `gorm:"column:room_code" json:"room_code"`
	RoomType          string `gorm:"column:room_type" json:"room_type"`
	RoomColor         string `gorm:"column:room_color" json:"room_color"`
	DayName           string `gorm:"column:day_name" json:"day_name"`
	DayOrder          int    `gorm:"column:day_order" json:"day_order"`
	SessionOrder      int    `gorm:"column:session_order" json:"session_order"`
	StartTime         string `gorm:"column:start_time" json:"start_time"`
	EndTime           string `gorm:"column:end_time" json:"end_time"`
	SlotLabel         string `gorm:"column:slot_label" json:"slot_label"`
}

func (a *AppContext) GetCurriculumOverview(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	subjects, rooms, teacherLoads, classDistributions, scheduleSlots, generatedEntries := a.loadCurriculumOverviewData(schoolID)
	summary := fiber.Map{
		"subjects":            len(subjects),
		"rooms":               len(rooms),
		"teacher_loads":       len(teacherLoads),
		"class_distributions": len(classDistributions),
		"schedule_slots":      len(scheduleSlots),
		"generated_entries":   len(generatedEntries),
	}

	return utils.Success(c, 200, "Success Get Curriculum Overview", fiber.Map{
		"subjects":            subjects,
		"rooms":               rooms,
		"teacher_loads":       teacherLoads,
		"class_distributions": classDistributions,
		"schedule_slots":      scheduleSlots,
		"generated_entries":   generatedEntries,
		"summary":             summary,
	})
}

func (a *AppContext) CreateCurriculumSubject(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Code             string `json:"code"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		WeeklyHours      int    `json:"weekly_hours"`
		RequiredRoomType string `json:"required_room_type"`
		PreferredRoomID  uint   `json:"preferred_room_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama mata pelajaran wajib diisi")
	}
	if body.WeeklyHours <= 0 {
		body.WeeklyHours = 2
	}
	roomType := normalizeCurriculumRoomType(body.RequiredRoomType)
	if body.PreferredRoomID > 0 && !a.curriculumRoomBelongsToSchool(schoolID, body.PreferredRoomID) {
		return utils.Error(c, 400, "Ruang pilihan tidak valid")
	}

	var row curriculumSubjectRow
	a.DB.Raw(`
		INSERT INTO curriculum_subjects (school_id, code, name, description, weekly_hours, required_room_type, preferred_room_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		RETURNING id, school_id, COALESCE(code, '') AS code, name, COALESCE(description, '') AS description, weekly_hours, required_room_type, COALESCE(preferred_room_id, 0) AS preferred_room_id
	`, schoolID, nullIfEmpty(strings.ToUpper(strings.TrimSpace(body.Code))), name, nullIfEmpty(body.Description), body.WeeklyHours, roomType, nullIfZero(int(body.PreferredRoomID))).Scan(&row)
	a.DB.Raw(curriculumSubjectQuery()+` WHERE cs.id = ?`, row.ID).Scan(&row)

	return utils.Success(c, 201, "Success Create Curriculum Subject", row)
}

func (a *AppContext) UpdateCurriculumSubject(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")
	var body struct {
		Code             string `json:"code"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		WeeklyHours      int    `json:"weekly_hours"`
		RequiredRoomType string `json:"required_room_type"`
		PreferredRoomID  uint   `json:"preferred_room_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama mata pelajaran wajib diisi")
	}
	if body.WeeklyHours <= 0 {
		body.WeeklyHours = 2
	}
	roomType := normalizeCurriculumRoomType(body.RequiredRoomType)
	if body.PreferredRoomID > 0 && !a.curriculumRoomBelongsToSchool(schoolID, body.PreferredRoomID) {
		return utils.Error(c, 400, "Ruang pilihan tidak valid")
	}

	var row curriculumSubjectRow
	a.DB.Raw(`
		UPDATE curriculum_subjects
		SET code = ?, name = ?, description = ?, weekly_hours = ?, required_room_type = ?, preferred_room_id = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, COALESCE(code, '') AS code, name, COALESCE(description, '') AS description, weekly_hours, required_room_type, COALESCE(preferred_room_id, 0) AS preferred_room_id
	`, nullIfEmpty(strings.ToUpper(strings.TrimSpace(body.Code))), name, nullIfEmpty(body.Description), body.WeeklyHours, roomType, nullIfZero(int(body.PreferredRoomID)), id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data mapel kurikulum tidak ditemukan")
	}
	a.DB.Raw(curriculumSubjectQuery()+` WHERE cs.id = ?`, row.ID).Scan(&row)

	return utils.Success(c, 200, "Success Update Curriculum Subject", row)
}

func (a *AppContext) DeleteCurriculumSubject(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")

	var row curriculumSubjectRow
	a.DB.Raw(`
		DELETE FROM curriculum_subjects
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, COALESCE(code, '') AS code, name, COALESCE(description, '') AS description, weekly_hours
	`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data mapel kurikulum tidak ditemukan")
	}

	a.DB.Exec(`DELETE FROM curriculum_class_distributions WHERE school_id = ? AND curriculum_teacher_load_id IN (SELECT id FROM curriculum_teacher_loads WHERE school_id = ? AND curriculum_subject_id = ?)`, schoolID, schoolID, row.ID)
	a.DB.Exec(`DELETE FROM curriculum_teacher_loads WHERE school_id = ? AND curriculum_subject_id = ?`, schoolID, row.ID)
	a.DB.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ? AND curriculum_subject_id = ?`, schoolID, row.ID)
	return utils.Success(c, 200, "Success Delete Curriculum Subject", row)
}

func (a *AppContext) CreateCurriculumRoom(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		RoomType string `json:"room_type"`
		Color    string `json:"color"`
		Capacity int    `json:"capacity"`
		IsActive *bool  `json:"is_active"`
		Notes    string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}

	code := strings.ToUpper(strings.TrimSpace(body.Code))
	name := strings.TrimSpace(body.Name)
	if code == "" || name == "" {
		return utils.Error(c, 400, "Kode dan nama ruang wajib diisi")
	}
	active := true
	if body.IsActive != nil {
		active = *body.IsActive
	}

	var row curriculumRoomRow
	a.DB.Raw(`
		INSERT INTO curriculum_rooms (school_id, code, name, room_type, color, capacity, is_active, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (school_id, code)
		DO UPDATE SET name = EXCLUDED.name, room_type = EXCLUDED.room_type, color = EXCLUDED.color, capacity = EXCLUDED.capacity, is_active = EXCLUDED.is_active, notes = EXCLUDED.notes, updated_at = NOW()
		RETURNING id, school_id, code, name, room_type, color, capacity, is_active, COALESCE(notes, '') AS notes
	`, schoolID, code, name, normalizeCurriculumRoomType(body.RoomType), normalizeRoomColor(body.Color), body.Capacity, active, nullIfEmpty(body.Notes)).Scan(&row)

	return utils.Success(c, 201, "Success Save Curriculum Room", row)
}

func (a *AppContext) UpdateCurriculumRoom(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")
	var body struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		RoomType string `json:"room_type"`
		Color    string `json:"color"`
		Capacity int    `json:"capacity"`
		IsActive *bool  `json:"is_active"`
		Notes    string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}

	code := strings.ToUpper(strings.TrimSpace(body.Code))
	name := strings.TrimSpace(body.Name)
	if code == "" || name == "" {
		return utils.Error(c, 400, "Kode dan nama ruang wajib diisi")
	}
	active := true
	if body.IsActive != nil {
		active = *body.IsActive
	}

	var row curriculumRoomRow
	a.DB.Raw(`
		UPDATE curriculum_rooms
		SET code = ?, name = ?, room_type = ?, color = ?, capacity = ?, is_active = ?, notes = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, code, name, room_type, color, capacity, is_active, COALESCE(notes, '') AS notes
	`, code, name, normalizeCurriculumRoomType(body.RoomType), normalizeRoomColor(body.Color), body.Capacity, active, nullIfEmpty(body.Notes), id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data ruang tidak ditemukan")
	}

	return utils.Success(c, 200, "Success Update Curriculum Room", row)
}

func (a *AppContext) DeleteCurriculumRoom(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")

	var usageCount int64
	a.DB.Raw(`
		SELECT
			(SELECT COUNT(*) FROM curriculum_subjects WHERE school_id = ? AND preferred_room_id = ?) +
			(SELECT COUNT(*) FROM curriculum_schedule_entries WHERE school_id = ? AND room_id = ?)
	`, schoolID, id, schoolID, id).Scan(&usageCount)
	if usageCount > 0 {
		return utils.Error(c, 400, "Ruang masih dipakai oleh mapel atau hasil jadwal")
	}

	var row curriculumRoomRow
	a.DB.Raw(`
		DELETE FROM curriculum_rooms
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, code, name, room_type, color, capacity, is_active, COALESCE(notes, '') AS notes
	`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data ruang tidak ditemukan")
	}

	return utils.Success(c, 200, "Success Delete Curriculum Room", row)
}

func (a *AppContext) CreateCurriculumTeacherLoad(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		TeacherID           uint   `json:"teacher_id"`
		CurriculumSubjectID uint   `json:"curriculum_subject_id"`
		MaxWeeklyHours      int    `json:"max_weekly_hours"`
		Notes               string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}
	if body.TeacherID == 0 || body.CurriculumSubjectID == 0 {
		return utils.Error(c, 400, "Guru dan mapel wajib dipilih")
	}
	if body.MaxWeeklyHours <= 0 {
		return utils.Error(c, 400, "Kapasitas jam mengajar harus lebih dari 0")
	}

	var row curriculumTeacherLoadRow
	a.DB.Raw(`
		INSERT INTO curriculum_teacher_loads (school_id, teacher_id, curriculum_subject_id, max_weekly_hours, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (school_id, teacher_id, curriculum_subject_id)
		DO UPDATE SET max_weekly_hours = EXCLUDED.max_weekly_hours, notes = EXCLUDED.notes, updated_at = NOW()
		RETURNING id, school_id, teacher_id, curriculum_subject_id, max_weekly_hours, COALESCE(notes, '') AS notes
	`, schoolID, body.TeacherID, body.CurriculumSubjectID, body.MaxWeeklyHours, nullIfEmpty(body.Notes)).Scan(&row)

	a.DB.Raw(curriculumTeacherLoadQuery()+` WHERE ctl.id = ?`, row.ID).Scan(&row)
	return utils.Success(c, 201, "Success Save Curriculum Teacher Load", row)
}

func (a *AppContext) UpdateCurriculumTeacherLoad(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")
	var body struct {
		TeacherID           uint   `json:"teacher_id"`
		CurriculumSubjectID uint   `json:"curriculum_subject_id"`
		MaxWeeklyHours      int    `json:"max_weekly_hours"`
		Notes               string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}
	if body.TeacherID == 0 || body.CurriculumSubjectID == 0 {
		return utils.Error(c, 400, "Guru dan mapel wajib dipilih")
	}
	if body.MaxWeeklyHours <= 0 {
		return utils.Error(c, 400, "Kapasitas jam mengajar harus lebih dari 0")
	}

	var row curriculumTeacherLoadRow
	a.DB.Raw(`
		UPDATE curriculum_teacher_loads
		SET teacher_id = ?, curriculum_subject_id = ?, max_weekly_hours = ?, notes = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, teacher_id, curriculum_subject_id, max_weekly_hours, COALESCE(notes, '') AS notes
	`, body.TeacherID, body.CurriculumSubjectID, body.MaxWeeklyHours, nullIfEmpty(body.Notes), id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data beban guru tidak ditemukan")
	}

	a.DB.Raw(curriculumTeacherLoadQuery()+` WHERE ctl.id = ?`, row.ID).Scan(&row)
	return utils.Success(c, 200, "Success Update Curriculum Teacher Load", row)
}

func (a *AppContext) DeleteCurriculumTeacherLoad(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")

	var row curriculumTeacherLoadRow
	a.DB.Raw(`
		DELETE FROM curriculum_teacher_loads
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, teacher_id, curriculum_subject_id, max_weekly_hours, COALESCE(notes, '') AS notes
	`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data beban guru tidak ditemukan")
	}

	a.DB.Exec(`DELETE FROM curriculum_class_distributions WHERE school_id = ? AND curriculum_teacher_load_id = ?`, schoolID, row.ID)
	return utils.Success(c, 200, "Success Delete Curriculum Teacher Load", row)
}

func (a *AppContext) CreateCurriculumClassDistribution(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		CurriculumTeacherLoadID uint   `json:"curriculum_teacher_load_id"`
		ClassID                 uint   `json:"class_id"`
		WeeklyHours             int    `json:"weekly_hours"`
		Notes                   string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}
	if body.CurriculumTeacherLoadID == 0 || body.ClassID == 0 {
		return utils.Error(c, 400, "Beban guru dan kelas wajib dipilih")
	}
	if body.WeeklyHours <= 0 {
		return utils.Error(c, 400, "Jam distribusi kelas harus lebih dari 0")
	}

	var load curriculumTeacherLoadRow
	a.DB.Raw(curriculumTeacherLoadQuery()+` WHERE ctl.id = ? AND ctl.school_id = ?`, body.CurriculumTeacherLoadID, schoolID).Scan(&load)
	if load.ID == 0 {
		return utils.Error(c, 404, "Beban guru tidak ditemukan")
	}
	if conflictMessage := a.validateCurriculumDistributionConflict(schoolID, 0, body.CurriculumTeacherLoadID, body.ClassID); conflictMessage != "" {
		return utils.Error(c, 400, conflictMessage)
	}

	var existingID uint
	a.DB.Raw(`
		SELECT id FROM curriculum_class_distributions
		WHERE school_id = ? AND curriculum_teacher_load_id = ? AND class_id = ?
		LIMIT 1
	`, schoolID, body.CurriculumTeacherLoadID, body.ClassID).Scan(&existingID)

	var currentTotal int
	a.DB.Raw(`
		SELECT COALESCE(SUM(weekly_hours), 0)
		FROM curriculum_class_distributions
		WHERE school_id = ? AND curriculum_teacher_load_id = ? AND id <> COALESCE(NULLIF(?, 0), -1)
	`, schoolID, body.CurriculumTeacherLoadID, existingID).Scan(&currentTotal)
	if currentTotal+body.WeeklyHours > load.MaxWeeklyHours {
		return utils.Error(c, 400, fmt.Sprintf("Distribusi melebihi kapasitas beban guru. Maksimal %d JP, terpakai %d JP.", load.MaxWeeklyHours, currentTotal))
	}

	var row curriculumClassDistributionRow
	a.DB.Raw(`
		INSERT INTO curriculum_class_distributions (school_id, curriculum_teacher_load_id, class_id, weekly_hours, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (school_id, curriculum_teacher_load_id, class_id)
		DO UPDATE SET weekly_hours = EXCLUDED.weekly_hours, notes = EXCLUDED.notes, updated_at = NOW()
		RETURNING id, school_id, curriculum_teacher_load_id, class_id, weekly_hours, COALESCE(notes, '') AS notes
	`, schoolID, body.CurriculumTeacherLoadID, body.ClassID, body.WeeklyHours, nullIfEmpty(body.Notes)).Scan(&row)

	a.DB.Raw(curriculumClassDistributionQuery()+` WHERE ccd.id = ?`, row.ID).Scan(&row)
	return utils.Success(c, 201, "Success Save Curriculum Class Distribution", row)
}

func (a *AppContext) UpdateCurriculumClassDistribution(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")
	var body struct {
		CurriculumTeacherLoadID uint   `json:"curriculum_teacher_load_id"`
		ClassID                 uint   `json:"class_id"`
		WeeklyHours             int    `json:"weekly_hours"`
		Notes                   string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}
	if body.CurriculumTeacherLoadID == 0 || body.ClassID == 0 {
		return utils.Error(c, 400, "Beban guru dan kelas wajib dipilih")
	}
	if body.WeeklyHours <= 0 {
		return utils.Error(c, 400, "Jam distribusi kelas harus lebih dari 0")
	}

	var load curriculumTeacherLoadRow
	a.DB.Raw(curriculumTeacherLoadQuery()+` WHERE ctl.id = ? AND ctl.school_id = ?`, body.CurriculumTeacherLoadID, schoolID).Scan(&load)
	if load.ID == 0 {
		return utils.Error(c, 404, "Beban guru tidak ditemukan")
	}
	if conflictMessage := a.validateCurriculumDistributionConflict(schoolID, id, body.CurriculumTeacherLoadID, body.ClassID); conflictMessage != "" {
		return utils.Error(c, 400, conflictMessage)
	}

	var currentTotal int
	a.DB.Raw(`
		SELECT COALESCE(SUM(weekly_hours), 0)
		FROM curriculum_class_distributions
		WHERE school_id = ? AND curriculum_teacher_load_id = ? AND id <> ?
	`, schoolID, body.CurriculumTeacherLoadID, id).Scan(&currentTotal)
	if currentTotal+body.WeeklyHours > load.MaxWeeklyHours {
		return utils.Error(c, 400, fmt.Sprintf("Distribusi melebihi kapasitas beban guru. Maksimal %d JP, terpakai %d JP.", load.MaxWeeklyHours, currentTotal))
	}

	var row curriculumClassDistributionRow
	a.DB.Raw(`
		UPDATE curriculum_class_distributions
		SET curriculum_teacher_load_id = ?, class_id = ?, weekly_hours = ?, notes = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, curriculum_teacher_load_id, class_id, weekly_hours, COALESCE(notes, '') AS notes
	`, body.CurriculumTeacherLoadID, body.ClassID, body.WeeklyHours, nullIfEmpty(body.Notes), id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data distribusi kelas tidak ditemukan")
	}

	a.DB.Raw(curriculumClassDistributionQuery()+` WHERE ccd.id = ?`, row.ID).Scan(&row)
	return utils.Success(c, 200, "Success Update Curriculum Class Distribution", row)
}

func (a *AppContext) DeleteCurriculumClassDistribution(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")

	var row curriculumClassDistributionRow
	a.DB.Raw(`
		DELETE FROM curriculum_class_distributions
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, curriculum_teacher_load_id, class_id, weekly_hours, COALESCE(notes, '') AS notes
	`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Data distribusi kelas tidak ditemukan")
	}

	return utils.Success(c, 200, "Success Delete Curriculum Class Distribution", row)
}

func (a *AppContext) CreateCurriculumScheduleSlot(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		DayName      string `json:"day_name"`
		DayOrder     int    `json:"day_order"`
		SessionOrder int    `json:"session_order"`
		StartTime    string `json:"start_time"`
		EndTime      string `json:"end_time"`
		Label        string `json:"label"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}
	if strings.TrimSpace(body.DayName) == "" || body.DayOrder <= 0 || body.SessionOrder <= 0 || strings.TrimSpace(body.StartTime) == "" || strings.TrimSpace(body.EndTime) == "" {
		return utils.Error(c, 400, "Hari, urutan, dan jam slot wajib diisi")
	}

	var row curriculumScheduleSlotRow
	a.DB.Raw(`
		INSERT INTO curriculum_schedule_slots (school_id, day_name, day_order, session_order, start_time, end_time, label, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		RETURNING id, school_id, day_name, day_order, session_order, start_time, end_time, COALESCE(label, '') AS label
	`, schoolID, strings.TrimSpace(body.DayName), body.DayOrder, body.SessionOrder, strings.TrimSpace(body.StartTime), strings.TrimSpace(body.EndTime), nullIfEmpty(body.Label)).Scan(&row)

	return utils.Success(c, 201, "Success Create Curriculum Schedule Slot", row)
}

func (a *AppContext) BulkCreateCurriculumScheduleSlots(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Days []struct {
			DayName  string `json:"day_name"`
			DayOrder int    `json:"day_order"`
		} `json:"days"`
		SessionsPerDay    int    `json:"sessions_per_day"`
		StartTime         string `json:"start_time"`
		DurationMinutes   int    `json:"duration_minutes"`
		GapMinutes        int    `json:"gap_minutes"`
		BreakAfterSession int    `json:"break_after_session"`
		BreakMinutes      int    `json:"break_minutes"`
		Breaks            []struct {
			AfterSession int `json:"after_session"`
			Minutes      int `json:"minutes"`
		} `json:"breaks"`
		LabelMode         string `json:"label_mode"`
		OverwriteExisting bool   `json:"overwrite_existing"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}

	if len(body.Days) == 0 {
		return utils.Error(c, 400, "Hari wajib dipilih")
	}
	if body.SessionsPerDay <= 0 {
		return utils.Error(c, 400, "Jumlah sesi per hari harus lebih dari 0")
	}
	if body.DurationMinutes <= 0 {
		body.DurationMinutes = 45
	}
	if body.GapMinutes < 0 {
		body.GapMinutes = 0
	}
	if body.BreakMinutes < 0 {
		body.BreakMinutes = 0
	}

	startTimeText := strings.TrimSpace(body.StartTime)
	if startTimeText == "" {
		startTimeText = "07:00"
	}
	startParsed, err := time.Parse("15:04", startTimeText)
	if err != nil {
		return utils.Error(c, 400, "Format jam mulai tidak valid (HH:MM)")
	}

	labelMode := strings.ToLower(strings.TrimSpace(body.LabelMode))
	if labelMode == "" {
		labelMode = "jam"
	}

	breakMinutesBySession := map[int]int{}
	if len(body.Breaks) > 0 {
		for _, breakItem := range body.Breaks {
			if breakItem.AfterSession <= 0 || breakItem.AfterSession >= body.SessionsPerDay {
				return utils.Error(c, 400, "Posisi jeda tidak valid")
			}
			if breakItem.Minutes < 0 {
				return utils.Error(c, 400, "Durasi jeda tidak valid")
			}
			if _, exists := breakMinutesBySession[breakItem.AfterSession]; exists {
				return utils.Error(c, 400, "Posisi jeda tidak boleh sama")
			}
			breakMinutesBySession[breakItem.AfterSession] = breakItem.Minutes
		}
	} else if body.BreakAfterSession > 0 || body.BreakMinutes > 0 {
		if body.BreakAfterSession <= 0 || body.BreakAfterSession >= body.SessionsPerDay {
			return utils.Error(c, 400, "Posisi jeda tidak valid")
		}
		breakMinutesBySession[body.BreakAfterSession] = body.BreakMinutes
	}

	upsertSuffix := "DO NOTHING"
	if body.OverwriteExisting {
		upsertSuffix = "DO UPDATE SET day_name = EXCLUDED.day_name, start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time, label = EXCLUDED.label, updated_at = NOW()"
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		return utils.Error(c, 500, "Gagal memulai transaksi")
	}

	processed := 0
	affected := int64(0)
	minute := time.Minute
	for _, day := range body.Days {
		dayName := strings.TrimSpace(day.DayName)
		if dayName == "" || day.DayOrder <= 0 {
			tx.Rollback()
			return utils.Error(c, 400, "Hari dan urutan hari wajib diisi")
		}

		cursor := startParsed
		for session := 1; session <= body.SessionsPerDay; session += 1 {
			slotStart := cursor
			slotEnd := cursor.Add(time.Duration(body.DurationMinutes) * minute)

			var label string
			switch labelMode {
			case "none":
				label = ""
			case "sesi":
				label = fmt.Sprintf("Sesi %d", session)
			default:
				label = fmt.Sprintf("Jam %d", session)
			}

			res := tx.Exec(
				fmt.Sprintf(`
					INSERT INTO curriculum_schedule_slots (school_id, day_name, day_order, session_order, start_time, end_time, label, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
					ON CONFLICT (school_id, day_order, session_order)
					%s
				`, upsertSuffix),
				schoolID,
				dayName,
				day.DayOrder,
				session,
				slotStart.Format("15:04"),
				slotEnd.Format("15:04"),
				nullIfEmpty(label),
			)
			if res.Error != nil {
				tx.Rollback()
				return utils.Error(c, 400, res.Error.Error())
			}

			processed += 1
			affected += res.RowsAffected

			cursor = slotEnd.Add(time.Duration(body.GapMinutes) * minute)
			if breakMinutes, ok := breakMinutesBySession[session]; ok {
				cursor = cursor.Add(time.Duration(breakMinutes) * minute)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan slot jadwal")
	}

	return utils.Success(c, 201, "Success Bulk Create Curriculum Schedule Slots", fiber.Map{
		"processed": processed,
		"affected":  affected,
	})
}

func (a *AppContext) UpdateCurriculumScheduleSlot(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")
	var body struct {
		DayName      string `json:"day_name"`
		DayOrder     int    `json:"day_order"`
		SessionOrder int    `json:"session_order"`
		StartTime    string `json:"start_time"`
		EndTime      string `json:"end_time"`
		Label        string `json:"label"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}
	if strings.TrimSpace(body.DayName) == "" || body.DayOrder <= 0 || body.SessionOrder <= 0 || strings.TrimSpace(body.StartTime) == "" || strings.TrimSpace(body.EndTime) == "" {
		return utils.Error(c, 400, "Hari, urutan, dan jam slot wajib diisi")
	}

	var row curriculumScheduleSlotRow
	a.DB.Raw(`
		UPDATE curriculum_schedule_slots
		SET day_name = ?, day_order = ?, session_order = ?, start_time = ?, end_time = ?, label = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, day_name, day_order, session_order, start_time, end_time, COALESCE(label, '') AS label
	`, strings.TrimSpace(body.DayName), body.DayOrder, body.SessionOrder, strings.TrimSpace(body.StartTime), strings.TrimSpace(body.EndTime), nullIfEmpty(body.Label), id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Slot jadwal tidak ditemukan")
	}

	return utils.Success(c, 200, "Success Update Curriculum Schedule Slot", row)
}

func (a *AppContext) DeleteCurriculumScheduleSlot(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	id := c.Params("id")

	var row curriculumScheduleSlotRow
	a.DB.Raw(`
		DELETE FROM curriculum_schedule_slots
		WHERE id = ? AND school_id = ?
		RETURNING id, school_id, day_name, day_order, session_order, start_time, end_time, COALESCE(label, '') AS label
	`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Slot jadwal tidak ditemukan")
	}

	a.DB.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ? AND schedule_slot_id = ?`, schoolID, row.ID)
	return utils.Success(c, 200, "Success Delete Curriculum Schedule Slot", row)
}

func (a *AppContext) BulkDeleteCurriculumScheduleSlots(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		DayNames []string `json:"day_names"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request body")
	}

	cleaned := make([]string, 0, len(body.DayNames))
	for _, dayName := range body.DayNames {
		trimmed := strings.TrimSpace(dayName)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return utils.Error(c, 400, "Hari yang akan dihapus wajib dipilih")
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		return utils.Error(c, 500, "Gagal memulai transaksi")
	}

	type deletedRow struct {
		ID       uint   `gorm:"column:id" json:"id"`
		DayName  string `gorm:"column:day_name" json:"day_name"`
		DayOrder int    `gorm:"column:day_order" json:"day_order"`
	}

	deleted := make([]deletedRow, 0)
	for _, dayName := range cleaned {
		var rows []deletedRow
		if err := tx.Raw(`
			SELECT id, day_name, day_order
			FROM curriculum_schedule_slots
			WHERE school_id = ? AND day_name = ?
			ORDER BY day_order ASC, session_order ASC
		`, schoolID, dayName).Scan(&rows).Error; err != nil {
			tx.Rollback()
			return utils.Error(c, 400, err.Error())
		}
		deleted = append(deleted, rows...)
	}

	for _, row := range deleted {
		if err := tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ? AND schedule_slot_id = ?`, schoolID, row.ID).Error; err != nil {
			tx.Rollback()
			return utils.Error(c, 400, err.Error())
		}
	}

	for _, dayName := range cleaned {
		if err := tx.Exec(`DELETE FROM curriculum_schedule_slots WHERE school_id = ? AND day_name = ?`, schoolID, dayName).Error; err != nil {
			tx.Rollback()
			return utils.Error(c, 400, err.Error())
		}
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus slot jadwal")
	}

	return utils.Success(c, 200, "Success Bulk Delete Curriculum Schedule Slots", fiber.Map{
		"deleted_count": len(deleted),
		"day_names":     cleaned,
	})
}

func (a *AppContext) GenerateCurriculumSchedule(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	subjects, rooms, teacherLoads, classDistributions, scheduleSlots, _ := a.loadCurriculumOverviewData(schoolID)
	if len(teacherLoads) == 0 {
		return utils.Error(c, 400, "Beban guru belum tersedia")
	}
	if len(classDistributions) == 0 {
		return utils.Error(c, 400, "Distribusi guru ke kelas belum tersedia")
	}
	if len(scheduleSlots) == 0 {
		return utils.Error(c, 400, "Slot jadwal pembelajaran belum tersedia")
	}

	sort.Slice(scheduleSlots, func(i, j int) bool {
		if scheduleSlots[i].DayOrder == scheduleSlots[j].DayOrder {
			return scheduleSlots[i].SessionOrder < scheduleSlots[j].SessionOrder
		}
		return scheduleSlots[i].DayOrder < scheduleSlots[j].DayOrder
	})

	loadByID := map[uint]curriculumTeacherLoadRow{}
	distributionTotals := map[uint]int{}
	for _, load := range teacherLoads {
		loadByID[load.ID] = load
	}
	for _, distribution := range classDistributions {
		distributionTotals[distribution.CurriculumTeacherLoad] += distribution.WeeklyHours
	}

	issues := make([]string, 0)
	for _, load := range teacherLoads {
		total := distributionTotals[load.ID]
		if total > load.MaxWeeklyHours {
			issues = append(issues, fmt.Sprintf("Distribusi %s untuk %s melebihi kapasitas: %d/%d JP.", load.TeacherName, load.SubjectName, total, load.MaxWeeklyHours))
		}
	}

	type classSubjectKey struct {
		ClassID   uint
		SubjectID uint
	}
	classSubjectHours := map[classSubjectKey]int{}
	classSubjectTeacher := map[classSubjectKey]uint{}
	classSubjectName := map[classSubjectKey]string{}
	classNameByKey := map[classSubjectKey]string{}
	for _, distribution := range classDistributions {
		key := classSubjectKey{
			ClassID:   distribution.ClassID,
			SubjectID: distribution.SubjectID,
		}
		classSubjectHours[key] += distribution.WeeklyHours
		if classSubjectTeacher[key] == 0 {
			classSubjectTeacher[key] = distribution.TeacherID
		} else if classSubjectTeacher[key] != distribution.TeacherID {
			issues = append(issues, fmt.Sprintf("Mapel %s di kelas %s terdistribusi ke lebih dari satu guru. Tetapkan satu guru pengampu per kelas-mapel.", distribution.SubjectName, distribution.ClassName))
		}
		classSubjectName[key] = distribution.SubjectName
		classNameByKey[key] = distribution.ClassName
	}
	subjectWeeklyHours := map[uint]int{}
	subjectByID := map[uint]curriculumSubjectRow{}
	for _, subject := range subjects {
		subjectWeeklyHours[subject.ID] = subject.WeeklyHours
		subjectByID[subject.ID] = subject
	}
	activeRoomsByType := map[string][]curriculumRoomRow{}
	roomByID := map[uint]curriculumRoomRow{}
	for _, room := range rooms {
		roomByID[room.ID] = room
		if room.IsActive {
			activeRoomsByType[normalizeCurriculumRoomType(room.RoomType)] = append(activeRoomsByType[normalizeCurriculumRoomType(room.RoomType)], room)
		}
	}
	for key, totalHours := range classSubjectHours {
		requiredHours := subjectWeeklyHours[key.SubjectID]
		if requiredHours > 0 && totalHours != requiredHours {
			issues = append(issues, fmt.Sprintf("Distribusi %s di kelas %s adalah %d JP, sedangkan kebutuhan mapel per rombel adalah %d JP.", classSubjectName[key], classNameByKey[key], totalHours, requiredHours))
		}
	}
	if len(issues) > 0 {
		return utils.Error(c, 400, strings.Join(issues, " "))
	}

	type generatedSlotAssignment struct {
		SlotID uint
		RoomID uint
	}

	type generatedAssignment struct {
		ClassID             uint
		ClassName           string
		SubjectID           uint
		SubjectName         string
		SubjectCode         string
		TeacherID           uint
		TeacherName         string
		RequestedWeeklyHour int
		AssignedSlots       []generatedSlotAssignment
	}

	type classSubjectKeyWithTeacher struct {
		ClassID   uint
		SubjectID uint
		TeacherID uint
	}

	slotsByDay := map[int][]curriculumScheduleSlotRow{}
	for _, slot := range scheduleSlots {
		slotsByDay[slot.DayOrder] = append(slotsByDay[slot.DayOrder], slot)
	}
	for dayOrder := range slotsByDay {
		sort.Slice(slotsByDay[dayOrder], func(i, j int) bool {
			return slotsByDay[dayOrder][i].SessionOrder < slotsByDay[dayOrder][j].SessionOrder
		})
	}

	classOccupied := map[uint]map[uint]bool{}
	teacherOccupied := map[uint]map[uint]bool{}
	roomOccupied := map[uint]map[uint]bool{}
	classDayLoad := map[uint]map[int]int{}
	teacherDayLoad := map[uint]map[int]int{}
	roomDayLoad := map[uint]map[int]int{}
	classSubjectDayLoad := map[classSubjectKey]map[int]int{}
	assignmentsByKey := map[classSubjectKeyWithTeacher]*generatedAssignment{}
	assignments := make([]generatedAssignment, 0, len(classDistributions))

	distributionQueue := append([]curriculumClassDistributionRow(nil), classDistributions...)
	sort.SliceStable(distributionQueue, func(i, j int) bool {
		if distributionQueue[i].WeeklyHours == distributionQueue[j].WeeklyHours {
			if distributionQueue[i].ClassName == distributionQueue[j].ClassName {
				return distributionQueue[i].SubjectName < distributionQueue[j].SubjectName
			}
			return distributionQueue[i].ClassName < distributionQueue[j].ClassName
		}
		return distributionQueue[i].WeeklyHours > distributionQueue[j].WeeklyHours
	})

	ensureSlotMap := func(target map[uint]map[uint]bool, id uint) map[uint]bool {
		if target[id] == nil {
			target[id] = map[uint]bool{}
		}
		return target[id]
	}
	ensureDayLoad := func(target map[uint]map[int]int, id uint) map[int]int {
		if target[id] == nil {
			target[id] = map[int]int{}
		}
		return target[id]
	}
	ensureSubjectDayLoad := func(key classSubjectKey) map[int]int {
		if classSubjectDayLoad[key] == nil {
			classSubjectDayLoad[key] = map[int]int{}
		}
		return classSubjectDayLoad[key]
	}
	pickCandidateRoom := func(subject curriculumSubjectRow, slotIDs []uint, dayOrder int) (uint, bool, int) {
		preferredRoomID := subject.PreferredRoomID
		if preferredRoomID > 0 {
			room := roomByID[preferredRoomID]
			if room.ID == 0 || !room.IsActive {
				return 0, false, 0
			}
			for _, slotID := range slotIDs {
				if roomOccupied[preferredRoomID] != nil && roomOccupied[preferredRoomID][slotID] {
					return 0, false, 0
				}
			}
			return preferredRoomID, true, roomDayLoad[preferredRoomID][dayOrder]
		}

		roomType := normalizeCurriculumRoomType(subject.RequiredRoomType)
		if roomType == "" || roomType == "CLASSROOM" || roomType == "UMUM" {
			return 0, true, 0
		}

		candidates := activeRoomsByType[roomType]
		if len(candidates) == 0 {
			return 0, false, 0
		}

		bestRoomID := uint(0)
		bestScore := 1<<31 - 1
		for _, room := range candidates {
			valid := true
			for _, slotID := range slotIDs {
				if roomOccupied[room.ID] != nil && roomOccupied[room.ID][slotID] {
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			score := roomDayLoad[room.ID][dayOrder]*12 + int(room.ID)
			if score < bestScore {
				bestScore = score
				bestRoomID = room.ID
			}
		}
		return bestRoomID, bestRoomID > 0, bestScore
	}

	findBestBlock := func(distribution curriculumClassDistributionRow, load curriculumTeacherLoadRow, blockSize int) []generatedSlotAssignment {
		type candidate struct {
			slots  []generatedSlotAssignment
			score  int
			roomID uint
		}
		best := candidate{score: 1<<31 - 1}
		subjectKey := classSubjectKey{ClassID: distribution.ClassID, SubjectID: load.CurriculumSubject}
		subject := subjectByID[load.CurriculumSubject]

		for dayOrder, daySlots := range slotsByDay {
			if blockSize <= 1 {
				for _, slot := range daySlots {
					if classOccupied[distribution.ClassID][slot.ID] || teacherOccupied[load.TeacherID][slot.ID] {
						continue
					}
					roomID, roomOK, roomScore := pickCandidateRoom(subject, []uint{slot.ID}, dayOrder)
					if !roomOK {
						continue
					}
					score := classDayLoad[distribution.ClassID][dayOrder]*20 +
						teacherDayLoad[load.TeacherID][dayOrder]*14 +
						classSubjectDayLoad[subjectKey][dayOrder]*55 +
						roomScore +
						dayOrder*3 +
						slot.SessionOrder
					if score < best.score {
						best = candidate{slots: []generatedSlotAssignment{{SlotID: slot.ID, RoomID: roomID}}, score: score, roomID: roomID}
					}
				}
				continue
			}

			for startIdx := 0; startIdx+blockSize <= len(daySlots); startIdx += 1 {
				block := daySlots[startIdx : startIdx+blockSize]
				valid := true
				for idx := range block {
					if idx > 0 && block[idx].SessionOrder != block[idx-1].SessionOrder+1 {
						valid = false
						break
					}
					if classOccupied[distribution.ClassID][block[idx].ID] || teacherOccupied[load.TeacherID][block[idx].ID] {
						valid = false
						break
					}
				}
				if !valid {
					continue
				}
				blockSlotIDs := make([]uint, 0, len(block))
				for _, slot := range block {
					blockSlotIDs = append(blockSlotIDs, slot.ID)
				}
				roomID, roomOK, roomScore := pickCandidateRoom(subject, blockSlotIDs, dayOrder)
				if !roomOK {
					continue
				}
				score := classDayLoad[distribution.ClassID][dayOrder]*20 +
					teacherDayLoad[load.TeacherID][dayOrder]*14 +
					classSubjectDayLoad[subjectKey][dayOrder]*55 +
					roomScore +
					dayOrder*3 +
					block[0].SessionOrder
				if score < best.score {
					assignments := make([]generatedSlotAssignment, 0, len(block))
					for _, slot := range block {
						assignments = append(assignments, generatedSlotAssignment{SlotID: slot.ID, RoomID: roomID})
					}
					best = candidate{slots: assignments, score: score, roomID: roomID}
				}
			}
		}

		return best.slots
	}

	for _, distribution := range distributionQueue {
		load := loadByID[distribution.CurriculumTeacherLoad]
		if load.ID == 0 {
			issues = append(issues, fmt.Sprintf("Distribusi kelas %s tidak punya referensi beban guru yang valid.", distribution.ClassName))
			continue
		}
		ensureSlotMap(classOccupied, distribution.ClassID)
		ensureSlotMap(teacherOccupied, load.TeacherID)
		ensureDayLoad(classDayLoad, distribution.ClassID)
		ensureDayLoad(teacherDayLoad, load.TeacherID)
		subjectKey := classSubjectKey{ClassID: distribution.ClassID, SubjectID: load.CurriculumSubject}
		ensureSubjectDayLoad(subjectKey)

		selectedSlots := make([]generatedSlotAssignment, 0, distribution.WeeklyHours)
		remainingHours := distribution.WeeklyHours
		for remainingHours > 0 {
			blockSize := 1
			if remainingHours >= 2 {
				blockSize = 2
			}
			block := findBestBlock(distribution, load, blockSize)
			if len(block) == 0 && blockSize > 1 {
				block = findBestBlock(distribution, load, 1)
			}
			if len(block) == 0 {
				break
			}
			for _, assignedSlot := range block {
				var slot curriculumScheduleSlotRow
				for _, item := range scheduleSlots {
					if item.ID == assignedSlot.SlotID {
						slot = item
						break
					}
				}
				selectedSlots = append(selectedSlots, assignedSlot)
				classOccupied[distribution.ClassID][assignedSlot.SlotID] = true
				teacherOccupied[load.TeacherID][assignedSlot.SlotID] = true
				if assignedSlot.RoomID > 0 {
					ensureSlotMap(roomOccupied, assignedSlot.RoomID)
					ensureDayLoad(roomDayLoad, assignedSlot.RoomID)
					roomOccupied[assignedSlot.RoomID][assignedSlot.SlotID] = true
					roomDayLoad[assignedSlot.RoomID][slot.DayOrder] += 1
				}
				classDayLoad[distribution.ClassID][slot.DayOrder] += 1
				teacherDayLoad[load.TeacherID][slot.DayOrder] += 1
				classSubjectDayLoad[subjectKey][slot.DayOrder] += 1
			}
			remainingHours -= len(block)
		}

		if len(selectedSlots) == 0 {
			issues = append(issues, fmt.Sprintf("Tidak ada slot tersedia untuk %s mengajar %s di kelas %s.", load.TeacherName, load.SubjectName, distribution.ClassName))
			continue
		}
		if len(selectedSlots) < distribution.WeeklyHours {
			issues = append(issues, fmt.Sprintf("Alokasi %s di kelas %s hanya mendapat %d dari %d jam.", load.SubjectName, distribution.ClassName, len(selectedSlots), distribution.WeeklyHours))
		}

		assignmentKey := classSubjectKeyWithTeacher{
			ClassID:   distribution.ClassID,
			SubjectID: load.CurriculumSubject,
			TeacherID: load.TeacherID,
		}
		if existing := assignmentsByKey[assignmentKey]; existing != nil {
			existing.RequestedWeeklyHour += distribution.WeeklyHours
			existing.AssignedSlots = append(existing.AssignedSlots, selectedSlots...)
			continue
		}

		assignments = append(assignments, generatedAssignment{
			ClassID:             distribution.ClassID,
			ClassName:           distribution.ClassName,
			SubjectID:           load.CurriculumSubject,
			SubjectName:         load.SubjectName,
			SubjectCode:         load.SubjectCode,
			TeacherID:           load.TeacherID,
			TeacherName:         load.TeacherName,
			RequestedWeeklyHour: distribution.WeeklyHours,
			AssignedSlots:       selectedSlots,
		})
		assignmentsByKey[assignmentKey] = &assignments[len(assignments)-1]
	}

	if len(assignments) == 0 {
		return utils.Error(c, 400, "Generate gagal karena belum ada distribusi kelas yang bisa dijadwalkan")
	}

	type generatedLearningSubjectRow struct {
		ID uint `gorm:"column:id"`
	}
	learningSubjectByKey := map[string]uint{}
	for _, assignment := range assignments {
		key := fmt.Sprintf("%d:%d", assignment.ClassID, assignment.SubjectID)
		if learningSubjectByKey[key] != 0 {
			continue
		}

		var existing generatedLearningSubjectRow
		a.DB.Raw(`
			SELECT id
			FROM learning_subjects
			WHERE school_id = ? AND class_id = ? AND curriculum_subject_id = ? AND curriculum_auto_generated = true
			LIMIT 1
		`, schoolID, assignment.ClassID, assignment.SubjectID).Scan(&existing)

		description := fmt.Sprintf("Dibuat otomatis dari modul kurikulum admin. Kode: %s", strings.TrimSpace(assignment.SubjectCode))
		if existing.ID == 0 {
			a.DB.Raw(`
				INSERT INTO learning_subjects (
					school_id, class_id, teacher_id, name, description, curriculum_subject_id, curriculum_auto_generated, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, ?, ?, true, NOW(), NOW())
				RETURNING id
			`, schoolID, assignment.ClassID, assignment.TeacherID, assignment.SubjectName, nullIfEmpty(description), assignment.SubjectID).Scan(&existing)
		} else {
			a.DB.Exec(`
				UPDATE learning_subjects
				SET teacher_id = ?, name = ?, description = ?, updated_at = NOW()
				WHERE id = ?
			`, assignment.TeacherID, assignment.SubjectName, nullIfEmpty(description), existing.ID)
		}

		learningSubjectByKey[key] = existing.ID
	}

	a.DB.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, schoolID)
	for _, assignment := range assignments {
		learningSubjectID := learningSubjectByKey[fmt.Sprintf("%d:%d", assignment.ClassID, assignment.SubjectID)]
		for _, assignedSlot := range assignment.AssignedSlots {
			a.DB.Exec(`
				INSERT INTO curriculum_schedule_entries (
					school_id, class_id, curriculum_subject_id, teacher_id, schedule_slot_id, room_id, learning_subject_id, generated_at, created_at, updated_at
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW(), NOW())
			`, schoolID, assignment.ClassID, assignment.SubjectID, assignment.TeacherID, assignedSlot.SlotID, nullIfZero(int(assignedSlot.RoomID)), nullIfZero(int(learningSubjectID)))
		}
	}

	_, _, _, _, _, generatedEntries := a.loadCurriculumOverviewData(schoolID)
	classCounter := map[uint]struct{}{}
	subjectCounter := map[uint]struct{}{}
	for _, assignment := range assignments {
		classCounter[assignment.ClassID] = struct{}{}
		subjectCounter[assignment.SubjectID] = struct{}{}
	}

	return utils.Success(c, 200, "Success Generate Curriculum Schedule", fiber.Map{
		"generated_entries": generatedEntries,
		"issues":            issues,
		"summary": fiber.Map{
			"classes":            len(classCounter),
			"subjects":           len(subjectCounter),
			"teacher_loads":      len(teacherLoads),
			"generated_entries":  len(generatedEntries),
			"generated_subjects": len(learningSubjectByKey),
			"issues":             len(issues),
		},
	})
}

func (a *AppContext) loadCurriculumOverviewData(schoolID uint) ([]curriculumSubjectRow, []curriculumRoomRow, []curriculumTeacherLoadRow, []curriculumClassDistributionRow, []curriculumScheduleSlotRow, []curriculumScheduleEntryRow) {
	var subjects []curriculumSubjectRow
	var rooms []curriculumRoomRow
	var teacherLoads []curriculumTeacherLoadRow
	var classDistributions []curriculumClassDistributionRow
	var scheduleSlots []curriculumScheduleSlotRow
	var generatedEntries []curriculumScheduleEntryRow

	a.DB.Raw(curriculumSubjectQuery()+` WHERE cs.school_id = ? ORDER BY cs.name ASC`, schoolID).Scan(&subjects)
	if len(subjects) > 0 {
		var subjectAssignments []struct {
			CurriculumSubjectID uint `gorm:"column:curriculum_subject_id"`
			TeacherID           uint `gorm:"column:teacher_id"`
		}
		a.DB.Raw(`
			SELECT DISTINCT curriculum_subject_id, teacher_id
			FROM learning_subjects
			WHERE school_id = ?
			  AND curriculum_subject_id IS NOT NULL
			  AND teacher_id IS NOT NULL
			ORDER BY curriculum_subject_id ASC, teacher_id ASC
		`, schoolID).Scan(&subjectAssignments)

		teacherIDsBySubject := make(map[uint][]uint, len(subjectAssignments))
		for _, assignment := range subjectAssignments {
			if assignment.CurriculumSubjectID == 0 || assignment.TeacherID == 0 {
				continue
			}
			teacherIDsBySubject[assignment.CurriculumSubjectID] = append(
				teacherIDsBySubject[assignment.CurriculumSubjectID],
				assignment.TeacherID,
			)
		}

		for idx := range subjects {
			subjects[idx].AssignedTeacherIDs = teacherIDsBySubject[subjects[idx].ID]
		}
	}

	a.DB.Raw(`
		SELECT id, school_id, code, name, room_type, color, capacity, is_active, COALESCE(notes, '') AS notes
		FROM curriculum_rooms
		WHERE school_id = ?
		ORDER BY room_type ASC, code ASC, name ASC
	`, schoolID).Scan(&rooms)

	a.DB.Raw(curriculumTeacherLoadQuery()+` WHERE ctl.school_id = ? ORDER BY teacher_name ASC, subject_name ASC`, schoolID).Scan(&teacherLoads)
	a.DB.Raw(curriculumClassDistributionQuery()+` WHERE ccd.school_id = ? ORDER BY class_name ASC, teacher_name ASC, subject_name ASC`, schoolID).Scan(&classDistributions)

	a.DB.Raw(`
		SELECT id, school_id, day_name, day_order, session_order, start_time, end_time, COALESCE(label, '') AS label
		FROM curriculum_schedule_slots
		WHERE school_id = ?
		ORDER BY day_order ASC, session_order ASC
	`, schoolID).Scan(&scheduleSlots)

	a.DB.Raw(`
		SELECT
			cse.id,
			cse.school_id,
			cse.class_id,
			cse.curriculum_subject_id,
			cse.teacher_id,
			cse.schedule_slot_id,
			COALESCE(cse.room_id, 0) AS room_id,
			COALESCE(cse.learning_subject_id, 0) AS learning_subject_id,
			TO_CHAR(cse.generated_at, 'YYYY-MM-DD HH24:MI:SS') AS generated_at,
			COALESCE(cls.class_name, '-') AS class_name,
			COALESCE(u.full_name, u.username, '-') AS teacher_name,
			COALESCE(cs.name, '-') AS subject_name,
			COALESCE(cs.code, '') AS subject_code,
			COALESCE(room.name, '') AS room_name,
			COALESCE(room.code, '') AS room_code,
			COALESCE(room.room_type, '') AS room_type,
			COALESCE(room.color, '') AS room_color,
			slot.day_name,
			slot.day_order,
			slot.session_order,
			slot.start_time,
			slot.end_time,
			COALESCE(slot.label, '') AS slot_label
		FROM curriculum_schedule_entries cse
		LEFT JOIN class cls ON cls.id = cse.class_id
		LEFT JOIN users u ON u.id = cse.teacher_id
		LEFT JOIN curriculum_subjects cs ON cs.id = cse.curriculum_subject_id
		LEFT JOIN curriculum_rooms room ON room.id = cse.room_id
		LEFT JOIN curriculum_schedule_slots slot ON slot.id = cse.schedule_slot_id
		WHERE cse.school_id = ?
		ORDER BY cls.class_name ASC, slot.day_order ASC, slot.session_order ASC, subject_name ASC
	`, schoolID).Scan(&generatedEntries)

	for idx := range generatedEntries {
		if generatedEntries[idx].GeneratedAt != "" {
			parsed := parseJakartaTimestamp(generatedEntries[idx].GeneratedAt)
			if parsed != nil {
				generatedEntries[idx].GeneratedAt = parsed.Format(jakartaDateTimeLayout)
			}
		}
	}

	return subjects, rooms, teacherLoads, classDistributions, scheduleSlots, generatedEntries
}

func curriculumSubjectQuery() string {
	return `
		SELECT
			cs.id,
			cs.school_id,
			COALESCE(cs.code, '') AS code,
			cs.name,
			COALESCE(cs.description, '') AS description,
			cs.weekly_hours,
			COALESCE(cs.required_room_type, 'CLASSROOM') AS required_room_type,
			COALESCE(cs.preferred_room_id, 0) AS preferred_room_id,
			COALESCE(room.code, '') AS preferred_room_code,
			COALESCE(room.name, '') AS preferred_room_name,
			COALESCE(room.color, '') AS preferred_room_color
		FROM curriculum_subjects cs
		LEFT JOIN curriculum_rooms room ON room.id = cs.preferred_room_id
	`
}

func normalizeCurriculumRoomType(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "CLASSROOM"
	}
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func normalizeRoomColor(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "#E2E8F0"
	}
	if strings.HasPrefix(trimmed, "#") && len(trimmed) == 7 {
		return strings.ToUpper(trimmed)
	}
	return trimmed
}

func (a *AppContext) curriculumRoomBelongsToSchool(schoolID uint, roomID uint) bool {
	if roomID == 0 {
		return true
	}
	var count int64
	a.DB.Raw(`SELECT COUNT(*) FROM curriculum_rooms WHERE id = ? AND school_id = ?`, roomID, schoolID).Scan(&count)
	return count > 0
}

func curriculumTeacherLoadQuery() string {
	return `
		SELECT
			ctl.id,
			ctl.school_id,
			ctl.teacher_id,
			ctl.curriculum_subject_id,
			ctl.max_weekly_hours,
			COALESCE(ctl.notes, '') AS notes,
			COALESCE(u.full_name, u.username, '-') AS teacher_name,
			cs.name AS subject_name,
			COALESCE(cs.code, '') AS subject_code,
			COALESCE(dist.total_hours, 0) AS distributed_hours,
			GREATEST(ctl.max_weekly_hours - COALESCE(dist.total_hours, 0), 0) AS remaining_hours
		FROM curriculum_teacher_loads ctl
		LEFT JOIN users u ON u.id = ctl.teacher_id
		LEFT JOIN curriculum_subjects cs ON cs.id = ctl.curriculum_subject_id
		LEFT JOIN (
			SELECT curriculum_teacher_load_id, SUM(weekly_hours) AS total_hours
			FROM curriculum_class_distributions
			GROUP BY curriculum_teacher_load_id
		) dist ON dist.curriculum_teacher_load_id = ctl.id
	`
}

func curriculumClassDistributionQuery() string {
	return `
		SELECT
			ccd.id,
			ccd.school_id,
			ccd.curriculum_teacher_load_id,
			ccd.class_id,
			ccd.weekly_hours,
			COALESCE(ccd.notes, '') AS notes,
			ctl.teacher_id,
			COALESCE(u.full_name, u.username, '-') AS teacher_name,
			COALESCE(cls.class_name, '-') AS class_name,
			ctl.curriculum_subject_id,
			cs.name AS subject_name,
			COALESCE(cs.code, '') AS subject_code,
			ctl.max_weekly_hours AS load_capacity
		FROM curriculum_class_distributions ccd
		LEFT JOIN curriculum_teacher_loads ctl ON ctl.id = ccd.curriculum_teacher_load_id
		LEFT JOIN users u ON u.id = ctl.teacher_id
		LEFT JOIN class cls ON cls.id = ccd.class_id
		LEFT JOIN curriculum_subjects cs ON cs.id = ctl.curriculum_subject_id
	`
}

func (a *AppContext) validateCurriculumDistributionConflict(schoolID uint, currentID interface{}, teacherLoadID uint, classID uint) string {
	var currentLoad struct {
		ID                uint   `gorm:"column:id"`
		CurriculumSubject uint   `gorm:"column:curriculum_subject_id"`
		SubjectName       string `gorm:"column:subject_name"`
		TeacherName       string `gorm:"column:teacher_name"`
	}
	a.DB.Raw(curriculumTeacherLoadQuery()+` WHERE ctl.id = ? AND ctl.school_id = ?`, teacherLoadID, schoolID).Scan(&currentLoad)
	if currentLoad.ID == 0 {
		return ""
	}

	var conflict struct {
		ID          uint   `gorm:"column:id"`
		TeacherName string `gorm:"column:teacher_name"`
		SubjectName string `gorm:"column:subject_name"`
		ClassName   string `gorm:"column:class_name"`
	}
	a.DB.Raw(curriculumClassDistributionQuery()+`
		WHERE ccd.school_id = ?
		  AND ccd.class_id = ?
		  AND ctl.curriculum_subject_id = ?
		  AND ccd.id <> COALESCE(NULLIF(?, ''), '0')::BIGINT
		LIMIT 1
	`, schoolID, classID, currentLoad.CurriculumSubject, fmt.Sprintf("%v", currentID)).Scan(&conflict)
	if conflict.ID == 0 {
		return ""
	}

	return fmt.Sprintf("Mapel %s di kelas %s sudah ditetapkan untuk %s. Satu kelas-mapel hanya boleh memiliki satu guru pengampu.", conflict.SubjectName, conflict.ClassName, conflict.TeacherName)
}
