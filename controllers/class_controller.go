package controllers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/utils"
)

type classLevelRow struct {
	ID        uint   `json:"id" gorm:"column:id"`
	Name      string `json:"name" gorm:"column:name"`
	SortOrder int    `json:"sort_order" gorm:"column:sort_order"`
}

func normalizeClassLevelName(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func classLevelDefaultSortOrder(name string) int {
	if value := utils.ToInt(name, 0); value > 0 {
		return value
	}
	return 0
}

func classNameFromLevel(levelName, rombelName string) string {
	levelName = normalizeClassLevelName(levelName)
	rombelName = strings.Join(strings.Fields(strings.TrimSpace(rombelName)), " ")
	if levelName == "" {
		return rombelName
	}
	if rombelName == "" {
		return levelName
	}
	if strings.EqualFold(rombelName, levelName) || strings.HasPrefix(strings.ToLower(rombelName), strings.ToLower(levelName+" ")) {
		return rombelName
	}
	return strings.TrimSpace(levelName + " " + rombelName)
}

func (a *AppContext) ensureDefaultClassLevels(schoolID uint) error {
	var count int64
	if err := a.DB.Table("class_levels").Where("school_id = ?", schoolID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaultLevels := []string{"10", "11", "12"}
	for _, level := range defaultLevels {
		if err := a.DB.Exec(`
			INSERT INTO class_levels (school_id, name, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
			ON CONFLICT (school_id, name) DO NOTHING
		`, schoolID, level, classLevelDefaultSortOrder(level)).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *AppContext) classLevelByID(id uint, schoolID uint) (*classLevelRow, error) {
	if id == 0 {
		return nil, nil
	}
	var row classLevelRow
	if err := a.DB.Raw(`
		SELECT id, name, sort_order
		FROM class_levels
		WHERE id = ? AND school_id = ?
	`, id, schoolID).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, fmt.Errorf("Tingkat kelas tidak valid")
	}
	return &row, nil
}

func (a *AppContext) GetClassLevels(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	if err := a.ensureDefaultClassLevels(schoolID); err != nil {
		return utils.Error(c, 500, "Gagal menyiapkan master tingkat kelas", err.Error())
	}
	var rows []classLevelRow
	if err := a.DB.Raw(`
		SELECT id, name, sort_order
		FROM class_levels
		WHERE school_id = ?
		ORDER BY sort_order ASC, name ASC
	`, schoolID).Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat master tingkat kelas", err.Error())
	}
	return utils.Success(c, 200, "Success Get Class Levels", rows)
}

func (a *AppContext) CreateClassLevel(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Name      string `json:"name"`
		SortOrder *int   `json:"sort_order"`
	}
	_ = c.BodyParser(&body)
	name := normalizeClassLevelName(body.Name)
	if name == "" {
		return utils.Error(c, 400, "Nama tingkat kelas wajib diisi")
	}
	sortOrder := classLevelDefaultSortOrder(name)
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	}
	var row classLevelRow
	if err := a.DB.Raw(`
		INSERT INTO class_levels (school_id, name, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		RETURNING id, name, sort_order
	`, schoolID, name, sortOrder).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat tingkat kelas", err.Error())
	}
	return utils.Success(c, 201, "Tingkat kelas berhasil dibuat", row)
}

func (a *AppContext) UpdateClassLevel(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Name      *string `json:"name"`
		SortOrder *int    `json:"sort_order"`
	}
	_ = c.BodyParser(&body)
	var current classLevelRow
	if err := a.DB.Raw(`SELECT id, name, sort_order FROM class_levels WHERE id = ? AND school_id = ?`, id, schoolID).Scan(&current).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat tingkat kelas", err.Error())
	}
	if current.ID == 0 {
		return utils.Error(c, 404, "Tingkat kelas tidak ditemukan")
	}
	name := current.Name
	if body.Name != nil {
		name = normalizeClassLevelName(*body.Name)
		if name == "" {
			return utils.Error(c, 400, "Nama tingkat kelas wajib diisi")
		}
	}
	sortOrder := current.SortOrder
	if body.SortOrder != nil {
		sortOrder = *body.SortOrder
	}
	var row classLevelRow
	if err := a.DB.Raw(`
		UPDATE class_levels
		SET name = ?, sort_order = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ?
		RETURNING id, name, sort_order
	`, name, sortOrder, id, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui tingkat kelas", err.Error())
	}
	return utils.Success(c, 200, "Tingkat kelas berhasil diperbarui", row)
}

