package controllers

import (
	archivezip "archive/zip"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

type studentAccountExportRow struct {
	FullName string
	Username string
	Password string
}

const (
	failedLoginAttemptLimit = 5
	failedLoginLockDuration = time.Minute
)

func (a *AppContext) RegisterUser(c *fiber.Ctx) error {
	var body struct {
		FullName string `json:"full_name"`
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
		SchoolID *uint  `json:"school_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	username := strings.TrimSpace(body.Username)
	if username == "" {
		return utils.Error(c, 400, "Username wajib diisi")
	}
	if err := ensureUsernameAvailable(a.DB, username, 0); err != nil {
		return utils.Error(c, 400, "Username sudah digunakan", err.Error())
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	user := models.User{FullName: utils.StringPtr(body.FullName), Username: username, Password: string(hash), Role: body.Role, SchoolID: body.SchoolID}
	if strings.EqualFold(strings.TrimSpace(body.Role), "SISWA") {
		if encrypted, err := encryptAccountPassword(body.Password); err == nil {
			user.InitialPasswordCiphertext = utils.StringPtr(encrypted)
		}
	}
	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Registration failed", err.Error())
	}
	return utils.Success(c, 201, "User registered successfully", fiber.Map{
		"id": user.ID, "username": user.Username, "role": user.Role, "school_id": user.SchoolID,
	})
}

func (a *AppContext) Login(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = c.BodyParser(&body)

	var user models.User
	if err := a.DB.Where("username = ?", body.Username).First(&user).Error; err != nil {
		return utils.Error(c, 404, "User not found")
	}

	now := time.Now().UTC()
	if user.FailedLoginLockedUntil != nil {
		if user.FailedLoginLockedUntil.After(now) {
			return utils.ErrorData(c, 403, "Akun terkunci sementara karena 5 kali gagal login. Coba lagi setelah 1 menit.", fiber.Map{
				"locked_until": user.FailedLoginLockedUntil.UTC().Format(time.RFC3339),
			})
		}
		if err := a.clearFailedLoginState(user.ID); err != nil {
			return utils.Error(c, 500, "Gagal memperbarui status login", err.Error())
		}
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)) != nil {
		locked, remaining, err := a.registerFailedLoginAttempt(user.ID, now)
		if err != nil {
			return utils.Error(c, 500, "Gagal memperbarui status login", err.Error())
		}
		if locked {
			lockedUntil := now.Add(failedLoginLockDuration).UTC().Format(time.RFC3339)
			return utils.ErrorData(c, 403, "Akun terkunci sementara karena 5 kali gagal login. Coba lagi setelah 1 menit.", fiber.Map{
				"locked_until": lockedUntil,
			})
		}
		return utils.Error(c, 401, fmt.Sprintf("Invalid Password. Sisa percobaan: %d", remaining))
	}

	if strings.ToUpper(strings.TrimSpace(user.Role)) != "ADMIN" && strings.ToUpper(strings.TrimSpace(user.Role)) != "SUPER_ADMIN" {
		locked, err := a.isSchoolAccountLocked(user.SchoolID)
		if err != nil {
			return utils.Error(c, 500, "Gagal memeriksa status akun", err.Error())
		}
		if locked {
			return utils.Error(c, 403, "Status akun terkunci, silakan hubungi admin untuk membuka akses")
		}
	}

	if err := a.clearFailedLoginState(user.ID); err != nil {
		return utils.Error(c, 500, "Gagal memperbarui status login", err.Error())
	}

	sessionDevice := detectLoginDevice(c.Get("User-Agent"))
	sessionIP := strings.TrimSpace(c.IP())
	loginAt := time.Now().UTC()

	var sessionRow struct {
		SessionVersion int64 `json:"session_version"`
	}
	if err := a.DB.Raw(`
		UPDATE users
		SET
			session_version = COALESCE(session_version, 0) + 1,
			current_session_device = ?,
			current_session_user_agent = ?,
			current_session_ip = ?,
			current_session_login_at = ?
		WHERE id = ?
		RETURNING session_version
	`, sessionDevice, nullIfSessionValueEmpty(c.Get("User-Agent")), nullIfSessionValueEmpty(sessionIP), loginAt, user.ID).Scan(&sessionRow).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat sesi login", err.Error())
	}
	user.SessionVersion = sessionRow.SessionVersion

	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id": user.ID, "role": user.Role, "schoolId": user.SchoolID, "username": user.Username, "sessionVersion": user.SessionVersion, "exp": time.Now().Add(24 * time.Hour).Unix(),
	}).SignedString([]byte(os.Getenv("JWT_SECRET")))

	var school models.School
	var schoolName interface{} = nil
	var schoolLogo interface{} = nil
	if user.SchoolID != nil {
		_ = a.DB.Select("name", "logo_url", "inventory_module_enabled", "official_exam_module_enabled", "koperasi_module_enabled").Where("id = ?", *user.SchoolID).First(&school).Error
		schoolName = school.Name
		schoolLogo = school.LogoURL
	}

	return utils.Success(c, 200, "Login successful", fiber.Map{
		"role": user.Role, "username": user.Username, "school_id": user.SchoolID, "school_name": schoolName, "school_logo": schoolLogo, "school_features": fiber.Map{
			"inventory_module_enabled":     school.InventoryModuleEnabled,
			"official_exam_module_enabled": school.OfficialExamModuleEnabled,
			"koperasi_module_enabled":      school.KoperasiModuleEnabled,
		}, "profile_image": user.ProfileImage, "face_reference_image": user.FaceReferenceImage, "face_reference_descriptor": user.FaceReferenceDescriptor, "token": token,
	})
}

func (a *AppContext) registerFailedLoginAttempt(userID uint, now time.Time) (bool, int, error) {
	var row struct {
		FailedLoginAttempts    int        `gorm:"column:failed_login_attempts"`
		FailedLoginLockedUntil *time.Time `gorm:"column:failed_login_locked_until"`
	}

	if err := a.DB.Raw(`
		UPDATE users
		SET
			failed_login_attempts = CASE
				WHEN COALESCE(failed_login_attempts, 0) + 1 >= ? THEN 0
				ELSE COALESCE(failed_login_attempts, 0) + 1
			END,
			failed_login_locked_until = CASE
				WHEN COALESCE(failed_login_attempts, 0) + 1 >= ? THEN ?
				ELSE failed_login_locked_until
			END
		WHERE id = ?
		RETURNING failed_login_attempts, failed_login_locked_until
	`, failedLoginAttemptLimit, failedLoginAttemptLimit, now.Add(failedLoginLockDuration), userID).Scan(&row).Error; err != nil {
		return false, 0, err
	}

	if row.FailedLoginLockedUntil != nil && row.FailedLoginLockedUntil.After(now) {
		return true, 0, nil
	}

	remaining := failedLoginAttemptLimit - row.FailedLoginAttempts
	if remaining < 1 {
		remaining = 1
	}
	return false, remaining, nil
}

func (a *AppContext) clearFailedLoginState(userID uint) error {
	return a.DB.Exec(`
		UPDATE users
		SET failed_login_attempts = 0,
		    failed_login_locked_until = NULL
		WHERE id = ?
	`, userID).Error
}

func (a *AppContext) isSchoolAccountLocked(schoolID *uint) (bool, error) {
	if schoolID == nil || *schoolID == 0 {
		return false, nil
	}

	today := time.Now().In(jakartaLocation()).Format("2006-01-02")

	var count int64
	if err := a.DB.Table("school_invoices").
		Where("school_id = ? AND status <> ? AND DATE(due_date) < ?::date", *schoolID, "PAID", today).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *AppContext) RegisterStudent(c *fiber.Ctx) error { return a.registerScopedUser(c, true) }
func (a *AppContext) RegisterUserSchool(c *fiber.Ctx) error {
	return a.registerScopedUser(c, false)
}

func (a *AppContext) ImportUserSchoolTeachers(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	file, err := c.FormFile("file")
	if err != nil {
		return utils.Error(c, 400, "File wajib diunggah")
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".xlsx" {
		return utils.Error(c, 400, "Format file harus .xlsx sesuai template")
	}

	handle, err := file.Open()
	if err != nil {
		return utils.Error(c, 500, "Gagal membuka file", err.Error())
	}
	defer handle.Close()

	payload, err := io.ReadAll(handle)
	if err != nil {
		return utils.Error(c, 500, "Gagal membaca file", err.Error())
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	headerIndex := -1
	columnIndex := map[string]int{}
	for rowIndex, row := range rows {
		columnIndex = map[string]int{}
		for cellIndex, cellValue := range row {
			normalized := normalizeExcelHeader(cellValue)
			switch {
			case strings.Contains(normalized, "username"):
				columnIndex["username"] = cellIndex
			case strings.Contains(normalized, "password"):
				columnIndex["password"] = cellIndex
			case strings.Contains(normalized, "nama lengkap") || strings.Contains(normalized, "full name") || strings.EqualFold(normalized, "nama") || strings.EqualFold(normalized, "name"):
				columnIndex["full_name"] = cellIndex
			case strings.Contains(normalized, "email"):
				columnIndex["parent_email"] = cellIndex
			case strings.Contains(normalized, "hp") || strings.Contains(normalized, "phone") || strings.Contains(normalized, "telepon"):
				columnIndex["phone_number"] = cellIndex
			}
		}

		if hasRequiredGuruHeaders(columnIndex) {
			headerIndex = rowIndex
			break
		}
	}
	if headerIndex < 0 {
		return utils.Error(c, 400, "Header template tidak dikenali, unduh ulang template terbaru")
	}

	requiredColumns := []string{"full_name"}
	for _, column := range requiredColumns {
		if _, ok := columnIndex[column]; !ok {
			return utils.Error(c, 400, "Header template tidak lengkap")
		}
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		return utils.Error(c, 500, "Gagal memulai transaksi")
	}

	imported := 0
	failedRows := make([]fiber.Map, 0)
	existingUsernames, err := loadUsernameSet(tx)
	if err != nil {
		tx.Rollback()
		return utils.Error(c, 500, "Gagal memeriksa data user", err.Error())
	}
	importedItems := make([]fiber.Map, 0)

	for rowIndex := headerIndex + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		if isExcelRowEmpty(row) {
			continue
		}

		record := map[string]string{
			"full_name": cellValueByKey(row, columnIndex, "full_name"),
		}

		if record["full_name"] == "" {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": "nama lengkap wajib diisi",
			})
			continue
		}

		username := nextAvailableUsername(record["full_name"], "", existingUsernames)
		rawPassword := generateStudentPassword()
		hashedPassword, encryptedPassword, err := hashAndStoreRawPassword(rawPassword)
		if err != nil {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": err.Error(),
			})
			continue
		}

		user := models.User{
			FullName:                  utils.StringPtr(record["full_name"]),
			Username:                  username,
			Password:                  hashedPassword,
			Role:                      "GURU",
			SchoolID:                  &schoolID,
			InitialPasswordCiphertext: utils.StringPtr(encryptedPassword),
		}

		if err := tx.Create(&user).Error; err != nil {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": err.Error(),
			})
			continue
		}
		existingUsernames[strings.ToLower(username)] = struct{}{}
		importedItems = append(importedItems, fiber.Map{
			"row":       rowIndex + 1,
			"full_name": record["full_name"],
			"username":  username,
			"password":  rawPassword,
			"role":      "GURU",
			"school_id": schoolID,
		})
		imported += 1
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan hasil import", err.Error())
	}

	return utils.Success(c, 200, "Import guru selesai", fiber.Map{
		"imported": imported,
		"failed":   len(failedRows),
		"errors":   failedRows,
		"items":    importedItems,
	})
}

func (a *AppContext) DownloadUserSchoolTeacherTemplate(c *fiber.Ctx) error {
	xlsxBytes, err := buildTeacherTemplateXLSX()
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat template", err.Error())
	}

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", `attachment; filename="template-guru.xlsx"`)
	return c.Send(xlsxBytes)
}

func (a *AppContext) DownloadStudentTemplate(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	classID := strings.TrimSpace(c.Query("class_id"))
	if classID == "" {
		return utils.Error(c, 400, "Pilih kelas terlebih dahulu")
	}

	var selectedClass models.Class
	if err := a.DB.Where("id = ? AND school_id = ?", classID, schoolID).First(&selectedClass).Error; err != nil {
		return utils.Error(c, 404, "Kelas tidak ditemukan")
	}

	var classes []models.Class
	if err := a.DB.Where("school_id = ?", schoolID).Order("class_name ASC").Find(&classes).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca data kelas", err.Error())
	}

	xlsxBytes, err := buildStudentTemplateXLSX(classes, &selectedClass)
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat template", err.Error())
	}

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="template-siswa-%s.xlsx"`, safeFilenamePart(selectedClass.ClassName)))
	return c.Send(xlsxBytes)
}

