// Хранение цен и уведомлений
package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"price-tracker/internal/model"
)

type Postgres struct {
	db *gorm.DB
}

// новое подключение к БД
func NewPostgres() (*Postgres, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN is empty")
	}

	// Настройка GORM
	config := &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Warn),
		SkipDefaultTransaction: true,
	}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// Настройка пула соединений
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Автоматическая миграция схемы
	if err := db.AutoMigrate(&Price{}, &Notification{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Postgres{db: db}, nil
}

func (p *Postgres) SavePrice(ctx context.Context, f model.Flight, route string) error {
	price := &Price{
		Price:      f.Price,
		Currency:   f.Currency,
		Departure:  f.Departure,
		URL:        f.URL,
		Provider:   f.Provider,
		Route:      route,
		FlightType: string(f.FlightType),
	}

	// Обработка даты возврата
	if !f.Return.IsZero() {
		price.ReturnDate = &f.Return
	}

	return p.db.WithContext(ctx).Create(price).Error
}

func (p *Postgres) GetMinPrice(ctx context.Context, route string) (int, error) {
	var minPrice int

	err := p.db.WithContext(ctx).
		Model(&Price{}).
		Where("route = ?", route).
		Select("COALESCE(MIN(price), 0)").
		Scan(&minPrice).Error

	return minPrice, err
}

func (p *Postgres) GetMinPriceByType(ctx context.Context, route string, flightType model.FlightType) (int, error) {
	var minPrice int

	err := p.db.WithContext(ctx).
		Model(&Price{}).
		Where("route = ? AND flight_type = ?", route, string(flightType)).
		Select("COALESCE(MIN(price), 0)").
		Scan(&minPrice).Error

	return minPrice, err
}

func (p *Postgres) SaveNotification(ctx context.Context, route string, price int) error {
	notification := &Notification{
		Route:  route,
		Price:  price,
		SentAt: time.Now(),
	}

	return p.db.WithContext(ctx).Create(notification).Error
}

func (p *Postgres) GetLastNotification(ctx context.Context, route string) (int, error) {
	var notification Notification

	err := p.db.WithContext(ctx).
		Where("route = ?", route).
		Order("sent_at DESC").
		First(&notification).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}

	return notification.Price, err
}

func (p *Postgres) GetLastNotificationByType(ctx context.Context, route string, flightType string) (int, error) {
	var notification Notification

	err := p.db.WithContext(ctx).
		Table("notifications").
		Select("notifications.price").
		Joins("JOIN prices ON notifications.route = prices.route AND notifications.price = prices.price").
		Where("notifications.route = ? AND prices.flight_type = ?", route, flightType).
		Order("notifications.sent_at DESC").
		Limit(1).
		Scan(&notification).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}

	return notification.Price, err
}

func (p *Postgres) GetPriceHistory(ctx context.Context, route string, limit int) ([]Price, error) {
	var prices []Price

	err := p.db.WithContext(ctx).
		Where("route = ?", route).
		Order("created_at DESC").
		Limit(limit).
		Find(&prices).Error

	return prices, err
}

func (p *Postgres) GetBestPricesByDate(ctx context.Context, route string) (map[string]int, error) {
	type Result struct {
		Date  time.Time
		Price int
	}

	var results []Result

	err := p.db.WithContext(ctx).
		Model(&Price{}).
		Select("DATE(departure) as date, MIN(price) as price").
		Where("route = ?", route).
		Group("DATE(departure)").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	priceMap := make(map[string]int)
	for _, r := range results {
		priceMap[r.Date.Format("2006-01-02")] = r.Price
	}

	return priceMap, nil
}

func (p *Postgres) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (p *Postgres) DB() *gorm.DB {
	return p.db
}