func (a *AppContext) DeleteClassLevel(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var usedCount int64
	if err := a.DB.Table("class").Where("school_id = ? AND class_level_id = ?", schoolID, id).Count(&usedCount).Error; err != nil {
		return utils.Error(c, 500, "Gagal memeriksa penggunaan tingkat kelas", err.Error())
	}
	if usedCount > 0 {
		return utils.Error(c, 400, "Tingkat kelas masih digunakan oleh rombel")
	}
	var row classLevelRow
	if err := a.DB.Raw(`
		DELETE FROM class_levels
		WHERE id = ? AND school_id = ?
		RETURNING id, name, sort_order
	`, id, schoolID).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus tingkat kelas", err.Error())
	}
	if row.ID == 0 {
		return utils.Error(c, 404, "Tingkat kelas tidak ditemukan")
	}
	return utils.Success(c, 200, fmt.Sprintf("Tingkat kelas %s berhasil dihapus", row.Name), nil)
}

func (a *AppContext) CreateClass(c *fiber.Ctx) error {
	var body struct {
		ClassName    string `json:"class_name"`
		ClassLevelID *uint  `json:"class_level_id"`
		WaliGuruID   *uint  `json:"wali_guru_id"`
		MajorID      *uint  `json:"major_id"`
	}
	_ = c.BodyParser(&body)
	schoolID := c.Locals("schoolID").(uint)
	if body.MajorID != nil && !a.majorBelongsToSchool(*body.MajorID, schoolID) {
		return utils.Error(c, 400, "Jurusan tidak valid")
	}
	var level *classLevelRow
	if body.ClassLevelID != nil {
		var err error
		level, err = a.classLevelByID(*body.ClassLevelID, schoolID)
		if err != nil {
			return utils.Error(c, 400, err.Error())
		}
	}
	className := classNameFromLevel("", body.ClassName)
	if level != nil {
		className = classNameFromLevel(level.Name, body.ClassName)
	}
	if className == "" {
		return utils.Error(c, 400, "Nama kelas wajib diisi")
	}
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO class (class_name, school_id, wali_guru_id, major_id, class_level_id)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, class_name, school_id, wali_guru_id, major_id, class_level_id
	`, className, schoolID, body.WaliGuruID, body.MajorID, body.ClassLevelID).Scan(&row)
	return utils.Success(c, 201, "class registered successfully", row)
}

func (a *AppContext) BulkGenerateClasses(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Items []struct {
			ClassLevelID uint   `json:"class_level_id"`
			BaseName     string `json:"base_name"`
			Count        int    `json:"count"`
			MajorID      *uint  `json:"major_id"`
		} `json:"items"`
	}
	_ = c.BodyParser(&body)
	if len(body.Items) == 0 {
		return utils.Error(c, 400, "Minimal satu pola kelas wajib diisi")
	}

	createdRows := make([]map[string]interface{}, 0)
	skippedNames := make([]string, 0)
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		existing := map[string]struct{}{}
		var existingRows []struct {
			ClassName string `gorm:"column:class_name"`
		}
		if err := tx.Raw(`SELECT class_name FROM class WHERE school_id = ?`, schoolID).Scan(&existingRows).Error; err != nil {
			return err
		}
		for _, row := range existingRows {
			existing[strings.ToLower(strings.TrimSpace(row.ClassName))] = struct{}{}
		}

		for _, item := range body.Items {
			baseName := strings.Join(strings.Fields(strings.TrimSpace(item.BaseName)), " ")
			if item.ClassLevelID == 0 {
				return fmt.Errorf("Tingkat kelas wajib dipilih")
			}
			if baseName == "" {
				return fmt.Errorf("Nama rombel wajib diisi")
			}
			if item.Count <= 0 {
				return fmt.Errorf("Jumlah rombel wajib lebih dari 0")
			}
			if item.Count > 50 {
				return fmt.Errorf("Jumlah rombel maksimal 50 per pola")
			}
			level, err := a.classLevelByID(item.ClassLevelID, schoolID)
			if err != nil {
				return err
			}
			if item.MajorID != nil {
				var majorCount int64
				if err := tx.Table("majors").Where("id = ? AND school_id = ?", *item.MajorID, schoolID).Count(&majorCount).Error; err != nil {
					return err
				}
				if majorCount == 0 {
					return fmt.Errorf("Jurusan tidak valid")
				}
			}

			for index := 1; index <= item.Count; index++ {
				rombelName := fmt.Sprintf("%s %d", baseName, index)
				className := classNameFromLevel(level.Name, rombelName)
				key := strings.ToLower(strings.TrimSpace(className))
				if _, ok := existing[key]; ok {
					skippedNames = append(skippedNames, className)
					continue
				}
				var row map[string]interface{}
				if err := tx.Raw(`
					INSERT INTO class (class_name, school_id, wali_guru_id, major_id, class_level_id)
					VALUES (?, ?, NULL, ?, ?)
					RETURNING id, class_name, school_id, wali_guru_id, major_id, class_level_id
				`, className, schoolID, item.MajorID, item.ClassLevelID).Scan(&row).Error; err != nil {
					return err
				}
				existing[key] = struct{}{}
				createdRows = append(createdRows, row)
			}
		}
		return nil
	})
	if err != nil {
		return utils.Error(c, 400, "Gagal generate kelas cepat", err.Error())
	}

	return utils.Success(c, 201, "Generate kelas cepat selesai", fiber.Map{
		"created":       len(createdRows),
		"skipped":       len(skippedNames),
		"created_items": createdRows,
		"skipped_names": skippedNames,
	})
}

func (a *AppContext) GetClasses(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
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

	if usePagination {
		var totalRow struct {
			Total int64 `json:"total"`
		}
		_ = a.DB.Raw(`SELECT COUNT(*) AS total FROM class WHERE school_id = ?`, schoolID).Scan(&totalRow).Error

		var rows []map[string]interface{}
		a.DB.Raw(`
			SELECT c.id, c.class_name, c.school_id, c.wali_guru_id, c.major_id, c.class_level_id,
			       cl.name AS class_level_name, cl.sort_order AS class_level_order,
			       CASE
			         WHEN cl.name IS NOT NULL AND LEFT(c.class_name, LENGTH(cl.name) + 1) = cl.name || ' '
			         THEN BTRIM(SUBSTRING(c.class_name FROM LENGTH(cl.name) + 2))
			         ELSE c.class_name
			       END AS class_rombel_name,
			       m.name AS major_name, m.code AS major_code,
			       u.username AS wali_guru_name, u.parent_email AS wali_guru_email, u.phone_number AS wali_guru_phone_number
			FROM class c
			LEFT JOIN users u ON c.wali_guru_id = u.id
			LEFT JOIN majors m ON m.id = c.major_id
			LEFT JOIN class_levels cl ON cl.id = c.class_level_id
			WHERE c.school_id = ?
			ORDER BY COALESCE(cl.sort_order, 9999) ASC, c.class_name ASC
			LIMIT ? OFFSET ?
		`, schoolID, limit, offset).Scan(&rows)
		return utils.Success(c, 200, "Succes Get Data class", fiber.Map{
			"page":  page,
			"limit": limit,
			"total": totalRow.Total,
			"data":  rows,
		})
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT c.id, c.class_name, c.school_id, c.wali_guru_id, c.major_id, c.class_level_id,
		       cl.name AS class_level_name, cl.sort_order AS class_level_order,
		       CASE
		         WHEN cl.name IS NOT NULL AND LEFT(c.class_name, LENGTH(cl.name) + 1) = cl.name || ' '
		         THEN BTRIM(SUBSTRING(c.class_name FROM LENGTH(cl.name) + 2))
		         ELSE c.class_name
		       END AS class_rombel_name,
		       m.name AS major_name, m.code AS major_code,
		       u.username AS wali_guru_name, u.parent_email AS wali_guru_email, u.phone_number AS wali_guru_phone_number
		FROM class c
		LEFT JOIN users u ON c.wali_guru_id = u.id
		LEFT JOIN majors m ON m.id = c.major_id
		LEFT JOIN class_levels cl ON cl.id = c.class_level_id
		WHERE c.school_id = ?
		ORDER BY COALESCE(cl.sort_order, 9999) ASC, c.class_name ASC
	`, schoolID).Scan(&rows)
	return utils.Success(c, 200, "Succes Get Data class", rows)
}

