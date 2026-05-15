package models

import "time"

type InventoryItem struct {
	ID                uint      `gorm:"column:id;primaryKey" json:"id"`
	SchoolID          uint      `gorm:"column:school_id" json:"school_id"`
	Name              string    `gorm:"column:name" json:"name"`
	Code              *string   `gorm:"column:code" json:"code"`
	Category          *string   `gorm:"column:category" json:"category"`
	Description       *string   `gorm:"column:description" json:"description"`
	ConditionStatus   string    `gorm:"column:condition_status" json:"condition_status"`
	TotalQuantity     int       `gorm:"column:total_quantity" json:"total_quantity"`
	AvailableQuantity int       `gorm:"column:available_quantity" json:"available_quantity"`
	IsActive          bool      `gorm:"column:is_active" json:"is_active"`
	CreatedBy         *uint     `gorm:"column:created_by" json:"created_by"`
	UpdatedBy         *uint     `gorm:"column:updated_by" json:"updated_by"`
	CreatedAt         time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (InventoryItem) TableName() string { return "inventory_items" }

type InventoryLoan struct {
	ID         uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolID   uint       `gorm:"column:school_id" json:"school_id"`
	ItemID     uint       `gorm:"column:item_id" json:"item_id"`
	BorrowerID uint       `gorm:"column:borrower_id" json:"borrower_id"`
	TeacherID  *uint      `gorm:"column:teacher_id" json:"teacher_id"`
	Quantity   int        `gorm:"column:quantity" json:"quantity"`
	BorrowedAt time.Time  `gorm:"column:borrowed_at" json:"borrowed_at"`
	DueDate    *time.Time `gorm:"column:due_date" json:"due_date"`
	ReturnedAt *time.Time `gorm:"column:returned_at" json:"returned_at"`
	Status     string     `gorm:"column:status" json:"status"`
	Notes      *string    `gorm:"column:notes" json:"notes"`
	HandledBy  *uint      `gorm:"column:handled_by" json:"handled_by"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (InventoryLoan) TableName() string { return "inventory_loans" }
