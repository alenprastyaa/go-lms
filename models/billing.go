package models

import "time"

type SchoolBilling struct {
	ID            uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolID      uint       `gorm:"column:school_id" json:"school_id"`
	BillingName   string     `gorm:"column:billing_name" json:"billing_name"`
	Amount        int64      `gorm:"column:amount" json:"amount"`
	Currency      string     `gorm:"column:currency" json:"currency"`
	DueDate       *time.Time `gorm:"column:due_date" json:"due_date"`
	DueDayOfMonth int        `gorm:"column:due_day_of_month" json:"due_day_of_month"`
	IsActive      bool       `gorm:"column:is_active" json:"is_active"`
	Notes         *string    `gorm:"column:notes" json:"notes"`
	CreatedAt     *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SchoolBilling) TableName() string { return "school_billings" }

type SchoolInvoice struct {
	ID              uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolBillingID uint       `gorm:"column:school_billing_id" json:"school_billing_id"`
	SchoolID        uint       `gorm:"column:school_id" json:"school_id"`
	InvoiceNumber   string     `gorm:"column:invoice_number" json:"invoice_number"`
	Amount          int64      `gorm:"column:amount" json:"amount"`
	DueDate         time.Time  `gorm:"column:due_date" json:"due_date"`
	Status          string     `gorm:"column:status" json:"status"`
	PaymentMethod   *string    `gorm:"column:payment_method" json:"payment_method"`
	GrossAmount     *int64     `gorm:"column:gross_amount" json:"gross_amount"`
	TransactionID   *string    `gorm:"column:transaction_id" json:"transaction_id"`
	SnapToken       *string    `gorm:"column:snap_token" json:"snap_token"`
	SnapRedirectURL *string    `gorm:"column:snap_redirect_url" json:"snap_redirect_url"`
	PaidAt          *time.Time `gorm:"column:paid_at" json:"paid_at"`
	CreatedAt       *time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       *time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SchoolInvoice) TableName() string { return "school_invoices" }
