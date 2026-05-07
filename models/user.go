package models

import "time"

type User struct {
	ID                      uint       `gorm:"column:id;primaryKey" json:"id"`
	FullName                *string    `gorm:"column:full_name" json:"full_name"`
	Username                string     `gorm:"column:username" json:"username"`
	Password                string     `gorm:"column:password" json:"-"`
	Role                    string     `gorm:"column:role" json:"role"`
	SessionVersion          int64      `gorm:"column:session_version" json:"session_version"`
	CurrentSessionDevice    *string    `gorm:"column:current_session_device" json:"current_session_device"`
	CurrentSessionUserAgent *string    `gorm:"column:current_session_user_agent" json:"current_session_user_agent"`
	CurrentSessionIP        *string    `gorm:"column:current_session_ip" json:"current_session_ip"`
	CurrentSessionLoginAt   *time.Time `gorm:"column:current_session_login_at" json:"current_session_login_at"`
	SchoolID                *uint      `gorm:"column:school_id" json:"school_id"`
	ClassID                 *uint      `gorm:"column:class_id" json:"class_id"`
	ParentEmail             *string    `gorm:"column:parent_email" json:"parent_email"`
	PhoneNumber             *string    `gorm:"column:phone_number" json:"phone_number"`
	ProfileImage            *string    `gorm:"column:profile_image" json:"profile_image"`
	FaceReferenceImage      *string    `gorm:"column:face_reference_image" json:"face_reference_image"`
	FaceReferenceDescriptor *string    `gorm:"column:face_reference_descriptor" json:"face_reference_descriptor"`
}

func (User) TableName() string { return "users" }
