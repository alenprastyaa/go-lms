package controllers

import (
	"fmt"
	"strings"

	"lms/models"
	"lms/services"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Daftar tool yang boleh dipanggil asisten AI.
//
// Prinsip yang dipegang berkas ini:
//  1. Model hanya boleh memilih tool, tidak pernah menentukan haknya sendiri.
//     Pemeriksaan peran dilakukan di sini, bukan di prompt.
//  2. Admin sekolah selalu dikunci pada sekolahnya sendiri, apa pun yang diminta model.
//  3. Aksi yang mengubah data ditandai IsWrite agar wajib melewati konfirmasi pengguna.

type agentActor struct {
	UserID   uint
	SchoolID uint
	Role     string
	Username string
}

func (a agentActor) isSuperAdmin() bool { return a.Role == "SUPER_ADMIN" }

type agentTool struct {
	Definition services.AgentTool
	// Peran yang boleh memakai tool ini.
	AllowedRoles []string
	// True bila tool mengubah data sehingga perlu konfirmasi pengguna.
	IsWrite bool
	// Medan opsional yang tetap ditawarkan pada formulir isian, meski tidak wajib.
	FormOptional []string
	// Ringkasan yang ditampilkan pada kartu konfirmasi.
	Summarize func(args map[string]any) string
	// Handler menjalankan aksi dan mengembalikan hasil yang dibacakan kembali ke model.
	Run func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error)
}

func (t agentTool) allowedFor(role string) bool {
	for _, allowed := range t.AllowedRoles {
		if allowed == role {
			return true
		}
	}
	return false
}

// ---------- pembantu pembacaan argumen ----------

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, exists := args[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func argUint(args map[string]any, key string) uint {
	raw := argString(args, key)
	if raw == "" {
		return 0
	}
	var parsed uint64
	if _, err := fmt.Sscanf(raw, "%d", &parsed); err != nil {
		return 0
	}
	return uint(parsed)
}

func argBoolPtr(args map[string]any, key string) *bool {
	if args == nil {
		return nil
	}
	value, exists := args[key]
	if !exists || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case bool:
		return &typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "ya", "aktif":
			result := true
			return &result
		case "false", "0", "tidak", "nonaktif":
			result := false
			return &result
		}
	}
	return nil
}

func schoolToMap(school models.School) map[string]any {
	return map[string]any{
		"id":                  school.ID,
		"nama":                school.Name,
		"modul_sarpras":       school.InventoryModuleEnabled,
		"modul_absensi_siswa": school.AttendanceModuleEnabled,
		"modul_absensi_guru":  school.AttendanceTeacherModuleEnabled,
		"modul_koperasi":      school.KoperasiModuleEnabled,
		"modul_chat_pribadi":  school.PrivateChatModuleEnabled,
		"modul_ujian_resmi":   school.OfficialExamModuleEnabled,
		"modul_ajar_ai":       school.TeachingModuleAIEnabled,
		"modul_payroll":       school.PayrollModuleEnabled,
	}
}

// Admin sekolah hanya boleh menyentuh sekolahnya sendiri. Bila model mengirim id lain,
// permintaan ditolak, bukan diam-diam dialihkan.
func resolveSchoolScope(actor agentActor, requested uint) (uint, error) {
	if actor.isSuperAdmin() {
		if requested == 0 {
			return 0, fmt.Errorf("id sekolah wajib disebutkan")
		}
		return requested, nil
	}
	if actor.SchoolID == 0 {
		return 0, fmt.Errorf("akun ini belum terhubung ke sekolah mana pun")
	}
	if requested != 0 && requested != actor.SchoolID {
		return 0, fmt.Errorf("anda hanya berwenang atas sekolah sendiri")
	}
	return actor.SchoolID, nil
}

// ---------- daftar tool ----------

