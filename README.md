# TripGo — Trip Service

Сервис поездок для лабораторной работы 1 курса «Разработка микросервисов на
Go». Принимает HTTP-запросы на создание, получение и завершение поездки,
хранит данные в PostgreSQL.

## Запуск

Нужны: Go 1.27+, `tripgoctl`, Docker (для локального окружения).

1. Поднять окружение:

```bash
   tripgoctl cluster start
   tripgoctl environment start
   tripgoctl connect
```

   Эта команда сгенерирует `.env` в корне репозитория с актуальным адресом
   PostgreSQL — используйте его, а не `.env.example`.

2. Накатить миграции:

```bash
   make migrate
```

3. Запустить сервис:

```bash
   make run
```

   Сервис слушает `HTTP_ADDR` (по умолчанию `:8080`).

4. Проверить:

```bash
   curl -sS localhost:8080/health
   curl -sS localhost:8080/ready

   curl -sS -X POST localhost:8080/api/v1/trips \
     -H 'content-type: application/json' \
     -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
          "driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",
          "start_point":{"latitude":59.9398,"longitude":30.3146},
          "end_point":{"latitude":59.9290,"longitude":30.3626},
          "price":1450}' | jq
```
