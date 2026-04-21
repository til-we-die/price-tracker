package storage

import (
	"time"

	"gorm.io/gorm"
)

// модель для таблицы цен
type Price struct {
	ID         uint           `gorm:"primaryKey"`
	Price      int            `gorm:"not null;index"`
	Currency   string         `gorm:"size:3;default:RUB"`
	Departure  time.Time      `gorm:"not null;index"`
	ReturnDate *time.Time     `gorm:"column:return_date"`
	URL        string         `gorm:"size:500"`
	Provider   string         `gorm:"size:50;index"`
	Route      string         `gorm:"size:50;index;not null"`
	FlightType string         `gorm:"column:flight_type;size:20;index"`
	CreatedAt  time.Time      `gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// модель для таблицы уведомлений
type Notification struct {
	ID        uint      `gorm:"primaryKey"`
	Route     string    `gorm:"size:50;index;not null"`
	Price     int       `gorm:"not null"`
	SentAt    time.Time `gorm:"not null;index"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (Price) TableName() string {
	return "prices"
}

func (Notification) TableName() string {
	return "notifications"
}