func agentToolRegistry() []agentTool {
	return []agentTool{
		{
			Definition: services.AgentTool{
				Name:        "list_schools",
				Description: "Menampilkan daftar sekolah beserta jumlah admin, guru, dan siswa. Pakai untuk mencari sekolah atau mengetahui id sekolah.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"cari": map[string]any{
							"type":        "string",
							"description": "Kata kunci nama sekolah. Kosongkan untuk menampilkan semua.",
						},
					},
				},
			},
			AllowedRoles: []string{"SUPER_ADMIN", "ADMIN"},
			Run: func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error) {
				query := db.Model(&models.School{})
				if !actor.isSuperAdmin() {
					if actor.SchoolID == 0 {
						return nil, fmt.Errorf("akun ini belum terhubung ke sekolah mana pun")
					}
					query = query.Where("id = ?", actor.SchoolID)
				}
				if keyword := argString(args, "cari"); keyword != "" {
					query = query.Where("name ILIKE ?", "%"+keyword+"%")
				}

				var schools []models.School
				if err := query.Order("name ASC").Limit(50).Find(&schools).Error; err != nil {
					return nil, err
				}

				items := make([]map[string]any, 0, len(schools))
				for _, school := range schools {
					entry := schoolToMap(school)
					var admins, teachers, students int64
					db.Model(&models.User{}).Where("school_id = ? AND role = ?", school.ID, "ADMIN").Count(&admins)
					db.Model(&models.User{}).Where("school_id = ? AND role = ?", school.ID, "GURU").Count(&teachers)
					db.Model(&models.User{}).Where("school_id = ? AND role = ?", school.ID, "SISWA").Count(&students)
					entry["jumlah_admin"] = admins
					entry["jumlah_guru"] = teachers
					entry["jumlah_siswa"] = students
					items = append(items, entry)
				}

				return map[string]any{"jumlah": len(items), "sekolah": items}, nil
			},
		},
		{
			Definition: services.AgentTool{
				Name:        "create_school",
				Description: "Membuat sekolah baru. Seluruh modul otomatis aktif. Hanya perlu nama sekolah.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"nama": map[string]any{
							"type":        "string",
							"description": "Nama lengkap sekolah, contoh: SMK PGRI 1 Medan",
						},
					},
					"required": []string{"nama"},
				},
			},
			AllowedRoles: []string{"SUPER_ADMIN"},
			IsWrite:      true,
			Summarize: func(args map[string]any) string {
				return fmt.Sprintf("Membuat sekolah baru bernama %q dengan seluruh modul aktif.", argString(args, "nama"))
			},
			Run: func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error) {
				name := argString(args, "nama")
				if name == "" {
					return nil, fmt.Errorf("nama sekolah wajib diisi")
				}

				var existing int64
				db.Model(&models.School{}).Where("LOWER(name) = LOWER(?)", name).Count(&existing)
				if existing > 0 {
					return nil, fmt.Errorf("sekolah bernama %q sudah ada", name)
				}

				school := models.School{
					Name:                           name,
					InventoryModuleEnabled:         true,
					AttendanceModuleEnabled:        true,
					AttendanceTeacherModuleEnabled: true,
					OfficialExamModuleEnabled:      true,
					KoperasiModuleEnabled:          true,
					PrivateChatModuleEnabled:       true,
					TeachingModuleAIEnabled:        true,
					PayrollModuleEnabled:           true,
					AttendanceSeatMapColumns:       4,
				}
				if err := db.Create(&school).Error; err != nil {
					return nil, err
				}
				return map[string]any{"status": "berhasil", "sekolah": schoolToMap(school)}, nil
			},
		},
		{
			Definition: services.AgentTool{
				Name:        "rename_school",
				Description: "Mengubah nama sekolah yang sudah ada.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id_sekolah": map[string]any{"type": "integer", "description": "Id sekolah yang akan diubah"},
						"nama_baru":  map[string]any{"type": "string", "description": "Nama sekolah yang baru"},
					},
					"required": []string{"nama_baru"},
				},
			},
			AllowedRoles: []string{"SUPER_ADMIN", "ADMIN"},
			IsWrite:      true,
			Summarize: func(args map[string]any) string {
				return fmt.Sprintf("Mengubah nama sekolah id %s menjadi %q.", argString(args, "id_sekolah"), argString(args, "nama_baru"))
			},
			Run: func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error) {
				schoolID, err := resolveSchoolScope(actor, argUint(args, "id_sekolah"))
				if err != nil {
					return nil, err
				}
				newName := argString(args, "nama_baru")
				if newName == "" {
					return nil, fmt.Errorf("nama baru wajib diisi")
				}

				var school models.School
				if err := db.First(&school, schoolID).Error; err != nil {
					return nil, fmt.Errorf("sekolah tidak ditemukan")
				}
				if err := db.Model(&school).Update("name", newName).Error; err != nil {
					return nil, err
				}
				school.Name = newName
				return map[string]any{"status": "berhasil", "sekolah": schoolToMap(school)}, nil
			},
		},
		{
			Definition: services.AgentTool{
				Name:        "set_school_modules",
				Description: "Menyalakan atau mematikan modul pada sebuah sekolah. Sebutkan hanya modul yang ingin diubah.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id_sekolah":          map[string]any{"type": "integer", "description": "Id sekolah"},
						"modul_sarpras":       map[string]any{"type": "boolean"},
						"modul_absensi_siswa": map[string]any{"type": "boolean"},
						"modul_absensi_guru":  map[string]any{"type": "boolean"},
						"modul_koperasi":      map[string]any{"type": "boolean"},
						"modul_chat_pribadi":  map[string]any{"type": "boolean"},
						"modul_ujian_resmi":   map[string]any{"type": "boolean"},
						"modul_ajar_ai":       map[string]any{"type": "boolean"},
						"modul_payroll":       map[string]any{"type": "boolean"},
					},
				},
			},
			AllowedRoles: []string{"SUPER_ADMIN", "ADMIN"},
			IsWrite:      true,
			Summarize: func(args map[string]any) string {
				changes := make([]string, 0)
				for key, column := range agentModuleColumns {
					if value := argBoolPtr(args, key); value != nil {
						state := "dimatikan"
						if *value {
							state = "dinyalakan"
						}
						changes = append(changes, fmt.Sprintf("%s %s", strings.ReplaceAll(strings.TrimPrefix(key, "modul_"), "_", " "), state))
					}
					_ = column
				}
				if len(changes) == 0 {
					return "Tidak ada modul yang diubah."
				}
				return fmt.Sprintf("Mengubah modul sekolah id %s: %s.", argString(args, "id_sekolah"), strings.Join(changes, ", "))
			},
			Run: func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error) {
				schoolID, err := resolveSchoolScope(actor, argUint(args, "id_sekolah"))
				if err != nil {
					return nil, err
				}

				updates := map[string]any{}
				for key, column := range agentModuleColumns {
					if value := argBoolPtr(args, key); value != nil {
						updates[column] = *value
					}
				}
				if len(updates) == 0 {
					return nil, fmt.Errorf("tidak ada modul yang disebutkan untuk diubah")
				}

				var school models.School
				if err := db.First(&school, schoolID).Error; err != nil {
					return nil, fmt.Errorf("sekolah tidak ditemukan")
				}
				if err := db.Model(&school).Updates(updates).Error; err != nil {
					return nil, err
				}
				if err := db.First(&school, schoolID).Error; err != nil {
					return nil, err
				}
				return map[string]any{"status": "berhasil", "sekolah": schoolToMap(school)}, nil
			},
		},
		{
			Definition: services.AgentTool{
				Name:        "create_school_admin",
				Description: "Membuat akun admin untuk sebuah sekolah.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id_sekolah": map[string]any{"type": "integer", "description": "Id sekolah tempat admin ditugaskan"},
						"nama":       map[string]any{"type": "string", "description": "Nama lengkap admin"},
						"username":   map[string]any{"type": "string", "description": "Username untuk masuk"},
						"password":   map[string]any{"type": "string", "description": "Kata sandi awal, minimal 6 karakter"},
					},
					"required": []string{"username", "password"},
				},
			},
			AllowedRoles: []string{"SUPER_ADMIN"},
			IsWrite:      true,
			FormOptional: []string{"nama"},
			Summarize: func(args map[string]any) string {
				return fmt.Sprintf("Membuat admin sekolah dengan username %q pada sekolah id %s.", argString(args, "username"), argString(args, "id_sekolah"))
			},
			Run: func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error) {
				schoolID, err := resolveSchoolScope(actor, argUint(args, "id_sekolah"))
				if err != nil {
					return nil, err
				}
				username := argString(args, "username")
				password := argString(args, "password")
				if username == "" {
					return nil, fmt.Errorf("username wajib diisi")
				}
				if len(password) < 6 {
					return nil, fmt.Errorf("kata sandi minimal 6 karakter")
				}

				var school models.School
				if err := db.First(&school, schoolID).Error; err != nil {
					return nil, fmt.Errorf("sekolah tidak ditemukan")
				}
				var taken int64
				db.Model(&models.User{}).Where("LOWER(username) = LOWER(?)", username).Count(&taken)
				if taken > 0 {
					return nil, fmt.Errorf("username %q sudah dipakai", username)
				}

				hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), 8)
				if hashErr != nil {
					return nil, hashErr
				}
				fullName := argString(args, "nama")
				user := models.User{
					Username: username,
					Password: string(hash),
					Role:     "ADMIN",
					SchoolID: &schoolID,
				}
				if fullName != "" {
					user.FullName = &fullName
				}
				if err := db.Create(&user).Error; err != nil {
					return nil, err
				}

				return map[string]any{
					"status":  "berhasil",
					"admin":   map[string]any{"id": user.ID, "username": user.Username, "nama": fullName},
					"sekolah": school.Name,
					"catatan": "Kata sandi tidak ditampilkan ulang. Sampaikan langsung kepada admin yang bersangkutan.",
				}, nil
			},
		},
		{
			Definition: services.AgentTool{
				Name:        "delete_school",
				Description: "Menghapus sekolah. Hanya boleh bila sekolah belum memiliki pengguna sama sekali.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id_sekolah": map[string]any{"type": "integer", "description": "Id sekolah yang akan dihapus"},
					},
					"required": []string{"id_sekolah"},
				},
			},
			AllowedRoles: []string{"SUPER_ADMIN"},
			IsWrite:      true,
			Summarize: func(args map[string]any) string {
				return fmt.Sprintf("MENGHAPUS sekolah id %s secara permanen.", argString(args, "id_sekolah"))
			},
			Run: func(db *gorm.DB, actor agentActor, args map[string]any) (map[string]any, error) {
				schoolID := argUint(args, "id_sekolah")
				if schoolID == 0 {
					return nil, fmt.Errorf("id sekolah wajib disebutkan")
				}

				var school models.School
				if err := db.First(&school, schoolID).Error; err != nil {
					return nil, fmt.Errorf("sekolah tidak ditemukan")
				}
				var users int64
				db.Model(&models.User{}).Where("school_id = ?", schoolID).Count(&users)
				if users > 0 {
					return nil, fmt.Errorf("sekolah %q masih memiliki %d pengguna, hapus penggunanya lebih dahulu lewat menu Sekolah", school.Name, users)
				}
				if err := db.Delete(&models.School{}, schoolID).Error; err != nil {
					return nil, err
				}
				return map[string]any{"status": "berhasil", "terhapus": school.Name}, nil
			},
		},
	}
}

var agentModuleColumns = map[string]string{
	"modul_sarpras":       "inventory_module_enabled",
	"modul_absensi_siswa": "attendance_module_enabled",
	"modul_absensi_guru":  "attendance_teacher_module_enabled",
	"modul_koperasi":      "koperasi_module_enabled",
	"modul_chat_pribadi":  "private_chat_module_enabled",
	"modul_ujian_resmi":   "official_exam_module_enabled",
	"modul_ajar_ai":       "teaching_module_ai_enabled",
	"modul_payroll":       "payroll_module_enabled",
}

func agentToolsForRole(role string) []agentTool {
	all := agentToolRegistry()
	tools := make([]agentTool, 0, len(all))
	for _, tool := range all {
		if tool.allowedFor(role) {
			tools = append(tools, tool)
		}
	}
	return tools
}

func findAgentTool(name string) (agentTool, bool) {
	for _, tool := range agentToolRegistry() {
		if tool.Definition.Name == name {
			return tool, true
		}
	}
	return agentTool{}, false
}
