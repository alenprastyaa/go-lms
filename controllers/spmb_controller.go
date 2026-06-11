package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/models"
	"lms/services"
	"lms/utils"
)

var validSPMBStatuses = map[string]struct{}{
	"DRAFT":                {},
	"SUBMITTED":            {},
	"VERIFIED":             {},
	"NEED_REVISION":        {},
	"TEST_SCHEDULED":       {},
	"INTERVIEW":            {},
	"ACCEPTED":             {},
	"ACCEPTED_OTHER_MAJOR": {},
	"REJECTED":             {},
	"RE_REGISTERED":        {},
	"CONVERTED_TO_STUDENT": {},
}

func normalizeSPMBStatus(value string) string {
	status := strings.ToUpper(strings.TrimSpace(value))
	status = strings.ReplaceAll(status, " ", "_")
	status = strings.ReplaceAll(status, "-", "_")
	if status == "" {
		return "SUBMITTED"
	}
	if _, ok := validSPMBStatuses[status]; ok {
		return status
	}
	return ""
}

func (a *AppContext) GetSPMBOverview(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var summary []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT status, COUNT(*)::int AS total
		FROM spmb_applicants
		WHERE school_id = ?
		GROUP BY status
		ORDER BY status ASC
	`, schoolID).Scan(&summary).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat ringkasan SPMB", err.Error())
	}

	var totals struct {
		Applicants int `json:"applicants"`
		Majors     int `json:"majors"`
		Accepted   int `json:"accepted"`
		Converted  int `json:"converted"`
	}
	_ = a.DB.Raw(`
		SELECT
			(SELECT COUNT(*) FROM spmb_applicants WHERE school_id = ?)::int AS applicants,
			(SELECT COUNT(*) FROM majors WHERE school_id = ? AND is_active = true)::int AS majors,
			(SELECT COUNT(*) FROM spmb_applicants WHERE school_id = ? AND status IN ('ACCEPTED', 'ACCEPTED_OTHER_MAJOR', 'RE_REGISTERED'))::int AS accepted,
			(SELECT COUNT(*) FROM spmb_applicants WHERE school_id = ? AND status = 'CONVERTED_TO_STUDENT')::int AS converted
	`, schoolID, schoolID, schoolID, schoolID).Scan(&totals).Error

	var latest []map[string]interface{}
	_ = a.DB.Raw(spmbApplicantSelectSQL()+`
		WHERE a.school_id = ?
		ORDER BY a.created_at DESC
		LIMIT 8
	`, schoolID).Scan(&latest).Error

	return utils.Success(c, 200, "Success Get SPMB Overview", fiber.Map{
		"totals":      totals,
		"by_status":   summary,
		"latest":      latest,
		"generatedAt": time.Now(),
	})
}

func (a *AppContext) GetSPMBApplicants(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	status := normalizeSPMBStatus(c.Query("status"))
	keyword := strings.TrimSpace(c.Query("q"))
	majorID := utils.ToInt(c.Query("major_id"), 0)

	where := []string{"a.school_id = ?"}
	args := []interface{}{schoolID}
	if status != "" {
		where = append(where, "a.status = ?")
		args = append(args, status)
	}
	if keyword != "" {
		where = append(where, "(a.full_name ILIKE ? OR a.registration_number ILIKE ? OR a.phone_number ILIKE ? OR COALESCE(a.nisn, '') ILIKE ?)")
		pattern := "%" + keyword + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if majorID > 0 {
		where = append(where, "? IN (a.first_major_id, a.second_major_id, a.third_major_id, a.accepted_major_id)")
		args = append(args, majorID)
	}

	var rows []map[string]interface{}
	if err := a.DB.Raw(spmbApplicantSelectSQL()+" WHERE "+strings.Join(where, " AND ")+` ORDER BY a.created_at DESC`, args...).Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat pendaftar SPMB", err.Error())
	}
	return utils.Success(c, 200, "Success Get SPMB Applicants", rows)
}

func (a *AppContext) CreateSPMBApplicant(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	row, err := a.createSPMBApplicantFromBody(c, schoolID, &userID)
	if err != nil {
		return err
	}
	return utils.Success(c, 201, "Pendaftar SPMB berhasil dibuat", row)
}

func (a *AppContext) RegisterSPMBApplicantPublic(c *fiber.Ctx) error {
	var schoolReq struct {
		SchoolID uint `json:"school_id"`
	}
	_ = c.BodyParser(&schoolReq)
	if schoolReq.SchoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}

	var enabled bool
	if err := a.DB.Table("schools").Select("spmb_module_enabled").Where("id = ?", schoolReq.SchoolID).Scan(&enabled).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa modul SPMB", err.Error())
	}
	if !enabled {
		return utils.Error(c, 403, "Pendaftaran SPMB belum dibuka")
	}

	row, err := a.createSPMBApplicantFromBody(c, schoolReq.SchoolID, nil)
	if err != nil {
		return err
	}

	token, tokenHash, expiresAt, err := generateSPMBAccessToken()
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat link akses", err.Error())
	}
	if err := a.DB.Model(&models.SPMBApplicant{}).Where("id = ?", row["id"]).Updates(map[string]interface{}{
		"access_token_hash":       tokenHash,
		"access_token_expires_at": expiresAt,
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan link akses", err.Error())
	}

	row["access_url"] = buildSPMBStatusURL(token)
	return utils.Success(c, 201, "Pendaftaran SPMB berhasil dikirim", row)
}

func (a *AppContext) GetSPMBPublicOptions(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Query("school_id"), 0))
	if schoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}

	var school struct {
		ID                uint   `json:"id"`
		Name              string `json:"name"`
		SPMBModuleEnabled bool   `json:"spmb_module_enabled"`
	}
	if err := a.DB.Table("schools").Select("id, name, spmb_module_enabled").Where("id = ?", schoolID).Take(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}
	if !school.SPMBModuleEnabled {
		return utils.Error(c, 403, "Pendaftaran SPMB belum dibuka")
	}

	var majors []map[string]interface{}
	if err := a.DB.Table("majors").
		Select("id, name, code, quota").
		Where("school_id = ? AND is_active = true", schoolID).
		Order("name ASC").
		Find(&majors).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat jurusan", err.Error())
	}

	return utils.Success(c, 200, "Success Get SPMB Options", fiber.Map{
		"school": fiber.Map{"id": school.ID, "name": school.Name},
		"majors": majors,
	})
}

func (a *AppContext) createSPMBApplicantFromBody(c *fiber.Ctx, schoolID uint, createdBy *uint) (map[string]interface{}, error) {
	var body struct {
		FullName      string `json:"full_name"`
		BirthPlace    string `json:"birth_place"`
		BirthDate     string `json:"birth_date"`
		Gender        string `json:"gender"`
		NISN          string `json:"nisn"`
		OriginSchool  string `json:"origin_school"`
		ParentName    string `json:"parent_name"`
		PhoneNumber   string `json:"phone_number"`
		Email         string `json:"email"`
		Address       string `json:"address"`
		FirstMajorID  *uint  `json:"first_major_id"`
		SecondMajorID *uint  `json:"second_major_id"`
		ThirdMajorID  *uint  `json:"third_major_id"`
		Notes         string `json:"notes"`
		Status        string `json:"status"`
	}
	_ = c.BodyParser(&body)

	fullName := strings.TrimSpace(body.FullName)
	phone := strings.TrimSpace(body.PhoneNumber)
	if fullName == "" {
		return nil, utils.Error(c, 400, "Nama lengkap wajib diisi")
	}
	if phone == "" {
		return nil, utils.Error(c, 400, "Nomor WhatsApp wajib diisi")
	}
	if body.FirstMajorID == nil || *body.FirstMajorID == 0 {
		return nil, utils.Error(c, 400, "Pilihan jurusan pertama wajib diisi")
	}
	if err := a.validateMajorChoices(schoolID, body.FirstMajorID, body.SecondMajorID, body.ThirdMajorID); err != nil {
		return nil, utils.Error(c, 400, err.Error())
	}

	status := normalizeSPMBStatus(body.Status)
	if status == "" {
		return nil, utils.Error(c, 400, "Status SPMB tidak valid")
	}

	var birthDate interface{} = nil
	if strings.TrimSpace(body.BirthDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(body.BirthDate))
		if err != nil {
			return nil, utils.Error(c, 400, "Tanggal lahir harus format YYYY-MM-DD")
		}
		birthDate = parsed
	}

	registrationNumber, err := a.nextSPMBRegistrationNumber(schoolID)
	if err != nil {
		return nil, utils.Error(c, 500, "Gagal membuat nomor pendaftaran", err.Error())
	}

	var row map[string]interface{}
	if err := a.DB.Raw(`
		INSERT INTO spmb_applicants (
			school_id, registration_number, full_name, birth_place, birth_date, gender, nisn,
			origin_school, parent_name, phone_number, email, address,
			first_major_id, second_major_id, third_major_id, status, notes, created_by,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		RETURNING id, school_id, registration_number, full_name, phone_number, status, first_major_id, second_major_id, third_major_id, created_at
	`, schoolID, registrationNumber, fullName, nullTrim(body.BirthPlace), birthDate, nullTrim(body.Gender), nullTrim(body.NISN), nullTrim(body.OriginSchool), nullTrim(body.ParentName), phone, nullTrim(body.Email), nullTrim(body.Address), body.FirstMajorID, body.SecondMajorID, body.ThirdMajorID, status, nullTrim(body.Notes), createdBy).Scan(&row).Error; err != nil {
		return nil, utils.Error(c, 500, "Gagal menyimpan pendaftar SPMB", err.Error())
	}
	return row, nil
}

func (a *AppContext) UpdateSPMBApplicant(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var current models.SPMBApplicant
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Pendaftar SPMB tidak ditemukan")
	}

	var body struct {
		FullName        *string `json:"full_name"`
		BirthPlace      *string `json:"birth_place"`
		BirthDate       *string `json:"birth_date"`
		Gender          *string `json:"gender"`
		NISN            *string `json:"nisn"`
		OriginSchool    *string `json:"origin_school"`
		ParentName      *string `json:"parent_name"`
		PhoneNumber     *string `json:"phone_number"`
		Email           *string `json:"email"`
		Address         *string `json:"address"`
		FirstMajorID    *uint   `json:"first_major_id"`
		SecondMajorID   *uint   `json:"second_major_id"`
		ThirdMajorID    *uint   `json:"third_major_id"`
		AcceptedMajorID *uint   `json:"accepted_major_id"`
		Status          *string `json:"status"`
		Notes           *string `json:"notes"`
		RevisionNote    *string `json:"revision_note"`
	}
	_ = c.BodyParser(&body)
	var raw map[string]interface{}
	_ = c.BodyParser(&raw)

	firstMajorID := current.FirstMajorID
	secondMajorID := current.SecondMajorID
	thirdMajorID := current.ThirdMajorID
	if _, ok := raw["first_major_id"]; ok {
		parsed, err := nullableUintFromRaw(raw["first_major_id"])
		if err != nil {
			return utils.Error(c, 400, "Pilihan jurusan pertama tidak valid")
		}
		firstMajorID = parsed
	}
	if _, ok := raw["second_major_id"]; ok {
		parsed, err := nullableUintFromRaw(raw["second_major_id"])
		if err != nil {
			return utils.Error(c, 400, "Pilihan jurusan kedua tidak valid")
		}
		secondMajorID = parsed
	}
	if _, ok := raw["third_major_id"]; ok {
		parsed, err := nullableUintFromRaw(raw["third_major_id"])
		if err != nil {
			return utils.Error(c, 400, "Pilihan jurusan ketiga tidak valid")
		}
		thirdMajorID = parsed
	}
	if err := a.validateMajorChoices(schoolID, firstMajorID, secondMajorID, thirdMajorID); err != nil {
		return utils.Error(c, 400, err.Error())
	}

	updates := map[string]interface{}{
		"updated_at": gorm.Expr("NOW()"),
	}
	if body.FullName != nil {
		value := strings.TrimSpace(*body.FullName)
		if value == "" {
			return utils.Error(c, 400, "Nama lengkap wajib diisi")
		}
		updates["full_name"] = value
	}
	if body.PhoneNumber != nil {
		value := strings.TrimSpace(*body.PhoneNumber)
		if value == "" {
			return utils.Error(c, 400, "Nomor WhatsApp wajib diisi")
		}
		updates["phone_number"] = value
	}
	if body.BirthDate != nil {
		value := strings.TrimSpace(*body.BirthDate)
		if value == "" {
			updates["birth_date"] = nil
		} else {
			parsed, err := time.Parse("2006-01-02", value)
			if err != nil {
				return utils.Error(c, 400, "Tanggal lahir harus format YYYY-MM-DD")
			}
			updates["birth_date"] = parsed
		}
	}
	if body.Status != nil {
		status := normalizeSPMBStatus(*body.Status)
		if status == "" {
			return utils.Error(c, 400, "Status SPMB tidak valid")
		}
		updates["status"] = status
	}
	if _, ok := raw["first_major_id"]; ok {
		if firstMajorID == nil || *firstMajorID == 0 {
			return utils.Error(c, 400, "Pilihan jurusan pertama wajib diisi")
		}
		updates["first_major_id"] = firstMajorID
	}
	if _, ok := raw["second_major_id"]; ok {
		updates["second_major_id"] = secondMajorID
	}
	if _, ok := raw["third_major_id"]; ok {
		updates["third_major_id"] = thirdMajorID
	}
	if _, ok := raw["accepted_major_id"]; ok {
		acceptedMajorID, err := nullableUintFromRaw(raw["accepted_major_id"])
		if err != nil {
			return utils.Error(c, 400, "Jurusan diterima tidak valid")
		}
		if acceptedMajorID != nil && !a.majorBelongsToSchool(*acceptedMajorID, schoolID) {
			return utils.Error(c, 400, "Jurusan diterima tidak valid")
		}
		updates["accepted_major_id"] = acceptedMajorID
	}
	optionalTextUpdates(updates, map[string]*string{
		"birth_place":   body.BirthPlace,
		"gender":        body.Gender,
		"nisn":          body.NISN,
		"origin_school": body.OriginSchool,
		"parent_name":   body.ParentName,
		"email":         body.Email,
		"address":       body.Address,
		"notes":         body.Notes,
		"revision_note": body.RevisionNote,
	})

	if err := a.DB.Model(&current).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui pendaftar SPMB", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(spmbApplicantSelectSQL()+" WHERE a.id = ? AND a.school_id = ?", current.ID, schoolID).Scan(&row)
	return utils.Success(c, 200, "Pendaftar SPMB berhasil diperbarui", row)
}

func (a *AppContext) DeleteSPMBApplicant(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var current models.SPMBApplicant
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Pendaftar SPMB tidak ditemukan")
	}
	if current.ConvertedUserID != nil {
		return utils.Error(c, 400, "Pendaftar yang sudah menjadi siswa tidak bisa dihapus")
	}
	if err := a.DB.Delete(&current).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus pendaftar SPMB", err.Error())
	}
	return utils.Success(c, 200, "Pendaftar SPMB berhasil dihapus", fiber.Map{"id": current.ID})
}

func (a *AppContext) SendSPMBApplicantLink(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var applicant models.SPMBApplicant
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&applicant).Error; err != nil {
		return utils.Error(c, 404, "Pendaftar SPMB tidak ditemukan")
	}

	token, tokenHash, expiresAt, err := generateSPMBAccessToken()
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat link akses", err.Error())
	}
	link := buildSPMBStatusURL(token)
	phone, phoneErr := normalizeWhatsAppPhone(&applicant.PhoneNumber)
	if phoneErr != nil {
		return utils.Error(c, 400, phoneErr.Error())
	}

	message := fmt.Sprintf(`Halo %s.

Berikut link akses SPMB Anda:
Nomor Pendaftaran: *%s*
Link Status: %s

Link berlaku sampai %s.`, applicant.FullName, applicant.RegistrationNumber, link, expiresAt.Format("02 Jan 2006 15:04"))

	if _, err := services.SendWhatsAppMessage(phone, message); err != nil {
		return utils.Error(c, 500, "Gagal mengirim link WhatsApp", err.Error())
	}

	now := time.Now().UTC()
	if err := a.DB.Model(&applicant).Updates(map[string]interface{}{
		"access_token_hash":       tokenHash,
		"access_token_expires_at": expiresAt,
		"last_link_sent_at":       now,
		"updated_at":              now,
	}).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan riwayat link", err.Error())
	}

	return utils.Success(c, 200, "Link status SPMB berhasil dikirim", fiber.Map{
		"id":         applicant.ID,
		"sent_to":    phone,
		"expires_at": expiresAt,
	})
}

func (a *AppContext) ConvertSPMBApplicantToStudent(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var body struct {
		ClassID *uint `json:"class_id"`
	}
	_ = c.BodyParser(&body)

	var applicant models.SPMBApplicant
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&applicant).Error; err != nil {
		return utils.Error(c, 404, "Pendaftar SPMB tidak ditemukan")
	}
	if applicant.ConvertedUserID != nil || applicant.Status == "CONVERTED_TO_STUDENT" {
		return utils.Error(c, 400, "Pendaftar sudah dikonversi menjadi siswa")
	}
	if applicant.Status != "ACCEPTED" && applicant.Status != "ACCEPTED_OTHER_MAJOR" && applicant.Status != "RE_REGISTERED" {
		return utils.Error(c, 400, "Hanya pendaftar yang sudah diterima atau daftar ulang yang bisa dijadikan siswa")
	}

	allowedClasses, err := a.allowedSPMBConvertClasses(schoolID, applicant)
	if err != nil {
		return utils.Error(c, 500, "Gagal memuat kelas tujuan SPMB", err.Error())
	}
	if len(allowedClasses) == 0 {
		return utils.Error(c, 400, "Belum ada kelas level terendah yang sesuai jurusan pendaftar")
	}

	if body.ClassID == nil || *body.ClassID == 0 {
		return utils.Error(c, 400, "Kelas awal wajib dipilih dari kelas level terendah yang sesuai jurusan pendaftar")
	}

	selectedAllowed := false
	for _, classID := range allowedClasses {
		if classID == *body.ClassID {
			selectedAllowed = true
			break
		}
	}
	if !selectedAllowed {
		return utils.Error(c, 400, "Kelas tujuan harus berasal dari jurusan pendaftar dan level kelas terendah")
	}

	existingUsernames, _ := loadUsernameSet(a.DB)
	rawPassword := generateStudentPassword()
	username := nextAvailableUsername(applicant.FullName, "", existingUsernames)
	hashedPassword, encryptedPassword, err := hashAndStoreRawPassword(rawPassword)
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat password siswa", err.Error())
	}

	student := models.User{
		FullName:                  utils.StringPtr(applicant.FullName),
		Username:                  username,
		Password:                  hashedPassword,
		Role:                      "SISWA",
		SchoolID:                  &schoolID,
		ClassID:                   body.ClassID,
		ParentEmail:               applicant.Email,
		PhoneNumber:               utils.StringPtr(applicant.PhoneNumber),
		InitialPasswordCiphertext: utils.StringPtr(encryptedPassword),
	}

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&student).Error; err != nil {
			return err
		}
		if student.ClassID != nil && *student.ClassID > 0 {
			if err := ensureParentAccountLinkedTx(tx, schoolID, student.ID, student.PhoneNumber); err != nil {
				return err
			}
			if err := ensureInitialStudentClassEnrollmentTx(tx, schoolID, student.ID, *student.ClassID, &userID); err != nil {
				return err
			}
		}
		return tx.Model(&models.SPMBApplicant{}).Where("id = ? AND school_id = ?", applicant.ID, schoolID).Updates(map[string]interface{}{
			"status":            "CONVERTED_TO_STUDENT",
			"converted_user_id": student.ID,
			"updated_at":        time.Now().UTC(),
		}).Error
	}); err != nil {
		return utils.Error(c, 500, "Gagal menjadikan pendaftar sebagai siswa", err.Error())
	}

	return utils.Success(c, 200, "Pendaftar berhasil dijadikan siswa", fiber.Map{
		"applicant_id": applicant.ID,
		"student_id":   student.ID,
		"username":     student.Username,
		"password":     rawPassword,
	})
}

func (a *AppContext) allowedSPMBConvertClasses(schoolID uint, applicant models.SPMBApplicant) ([]uint, error) {
	majorIDs := orderedSPMBApplicantMajorIDs(applicant)
	if len(majorIDs) == 0 {
		return nil, nil
	}

	type classOption struct {
		ID              uint  `gorm:"column:id"`
		ClassLevelOrder *int  `gorm:"column:class_level_order"`
		MajorID         *uint `gorm:"column:major_id"`
	}

	var rows []classOption
	if err := a.DB.Raw(`
		SELECT c.id, c.major_id, cl.sort_order AS class_level_order
		FROM class c
		LEFT JOIN class_levels cl ON cl.id = c.class_level_id
		WHERE c.school_id = ?
		  AND c.major_id IN ?
		ORDER BY COALESCE(cl.sort_order, 9999) ASC, c.class_name ASC
	`, schoolID, majorIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	lowestOrder := int(^uint(0) >> 1)
	foundOrderedLevel := false
	for _, row := range rows {
		if row.ClassLevelOrder == nil {
			continue
		}
		if !foundOrderedLevel || *row.ClassLevelOrder < lowestOrder {
			lowestOrder = *row.ClassLevelOrder
			foundOrderedLevel = true
		}
	}

	allowed := make([]uint, 0, len(rows))
	for _, row := range rows {
		if foundOrderedLevel {
			if row.ClassLevelOrder == nil || *row.ClassLevelOrder != lowestOrder {
				continue
			}
		}
		allowed = append(allowed, row.ID)
	}
	return allowed, nil
}

func orderedSPMBApplicantMajorIDs(applicant models.SPMBApplicant) []uint {
	seen := map[uint]struct{}{}
	ordered := make([]uint, 0, 4)
	appendID := func(value *uint) {
		if value == nil || *value == 0 {
			return
		}
		if _, exists := seen[*value]; exists {
			return
		}
		seen[*value] = struct{}{}
		ordered = append(ordered, *value)
	}

	appendID(applicant.AcceptedMajorID)
	appendID(applicant.FirstMajorID)
	appendID(applicant.SecondMajorID)
	appendID(applicant.ThirdMajorID)
	return ordered
}

func (a *AppContext) GetSPMBApplicantPublicStatus(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Params("token"))
	if token == "" {
		return utils.Error(c, 400, "Token akses wajib diisi")
	}
	tokenHash := hashSPMBAccessToken(token)
	var row map[string]interface{}
	if err := a.DB.Raw(spmbApplicantSelectSQL()+`
		WHERE a.access_token_hash = ?
		  AND a.access_token_expires_at IS NOT NULL
		  AND a.access_token_expires_at > NOW()
	`, tokenHash).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat status SPMB", err.Error())
	}
	if len(row) == 0 {
		return utils.Error(c, 404, "Link status tidak valid atau sudah kedaluwarsa")
	}
	delete(row, "access_token_hash")
	return utils.Success(c, 200, "Success Get SPMB Status", row)
}

func (a *AppContext) validateMajorChoices(schoolID uint, ids ...*uint) error {
	seen := map[uint]struct{}{}
	for _, id := range ids {
		if id == nil || *id == 0 {
			continue
		}
		if _, exists := seen[*id]; exists {
			return fmt.Errorf("Pilihan jurusan tidak boleh sama")
		}
		seen[*id] = struct{}{}
		if !a.majorBelongsToSchool(*id, schoolID) {
			return fmt.Errorf("Pilihan jurusan tidak valid")
		}
	}
	return nil
}

func (a *AppContext) nextSPMBRegistrationNumber(schoolID uint) (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("SPMB-%d-", year)
	for i := 0; i < 5; i++ {
		var count int64
		if err := a.DB.Table("spmb_applicants").
			Where("school_id = ? AND registration_number LIKE ?", schoolID, prefix+"%").
			Count(&count).Error; err != nil {
			return "", err
		}
		number := fmt.Sprintf("%s%06d", prefix, count+1+int64(i))
		var exists int64
		a.DB.Table("spmb_applicants").Where("registration_number = ?", number).Count(&exists)
		if exists == 0 {
			return number, nil
		}
	}
	return "", fmt.Errorf("nomor pendaftaran gagal dibuat")
}

func spmbApplicantSelectSQL() string {
	return `
		SELECT
			a.id, a.school_id, a.registration_number, a.full_name, a.birth_place, a.birth_date,
			a.gender, a.nisn, a.origin_school, a.parent_name, a.phone_number, a.email, a.address,
			a.first_major_id, fm.name AS first_major_name, fm.code AS first_major_code,
			a.second_major_id, sm.name AS second_major_name, sm.code AS second_major_code,
			a.third_major_id, tm.name AS third_major_name, tm.code AS third_major_code,
			a.accepted_major_id, am.name AS accepted_major_name, am.code AS accepted_major_code,
			a.status, a.notes, a.revision_note, a.last_link_sent_at, a.access_token_expires_at,
			a.converted_user_id, a.created_by, a.created_at, a.updated_at
		FROM spmb_applicants a
		LEFT JOIN majors fm ON fm.id = a.first_major_id
		LEFT JOIN majors sm ON sm.id = a.second_major_id
		LEFT JOIN majors tm ON tm.id = a.third_major_id
		LEFT JOIN majors am ON am.id = a.accepted_major_id
	`
}

func generateSPMBAccessToken() (string, string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashSPMBAccessToken(token), time.Now().UTC().Add(14 * 24 * time.Hour), nil
}

func hashSPMBAccessToken(token string) string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	sum := sha256.Sum256([]byte(secret + ":spmb:" + strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func buildSPMBStatusURL(token string) string {
	baseURL := strings.TrimSpace(os.Getenv("SPMB_PUBLIC_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = "https://lms.school-system.my.id"
	}
	return strings.TrimRight(baseURL, "/") + "/spmb/status/" + token
}

func nullTrim(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func optionalTextUpdates(updates map[string]interface{}, values map[string]*string) {
	for key, value := range values {
		if value == nil {
			continue
		}
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			updates[key] = nil
		} else {
			updates[key] = trimmed
		}
	}
}

func nullableUintFromRaw(value interface{}) (*uint, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return nil, nil
		}
		id := uint(v)
		return &id, nil
	case int:
		if v <= 0 {
			return nil, nil
		}
		id := uint(v)
		return &id, nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, nil
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed <= 0 {
			return nil, err
		}
		id := uint(parsed)
		return &id, nil
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return nil, nil
		}
		parsed, err := strconv.Atoi(text)
		if err != nil || parsed <= 0 {
			return nil, err
		}
		id := uint(parsed)
		return &id, nil
	}
}
