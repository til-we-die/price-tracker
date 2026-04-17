// Основные структуры данных для трекера цен
package model

import "time"

// FlightType определяет тип перелёта: туда или туда-обратно (на будущее)
type FlightType string

const (
	// OneWay — перелёт в одну сторону
	OneWay FlightType = "one_way"
	// Return — перелёт туда-обратно
	Return FlightType = "return"
)

// Информация о найденном авиабилете
type Flight struct {
	Price      int        // Цена
	Currency   string     // Валюта
	Departure  time.Time  // Дата и время вылета
	Return     time.Time  // Дата и время обратного вылета (если есть)
	URL        string     // Ссылка на покупку
	Provider   string     // Название провайдера (aviasales, etc)
	From       string     // Код города вылета
	To         string     // Код города прилёта
	FlightType FlightType // Тип перелёта
}

// Параметры для поиска билетов
type SearchParams struct {
	From      string    // Код города вылета
	To        string    // Код города прилёта
	DateFrom  time.Time // Дата вылета туда
	DateTo    time.Time // Дата вылета обратно
	RoundTrip bool      // true — туда-обратно, false — только туда
}
