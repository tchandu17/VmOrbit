package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Base contains common fields for all domain models.
// All primary keys are UUID v4 generated server-side before insert.
type Base struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey"                json:"id"`
	CreatedAt time.Time      `gorm:"not null;index"                      json:"created_at"`
	UpdatedAt time.Time      `gorm:"not null"                            json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"                               json:"-"`
}

// BeforeCreate sets a UUID primary key before inserting.
func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