func (a *AppContext) UpdateClass(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		ClassName    *string `json:"class_name"`
		ClassLevelID *uint   `json:"class_level_id"`
		WaliGuruID   *uint   `json:"wali_guru_id"`
		MajorID      *uint   `json:"major_id"`
	}
	_ = c.BodyParser(&body)
	var raw map[string]interface{}
	_ = c.BodyParser(&raw)
	var current struct {
		ClassName    string `json:"class_name"`
		ClassLevelID *uint  `json:"class_level_id"`
		WaliGuruID   *uint  `json:"wali_guru_id"`
		MajorID      *uint  `json:"major_id"`
	}
	if err := a.DB.Raw(`SELECT class_name, class_level_id, wali_guru_id, major_id FROM class WHERE id = ? AND school_id = ?`, id, schoolID).Scan(&current).Error; err != nil {
		return utils.Error(c, 404, "Class not found")
	}
	className := current.ClassName
	if body.ClassName != nil {
		className = *body.ClassName
	}
	classLevelID := current.ClassLevelID
	if _, ok := raw["class_level_id"]; ok {
		parsedClassLevelID, err := nullableUintFromRaw(raw["class_level_id"])
		if err != nil {
			return utils.Error(c, 400, "Tingkat kelas tidak valid")
		}
		classLevelID = parsedClassLevelID
	}
	if classLevelID != nil {
		level, err := a.classLevelByID(*classLevelID, schoolID)
		if err != nil {
			return utils.Error(c, 400, err.Error())
		}
		className = classNameFromLevel(level.Name, className)
	} else {
		className = classNameFromLevel("", className)
	}
	if className == "" {
		return utils.Error(c, 400, "Nama kelas wajib diisi")
	}
	waliGuruID := current.WaliGuruID
	if body.WaliGuruID != nil {
		waliGuruID = body.WaliGuruID
	}
	majorID := current.MajorID
	if _, ok := raw["major_id"]; ok {
		parsedMajorID, err := nullableUintFromRaw(raw["major_id"])
		if err != nil {
			return utils.Error(c, 400, "Jurusan tidak valid")
		}
		if parsedMajorID != nil && !a.majorBelongsToSchool(*parsedMajorID, schoolID) {
			return utils.Error(c, 400, "Jurusan tidak valid")
		}
		majorID = parsedMajorID
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE class SET class_name = ?, school_id = ?, wali_guru_id = ?, major_id = ?, class_level_id = ?
		WHERE id = ? RETURNING id, class_name, school_id, wali_guru_id, major_id, class_level_id
	`, className, schoolID, waliGuruID, majorID, classLevelID, id).Scan(&row)
	return utils.Success(c, 200, "Success Update Class", row)
}
func (a *AppContext) DeleteClass(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var row struct {
		ID        int    `json:"id"`
		ClassName string `json:"class_name"`
	}
	a.DB.Raw(`DELETE FROM class WHERE id = ? AND school_id = ? RETURNING id, class_name`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Class not found")
	}
	return utils.Success(c, 200, `Kelas "`+row.ClassName+`" berhasil dihapus`, nil)
}

func (a *AppContext) GetMyClass(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var row map[string]interface{}
	a.DB.Raw(`
		SELECT c.id, c.class_name, c.school_id, c.wali_guru_id, c.major_id, c.class_level_id,
		       cl.name AS class_level_name, cl.sort_order AS class_level_order,
		       m.name AS major_name, m.code AS major_code,
		       u.username AS wali_guru_name, u.parent_email AS wali_guru_email, u.phone_number AS wali_guru_phone_number
		FROM class c
		LEFT JOIN users u ON c.wali_guru_id = u.id
		LEFT JOIN majors m ON m.id = c.major_id
		LEFT JOIN class_levels cl ON cl.id = c.class_level_id
		WHERE c.wali_guru_id = ? AND c.school_id = ?
	`, userID, schoolID).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Homeroom class not found")
	}
	return utils.Success(c, 200, "Success Get Homeroom Class", row)
}
