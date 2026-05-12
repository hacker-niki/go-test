package models

import (
	"time"
)

type Employee struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	DepartmentID uint      `gorm:"not null" json:"department_id"`
	FullName     string    `gorm:"not null;size:200" json:"full_name"`
	Position     string    `gorm:"not null;size:200" json:"position"`
	HiredAt      Date      `gorm:"type:date"  json:"hired_at"`
	CreatedAt    time.Time `json:"created_at"`
}
