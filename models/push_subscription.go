package models

import "time"

type PushSubscription struct {
	ID               uint       `gorm:"column:id;primaryKey" json:"id"`
	SchoolID         uint       `gorm:"column:school_id" json:"school_id"`
	UserID           uint       `gorm:"column:user_id" json:"user_id"`
	Endpoint         string     `gorm:"column:endpoint" json:"endpoint"`
	P256DH           string     `gorm:"column:p256dh" json:"p256dh"`
	Auth             string     `gorm:"column:auth" json:"auth"`
	ExpirationTime   *time.Time `gorm:"column:expiration_time" json:"expiration_time"`
	SubscriptionJSON string     `gorm:"column:subscription_json" json:"subscription_json"`
	UserAgent        *string    `gorm:"column:user_agent" json:"user_agent"`
	IsActive         bool       `gorm:"column:is_active" json:"is_active"`
	CreatedAt        time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (PushSubscription) TableName() string { return "push_subscriptions" }