func (a *AppContext) DownloadStudentAccountsByClass(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	classID := strings.TrimSpace(c.Query("class_id"))
	if classID == "" {
		return utils.Error(c, 400, "Pilih kelas terlebih dahulu")
	}

	var selectedClass models.Class
	if err := a.DB.Where("id = ? AND school_id = ?", classID, schoolID).First(&selectedClass).Error; err != nil {
		return utils.Error(c, 404, "Kelas tidak ditemukan")
	}

	rows, err := a.loadStudentAccountRowsByClass(schoolID, selectedClass.ID)
	if err != nil {
		return utils.Error(c, 500, "Gagal membaca akun siswa", err.Error())
	}

	xlsxBytes, err := buildStudentAccountsXLSX(selectedClass.ClassName, rows)
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat file akun siswa", err.Error())
	}

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="akun-siswa-%s.xlsx"`, safeFilenamePart(selectedClass.ClassName)))
	return c.Send(xlsxBytes)
}

func (a *AppContext) ImportStudents(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	file, err := c.FormFile("file")
	if err != nil {
		return utils.Error(c, 400, "File template wajib diupload")
	}
	handle, err := file.Open()
	if err != nil {
		return utils.Error(c, 500, "Gagal membuka file", err.Error())
	}
	defer handle.Close()

	payload, err := io.ReadAll(handle)
	if err != nil {
		return utils.Error(c, 500, "Gagal membaca file", err.Error())
	}

	rows, err := parseTeacherImportXLSXRows(payload)
	if err != nil {
		return utils.Error(c, 400, err.Error())
	}

	classID := strings.TrimSpace(c.FormValue("class_id"))
	var selectedClass *models.Class
	if classID != "" {
		var classItem models.Class
		if err := a.DB.Where("id = ? AND school_id = ?", classID, schoolID).First(&classItem).Error; err != nil {
			return utils.Error(c, 404, "Kelas tidak ditemukan")
		}
		selectedClass = &classItem
	}
	if selectedClass == nil {
		return utils.Error(c, 400, "Pilih kelas terlebih dahulu sebelum import")
	}

	if err := validateStudentTemplateClass(rows, selectedClass); err != nil {
		return utils.Error(c, 400, "Template siswa tidak sesuai", err.Error())
	}

	headerIndex := -1
	columnIndex := map[string]int{}
	for rowIndex, row := range rows {
		columnIndex = map[string]int{}
		for cellIndex, cellValue := range row {
			normalized := normalizeExcelHeader(cellValue)
			switch {
			case strings.Contains(normalized, "username"):
				columnIndex["username"] = cellIndex
			case strings.Contains(normalized, "nama lengkap") || strings.Contains(normalized, "full name") || strings.EqualFold(normalized, "nama") || strings.EqualFold(normalized, "name"):
				columnIndex["full_name"] = cellIndex
			case strings.Contains(normalized, "kelas") || strings.Contains(normalized, "class"):
				columnIndex["class_name"] = cellIndex
			case strings.Contains(normalized, "email"):
				columnIndex["parent_email"] = cellIndex
			case strings.Contains(normalized, "hp") || strings.Contains(normalized, "phone") || strings.Contains(normalized, "telepon"):
				columnIndex["phone_number"] = cellIndex
			}
		}

		if hasRequiredStudentHeaders(columnIndex) {
			headerIndex = rowIndex
			break
		}
	}
	if headerIndex < 0 {
		return utils.Error(c, 400, "Header template siswa tidak dikenali, unduh ulang template terbaru")
	}

	if selectedClass == nil {
		return utils.Error(c, 400, "Pilih kelas terlebih dahulu sebelum import")
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		return utils.Error(c, 500, "Gagal memulai transaksi")
	}

	imported := 0
	failedRows := make([]fiber.Map, 0)
	existingUsernames, err := loadUsernameSet(tx)
	if err != nil {
		tx.Rollback()
		return utils.Error(c, 500, "Gagal memeriksa data siswa", err.Error())
	}
	importedItems := make([]fiber.Map, 0)

	for rowIndex := headerIndex + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		if isStudentImportRowEmpty(row, columnIndex) {
			continue
		}

		record := map[string]string{
			"full_name":    cellValueByKey(row, columnIndex, "full_name"),
			"parent_email": cellValueByKey(row, columnIndex, "parent_email"),
			"phone_number": cellValueByKey(row, columnIndex, "phone_number"),
		}

		if record["full_name"] == "" {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": "nama lengkap wajib diisi",
			})
			continue
		}

		username := nextAvailableUsername(record["full_name"], "", existingUsernames)
		rawPassword := generateStudentPassword()
		hashedPassword, encryptedPassword, err := hashAndStoreRawPassword(rawPassword)
		if err != nil {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": err.Error(),
			})
			continue
		}

		user := models.User{
			FullName:                  utils.StringPtr(record["full_name"]),
			Username:                  username,
			Password:                  hashedPassword,
			InitialPasswordCiphertext: utils.StringPtr(encryptedPassword),
			Role:                      "SISWA",
			SchoolID:                  &schoolID,
			ClassID:                   &selectedClass.ID,
			ParentEmail:               utils.StringPtr(record["parent_email"]),
			PhoneNumber:               utils.StringPtr(record["phone_number"]),
		}

		if err := tx.Create(&user).Error; err != nil {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": err.Error(),
			})
			continue
		}
		if err := ensureInitialStudentClassEnrollmentTx(tx, schoolID, user.ID, selectedClass.ID, uintPointerFromLocal(c, "userID")); err != nil {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": err.Error(),
			})
			continue
		}
		existingUsernames[strings.ToLower(username)] = struct{}{}
		importedItems = append(importedItems, fiber.Map{
			"row":        rowIndex + 1,
			"full_name":  record["full_name"],
			"username":   username,
			"role":       "SISWA",
			"class_name": selectedClass.ClassName,
			"school_id":  schoolID,
		})
		imported += 1
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan hasil import siswa", err.Error())
	}

	return utils.Success(c, 200, "Import siswa selesai", fiber.Map{
		"imported": imported,
		"failed":   len(failedRows),
		"errors":   failedRows,
		"items":    importedItems,
	})
}

func (a *AppContext) registerScopedUser(c *fiber.Ctx, asStudent bool) error {
	var body map[string]interface{}
	_ = c.BodyParser(&body)

	schoolID := c.Locals("schoolID").(uint)
	role := utils.ToString(body["role"])
	if asStudent {
		role = "SISWA"
	}

	normalizedRole := strings.ToUpper(strings.TrimSpace(role))
	fullName := strings.TrimSpace(utils.ToString(body["full_name"]))
	if fullName == "" {
		return utils.Error(c, 400, "Nama lengkap wajib diisi")
	}
	rawPassword := strings.TrimSpace(utils.ToString(body["password"]))
	existingUsernames := map[string]struct{}{}
	if asStudent || normalizedRole == "GURU" {
		if usernames, err := loadUsernameSet(a.DB); err == nil {
			existingUsernames = usernames
		}
	}

	username := strings.TrimSpace(utils.ToString(body["username"]))
	if (asStudent || normalizedRole == "GURU") && username == "" {
		username = nextAvailableUsername(fullName, "", existingUsernames)
	}
	if username == "" {
		return utils.Error(c, 400, "Username wajib diisi")
	}
	if err := ensureUsernameAvailable(a.DB, username, 0); err != nil {
		return utils.Error(c, 400, "Username sudah digunakan", err.Error())
	}
	if (asStudent || normalizedRole == "GURU") && rawPassword == "" {
		rawPassword = generateStudentPassword()
	}
	if rawPassword == "" {
		return utils.Error(c, 400, "Password wajib diisi")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(rawPassword), 8)
	user := models.User{
		FullName:    utils.StringPtr(fullName),
		Username:    username,
		Password:    string(hash),
		Role:        role,
		SchoolID:    &schoolID,
		ParentEmail: utils.StringPtr(body["parent_email"]),
		PhoneNumber: utils.StringPtr(body["phone_number"]),
	}
	if asStudent {
		classID := uint(utils.ToInt(utils.ToString(body["class_id"]), 0))
		user.ClassID = &classID
		if encrypted, err := encryptAccountPassword(rawPassword); err == nil {
			user.InitialPasswordCiphertext = utils.StringPtr(encrypted)
		}
	} else if normalizedRole == "GURU" {
		if encrypted, err := encryptAccountPassword(rawPassword); err == nil {
			user.InitialPasswordCiphertext = utils.StringPtr(encrypted)
		}
	}

	if err := a.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if asStudent && user.SchoolID != nil && user.ClassID != nil {
			return ensureInitialStudentClassEnrollmentTx(tx, *user.SchoolID, user.ID, *user.ClassID, uintPointerFromLocal(c, "userID"))
		}
		return nil
	}); err != nil {
		return utils.Error(c, 500, "Registration failed", err.Error())
	}
	response := fiber.Map{
		"id":        user.ID,
		"username":  user.Username,
		"role":      user.Role,
		"school_id": user.SchoolID,
		"class_id":  user.ClassID,
		"full_name": user.FullName,
	}
	if asStudent || normalizedRole == "GURU" {
		response["password"] = rawPassword
	}
	message := "User registered successfully"
	if normalizedRole == "GURU" {
		message = fmt.Sprintf("User guru berhasil dibuat. Username: %s. Password: %s", user.Username, rawPassword)
	}
	return utils.Success(c, 201, message, response)
}

func buildTeacherTemplateXLSX() ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := archivezip.NewWriter(&buffer)

	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML(),
		"_rels/.rels":                xlsxRootRelsXML(),
		"xl/workbook.xml":            xlsxWorkbookXML("Template Guru"),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML(),
		"xl/styles.xml":              xlsxStylesXML(),
		"xl/worksheets/sheet1.xml":   xlsxTeacherTemplateSheetXML(),
	}

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func buildStudentTemplateXLSX(classes []models.Class, selectedClass *models.Class) ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := archivezip.NewWriter(&buffer)

	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML(),
		"_rels/.rels":                xlsxRootRelsXML(),
		"xl/workbook.xml":            xlsxWorkbookXML("Template Siswa"),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML(),
		"xl/styles.xml":              xlsxStylesXML(),
		"xl/worksheets/sheet1.xml":   xlsxStudentTemplateSheetXML(classes, selectedClass),
	}

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func buildStudentAccountsXLSX(className string, rows []studentAccountExportRow) ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := archivezip.NewWriter(&buffer)

	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML(),
		"_rels/.rels":                xlsxRootRelsXML(),
		"xl/workbook.xml":            xlsxWorkbookXML("Akun Siswa"),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML(),
		"xl/styles.xml":              xlsxStylesXML(),
		"xl/worksheets/sheet1.xml":   xlsxStudentAccountsSheetXML(className, rows),
	}

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		if err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func studentTemplateClassNameFromRows(rows [][]string) string {
	const prefix = "Template ini khusus untuk kelas "
	const suffix = ". Username dan password dibuat otomatis oleh sistem."
	for _, row := range rows {
		for _, cell := range row {
			value := strings.TrimSpace(cell)
			if value == "" || !strings.Contains(value, prefix) || !strings.Contains(value, suffix) {
				continue
			}
			start := strings.Index(value, prefix)
			if start < 0 {
				continue
			}
			start += len(prefix)
			end := strings.Index(value[start:], suffix)
			if end < 0 {
				continue
			}
			return strings.TrimSpace(value[start : start+end])
		}
	}
	return ""
}

func validateStudentTemplateClass(rows [][]string, selectedClass *models.Class) error {
	if selectedClass == nil {
		return fmt.Errorf("kelas harus dipilih terlebih dahulu")
	}

	templateClassName := studentTemplateClassNameFromRows(rows)
	if templateClassName == "" {
		return fmt.Errorf("kelas pada template siswa tidak ditemukan, unduh ulang template terbaru")
	}
	if !strings.EqualFold(strings.TrimSpace(templateClassName), strings.TrimSpace(selectedClass.ClassName)) {
		return fmt.Errorf("template untuk kelas %s tidak cocok dengan kelas %s", templateClassName, selectedClass.ClassName)
	}
	return nil
}

func xlsxContentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`
}

func xlsxRootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
}

func xlsxWorkbookXML(sheetName string) string {
	if strings.TrimSpace(sheetName) == "" {
		sheetName = "Template"
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="` + xlsxEscape(sheetName) + `" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`
}

func xlsxWorkbookRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`
}

func xlsxStylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="6">
    <font><sz val="11"/><color rgb="FF334155"/><name val="Calibri"/></font>
    <font><b/><sz val="18"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
    <font><sz val="11"/><color rgb="FFE0F2FE"/><name val="Calibri"/></font>
    <font><b/><sz val="11"/><color rgb="FF0F172A"/><name val="Calibri"/></font>
    <font><b/><sz val="11"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
    <font><i/><sz val="10"/><color rgb="FF64748B"/><name val="Calibri"/></font>
  </fonts>
  <fills count="7">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF0F172A"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFE0F2FE"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FF0369A1"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFDCFCE7"/><bgColor indexed="64"/></patternFill></fill>
    <fill><patternFill patternType="solid"><fgColor rgb="FFF8FAFC"/><bgColor indexed="64"/></patternFill></fill>
  </fills>
  <borders count="2">
    <border><left/><right/><top/><bottom/><diagonal/></border>
    <border>
      <left style="thin"><color rgb="FFCBD5E1"/></left>
      <right style="thin"><color rgb="FFCBD5E1"/></right>
      <top style="thin"><color rgb="FFCBD5E1"/></top>
      <bottom style="thin"><color rgb="FFCBD5E1"/></bottom>
      <diagonal/>
    </border>
  </borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="9">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="1" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="2" fillId="2" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="3" fillId="3" borderId="0" xfId="0" applyFont="1" applyFill="1" applyAlignment="1"><alignment vertical="center" wrapText="1"/></xf>
    <xf numFmtId="0" fontId="4" fillId="4" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment horizontal="center" vertical="center"/></xf>
    <xf numFmtId="0" fontId="0" fillId="6" borderId="1" xfId="0" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center"/></xf>
    <xf numFmtId="0" fontId="5" fillId="6" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center"/></xf>
    <xf numFmtId="0" fontId="3" fillId="5" borderId="1" xfId="0" applyFont="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center" wrapText="1"/></xf>
    <xf numFmtId="49" fontId="0" fillId="6" borderId="1" xfId="0" applyNumberFormat="1" applyFill="1" applyBorder="1" applyAlignment="1"><alignment vertical="center"/></xf>
  </cellXfs>
  <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
  <dxfs count="0"/>
  <tableStyles count="0" defaultTableStyle="TableStyleMedium2" defaultPivotStyle="PivotStyleLight16"/>
</styleSheet>`
}

func xlsxTeacherTemplateSheetXML() string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <dimension ref="A1:B40"/>
  <sheetViews>
    <sheetView workbookViewId="0" showGridLines="0">
      <pane ySplit="11" topLeftCell="A12" activePane="bottomLeft" state="frozen"/>
      <selection pane="bottomLeft" activeCell="A12" sqref="A12"/>
    </sheetView>
  </sheetViews>
  <sheetFormatPr defaultRowHeight="22"/>
  <cols>
    <col min="1" max="1" width="10" customWidth="1"/>
    <col min="2" max="2" width="44" customWidth="1"/>
  </cols>
  <sheetData>
    <row r="1" ht="34" customHeight="1">` + xlsxStyledCell("A1", "Template Import Data Guru", 1) + `</row>
    <row r="2" ht="30" customHeight="1">` + xlsxStyledCell("A2", "Gunakan template ini untuk menambahkan akun guru secara massal. Username dan password dibuat otomatis oleh sistem.", 3) + `</row>
    <row r="4" ht="24" customHeight="1">` + xlsxStyledCell("A4", "Petunjuk Pengisian", 3) + `</row>
    <row r="5" ht="40" customHeight="1">` + xlsxStyledCell("A5", "1", 4) + xlsxStyledCell("B5", "Template ini hanya membutuhkan Nama Lengkap. Username dan password akan dibuat otomatis oleh sistem.", 3) + `</row>
    <row r="6" ht="30" customHeight="1">` + xlsxStyledCell("A6", "2", 4) + xlsxStyledCell("B6", "Role akan otomatis dibuat sebagai GURU. Jangan mengubah nama kolom pada baris header.", 3) + `</row>
    <row r="7" ht="28" customHeight="1">` + xlsxStyledCell("A7", "3", 4) + xlsxStyledCell("B7", "Isi data guru mulai baris 12. Baris contoh hanya sebagai referensi format.", 3) + `</row>
    <row r="9" ht="24" customHeight="1">` + xlsxStyledCell("A9", "Contoh Format", 4) + `</row>
    <row r="10" ht="24" customHeight="1">` + xlsxStyledCell("A10", "1", 4) + xlsxStyledCell("B10", "Ahmad Fajri", 7) + `</row>
    <row r="11" ht="24" customHeight="1">` + xlsxStyledCell("A11", "No.", 4) + xlsxStyledCell("B11", "Nama Lengkap", 4) + `</row>
`)

	for row := 12; row <= 40; row++ {
		builder.WriteString(fmt.Sprintf(`    <row r="%d" ht="22" customHeight="1">`, row))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("A%d", row), fmt.Sprintf("%d", row-11), 4))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("B%d", row), "", 5))
		builder.WriteString("</row>\n")
	}

	builder.WriteString(`  </sheetData>
  <autoFilter ref="A11:B40"/>
  <mergeCells count="4">
    <mergeCell ref="A1:B1"/>
    <mergeCell ref="A2:B2"/>
    <mergeCell ref="A4:B4"/>
    <mergeCell ref="A9:B9"/>
  </mergeCells>
  <pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>
</worksheet>`)
	return builder.String()
}

