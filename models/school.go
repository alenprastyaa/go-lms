package models

type School struct {
	ID                        uint    `gorm:"column:id;primaryKey" json:"id"`
	Name                      string  `gorm:"column:name" json:"name"`
	LogoURL                   *string `gorm:"column:logo_url" json:"logo_url"`
	InventoryModuleEnabled    bool    `gorm:"column:inventory_module_enabled" json:"inventory_module_enabled"`
	AttendanceModuleEnabled   bool    `gorm:"column:attendance_module_enabled" json:"attendance_module_enabled"`
	AttendanceLatitude        *float64 `gorm:"column:attendance_latitude" json:"attendance_latitude"`
	AttendanceLongitude       *float64 `gorm:"column:attendance_longitude" json:"attendance_longitude"`
	AttendanceRadiusMeters    *int     `gorm:"column:attendance_radius_meters" json:"attendance_radius_meters"`
	OfficialExamModuleEnabled bool    `gorm:"column:official_exam_module_enabled" json:"official_exam_module_enabled"`
	KoperasiModuleEnabled     bool    `gorm:"column:koperasi_module_enabled" json:"koperasi_module_enabled"`
	PrivateChatModuleEnabled  bool    `gorm:"column:private_chat_module_enabled" json:"private_chat_module_enabled"`
	TeachingModuleAIEnabled   bool    `gorm:"column:teaching_module_ai_enabled" json:"teaching_module_ai_enabled"`
}

func (School) TableName() string { return "schools" }
