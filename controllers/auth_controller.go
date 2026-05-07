package controllers

import (
	archivezip "archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
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

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	user := models.User{FullName: utils.StringPtr(body.FullName), Username: body.Username, Password: string(hash), Role: body.Role, SchoolID: body.SchoolID}
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
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)) != nil {
		return utils.Error(c, 401, "Invalid Password")
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
		_ = a.DB.Select("name", "logo_url").Where("id = ?", *user.SchoolID).First(&school).Error
		schoolName = school.Name
		schoolLogo = school.LogoURL
	}

	return utils.Success(c, 200, "Login successful", fiber.Map{
		"role": user.Role, "username": user.Username, "school_id": user.SchoolID, "school_name": schoolName, "school_logo": schoolLogo, "profile_image": user.ProfileImage, "face_reference_image": user.FaceReferenceImage, "face_reference_descriptor": user.FaceReferenceDescriptor, "token": token,
	})
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

	requiredColumns := []string{"username", "password"}
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
	usernameSet := make([]string, 0, len(rows))
	for rowIndex := headerIndex + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		if isExcelRowEmpty(row) {
			continue
		}
		username := strings.TrimSpace(cellValue(row, columnIndex["username"]))
		if username != "" {
			usernameSet = append(usernameSet, username)
		}
	}

	existingUsernames := make(map[string]struct{}, len(usernameSet))
	if len(usernameSet) > 0 {
		var existingRows []struct {
			Username string `gorm:"column:username"`
		}
		if err := tx.Table("users").
			Select("username").
			Where("school_id = ? AND username IN ?", schoolID, usernameSet).
			Scan(&existingRows).Error; err != nil {
			tx.Rollback()
			return utils.Error(c, 500, "Gagal memeriksa data user", err.Error())
		}
		for _, item := range existingRows {
			existingUsernames[item.Username] = struct{}{}
		}
	}

	for rowIndex := headerIndex + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		if isExcelRowEmpty(row) {
			continue
		}

		record := map[string]string{
			"full_name":    cellValue(row, columnIndex["full_name"]),
			"username":     strings.TrimSpace(cellValue(row, columnIndex["username"])),
			"password":     strings.TrimSpace(cellValue(row, columnIndex["password"])),
			"parent_email": strings.TrimSpace(cellValue(row, columnIndex["parent_email"])),
			"phone_number": strings.TrimSpace(cellValue(row, columnIndex["phone_number"])),
		}

		if record["username"] == "" || record["password"] == "" {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": "username dan password wajib diisi",
			})
			continue
		}

		if _, exists := existingUsernames[record["username"]]; exists {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": fmt.Sprintf("username %s sudah ada", record["username"]),
			})
			continue
		}

		hash, _ := bcrypt.GenerateFromPassword([]byte(record["password"]), 8)
		user := models.User{
			FullName:    utils.StringPtr(record["full_name"]),
			Username:    record["username"],
			Password:    string(hash),
			Role:        "GURU",
			SchoolID:    &schoolID,
			ParentEmail: utils.StringPtr(record["parent_email"]),
			PhoneNumber: utils.StringPtr(record["phone_number"]),
		}

		if err := tx.Create(&user).Error; err != nil {
			failedRows = append(failedRows, fiber.Map{
				"row":     rowIndex + 1,
				"message": err.Error(),
			})
			continue
		}
		existingUsernames[record["username"]] = struct{}{}
		imported += 1
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan hasil import", err.Error())
	}

	return utils.Success(c, 200, "Import guru selesai", fiber.Map{
		"imported": imported,
		"failed":   len(failedRows),
		"errors":   failedRows,
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

func (a *AppContext) registerScopedUser(c *fiber.Ctx, asStudent bool) error {
	var body map[string]interface{}
	_ = c.BodyParser(&body)

	schoolID := c.Locals("schoolID").(uint)
	role := utils.ToString(body["role"])
	if asStudent {
		role = "SISWA"
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(utils.ToString(body["password"])), 8)
	user := models.User{
		FullName:    utils.StringPtr(body["full_name"]),
		Username:    utils.ToString(body["username"]),
		Password:    string(hash),
		Role:        role,
		SchoolID:    &schoolID,
		ParentEmail: utils.StringPtr(body["parent_email"]),
		PhoneNumber: utils.StringPtr(body["phone_number"]),
	}
	if asStudent {
		classID := uint(utils.ToInt(utils.ToString(body["class_id"]), 0))
		user.ClassID = &classID
	}

	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Registration failed", err.Error())
	}
	return utils.Success(c, 201, "User registered successfully", user)
}

func buildTeacherTemplateXLSX() ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := archivezip.NewWriter(&buffer)

	files := map[string]string{
		"[Content_Types].xml":        xlsxContentTypesXML(),
		"_rels/.rels":                xlsxRootRelsXML(),
		"xl/workbook.xml":            xlsxWorkbookXML(),
		"xl/_rels/workbook.xml.rels": xlsxWorkbookRelsXML(),
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

func xlsxContentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`
}

func xlsxRootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
}

func xlsxWorkbookXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Template Guru" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`
}

func xlsxWorkbookRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`
}

func xlsxTeacherTemplateSheetXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="inlineStr"><is><t xml:space="preserve">Template Import Guru</t></is></c>
    </row>
    <row r="2">
      <c r="A2" t="inlineStr"><is><t xml:space="preserve">Role akan otomatis diisi sebagai GURU.</t></is></c>
    </row>
    <row r="3">
      <c r="A3" t="inlineStr"><is><t xml:space="preserve">Isi 1 baris per guru lalu upload kembali file ini.</t></is></c>
    </row>
    <row r="5">
      <c r="A5" t="inlineStr"><is><t xml:space="preserve">Nama Lengkap</t></is></c>
      <c r="B5" t="inlineStr"><is><t xml:space="preserve">Username</t></is></c>
      <c r="C5" t="inlineStr"><is><t xml:space="preserve">Password</t></is></c>
      <c r="D5" t="inlineStr"><is><t xml:space="preserve">Email</t></is></c>
      <c r="E5" t="inlineStr"><is><t xml:space="preserve">No. HP</t></is></c>
    </row>
  </sheetData>
</worksheet>`
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
	_, hasUsername := columnIndex["username"]
	_, hasPassword := columnIndex["password"]
	return hasUsername && hasPassword
}

func isExcelRowEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func cellValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
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
	q := a.DB.Table("users").Select("id, full_name, username, role, school_id, parent_email, phone_number, profile_image").Where("school_id = ?", schoolID)
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
	return utils.Success(c, 200, "Success Get User School", users)
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
	if current.Role != "ADMIN" && current.Role != "GURU" {
		return utils.Error(c, 400, "Only school admin and teacher can be updated here")
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
	if body.Password != nil && *body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(*body.Password), 8)
		updates["password"] = string(hash)
		updates["session_version"] = gorm.Expr("COALESCE(session_version, 0) + 1")
	}
	a.DB.Table("users").Where("id = ? AND school_id = ?", id, schoolID).Updates(updates)
	var updated map[string]interface{}
	a.DB.Table("users").Select("id, full_name, username, role, school_id, parent_email, phone_number, profile_image").Where("id = ?", id).Scan(&updated)
	return utils.Success(c, 200, "User school updated successfully", updated)
}
func (a *AppContext) DeleteUserSchool(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var current models.User
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "User school not found")
	}
	if current.Role != "ADMIN" && current.Role != "GURU" {
		return utils.Error(c, 400, "Only school admin and teacher can be deleted here")
	}
	a.DB.Exec(`DELETE FROM users WHERE id = ? AND school_id = ?`, id, schoolID)
	return utils.Success(c, 200, fmt.Sprintf(`User "%s" berhasil dihapus`, current.Username), nil)
}

func (a *AppContext) GetMyProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var profile struct {
		ID                      uint    `json:"id"`
		FullName                *string `json:"full_name"`
		Username                string  `json:"username"`
		Role                    string  `json:"role"`
		SchoolID                *uint   `json:"school_id"`
		ParentEmail             *string `json:"parent_email"`
		PhoneNumber             *string `json:"phone_number"`
		ProfileImage            *string `json:"profile_image"`
		FaceReferenceImage      *string `json:"face_reference_image"`
		FaceReferenceDescriptor *string `json:"face_reference_descriptor"`
		SchoolName              *string `json:"school_name"`
		SchoolLogo              *string `json:"school_logo"`
	}
	err := a.DB.Table("users u").
		Select("u.id, u.full_name, u.username, u.role, u.school_id, u.parent_email, u.phone_number, u.profile_image, u.face_reference_image, u.face_reference_descriptor, s.name as school_name, s.logo_url as school_logo").
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
