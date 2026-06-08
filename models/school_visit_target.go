package models

import "time"

type SchoolVisitTarget struct {
	ID            uint       `gorm:"column:id;primaryKey" json:"id"`
	Name          string     `gorm:"column:name" json:"name"`
	Wakur         *string    `gorm:"column:wakur" json:"wakur"`
	Kepsek        *string    `gorm:"column:kepsek" json:"kepsek"`
	FullAddress   *string    `gorm:"column:full_address" json:"full_address"`
	Province      *string    `gorm:"column:province" json:"province"`
	City          *string    `gorm:"column:city" json:"city"`
	District      *string    `gorm:"column:district" json:"district"`
	Latitude      *float64   `gorm:"column:latitude" json:"latitude"`
	Longitude     *float64   `gorm:"column:longitude" json:"longitude"`
	GoogleMapsURL *string    `gorm:"column:google_maps_url" json:"google_maps_url"`
	PlaceID       *string    `gorm:"column:place_id" json:"place_id"`
	IsPlanned     bool       `gorm:"column:is_planned" json:"is_planned"`
	PlannedAt     *time.Time `gorm:"column:planned_at" json:"planned_at"`
	IsVisited     bool       `gorm:"column:is_visited" json:"is_visited"`
	VisitedAt     *time.Time `gorm:"column:visited_at" json:"visited_at"`
	CreatedBy     *uint      `gorm:"column:created_by" json:"created_by"`
	UpdatedBy     *uint      `gorm:"column:updated_by" json:"updated_by"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (SchoolVisitTarget) TableName() string { return "school_visit_targets" }
