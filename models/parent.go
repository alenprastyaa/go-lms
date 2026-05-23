package models

import "time"

type ParentStudent struct {
	ID            uint      `gorm:"column:id;primaryKey" json:"id"`
	SchoolID      uint      `gorm:"column:school_id" json:"school_id"`
	ParentUserID  uint      `gorm:"column:parent_user_id" json:"parent_user_id"`
	StudentUserID uint      `gorm:"column:student_user_id" json:"student_user_id"`
	Relationship  string    `gorm:"column:relationship" json:"relationship"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ParentStudent) TableName() string { return "parent_students" }

type ParentLoginOTP struct {
	ID           uint       `gorm:"column:id;primaryKey" json:"id"`
	ParentUserID uint       `gorm:"column:parent_user_id" json:"parent_user_id"`
	Identifier   string     `gorm:"column:email" json:"identifier"`
	OTPHash      string     `gorm:"column:otp_hash" json:"-"`
	ExpiresAt    time.Time  `gorm:"column:expires_at" json:"expires_at"`
	UsedAt       *time.Time `gorm:"column:used_at" json:"used_at"`
	Attempts     int        `gorm:"column:attempts" json:"attempts"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (ParentLoginOTP) TableName() string { return "parent_login_otps" }
