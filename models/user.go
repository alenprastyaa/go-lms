package models

type User struct {
	ID           uint    `gorm:"column:id;primaryKey" json:"id"`
	FullName     *string `gorm:"column:full_name" json:"full_name"`
	Username     string  `gorm:"column:username" json:"username"`
	Password     string  `gorm:"column:password" json:"-"`
	Role         string  `gorm:"column:role" json:"role"`
	SchoolID     *uint   `gorm:"column:school_id" json:"school_id"`
	ClassID      *uint   `gorm:"column:class_id" json:"class_id"`
	ParentEmail  *string `gorm:"column:parent_email" json:"parent_email"`
	PhoneNumber  *string `gorm:"column:phone_number" json:"phone_number"`
	ProfileImage *string `gorm:"column:profile_image" json:"profile_image"`
}

func (User) TableName() string { return "users" }
