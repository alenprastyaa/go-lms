package models

import "time"

type KoperasiProduct struct {
	ID          uint      `gorm:"column:id;primaryKey" json:"id"`
	SchoolID    uint      `gorm:"column:school_id" json:"school_id"`
	Name        string    `gorm:"column:name" json:"name"`
	Code        *string   `gorm:"column:code" json:"code"`
	Category    *string   `gorm:"column:category" json:"category"`
	Description *string   `gorm:"column:description" json:"description"`
	ImageURL    *string   `gorm:"column:image_url" json:"image_url"`
	Price       int64     `gorm:"column:price" json:"price"`
	Stock       int       `gorm:"column:stock" json:"stock"`
	IsActive    bool      `gorm:"column:is_active" json:"is_active"`
	CreatedBy   *uint     `gorm:"column:created_by" json:"created_by"`
	UpdatedBy   *uint     `gorm:"column:updated_by" json:"updated_by"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (KoperasiProduct) TableName() string { return "koperasi_products" }

type KoperasiOrder struct {
	ID               uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolID         uint       `gorm:"column:school_id" json:"school_id"`
	OrderNumber      string     `gorm:"column:order_number" json:"order_number"`
	BuyerID          uint       `gorm:"column:buyer_id" json:"buyer_id"`
	BuyerRole        string     `gorm:"column:buyer_role" json:"buyer_role"`
	Status           string     `gorm:"column:status" json:"status"`
	PaymentMethod    *string    `gorm:"column:payment_method" json:"payment_method"`
	PaymentProvider  *string    `gorm:"column:payment_provider" json:"payment_provider"`
	PaymentStatus    string     `gorm:"column:payment_status" json:"payment_status"`
	PaymentRequestID *string    `gorm:"column:payment_request_id" json:"payment_request_id"`
	PaymentQRString  *string    `gorm:"column:payment_qr_string" json:"payment_qr_string"`
	PaymentExpiresAt *time.Time `gorm:"column:payment_expires_at" json:"payment_expires_at"`
	Note             *string    `gorm:"column:note" json:"note"`
	TotalAmount      int64      `gorm:"column:total_amount" json:"total_amount"`
	HandledBy        *uint      `gorm:"column:handled_by" json:"handled_by"`
	PaidAt           *time.Time `gorm:"column:paid_at" json:"paid_at"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (KoperasiOrder) TableName() string { return "koperasi_orders" }

type KoperasiPaymentLog struct {
	ID               uint      `gorm:"column:id;primaryKey" json:"id"`
	SchoolID         uint      `gorm:"column:school_id" json:"school_id"`
	OrderID          uint      `gorm:"column:order_id" json:"order_id"`
	EventType        string    `gorm:"column:event_type" json:"event_type"`
	Status           string    `gorm:"column:status" json:"status"`
	PaymentRequestID *string   `gorm:"column:payment_request_id" json:"payment_request_id"`
	Note             *string   `gorm:"column:note" json:"note"`
	Metadata         *string   `gorm:"column:metadata" json:"metadata"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
}

func (KoperasiPaymentLog) TableName() string { return "koperasi_payment_logs" }

type KoperasiOrderItem struct {
	ID                      uint      `gorm:"column:id;primaryKey" json:"id"`
	OrderID                 uint      `gorm:"column:order_id" json:"order_id"`
	ProductID               uint      `gorm:"column:product_id" json:"product_id"`
	Quantity                int       `gorm:"column:quantity" json:"quantity"`
	Price                   int64     `gorm:"column:price" json:"price"`
	Subtotal                int64     `gorm:"column:subtotal" json:"subtotal"`
	ProductNameSnapshot     string    `gorm:"column:product_name_snapshot" json:"product_name_snapshot"`
	ProductCodeSnapshot     *string   `gorm:"column:product_code_snapshot" json:"product_code_snapshot"`
	ProductCategorySnapshot *string   `gorm:"column:product_category_snapshot" json:"product_category_snapshot"`
	CreatedAt               time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt               time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (KoperasiOrderItem) TableName() string { return "koperasi_order_items" }

type KoperasiCartItem struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	SchoolID  uint      `gorm:"column:school_id" json:"school_id"`
	BuyerID   uint      `gorm:"column:buyer_id" json:"buyer_id"`
	ProductID uint      `gorm:"column:product_id" json:"product_id"`
	Quantity  int       `gorm:"column:quantity" json:"quantity"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (KoperasiCartItem) TableName() string { return "koperasi_cart_items" }
