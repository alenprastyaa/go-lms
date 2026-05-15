package models

type School struct {
	ID                     uint    `gorm:"column:id;primaryKey" json:"id"`
	Name                   string  `gorm:"column:name" json:"name"`
	LogoURL                *string `gorm:"column:logo_url" json:"logo_url"`
	InventoryModuleEnabled  bool    `gorm:"column:inventory_module_enabled" json:"inventory_module_enabled"`
	OfficialExamModuleEnabled bool  `gorm:"column:official_exam_module_enabled" json:"official_exam_module_enabled"`
}

func (School) TableName() string { return "schools" }
