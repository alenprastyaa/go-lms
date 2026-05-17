package models

import "time"

type SchoolAnnouncement struct {
	ID             uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolID       uint       `gorm:"column:school_id" json:"school_id"`
	Title          string     `gorm:"column:title" json:"title"`
	Content        string     `gorm:"column:content" json:"content"`
	TargetAudience string     `gorm:"column:target_audience" json:"target_audience"`
	Status         string     `gorm:"column:status" json:"status"`
	ReviewedAt     *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`
	PublishedAt    *time.Time `gorm:"column:published_at" json:"published_at"`
	DeactivatedAt  *time.Time `gorm:"column:deactivated_at" json:"deactivated_at"`
	CreatedBy      *uint      `gorm:"column:created_by" json:"created_by"`
	UpdatedBy      *uint      `gorm:"column:updated_by" json:"updated_by"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (SchoolAnnouncement) TableName() string { return "school_announcements" }
