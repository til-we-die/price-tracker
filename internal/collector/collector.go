// Package collector предоставляет интерфейс и утилиты для сбора данных с разных провайдеров.
package collector

import (
	"sync"

	"price-tracker/internal/model"
)

// интерфейс для поиска билетов
type Provider interface {
	Search(params model.SearchParams) ([]model.Flight, error)
}

// CollectAll запускает поиск у всех провайдеров параллельно и объединяет результаты
// Ошибки отдельных провайдеров игнорируются (возвращаются только успешные результаты, надо доработать!!!!!!!)
func CollectAll(providers []Provider, params model.SearchParams) []model.Flight {
	var wg sync.WaitGroup
	ch := make(chan []model.Flight, len(providers))

	for _, p := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			flights, _ := p.Search(params)
			ch <- flights
		}(p)
	}

	wg.Wait()
	close(ch)

	var result []model.Flight
	for flights := range ch {
		result = append(result, flights...)
	}

	return result
}
