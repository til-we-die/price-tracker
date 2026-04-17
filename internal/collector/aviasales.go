// Package collector реализует сбор данных с API авиакомпаний
package collector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"price-tracker/internal/model"
)

// AviasalesProvider — провайдер для API Aviasales (Travelpayouts)
type AviasalesProvider struct {
	token string // API токен
}

func NewAviasalesProvider(token string) *AviasalesProvider {
	return &AviasalesProvider{token: token}
}

// ответ API Aviasales
type apiResponse struct {
	Data []struct {
		Origin        string  `json:"origin"`
		Destination   string  `json:"destination"`
		Price         float64 `json:"price"`
		DepartureDate string  `json:"departure_at"`
		ReturnDate    string  `json:"return_at"`
		Link          string  `json:"link"`
	} `json:"data"`
}

// Search выполняет поиск билетов по заданным параметрам
// Возвращает отсортированный по цене список перелётов
func (a *AviasalesProvider) Search(params model.SearchParams) ([]model.Flight, error) {
	departureDate := params.DateFrom.Format("2006-01-02")

	var url string
	var flightType model.FlightType

	if params.RoundTrip {
		flightType = model.Return
		url = fmt.Sprintf(
			"https://api.travelpayouts.com/aviasales/v3/prices_for_dates?origin=%s&destination=%s&departure_at=%s&return_at=%s&token=%s",
			params.From,
			params.To,
			departureDate,
			params.DateTo.Format("2006-01-02"),
			a.token,
		)
	} else {
		flightType = model.OneWay
		url = fmt.Sprintf(
			"https://api.travelpayouts.com/aviasales/v3/prices_for_dates?origin=%s&destination=%s&departure_at=%s&token=%s",
			params.From,
			params.To,
			departureDate,
			a.token,
		)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	var flights []model.Flight

	for _, d := range apiResp.Data {
		if d.Price == 0 {
			continue
		}

		dep, err := time.Parse(time.RFC3339, d.DepartureDate)
		if err != nil {
			continue
		}

		// Проверяем дату вылета
		if dep.Format("2006-01-02") != params.DateFrom.Format("2006-01-02") {
			continue
		}

		flight := model.Flight{
			Price:      int(d.Price),
			Currency:   "RUB",
			Departure:  dep,
			URL:        "https://aviasales.ru" + d.Link,
			Provider:   "aviasales",
			From:       params.From,
			To:         params.To,
			FlightType: flightType,
		}

		if params.RoundTrip && d.ReturnDate != "" {
			ret, err := time.Parse(time.RFC3339, d.ReturnDate)
			if err == nil {
				flight.Return = ret
			}
		}

		flights = append(flights, flight)
	}

	// сортировка по возрастанию цены
	sort.Slice(flights, func(i, j int) bool {
		return flights[i].Price < flights[j].Price
	})

	return flights, nil
}
