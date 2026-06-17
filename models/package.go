package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// PackageModule is a single feature/module entry inside a subscription package.
type PackageModule struct {
	Label    string `json:"label"`
	Icon     string `json:"icon"`
	Included bool   `json:"included"`
}

// PackageModules is stored as a JSONB column. It implements sql.Scanner and
// driver.Valuer so GORM can persist it without extra dependencies.
type PackageModules []PackageModule

func (m PackageModules) Value() (driver.Value, error) {
	if m == nil {
		return "[]", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (m *PackageModules) Scan(value interface{}) error {
	if value == nil {
		*m = PackageModules{}
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("unsupported type for PackageModules")
	}

	if len(data) == 0 {
		*m = PackageModules{}
		return nil
	}
	return json.Unmarshal(data, m)
}

// Package is a subscription plan shown on the sales landing page and managed
// from the Super Admin CMS.
type Package struct {
	ID            uint           `gorm:"column:id;primaryKey" json:"id"`
	Name          string         `gorm:"column:name" json:"name"`
	Tagline       string         `gorm:"column:tagline" json:"tagline"`
	Price         float64        `gorm:"column:price" json:"price"`
	OriginalPrice *float64       `gorm:"column:original_price" json:"original_price"`
	BillingPeriod string         `gorm:"column:billing_period" json:"billing_period"`
	BadgeText     string         `gorm:"column:badge_text" json:"badge_text"`
	LogoURL       string         `gorm:"column:logo_url" json:"logo_url"`
	IsPopular     bool           `gorm:"column:is_popular" json:"is_popular"`
	IsActive      bool           `gorm:"column:is_active" json:"is_active"`
	SortOrder     int            `gorm:"column:sort_order" json:"sort_order"`
	CTALabel      string         `gorm:"column:cta_label" json:"cta_label"`
	CTAURL        string         `gorm:"column:cta_url" json:"cta_url"`
	Modules       PackageModules `gorm:"column:modules;type:jsonb" json:"modules"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updated_at"`
}

func (Package) TableName() string { return "lms_packages" }
