package models

import "time"

type SPMBApplicant struct {
	ID                   uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolID             uint       `gorm:"column:school_id" json:"school_id"`
	RegistrationNumber   string     `gorm:"column:registration_number" json:"registration_number"`
	AccessTokenHash      *string    `gorm:"column:access_token_hash" json:"-"`
	AccessTokenExpiresAt *time.Time `gorm:"column:access_token_expires_at" json:"access_token_expires_at"`
	FullName             string     `gorm:"column:full_name" json:"full_name"`
	BirthPlace           *string    `gorm:"column:birth_place" json:"birth_place"`
	BirthDate            *time.Time `gorm:"column:birth_date" json:"birth_date"`
	Gender               *string    `gorm:"column:gender" json:"gender"`
	NISN                 *string    `gorm:"column:nisn" json:"nisn"`
	OriginSchool         *string    `gorm:"column:origin_school" json:"origin_school"`
	ParentName           *string    `gorm:"column:parent_name" json:"parent_name"`
	PhoneNumber          string     `gorm:"column:phone_number" json:"phone_number"`
	Email                *string    `gorm:"column:email" json:"email"`
	Address              *string    `gorm:"column:address" json:"address"`
	FirstMajorID         *uint      `gorm:"column:first_major_id" json:"first_major_id"`
	SecondMajorID        *uint      `gorm:"column:second_major_id" json:"second_major_id"`
	ThirdMajorID         *uint      `gorm:"column:third_major_id" json:"third_major_id"`
	AcceptedMajorID      *uint      `gorm:"column:accepted_major_id" json:"accepted_major_id"`
	Status               string     `gorm:"column:status" json:"status"`
	Notes                *string    `gorm:"column:notes" json:"notes"`
	RevisionNote         *string    `gorm:"column:revision_note" json:"revision_note"`
	LastLinkSentAt       *time.Time `gorm:"column:last_link_sent_at" json:"last_link_sent_at"`
	ConvertedUserID      *uint      `gorm:"column:converted_user_id" json:"converted_user_id"`
	CreatedBy            *uint      `gorm:"column:created_by" json:"created_by"`
	CreatedAt            time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (SPMBApplicant) TableName() string { return "spmb_applicants" }
