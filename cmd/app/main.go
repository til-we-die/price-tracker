// Package main — точка входа в приложение
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
	"price-tracker/internal/storage"
)

func main() {
	// Загрузка переменных окружения из .env
	if err := godotenv.Load(); err != nil {
		log.Println("[WARN] .env file not found, using system env")
	}

	log.Println("[INFO] starting price tracker")

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

	// Поиск рейсов "туда"
	log.Println("[INFO] searching departure flights (LED -> NOZ)")

	for depDay := departureStart; !depDay.After(departureEnd); depDay = depDay.AddDate(0, 0, 1) {
		search := model.SearchParams{
			From:      "LED",
			To:        "NOZ",
			DateFrom:  depDay,
			DateTo:    depDay,
			RoundTrip: false,
		}

		log.Printf("[DEBUG] searching departure date: %s", depDay.Format("2006-01-02"))

		// таймаут 30 секунд
		flights, err := collector.CollectAllWithTimeout(providers, search, 30*time.Second)
		if err != nil {
			log.Printf("[ERROR] search failed for %s: %v", depDay.Format("2006-01-02"), err)
			time.Sleep(2 * time.Second) // Пауза перед следующей попыткой
			continue
		}

		if len(flights) > 0 {
			log.Printf("[DEBUG] found %d flights for %s", len(flights), depDay.Format("2006-01-02"))
		} else {
			log.Printf("[DEBUG] no flights found for %s", depDay.Format("2006-01-02"))
		}

		for _, f := range flights {
			totalFlightsDeparture++

			ctx := context.Background()

			f.FlightType = model.OneWay
			f.From = "LED"
			f.To = "NOZ"

			if err := db.SavePrice(ctx, f, departureRoute); err != nil {
				log.Printf("[ERROR] save price failed: %v", err)
				continue
			}

			if minDeparturePrice == 0 || f.Price < minDeparturePrice {
				minDeparturePrice = f.Price
				bestDepartureFlight = f
				log.Printf("[INFO] new min departure price: %d RUB on %s",
					f.Price,
					f.Departure.Format("2006-01-02"))
			}

			should, err := analyzer.ShouldNotifyByType(ctx, db, departureRoute, f.Price, model.OneWay)
			if err != nil {
				log.Printf("[ERROR] analyzer error: %v", err)
				continue
			}

			if should {
				if err := notifier.SendEmail(f); err != nil {
					log.Printf("[ERROR] email send failed: %v", err)
					continue
				}

				if err := db.SaveNotification(ctx, departureRoute, f.Price); err != nil {
					log.Printf("[ERROR] save notification failed: %v", err)
				}

				log.Printf("[INFO] notification sent for price %d RUB", f.Price)
			}
		}

		time.Sleep(1 * time.Second)
	}

	// Поиск рейсов "обратно"
	log.Println("[INFO] searching return flights (NOZ -> LED)")

	for retDay := returnStart; !retDay.After(returnEnd); retDay = retDay.AddDate(0, 0, 1) {
		search := model.SearchParams{
			From:      "NOZ",
			To:        "LED",
			DateFrom:  retDay,
			DateTo:    retDay,
			RoundTrip: false,
		}

		log.Printf("[DEBUG] searching return date: %s", retDay.Format("2006-01-02"))

		flights, err := collector.CollectAllWithTimeout(providers, search, 30*time.Second)
		if err != nil {
			log.Printf("[ERROR] search failed for %s: %v", retDay.Format("2006-01-02"), err)
			time.Sleep(2 * time.Second) // Пауза перед следующей попыткой
			continue
		}

		if len(flights) > 0 {
			log.Printf("[DEBUG] found %d flights for %s", len(flights), retDay.Format("2006-01-02"))
		} else {
			log.Printf("[DEBUG] no flights found for %s", retDay.Format("2006-01-02"))
		}

		for _, f := range flights {
			totalFlightsReturn++

			ctx := context.Background()

			f.FlightType = model.OneWay
			f.From = "NOZ"
			f.To = "LED"

			if err := db.SavePrice(ctx, f, returnRoute); err != nil {
				log.Printf("[ERROR] save price failed: %v", err)
				continue
			}

			if minReturnPrice == 0 || f.Price < minReturnPrice {
				minReturnPrice = f.Price
				bestReturnFlight = f
				log.Printf("[INFO] new min return price: %d RUB on %s",
					f.Price,
					f.Departure.Format("2006-01-02"))
			}

			should, err := analyzer.ShouldNotifyByType(ctx, db, returnRoute, f.Price, model.OneWay)
			if err != nil {
				log.Printf("[ERROR] analyzer error: %v", err)
				continue
			}

			if should {
				if err := notifier.SendEmail(f); err != nil {
					log.Printf("[ERROR] email send failed: %v", err)
					continue
				}

				if err := db.SaveNotification(ctx, returnRoute, f.Price); err != nil {
					log.Printf("[ERROR] save notification failed: %v", err)
				}

				log.Printf("[INFO] notification sent for price %d RUB", f.Price)
			}
		}

		time.Sleep(1 * time.Second)
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
		log.Printf("[INFO] one-way price (NOZ->LED): %d RUB", minReturnPrice)
	}
}
