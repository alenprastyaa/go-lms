package models

type School struct {
	ID   uint   `gorm:"column:id;primaryKey" json:"id"`
	Name string `gorm:"column:name" json:"name"`
}

func (School) TableName() string { return "schools" }