func xlsxStudentTemplateSheetXML(classes []models.Class, selectedClass *models.Class) string {
	className := selectedStudentTemplateClassName(classes, selectedClass)
	instructionText := fmt.Sprintf("1. Template ini khusus untuk kelas %s. Username dan password dibuat otomatis oleh sistem.", className)
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <dimension ref="A1:F40"/>
  <sheetViews>
    <sheetView workbookViewId="0" showGridLines="0">
      <pane ySplit="11" topLeftCell="A12" activePane="bottomLeft" state="frozen"/>
      <selection pane="bottomLeft" activeCell="A12" sqref="A12"/>
    </sheetView>
  </sheetViews>
  <sheetFormatPr defaultRowHeight="22"/>
  <cols>
    <col min="1" max="1" width="10" customWidth="1"/>
    <col min="2" max="2" width="44" customWidth="1"/>
    <col min="3" max="6" width="6" customWidth="1"/>
  </cols>
  <sheetData>
    <row r="1" ht="34" customHeight="1">` + xlsxStyledCell("A1", "Template Import Data Siswa", 1) + `</row>
    <row r="2" ht="30" customHeight="1">` + xlsxStyledCell("A2", "Gunakan template ini untuk menambahkan akun siswa secara massal. Username dan password dibuat otomatis oleh sistem.", 3) + `</row>
    <row r="4" ht="24" customHeight="1">` + xlsxStyledCell("A4", "Petunjuk Pengisian", 3) + `</row>
    <row r="5" ht="40" customHeight="1">` + xlsxStyledCell("A5", "1", 4) + xlsxStyledCell("B5", instructionText, 3) + `</row>
    <row r="6" ht="30" customHeight="1">` + xlsxStyledCell("A6", "2", 4) + xlsxStyledCell("B6", "Role akan otomatis dibuat sebagai SISWA. Kontak wali bisa dilengkapi nanti dari profil siswa.", 3) + `</row>
    <row r="7" ht="28" customHeight="1">` + xlsxStyledCell("A7", "3", 4) + xlsxStyledCell("B7", "Segera minta siswa mengganti password setelah login pertama.", 3) + `</row>
    <row r="9" ht="24" customHeight="1">` + xlsxStyledCell("A9", "Contoh Format", 4) + `</row>
    <row r="10" ht="24" customHeight="1">` + xlsxStyledCell("A10", "1", 4) + xlsxStyledCell("B10", "Ahmad Fajri", 7) + `</row>
    <row r="11" ht="24" customHeight="1">` + xlsxStyledCell("A11", "No.", 4) + xlsxStyledCell("B11", "Nama Lengkap", 4) + `</row>
`)

	for row := 12; row <= 40; row++ {
		builder.WriteString(fmt.Sprintf(`    <row r="%d" ht="22" customHeight="1">`, row))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("A%d", row), fmt.Sprintf("%d", row-11), 4))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("B%d", row), "", 5))
		builder.WriteString("</row>\n")
	}

	builder.WriteString(`  </sheetData>
  <autoFilter ref="A11:B40"/>
  <mergeCells count="10">
    <mergeCell ref="A1:F1"/>
    <mergeCell ref="A2:F2"/>
    <mergeCell ref="A4:F4"/>
    <mergeCell ref="B5:F5"/>
    <mergeCell ref="B6:F6"/>
    <mergeCell ref="B7:F7"/>
    <mergeCell ref="A9:F9"/>
    <mergeCell ref="A10:F10"/>
  </mergeCells>
  <pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>
</worksheet>`)
	return builder.String()
}

func xlsxStudentAccountsSheetXML(className string, rows []studentAccountExportRow) string {
	if strings.TrimSpace(className) == "" {
		className = "Semua Kelas"
	}
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <dimension ref="A1:C1000"/>
  <sheetViews>
    <sheetView workbookViewId="0">
      <pane ySplit="4" topLeftCell="A5" activePane="bottomLeft" state="frozen"/>
      <selection pane="bottomLeft" activeCell="A5" sqref="A5"/>
    </sheetView>
  </sheetViews>
  <sheetFormatPr defaultRowHeight="18"/>
  <cols>
    <col min="1" max="1" width="32" customWidth="1"/>
    <col min="2" max="2" width="26" customWidth="1"/>
    <col min="3" max="3" width="20" style="8" customWidth="1"/>
  </cols>
  <sheetData>
    <row r="1" ht="28" customHeight="1">` + xlsxStyledCell("A1", "Daftar Akun Siswa - "+className, 1) + `</row>
    <row r="2" ht="22" customHeight="1">` + xlsxStyledCell("A2", "Gunakan file ini untuk membagikan akun login siswa. Segera minta siswa mengganti password setelah login pertama.", 2) + `</row>
    <row r="4" ht="24" customHeight="1">` + xlsxStyledCell("A4", "Nama Lengkap", 4) + xlsxStyledCell("B4", "Username", 4) + xlsxStyledCell("C4", "Password", 4) + `</row>
`)

	for index, row := range rows {
		rowNumber := index + 5
		builder.WriteString(fmt.Sprintf(`    <row r="%d" ht="22" customHeight="1">`, rowNumber))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("A%d", rowNumber), row.FullName, 5))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("B%d", rowNumber), row.Username, 5))
		builder.WriteString(xlsxStyledCell(fmt.Sprintf("C%d", rowNumber), row.Password, 8))
		builder.WriteString("</row>\n")
	}

	builder.WriteString(fmt.Sprintf(`  </sheetData>
  <autoFilter ref="A4:C%d"/>
  <pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>
</worksheet>`, len(rows)+4))
	return builder.String()
}

