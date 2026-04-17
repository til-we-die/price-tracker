// Отправка уведомлений по email
package notifier

import (
	"fmt"
	"net/smtp"
	"os"

	"price-tracker/internal/model"
)

func SendEmail(f model.Flight) error {
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	to := os.Getenv("SMTP_TO")

	if user == "" || pass == "" || to == "" {
		return fmt.Errorf("smtp env not set")
	}

	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	direction := "One-way"
	if f.FlightType == model.Return {
		direction = "Round-trip"
	}

	subject := fmt.Sprintf("Cheap %s flight found! (%s-%s)", direction, f.From, f.To)

	body := fmt.Sprintf(
		"Route: %s → %s\n"+
			"Type: %s\n"+
			"Price: %d %s\n"+
			"Departure: %s\n"+
			"Return: %s\n"+
			"Provider: %s\n"+
			"Link: %s\n"+
			"\n---\nTracked by Price Tracker",
		f.From, f.To,
		direction,
		f.Price, f.Currency,
		f.Departure.Format("2006-01-02 15:04"),
		f.Return.Format("2006-01-02 15:04"),
		f.Provider,
		f.URL,
	)

	msg := []byte(
		"Subject: " + subject + "\r\n" +
			"MIME-version: 1.0;\r\n" +
			"Content-Type: text/plain; charset=\"UTF-8\";\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	auth := smtp.PlainAuth(
		"",
		user,
		pass,
		smtpHost,
	)

	return smtp.SendMail(
		smtpHost+":"+smtpPort,
		auth,
		user,
		[]string{to},
		msg,
	)
}
