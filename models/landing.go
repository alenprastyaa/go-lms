package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type LandingJSON map[string]interface{}

func (j LandingJSON) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (j *LandingJSON) Scan(value interface{}) error {
	if value == nil {
		*j = LandingJSON{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("unsupported type for LandingJSON")
	}
	if len(data) == 0 {
		*j = LandingJSON{}
		return nil
	}
	return json.Unmarshal(data, j)
}

type LandingSection struct {
	ID        uint        `gorm:"column:id;primaryKey" json:"id"`
	Key       string      `gorm:"column:section_key" json:"key"`
	Label     string      `gorm:"column:label" json:"label"`
	Content   LandingJSON `gorm:"column:content;type:jsonb" json:"content"`
	IsActive  bool        `gorm:"column:is_active" json:"is_active"`
	SortOrder int         `gorm:"column:sort_order" json:"sort_order"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updated_at"`
}

func (LandingSection) TableName() string { return "landing_sections" }

type LandingBlogPost struct {
	ID              uint       `gorm:"column:id;primaryKey" json:"id"`
	Title           string     `gorm:"column:title" json:"title"`
	Slug            string     `gorm:"column:slug" json:"slug"`
	Excerpt         string     `gorm:"column:excerpt" json:"excerpt"`
	Content         string     `gorm:"column:content" json:"content"`
	CoverImageURL   string     `gorm:"column:cover_image_url" json:"cover_image_url"`
	AuthorName      string     `gorm:"column:author_name" json:"author_name"`
	Category        string     `gorm:"column:category" json:"category"`
	Tags            string     `gorm:"column:tags" json:"tags"`
	MetaTitle       string     `gorm:"column:meta_title" json:"meta_title"`
	MetaDescription string     `gorm:"column:meta_description" json:"meta_description"`
	IsPublished     bool       `gorm:"column:is_published" json:"is_published"`
	IsFeatured      bool       `gorm:"column:is_featured" json:"is_featured"`
	SortOrder       int        `gorm:"column:sort_order" json:"sort_order"`
	PublishedAt     *time.Time `gorm:"column:published_at" json:"published_at"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (LandingBlogPost) TableName() string { return "landing_blog_posts" }