func parseTeacherImportXLSXRows(payload []byte) ([][]string, error) {
	reader, err := archivezip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("file bukan xlsx yang valid")
	}

	sheetData, err := readZipFile(reader, "xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, fmt.Errorf("sheet template tidak ditemukan")
	}

	sharedStrings := make([]string, 0)
	if sharedXML, err := readZipFile(reader, "xl/sharedStrings.xml"); err == nil {
		sharedStrings, err = parseSharedStringsXML(sharedXML)
		if err != nil {
			return nil, err
		}
	}

	return parseWorksheetRowsXML(sheetData, sharedStrings)
}

func readZipFile(reader *archivezip.Reader, target string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != target {
			continue
		}
		handle, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer handle.Close()
		return io.ReadAll(handle)
	}
	return nil, fmt.Errorf("file %s tidak ditemukan", target)
}

func parseSharedStringsXML(payload []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	values := make([]string, 0)
	inSi := false
	inText := false
	var current strings.Builder

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gagal membaca shared string: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "si":
				inSi = true
				current.Reset()
			case "t":
				if inSi {
					inText = true
				}
			}
		case xml.CharData:
			if inSi && inText {
				current.WriteString(string(value))
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t":
				inText = false
			case "si":
				values = append(values, current.String())
				inSi = false
			}
		}
	}

	return values, nil
}

func parseWorksheetRowsXML(payload []byte, sharedStrings []string) ([][]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	rows := make([][]string, 0)

	inRow := false
	inCell := false
	inValue := false
	inInlineText := false
	currentRow := map[int]string{}
	currentRef := ""
	currentType := ""
	var currentValue strings.Builder

	flushRow := func() {
		if len(currentRow) == 0 {
			return
		}
		maxIndex := -1
		for index := range currentRow {
			if index > maxIndex {
				maxIndex = index
			}
		}
		row := make([]string, maxIndex+1)
		for index, value := range currentRow {
			row[index] = value
		}
		rows = append(rows, row)
		currentRow = map[int]string{}
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gagal membaca isi worksheet: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "row":
				inRow = true
				currentRow = map[int]string{}
			case "c":
				if inRow {
					inCell = true
					currentRef = ""
					currentType = ""
					currentValue.Reset()
					for _, attr := range value.Attr {
						switch attr.Name.Local {
						case "r":
							currentRef = attr.Value
						case "t":
							currentType = attr.Value
						}
					}
				}
			case "v":
				if inCell {
					inValue = true
				}
			case "is":
				if inCell {
					inInlineText = true
				}
			case "t":
				if inCell && inInlineText {
					inValue = true
				}
			}
		case xml.CharData:
			if inCell && (inValue || inInlineText) {
				currentValue.WriteString(string(value))
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "v":
				inValue = false
			case "t":
				if inInlineText {
					inValue = false
				}
			case "is":
				inInlineText = false
			case "c":
				columnIndex := excelColumnIndex(currentRef)
				cellText := currentValue.String()
				if currentType == "s" && cellText != "" {
					if sharedIndex := utils.ToInt(cellText, -1); sharedIndex >= 0 && sharedIndex < len(sharedStrings) {
						cellText = sharedStrings[sharedIndex]
					}
				}
				if columnIndex >= 0 {
					currentRow[columnIndex] = strings.TrimSpace(cellText)
				}
				inCell = false
			case "row":
				flushRow()
				inRow = false
			}
		}
	}

	return rows, nil
}

func excelColumnIndex(ref string) int {
	col := ""
	for _, char := range ref {
		if char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' {
			col += strings.ToUpper(string(char))
		} else {
			break
		}
	}
	if col == "" {
		return -1
	}
	index := 0
	for _, char := range col {
		index = index*26 + int(char-'A'+1)
	}
	return index - 1
}

func xlsxEscape(value string) string {
	var buffer bytes.Buffer
	_ = xml.EscapeText(&buffer, []byte(value))
	return buffer.String()
}

func xlsxCell(reference, value string) string {
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, reference, xlsxEscape(value))
}

