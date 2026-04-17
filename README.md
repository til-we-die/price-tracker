# Price Tracker

[![Go Version](https://img.shields.io/badge/Go-≥1.21-00ADD8?logo=go)](https://golang.org/doc/devel/release)

Система для отслеживания цен на авиабилеты с уведомлением по email.

## Возможности

- Поиск билетов через Aviasales API
- Сохранение истории цен в PostgreSQL
- Email-уведомления при новом минимуме
- Поддержка односторонних перелётов

## Переменные окружения

| Переменная | Описание |
|------------|----------|
| `DB_DSN` | Строка подключения к PostgreSQL |
| `AVI_TOKEN` | API токен Travelpayouts |
| `SMTP_USER` | Email для отправки |
| `SMTP_PASS` | Пароль приложения SMTP |
| `SMTP_TO` | Email получателя |

## Структура

```
price-tracker/
├── cmd/app/main.go      # Точка входа
├── internal/
│   ├── model/               # Структуры данных
│   ├── storage/             # Работа с БД
│   ├── collector/           # API провайдеры
│   ├── analyzer/            # Логика уведомлений
│   └── notifier/            # Email уведомления
└── .env                     # Конфигурация
```

## Логирование

`[ERROR]` - критичные ошибки

`[WARN]` - предупреждения

`[INFO]` - важные события

`[DEBUG]` - отладочная информация
