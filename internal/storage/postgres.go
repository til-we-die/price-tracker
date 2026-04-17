// Хранение цен и уведомлений
package storage

import (
	"database/sql"
	"os"

	"price-tracker/internal/model"

	_ "github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres() (*Postgres, error) {
	dsn := os.Getenv("DB_DSN")
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	return &Postgres{db: db}, nil
}

func (p *Postgres) SavePrice(f model.Flight, route string) error {
	query := `
		INSERT INTO prices (price, currency, departure, return_date, url, provider, route, flight_type, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	`

	// Обработка нулевой даты возврата
	var returnDate interface{}
	if f.Return.IsZero() {
		returnDate = nil
	} else {
		returnDate = f.Return
	}

	_, err := p.db.Exec(
		query,
		f.Price,
		f.Currency,
		f.Departure,
		returnDate,
		f.URL,
		f.Provider,
		route,
		f.FlightType,
	)

	return err
}

func (p *Postgres) DB() *sql.DB {
	return p.db
}

func (p *Postgres) GetMinPrice(route string) (int, error) {
	var min int

	err := p.db.QueryRow(`
		SELECT COALESCE(MIN(price), 0)
		FROM prices
		WHERE route = $1
	`, route).Scan(&min)

	return min, err
}

func (p *Postgres) GetMinPriceByType(route string, flightType model.FlightType) (int, error) {
	var min int

	err := p.db.QueryRow(`
		SELECT COALESCE(MIN(price), 0)
		FROM prices
		WHERE route = $1 AND flight_type = $2
	`, route, flightType).Scan(&min)

	return min, err
}

func (p *Postgres) Close() error {
	return p.db.Close()
}
