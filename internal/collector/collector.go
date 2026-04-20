// Package collector предоставляет интерфейс и утилиты для сбора данных с разных провайдеров.
package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"price-tracker/internal/model"
)

// Provider интерфейс для поиска билетов
type Provider interface {
	Search(params model.SearchParams) ([]model.Flight, error)
	// SearchWithContext выполняет поиск с поддержкой контекста
	SearchWithContext(ctx context.Context, params model.SearchParams) ([]model.Flight, error)
}

// CollectAll запускает поиск у всех провайдеров параллельно и объединяет результаты
// Логирует ошибки отдельных провайдеров
func CollectAll(providers []Provider, params model.SearchParams) []model.Flight {
	var wg sync.WaitGroup
	type collectResult struct {
		flights []model.Flight
		err     error
	}

	ch := make(chan collectResult, len(providers))

	for _, p := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			flights, err := p.Search(params)
			ch <- collectResult{flights, err}
		}(p)
	}

	wg.Wait()
	close(ch)

	var allFlights []model.Flight
	for res := range ch {
		if res.err != nil {
			log.Printf("[WARN] provider search failed: %v", res.err)
			continue
		}
		allFlights = append(allFlights, res.flights...)
	}

	return allFlights
}

// CollectAllWithTimeout запускает поиск с таймаутом
func CollectAllWithTimeout(providers []Provider, params model.SearchParams, timeout time.Duration) ([]model.Flight, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return CollectAllWithContext(ctx, providers, params)
}

// CollectAllWithContext запускает поиск с поддержкой контекста
func CollectAllWithContext(ctx context.Context, providers []Provider, params model.SearchParams) ([]model.Flight, error) {
	var wg sync.WaitGroup
	type providerResult struct {
		flights      []model.Flight
		err          error
		providerName string
	}

	ch := make(chan providerResult, len(providers))

	for _, p := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()

			// Канал для получения результата конкретного провайдера
			done := make(chan providerResult, 1)

			go func() {
				flights, err := p.SearchWithContext(ctx, params)
				done <- providerResult{
					flights:      flights,
					err:          err,
					providerName: fmt.Sprintf("%T", p),
				}
			}()

			select {
			case <-ctx.Done():
				ch <- providerResult{
					err:          fmt.Errorf("timeout or cancelled: %w", ctx.Err()),
					providerName: fmt.Sprintf("%T", p),
				}
			case res := <-done:
				ch <- res
			}
		}(p)
	}

	wg.Wait()
	close(ch)

	var allFlights []model.Flight
	var errors []error

	for res := range ch {
		if res.err != nil {
			log.Printf("[WARN] provider %s failed: %v", res.providerName, res.err)
			errors = append(errors, fmt.Errorf("%s: %w", res.providerName, res.err))
			continue
		}
		log.Printf("[DEBUG] provider %s found %d flights", res.providerName, len(res.flights))
		allFlights = append(allFlights, res.flights...)
	}

	if len(allFlights) > 0 {
		return allFlights, nil
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("all providers failed: %v", errors)
	}

	return []model.Flight{}, nil
}
