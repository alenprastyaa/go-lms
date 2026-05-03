package controllers

import "gorm.io/gorm"
import "lms/realtime"

type AppContext struct {
	DB       *gorm.DB
	Realtime *realtime.Hub
}
