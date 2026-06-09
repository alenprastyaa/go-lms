package models

import "time"

type Major struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	SchoolID  uint      `gorm:"column:school_id" json:"school_id"`
	Name      string    `gorm:"column:name" json:"name"`
	Code      string    `gorm:"column:code" json:"code"`
	Quota     *int      `gorm:"column:quota" json:"quota"`
	IsActive  bool      `gorm:"column:is_active" json:"is_active"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Major) TableName() string { return "majors" }