func xlsxStyledCell(reference, value string, styleID int) string {
	if value == "" {
		return fmt.Sprintf(`<c r="%s" s="%d"/>`, reference, styleID)
	}
	return fmt.Sprintf(`<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, reference, styleID, xlsxEscape(value))
}

func selectedStudentTemplateClassName(classes []models.Class, selectedClass *models.Class) string {
	if selectedClass != nil && strings.TrimSpace(selectedClass.ClassName) != "" {
		return selectedClass.ClassName
	}
	if len(classes) > 0 && strings.TrimSpace(classes[0].ClassName) != "" {
		return classes[0].ClassName
	}
	return "Contoh: X IPA 1"
}

func safeFilenamePart(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "-")
	var builder strings.Builder
	for _, char := range normalized {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			builder.WriteRune(char)
		}
	}
	if builder.Len() == 0 {
		return "kelas"
	}
	return builder.String()
}

func parseSpreadsheetMLRows(payload []byte) ([][]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	rows := make([][]string, 0)
	var currentRow []string
	var currentCell strings.Builder
	inRow := false
	inCell := false
	inData := false
	rowHasCell := false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gagal membaca struktur file excel: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "Row":
				inRow = true
				rowHasCell = false
				currentRow = []string{}
			case "Cell":
				if inRow {
					inCell = true
					currentCell.Reset()
				}
			case "Data":
				if inRow && inCell {
					inData = true
				}
			}
		case xml.CharData:
			if inData {
				currentCell.WriteString(string(value))
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "Data":
				inData = false
			case "Cell":
				if inRow && inCell {
					currentRow = append(currentRow, strings.TrimSpace(currentCell.String()))
					rowHasCell = true
				}
				inCell = false
			case "Row":
				if inRow && rowHasCell {
					rows = append(rows, currentRow)
				}
				inRow = false
			}
		}
	}

	return rows, nil
}

func normalizeExcelHeader(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	lowered = strings.ReplaceAll(lowered, "_", " ")
	lowered = strings.ReplaceAll(lowered, ".", " ")
	lowered = strings.Join(strings.Fields(lowered), " ")
	return lowered
}

func hasRequiredGuruHeaders(columnIndex map[string]int) bool {
	_, hasFullName := columnIndex["full_name"]
	return hasFullName
}

func hasRequiredStudentHeaders(columnIndex map[string]int) bool {
	_, hasFullName := columnIndex["full_name"]
	return hasFullName
}

func isExcelRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func isStudentImportRowEmpty(row []string, columnIndex map[string]int) bool {
	nonEmpty := 0
	for _, key := range []string{"full_name", "username", "class_name", "parent_email", "phone_number"} {
		if strings.TrimSpace(cellValueByKey(row, columnIndex, key)) != "" {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		return true
	}
	if nonEmpty == 1 && strings.TrimSpace(cellValueByKey(row, columnIndex, "class_name")) != "" {
		return true
	}
	return false
}

func cellValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func cellValueByKey(row []string, columnIndex map[string]int, key string) string {
	index, ok := columnIndex[key]
	if !ok {
		return ""
	}
	return cellValue(row, index)
}

func loadUsernameSet(tx *gorm.DB) (map[string]struct{}, error) {
	var existingRows []struct {
		Username string `gorm:"column:username"`
	}
	if err := tx.Table("users").
		Select("username").
		Scan(&existingRows).Error; err != nil {
		return nil, err
	}

	existing := make(map[string]struct{}, len(existingRows))
	for _, item := range existingRows {
		key := strings.ToLower(strings.TrimSpace(item.Username))
		if key == "" {
			continue
		}
		existing[key] = struct{}{}
	}

	return existing, nil
}

func attachDecodedInitialPassword(user map[string]interface{}) {
	if user == nil {
		return
	}
	raw, _ := user["initial_password_ciphertext"].(string)
	if strings.TrimSpace(raw) == "" {
		user["initial_password"] = ""
		return
	}
	password, err := decryptAccountPassword(raw)
	if err != nil {
		user["initial_password"] = "-"
		return
	}
	user["initial_password"] = password
}

func (a *AppContext) loadStudentAccountRowsByClass(schoolID, classID uint) ([]studentAccountExportRow, error) {
	var rows []struct {
		FullName                  string `gorm:"column:full_name"`
		Username                  string `gorm:"column:username"`
		InitialPasswordCiphertext string `gorm:"column:initial_password_ciphertext"`
	}

	if err := a.DB.Table("users").
		Select("COALESCE(full_name, '') AS full_name, username, COALESCE(initial_password_ciphertext, '') AS initial_password_ciphertext").
		Where("school_id = ? AND class_id = ? AND role = 'SISWA'", schoolID, classID).
		Order("full_name ASC, username ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	exportRows := make([]studentAccountExportRow, 0, len(rows))
	for _, row := range rows {
		password := ""
		if strings.TrimSpace(row.InitialPasswordCiphertext) != "" {
			decrypted, err := decryptAccountPassword(row.InitialPasswordCiphertext)
			if err != nil {
				password = "-"
			} else {
				password = decrypted
			}
		}
		exportRows = append(exportRows, studentAccountExportRow{
			FullName: strings.TrimSpace(row.FullName),
			Username: row.Username,
			Password: password,
		})
	}

	return exportRows, nil
}

func generateStudentPassword() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const lowers = "abcdefghijkmnpqrstuvwxyz"
	const digits = "23456789"
	const totalLength = 10

	segments := make([]byte, 0, totalLength)
	segments = append(segments, randomCharsetChar(letters))
	segments = append(segments, randomCharsetChar(lowers))
	segments = append(segments, randomCharsetChar(digits))

	combined := letters + lowers + digits
	for len(segments) < totalLength {
		segments = append(segments, randomCharsetChar(combined))
	}

	for i := len(segments) - 1; i > 0; i-- {
		j := randomInt(i + 1)
		segments[i], segments[j] = segments[j], segments[i]
	}

	return string(segments)
}

func hashAndStoreRawPassword(rawPassword string) (string, string, error) {
	rawPassword = strings.TrimSpace(rawPassword)
	if rawPassword == "" {
		return "", "", fmt.Errorf("password tidak boleh kosong")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), 8)
	if err != nil {
		return "", "", err
	}
	encrypted, err := encryptAccountPassword(rawPassword)
	if err != nil {
		return "", "", err
	}
	return string(hash), encrypted, nil
}

func encryptAccountPassword(rawPassword string) (string, error) {
	key := sha256.Sum256([]byte(accountPasswordSecret()))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(rawPassword), nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func decryptAccountPassword(encoded string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", err
	}

	key := sha256.Sum256([]byte(accountPasswordSecret()))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", fmt.Errorf("password terenkripsi tidak valid")
	}

	nonce, ciphertext := payload[:nonceSize], payload[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func accountPasswordSecret() string {
	secret := strings.TrimSpace(os.Getenv("ACCOUNT_PASSWORD_SECRET"))
	if secret != "" {
		return secret
	}
	secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret != "" {
		return secret
	}
	return "school-system-account-password-secret"
}

func randomCharsetChar(charset string) byte {
	if charset == "" {
		return 'x'
	}
	return charset[randomInt(len(charset))]
}

func randomInt(max int) int {
	if max <= 1 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

func nextAvailableUsername(fullName, fallback string, existing map[string]struct{}) string {
	seed := strings.TrimSpace(fullName)
	if seed == "" {
		seed = strings.TrimSpace(fallback)
	}

	base := normalizeUsernameSeed(seed)
	if base == "" {
		base = "user"
	}

	candidate := base
	for suffix := 0; ; suffix++ {
		if _, exists := existing[strings.ToLower(candidate)]; !exists {
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d", base, suffix+1)
	}
}

func normalizeUsernameSeed(value string) string {
	lowered := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastWasUnderscore := false

	for _, char := range lowered {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
			lastWasUnderscore = false
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastWasUnderscore = false
		default:
			if builder.Len() > 0 && !lastWasUnderscore {
				builder.WriteByte('_')
				lastWasUnderscore = true
			}
		}
	}

	normalized := strings.Trim(builder.String(), "_")
	if normalized == "" {
		return "user"
	}
	return normalized
}

func (a *AppContext) GetUserSchoolList(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	role := c.Query("role")
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

	var users []map[string]interface{}
	q := a.DB.Table("users").Select("id, full_name, username, role, school_id, parent_email, phone_number, profile_image, COALESCE(initial_password_ciphertext, '') AS initial_password_ciphertext").Where("school_id = ?", schoolID)
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if usePagination {
		var total int64
		if err := q.Count(&total).Error; err != nil {
			return utils.Error(c, 500, "Failed Count User School", err.Error())
		}
		if err := q.Order("username asc").Limit(limit).Offset(offset).Scan(&users).Error; err != nil {
			return utils.Error(c, 500, "Failed Get User School", err.Error())
		}
		for _, user := range users {
			attachDecodedInitialPassword(user)
		}
		return utils.Success(c, 200, "Success Get User School", fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
			"data":  users,
		})
	}

	if err := q.Order("username asc").Scan(&users).Error; err != nil {
		return utils.Error(c, 500, "Failed Get User School", err.Error())
	}
	for _, user := range users {
		attachDecodedInitialPassword(user)
	}
	return utils.Success(c, 200, "Success Get User School", users)
}

func (a *AppContext) GetSchoolTeacherList(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	var users []map[string]interface{}
	if err := a.DB.Table("users").
		Select("id, full_name, username, role, school_id").
		Where("school_id = ? AND role = ?", schoolID, "GURU").
		Order("full_name ASC, username ASC").
		Scan(&users).Error; err != nil {
		return utils.Error(c, 500, "Failed Get School Teacher List", err.Error())
	}

	return utils.Success(c, 200, "Success Get School Teacher List", users)
}

func (a *AppContext) GetStudentTeacherList(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)

	var student struct {
		ClassID *uint `gorm:"column:class_id"`
	}
	if err := a.DB.Raw(`SELECT class_id FROM users WHERE id = ? AND school_id = ? AND role = 'SISWA'`, userID, schoolID).Scan(&student).Error; err != nil {
		return utils.Error(c, 500, "Failed Get Student Teacher List", err.Error())
	}
	if student.ClassID == nil || *student.ClassID == 0 {
		return utils.Success(c, 200, "Success Get Student Teacher List", fiber.Map{"data": []map[string]interface{}{}})
	}

	var teachers []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			u.id,
			u.full_name,
			u.username,
			u.role,
			u.school_id,
			COALESCE(string_agg(DISTINCT ls.name, ', '), '') AS subjects
		FROM learning_subjects ls
		INNER JOIN users u ON u.id = ls.teacher_id
		WHERE ls.school_id = ? AND ls.class_id = ?
		GROUP BY u.id, u.full_name, u.username, u.role, u.school_id
		ORDER BY MIN(COALESCE(u.full_name, u.username)) ASC, u.username ASC
	`, schoolID, *student.ClassID).Scan(&teachers).Error; err != nil {
		return utils.Error(c, 500, "Failed Get Student Teacher List", err.Error())
	}

	if len(teachers) == 0 {
		var homeroom map[string]interface{}
		_ = a.DB.Raw(`
			SELECT u.id, u.full_name, u.username, u.role, u.school_id, '' AS subjects
			FROM class c
			INNER JOIN users u ON u.id = c.wali_guru_id
			WHERE c.school_id = ? AND c.id = ?
			LIMIT 1
		`, schoolID, *student.ClassID).Scan(&homeroom).Error
		if len(homeroom) > 0 {
			teachers = append(teachers, homeroom)
		}
	}

	return utils.Success(c, 200, "Success Get Student Teacher List", fiber.Map{
		"data": teachers,
	})
}

