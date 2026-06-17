package models

import "time"

type PackageCheckoutOrder struct {
	ID               uint       `gorm:"column:id;primaryKey" json:"id"`
	ReferenceID      string     `gorm:"column:reference_id" json:"reference_id"`
	PackageID        uint       `gorm:"column:package_id" json:"package_id"`
	PackageName      string     `gorm:"column:package_name" json:"package_name"`
	SchoolName       string     `gorm:"column:school_name" json:"school_name"`
	Email            string     `gorm:"column:email" json:"email"`
	Amount           int64      `gorm:"column:amount" json:"amount"`
	Status           string     `gorm:"column:status" json:"status"`
	PaymentMethod    *string    `gorm:"column:payment_method" json:"payment_method"`
	TransactionID    *string    `gorm:"column:transaction_id" json:"transaction_id"`
	PaymentURL       *string    `gorm:"column:payment_url" json:"payment_url"`
	InvoiceSentAt    *time.Time `gorm:"column:invoice_sent_at" json:"invoice_sent_at"`
	CredentialSentAt *time.Time `gorm:"column:credential_sent_at" json:"credential_sent_at"`
	SchoolID         *uint      `gorm:"column:school_id" json:"school_id"`
	AdminUserID      *uint      `gorm:"column:admin_user_id" json:"admin_user_id"`
	PaidAt           *time.Time `gorm:"column:paid_at" json:"paid_at"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (PackageCheckoutOrder) TableName() string { return "package_checkout_orders" }
