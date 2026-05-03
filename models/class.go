package models

type Class struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	ClassName  string `gorm:"column:class_name" json:"class_name"`
	SchoolID   uint   `gorm:"column:school_id" json:"school_id"`
	WaliGuruID *uint  `gorm:"column:wali_guru_id" json:"wali_guru_id"`
}

func (Class) TableName() string { return "class" }