func (a *AppContext) GetStudentScheduleOptions(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)

	var student struct {
		ClassID *uint `gorm:"column:class_id"`
	}
	if err := a.DB.Raw(`SELECT class_id FROM users WHERE id = ? AND school_id = ? AND role = 'SISWA'`, userID, schoolID).Scan(&student).Error; err != nil {
		return utils.Error(c, 500, "Failed Get Student Schedule Options", err.Error())
	}
	if student.ClassID == nil || *student.ClassID == 0 {
		return utils.Success(c, 200, "Success Get Student Schedule Options", fiber.Map{"data": []map[string]interface{}{}})
	}

	var rows []map[string]interface{}
	if err := a.DB.Raw(`
		SELECT
			slot.id,
			slot.day_name,
			slot.day_order,
			slot.session_order,
			slot.start_time,
			slot.end_time,
			COALESCE(slot.label, '') AS label,
			COALESCE(u.full_name, u.username, '-') AS teacher_name,
			COALESCE(ls.name, '-') AS subject_name
		FROM curriculum_schedule_entries cse
		INNER JOIN curriculum_schedule_slots slot ON slot.id = cse.schedule_slot_id
		INNER JOIN users u ON u.id = cse.teacher_id
		LEFT JOIN curriculum_subjects ls ON ls.id = cse.curriculum_subject_id
		WHERE cse.school_id = ? AND cse.class_id = ?
		ORDER BY slot.day_order ASC, slot.session_order ASC, teacher_name ASC, subject_name ASC
	`, schoolID, *student.ClassID).Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Failed Get Student Schedule Options", err.Error())
	}

	return utils.Success(c, 200, "Success Get Student Schedule Options", fiber.Map{
		"data": rows,
	})
}

