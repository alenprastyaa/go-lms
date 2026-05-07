package models

type School struct {
	ID      uint    `gorm:"column:id;primaryKey" json:"id"`
	Name    string  `gorm:"column:name" json:"name"`
	LogoURL *string `gorm:"column:logo_url" json:"logo_url"`
}

func (School) TableName() string { return "schools" }
