package models

import "time"

type Receipt struct {
	ID          uint       `gorm:"column:id;primaryKey" json:"id"`
	ImagePath   string     `gorm:"column:image_path" json:"image_path"`
	PaymentDate *time.Time `gorm:"column:payment_date" json:"payment_date"`
	Description *string    `gorm:"column:description" json:"description"`
	CreatedAt   *time.Time `gorm:"column:created_at" json:"created_at"`
	UserID      uint       `gorm:"column:user_id"`
}

func (Receipt) TableName() string { return "payment_receipt" }