func (a *AppContext) UpdateUserSchool(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		FullName    *string `json:"full_name"`
		Username    *string `json:"username"`
		Password    *string `json:"password"`
		Role        *string `json:"role"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)
	var current models.User
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User school not found")
	}
	if current.Role != "ADMIN" && current.Role != "GURU" && current.Role != "SARPRAS" && current.Role != "KOPERASI" {
		return utils.Error(c, 400, "Only school admin, teacher, sarpras, and koperasi can be updated here")
	}
	nextUsername := current.Username
	if body.Username != nil {
		nextUsername = *body.Username
	}
	nextRole := current.Role
	if body.Role != nil {
		nextRole = *body.Role
	}
	updates := map[string]interface{}{
		"full_name":    coalesceStrPtr(body.FullName, current.FullName),
		"username":     nextUsername,
		"role":         nextRole,
		"parent_email": coalesceStrPtr(body.ParentEmail, current.ParentEmail),
		"phone_number": coalesceStrPtr(body.PhoneNumber, current.PhoneNumber),
	}
	if body.Username != nil {
		nextUsernameTrimmed := strings.TrimSpace(*body.Username)
		if nextUsernameTrimmed == "" {
			return utils.Error(c, 400, "Username wajib diisi")
		}
		if err := ensureUsernameAvailable(a.DB, nextUsernameTrimmed, current.ID); err != nil {
			return utils.Error(c, 400, "Username sudah digunakan", err.Error())
		}
		nextUsername = nextUsernameTrimmed
	}
	if body.Password != nil && *body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(*body.Password), 8)
		updates["password"] = string(hash)
		if encrypted, err := encryptAccountPassword(*body.Password); err == nil {
			updates["initial_password_ciphertext"] = encrypted
		}
		updates["session_version"] = gorm.Expr("COALESCE(session_version, 0) + 1")
	}
	if err := a.DB.Table("users").Where("id = ? AND school_id = ?", id, schoolID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui user sekolah", err.Error())
	}
	var updated map[string]interface{}
	a.DB.Table("users").Select("id, full_name, username, role, school_id, parent_email, phone_number, profile_image, COALESCE(initial_password_ciphertext, '') AS initial_password_ciphertext").Where("id = ?", id).Scan(&updated)
	attachDecodedInitialPassword(updated)
	return utils.Success(c, 200, "User school updated successfully", updated)
}

func (a *AppContext) ResetUserSchoolPassword(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)

	var current models.User
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User school not found")
	}

	rawPassword := generateStudentPassword()
	hashedPassword, encryptedPassword, err := hashAndStoreRawPassword(rawPassword)
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat password baru", err.Error())
	}

	updates := map[string]interface{}{
		"password":                    hashedPassword,
		"initial_password_ciphertext": encryptedPassword,
		"session_version":             gorm.Expr("COALESCE(session_version, 0) + 1"),
	}
	if err := a.DB.Table("users").Where("id = ? AND school_id = ?", id, schoolID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal mereset password user sekolah", err.Error())
	}

	return utils.Success(c, 200, "Password berhasil direset", fiber.Map{
		"id":               current.ID,
		"username":         current.Username,
		"role":             current.Role,
		"full_name":        current.FullName,
		"password":         rawPassword,
		"school_id":        current.SchoolID,
		"initial_password":  rawPassword,
		"reset_by_username": c.Locals("username"),
	})
}
func (a *AppContext) DeleteUserSchool(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var current models.User
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User school not found")
	}
	if current.Role != "ADMIN" && current.Role != "GURU" && current.Role != "SARPRAS" && current.Role != "KOPERASI" {
		return utils.Error(c, 400, "Only school admin, teacher, sarpras, and koperasi can be deleted here")
	}
	a.DB.Exec(`DELETE FROM users WHERE id = ? AND school_id = ?`, id, schoolID)
	return utils.Success(c, 200, fmt.Sprintf(`User "%s" berhasil dihapus`, current.Username), nil)
}

func (a *AppContext) GetMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var profile struct {
		ID                        uint    `json:"id"`
		FullName                  *string `json:"full_name"`
		Username                  string  `json:"username"`
		Role                      string  `json:"role"`
		SchoolID                  *uint   `json:"school_id"`
		ParentEmail               *string `json:"parent_email"`
		PhoneNumber               *string `json:"phone_number"`
		ProfileImage              *string `json:"profile_image"`
		FaceReferenceImage        *string `json:"face_reference_image"`
		FaceReferenceDescriptor   *string `json:"face_reference_descriptor"`
		SchoolName                *string `json:"school_name"`
		SchoolLogo                *string `json:"school_logo"`
		InventoryModuleEnabled    bool    `json:"inventory_module_enabled"`
		OfficialExamModuleEnabled bool    `json:"official_exam_module_enabled"`
		KoperasiModuleEnabled     bool    `json:"koperasi_module_enabled"`
	}
	err := a.DB.Table("users u").
		Select("u.id, u.full_name, u.username, u.role, u.school_id, u.parent_email, u.phone_number, u.profile_image, u.face_reference_image, u.face_reference_descriptor, s.name as school_name, s.logo_url as school_logo, COALESCE(s.inventory_module_enabled, true) as inventory_module_enabled, COALESCE(s.official_exam_module_enabled, true) as official_exam_module_enabled, COALESCE(s.koperasi_module_enabled, true) as koperasi_module_enabled").
		Joins("left join schools s on s.id = u.school_id").
		Where("u.id = ?", userID).
		Scan(&profile).Error
	if err != nil {
		return utils.Error(c, 500, "Failed Get Profile", err.Error())
	}
	return utils.Success(c, 200, "Success Get Profile", profile)
}

func (a *AppContext) UpdateMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var current models.User
	if err := a.DB.Where("id = ?", userID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User not found")
	}

	fullName := strings.TrimSpace(c.FormValue("full_name"))
	parentEmail := strings.TrimSpace(c.FormValue("parent_email"))
	phoneNumber := strings.TrimSpace(c.FormValue("phone_number"))
	currentPassword := c.FormValue("current_password")
	newPassword := c.FormValue("new_password")
	confirmPassword := c.FormValue("confirm_password")

	updates := map[string]interface{}{}

	if fullName != "" {
		updates["full_name"] = fullName
	} else {
		updates["full_name"] = nil
	}
	if parentEmail != "" {
		updates["parent_email"] = parentEmail
	} else {
		updates["parent_email"] = nil
	}
	if phoneNumber != "" {
		updates["phone_number"] = phoneNumber
	} else {
		updates["phone_number"] = nil
	}

	if newPassword != "" || confirmPassword != "" || currentPassword != "" {
		if currentPassword == "" {
			return utils.Error(c, 400, "Password saat ini wajib diisi untuk mengganti password")
		}
		if newPassword == "" {
			return utils.Error(c, 400, "Password baru wajib diisi")
		}
		if len(newPassword) < 6 {
			return utils.Error(c, 400, "Password baru minimal 6 karakter")
		}
		if newPassword != confirmPassword {
			return utils.Error(c, 400, "Konfirmasi password baru tidak cocok")
		}
		if bcrypt.CompareHashAndPassword([]byte(current.Password), []byte(currentPassword)) != nil {
			return utils.Error(c, 401, "Password saat ini salah")
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(newPassword), 8)
		updates["password"] = string(hash)
		updates["session_version"] = gorm.Expr("COALESCE(session_version, 0) + 1")
	}

	if f, err := c.FormFile("profile_image"); err == nil && f != nil {
		saved, upErr := utils.SaveUploadedFile(c, f)
		if upErr != nil {
			return utils.Error(c, 500, "Gagal upload foto profil", upErr.Error())
		}
		updates["profile_image"] = saved
	}
	if strings.EqualFold(strings.TrimSpace(c.FormValue("remove_face_reference")), "true") {
		updates["face_reference_image"] = nil
		updates["face_reference_descriptor"] = nil
	}
	faceReferenceDescriptor := strings.TrimSpace(c.FormValue("face_reference_descriptor"))
	if f, err := c.FormFile("face_reference_image"); err == nil && f != nil {
		saved, upErr := utils.SaveUploadedFile(c, f)
		if upErr != nil {
			return utils.Error(c, 500, "Gagal upload foto referensi wajah", upErr.Error())
		}
		updates["face_reference_image"] = saved
	}
	if faceReferenceDescriptor != "" {
		updates["face_reference_descriptor"] = faceReferenceDescriptor
	}

	if len(updates) == 0 {
		return utils.Error(c, 400, "Tidak ada perubahan profil")
	}

	if err := a.DB.Table("users").Where("id = ?", userID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui profil", err.Error())
	}

	return a.GetMyProfile(c)
}

func coalesceStrPtr(v *string, fallback *string) interface{} {
	if v != nil {
		return *v
	}
	if fallback == nil {
		return nil
	}
	return *fallback
}

func nullIfSessionValueEmpty(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func detectLoginDevice(userAgent string) string {
	normalized := strings.ToLower(strings.TrimSpace(userAgent))
	if normalized == "" {
		return "Perangkat tidak dikenal"
	}

	deviceType := "Desktop"
	switch {
	case strings.Contains(normalized, "ipad"):
		deviceType = "Tablet"
	case strings.Contains(normalized, "tablet"):
		deviceType = "Tablet"
	case strings.Contains(normalized, "mobile"), strings.Contains(normalized, "iphone"), strings.Contains(normalized, "android"):
		deviceType = "Mobile"
	}

	platform := "Unknown OS"
	switch {
	case strings.Contains(normalized, "windows"):
		platform = "Windows"
	case strings.Contains(normalized, "mac os"), strings.Contains(normalized, "macintosh"):
		platform = "macOS"
	case strings.Contains(normalized, "iphone"), strings.Contains(normalized, "ipad"), strings.Contains(normalized, "ios"):
		platform = "iOS"
	case strings.Contains(normalized, "android"):
		platform = "Android"
	case strings.Contains(normalized, "linux"):
		platform = "Linux"
	}

	browser := "Browser tidak dikenal"
	switch {
	case strings.Contains(normalized, "edg/"):
		browser = "Microsoft Edge"
	case strings.Contains(normalized, "opr/"), strings.Contains(normalized, "opera"):
		browser = "Opera"
	case strings.Contains(normalized, "chrome/") && !strings.Contains(normalized, "edg/") && !strings.Contains(normalized, "opr/"):
		browser = "Google Chrome"
	case strings.Contains(normalized, "firefox/"):
		browser = "Mozilla Firefox"
	case strings.Contains(normalized, "safari/") && !strings.Contains(normalized, "chrome/"):
		browser = "Safari"
	}

	return fmt.Sprintf("%s • %s • %s", deviceType, platform, browser)
}

func (a *AppContext) CheckUsernameAvailability(c *fiber.Ctx) error {
	username := strings.TrimSpace(c.Query("username"))
	if username == "" {
		return utils.Error(c, 400, "Username wajib diisi")
	}

	if len(username) < 3 {
		return utils.Error(c, 400, "Username minimal 3 karakter")
	}

	query := a.DB.Table("users").Where("LOWER(username) = LOWER(?)", username)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa username", err.Error())
	}

	if count > 0 {
		// Generate suggestions
		suggestions := make([]string, 0, 3)
		for i := 1; i <= 5; i++ {
			candidate := fmt.Sprintf("%s%d", username, i)
			var candidateCount int64
			if err := a.DB.Table("users").Where("LOWER(username) = LOWER(?)", candidate).Count(&candidateCount).Error; err == nil && candidateCount == 0 {
				suggestions = append(suggestions, candidate)
				if len(suggestions) >= 3 {
					break
				}
			}
		}

		return c.JSON(fiber.Map{
			"success":     true,
			"available":   false,
			"message":     fmt.Sprintf("Username '%s' sudah digunakan", username),
			"suggestions": suggestions,
		})
	}

	return c.JSON(fiber.Map{
		"success":   true,
		"available": true,
		"message":   fmt.Sprintf("Username '%s' tersedia", username),
	})
}
