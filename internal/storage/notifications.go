// Функции для работы с БД
package storage

import (
	"database/sql"
	"time"
)

// GetLastNotification возвращает последнюю сохранённую цену уведомления для маршрута
// Если уведомлений нет, возвращает 0, nil
func (p *Postgres) GetLastNotification(route string) (int, error) {
	var price int

	err := p.db.QueryRow(`
		SELECT price
		FROM notifications
		WHERE route = $1
		ORDER BY sent_at DESC
		LIMIT 1
	`, route).Scan(&price)

	if err == sql.ErrNoRows {
		return 0, nil
	}

	return price, err
}

// SaveNotification сохраняет запись об отправленном уведомлении
func (p *Postgres) SaveNotification(route string, price int) error {
	_, err := p.db.Exec(`
		INSERT INTO notifications (route, price, sent_at)
		VALUES ($1, $2, $3)
	`, route, price, time.Now())

	return err
}

// GetLastNotificationByType возвращает последнюю цену уведомления для маршрута и типа перелёта
// Выполняет JOIN с таблицей prices для фильтрации по типу (ORM сделать.)
func (p *Postgres) GetLastNotificationByType(route string, flightType string) (int, error) {
	var price int

	err := p.db.QueryRow(`
		SELECT n.price
		FROM notifications n
		JOIN prices p ON n.route = p.route AND n.price = p.price
		WHERE n.route = $1 AND p.flight_type = $2
		ORDER BY n.sent_at DESC
		LIMIT 1
	`, route, flightType).Scan(&price)

	if err == sql.ErrNoRows {
		return 0, nil
	}

	return price, err
}
