package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"price-tracker/internal/analyzer"
	"price-tracker/internal/collector"
	"price-tracker/internal/model"
	"price-tracker/internal/notifier"
	"price-tracker/internal/shutdown"
	"price-tracker/internal/storage"
)

func main() {
	// Загрузка переменных окружения
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] .env file not found, using system env")
	}

	log.Println("[INFO] starting price tracker")

	// менеджер graceful shutdown с таймаутом 60 секунд
	shutdownManager := shutdown.NewShutdownManager(60 * time.Second)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR] panic recovered: %v", r)
		}
	}()

	// Проверка подключения к БД
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("[ERROR] DB_DSN is empty")
	}

	db, err := storage.NewPostgres()
	if err != nil {
		log.Fatal("[ERROR] failed to init db:", err)
	}
	defer db.Close()

	// Очистка БД при завершении
	shutdownManager.AddCleanup(func() error {
		log.Println("[INFO] closing database connection...")
		return db.Close()
	})

	gormDB := db.DB()

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Fatal("[ERROR] failed to get sql.DB:", err)
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatal("[ERROR] db not reachable:", err)
	}

	log.Println("[INFO] database connected")

	// Проверка токена для Aviasales
	token := os.Getenv("AVI_TOKEN")
	if token == "" {
		log.Fatal("[ERROR] AVI_TOKEN is empty")
	}

	providers := []collector.Provider{
		collector.NewAviasalesProvider(token),
	}

	departureStart := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	departureEnd := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	returnStart := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	returnEnd := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)

	log.Printf("[INFO] search range: departure %s to %s, return %s to %s",
		departureStart.Format("2006-01-02"),
		departureEnd.Format("2006-01-02"),
		returnStart.Format("2006-01-02"),
		returnEnd.Format("2006-01-02"))

	// Маршруты
	departureRoute := "LED-NOZ"
	returnRoute := "NOZ-LED"

	// Результаты
	var minDeparturePrice int
	var minReturnPrice int
	var bestDepartureFlight model.Flight
	var bestReturnFlight model.Flight

	totalFlightsDeparture := 0
	totalFlightsReturn := 0

	// Поиск рейсов "туда" в отдельной горутине
	shutdownManager.AddDelta(1)
	go func() {
		defer shutdownManager.Done()
		defer log.Println("[INFO] departure search goroutine finished")

		log.Println("[INFO] searching departure flights (LED -> NOZ)")

		for depDay := departureStart; !depDay.After(departureEnd); depDay = depDay.AddDate(0, 0, 1) {
			// Проверка сигнала завершения
			select {
			case <-shutdownManager.Context().Done():
				log.Println("[INFO] shutdown signal received, stopping departure search")
				return
			default:
			}

			search := model.SearchParams{
				From:      "LED",
				To:        "NOZ",
				DateFrom:  depDay,
				DateTo:    depDay,
				RoundTrip: false,
			}

			log.Printf("[DEBUG] searching departure date: %s", depDay.Format("2006-01-02"))

			ctx, cancel := context.WithTimeout(shutdownManager.Context(), 30*time.Second)

			flights, err := collector.CollectAllWithContext(ctx, providers, search)
			cancel()

			if err != nil {
				log.Printf("[ERROR] search failed for %s: %v", depDay.Format("2006-01-02"), err)
				if shutdownManager.Context().Err() != nil {
					return
				}
				time.Sleep(2 * time.Second)
				continue
			}

			if len(flights) > 0 {
				log.Printf("[DEBUG] found %d flights for %s", len(flights), depDay.Format("2006-01-02"))
			} else {
				log.Printf("[DEBUG] no flights found for %s", depDay.Format("2006-01-02"))
			}

			for _, f := range flights {
				select {
				case <-shutdownManager.Context().Done():
					log.Println("[INFO] shutdown signal received, stopping flight processing")
					return
				default:
				}

				totalFlightsDeparture++

				f.FlightType = model.OneWay
				f.From = "LED"
				f.To = "NOZ"

				saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := db.SavePrice(saveCtx, f, departureRoute); err != nil {
					log.Printf("[ERROR] save price failed: %v", err)
					saveCancel()
					continue
				}
				saveCancel()

				// Атомарное обновление минимума (сменить на мьютекс)
				if minDeparturePrice == 0 || f.Price < minDeparturePrice {
					minDeparturePrice = f.Price
					bestDepartureFlight = f
					log.Printf("[INFO] new min departure price: %d RUB on %s",
						f.Price,
						f.Departure.Format("2006-01-02"))
				}

				notifyCtx, notifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				should, err := analyzer.ShouldNotifyByType(notifyCtx, db, departureRoute, f.Price, model.OneWay)
				notifyCancel()

				if err != nil {
					log.Printf("[ERROR] analyzer error: %v", err)
					continue
				}

				if should {
					if err := notifier.SendEmail(f); err != nil {
						log.Printf("[ERROR] email send failed: %v", err)
						continue
					}

					saveNotifCtx, saveNotifCancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := db.SaveNotification(saveNotifCtx, departureRoute, f.Price); err != nil {
						log.Printf("[ERROR] save notification failed: %v", err)
					}
					saveNotifCancel()

					log.Printf("[INFO] notification sent for price %d RUB", f.Price)
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()

	// Поиск рейсов "обратно" в отдельной горутине
	shutdownManager.AddDelta(1)
	go func() {
		defer shutdownManager.Done()
		defer log.Println("[INFO] return search goroutine finished")

		log.Println("[INFO] searching return flights (NOZ -> LED)")

		for retDay := returnStart; !retDay.After(returnEnd); retDay = retDay.AddDate(0, 0, 1) {
			select {
			case <-shutdownManager.Context().Done():
				log.Println("[INFO] shutdown signal received, stopping return search")
				return
			default:
			}

			search := model.SearchParams{
				From:      "NOZ",
				To:        "LED",
				DateFrom:  retDay,
				DateTo:    retDay,
				RoundTrip: false,
			}

			log.Printf("[DEBUG] searching return date: %s", retDay.Format("2006-01-02"))

			ctx, cancel := context.WithTimeout(shutdownManager.Context(), 30*time.Second)

			flights, err := collector.CollectAllWithContext(ctx, providers, search)
			cancel()

			if err != nil {
				log.Printf("[ERROR] search failed for %s: %v", retDay.Format("2006-01-02"), err)
				if shutdownManager.Context().Err() != nil {
					return
				}
				time.Sleep(2 * time.Second)
				continue
			}

			if len(flights) > 0 {
				log.Printf("[DEBUG] found %d flights for %s", len(flights), retDay.Format("2006-01-02"))
			} else {
				log.Printf("[DEBUG] no flights found for %s", retDay.Format("2006-01-02"))
			}

			for _, f := range flights {
				select {
				case <-shutdownManager.Context().Done():
					log.Println("[INFO] shutdown signal received, stopping flight processing")
					return
				default:
				}

				totalFlightsReturn++

				f.FlightType = model.OneWay
				f.From = "NOZ"
				f.To = "LED"

				saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := db.SavePrice(saveCtx, f, returnRoute); err != nil {
					log.Printf("[ERROR] save price failed: %v", err)
					saveCancel()
					continue
				}
				saveCancel()

				// Атомарное обновление минимума
				if minReturnPrice == 0 || f.Price < minReturnPrice {
					minReturnPrice = f.Price
					bestReturnFlight = f
					log.Printf("[INFO] new min return price: %d RUB on %s",
						f.Price,
						f.Departure.Format("2006-01-02"))
				}

				notifyCtx, notifyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				should, err := analyzer.ShouldNotifyByType(notifyCtx, db, returnRoute, f.Price, model.OneWay)
				notifyCancel()

				if err != nil {
					log.Printf("[ERROR] analyzer error: %v", err)
					continue
				}

				if should {
					if err := notifier.SendEmail(f); err != nil {
						log.Printf("[ERROR] email send failed: %v", err)
						continue
					}

					saveNotifCtx, saveNotifCancel := context.WithTimeout(context.Background(), 5*time.Second)
					if err := db.SaveNotification(saveNotifCtx, returnRoute, f.Price); err != nil {
						log.Printf("[ERROR] save notification failed: %v", err)
					}
					saveNotifCancel()

					log.Printf("[INFO] notification sent for price %d RUB", f.Price)
				}
			}

			time.Sleep(1 * time.Second)
		}
	}()

	// Запуск ожидания сигналов завершения
	go shutdownManager.WaitForShutdown()

	// Ожидание завершения всех поисковых горутин
	done := make(chan struct{})
	go func() {
		shutdownManager.WaitGroup().Wait()
		close(done)
	}()

	// Ожидание либо завершения всех поисков, либо сигнала о завершении
	select {
	case <-done:
		log.Println("[INFO] all search tasks completed")
	case <-shutdownManager.ShutdownComplete():
		log.Println("[INFO] shutdown signal received, waiting for goroutines to finish...")
		time.Sleep(2 * time.Second)
	}

	log.Printf("[INFO] departure flights (LED->NOZ): total=%d, min=%d RUB",
		totalFlightsDeparture, minDeparturePrice)
	if minDeparturePrice > 0 {
		log.Printf("[INFO] best departure: date=%s, price=%d RUB, url=%s",
			bestDepartureFlight.Departure.Format("2006-01-02"),
			minDeparturePrice,
			bestDepartureFlight.URL)
	} else {
		log.Printf("[WARN] no departure flights found")
	}

	log.Printf("[INFO] return flights (NOZ->LED): total=%d, min=%d RUB",
		totalFlightsReturn, minReturnPrice)
	if minReturnPrice > 0 {
		log.Printf("[INFO] best return: date=%s, price=%d RUB, url=%s",
			bestReturnFlight.Departure.Format("2006-01-02"),
			minReturnPrice,
			bestReturnFlight.URL)
	} else {
		log.Printf("[WARN] no return flights found")
	}

	if minDeparturePrice > 0 && minReturnPrice > 0 {
		totalPrice := minDeparturePrice + minReturnPrice
		log.Printf("[INFO] total round-trip price: %d RUB", totalPrice)
		log.Printf("[INFO] average price per flight: %d RUB", totalPrice/2)
	} else if minDeparturePrice > 0 {
		log.Printf("[INFO] one-way price (LED->NOZ): %d RUB", minDeparturePrice)
	} else if minReturnPrice > 0 {
		log.Printf("[INFO] one-way price (NOZ->LED): %d RMB", minReturnPrice)
	}
}
