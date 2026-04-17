// Package analyzer содержит логику принятия решения об отправке уведомлений
package analyzer

import (
	"price-tracker/internal/model"
	"price-tracker/internal/storage"
)

// ShouldNotify проверяет, нужно ли отправить уведомление о новой цене для маршрута.
// Возвращает true, если:
//   - это первый найденный билет для маршрута или
//   - цена ниже минимальной ранее сохранённой
func ShouldNotify(db *storage.Postgres, route string, price int) (bool, error) {
	min, err := db.GetMinPrice(route)
	if err != nil {
		return false, err
	}

	// Первая запись
	if min == 0 {
		return true, nil
	}

	// Новый минимум
	if price < min {
		return true, nil
	}

	return false, nil
}

// ShouldNotifyByType проверяет, нужно ли отправить уведомление для конкретного типа перелёта
// Аналогична ShouldNotify, но учитывает flight_type
func ShouldNotifyByType(db *storage.Postgres, route string, price int, flightType model.FlightType) (bool, error) {
	min, err := db.GetMinPriceByType(route, flightType)
	if err != nil {
		return false, err
	}

	// Первая запись для этого типа
	if min == 0 {
		return true, nil
	}

	// Новый минимум для этого типа
	if price < min {
		return true, nil
	}

	return false, nil
}
