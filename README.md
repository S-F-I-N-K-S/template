# TripGo — Trip Service

Сервис поездок для лабораторных работ 1–2 курса «Разработка микросервисов на
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
   PostgreSQL и настройками OpenTelemetry — используйте его, а не
   `.env.example`. Окружение ЛР2 включает компоненты `postgres` и
   `observability` (см. `environment.toml`); ссылки на Jaeger и Grafana
   печатает `tripgoctl connect`.

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

## Makefile

`.env` подхватывается автоматически (`-include .env` + `export`).

| Цель | Что делает |
|---|---|
| `make generate` | генерирует типы и серверный интерфейс из `contracts/openapi/trip-service.openapi.yaml`, затем `go generate ./...` (моки) |
| `make migrate` | накатывает миграции (алиас `migrate-up`) |
| `make migrate-down` | откатывает последнюю миграцию |
| `make migrate-status` | статус миграций |
| `make run` | запускает сервис локально |
| `make fmt` | форматирование (`golangci-lint fmt`) |
| `make lint` | `golangci-lint run ./...` (нужен golangci-lint v2) |
| `make test` | unit-тесты с `-race` и покрытие `usecase` + `domain` |
| `make test-integration` | интеграционные тесты (`-tags integration`), нужны накатанные миграции и `.env` |

## Переменные окружения

| Переменная | Обязательна | По умолчанию | Что означает |
|---|---|---|---|
| `HTTP_ADDR` | нет | `:8080` | адрес, на котором слушает HTTP-сервер |
| `LOG_LEVEL` | нет | `info` | уровень логирования (`debug`, `info`, `warn`, `error`); неверное значение — ошибка старта |
| `SHUTDOWN_TIMEOUT` | да | — | сколько ждать активные запросы при graceful shutdown |
| `HTTP_READ_TIMEOUT` | нет | `5s` | таймаут чтения запроса |
| `HTTP_READ_HEADER_TIMEOUT` | нет | `3s` | таймаут чтения заголовков |
| `HTTP_WRITE_TIMEOUT` | нет | `10s` | таймаут записи ответа |
| `HTTP_IDLE_TIMEOUT` | нет | `60s` | таймаут простаивающего keep-alive соединения |
| `DATABASE_URL` | да | — | строка подключения к PostgreSQL, подставляется `tripgoctl` |
| `DATABASE_MAX_CONNS` | да | — | максимальный размер пула соединений |
| `DATABASE_MIN_CONNS` | да | — | минимальный размер пула соединений |
| `DATABASE_MAX_CONN_LIFETIME` | да | — | максимальное время жизни соединения в пуле |
| `DATABASE_CONNECT_TIMEOUT` | да | — | таймаут установления соединения с БД при старте |
| `DATABASE_QUERY_TIMEOUT` | да | — | таймаут на каждый отдельный запрос к БД |
| `SERVICE_VERSION` | нет | `dev` | значение атрибута ресурса `service.version` |
| `OTEL_SERVICE_NAME` | нет | — | имя сервиса в телеметрии (читает SDK) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | да | — | адрес OTLP-коллектора, подставляется `tripgoctl` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | нет | `http/protobuf` | протокол экспорта; поддержан только `http/protobuf` |
| `OTEL_RESOURCE_ATTRIBUTES` | нет | — | дополнительные атрибуты ресурса (`service.namespace`, `deployment.environment`) |
| `OTEL_TRACES_SAMPLER` | нет | — | сэмплер трейсов (читает SDK) |

## Решения

### Уровень изоляции транзакций

Выбран `READ COMMITTED` (задаётся явно в `Manager.Do`). Его достаточно:
инвариант «у водителя не больше одной активной поездки» обеспечивается не
уровнем изоляции, а частичным уникальным индексом
`trips_one_active_per_driver_idx` на `(driver_id) WHERE status = 'active'` —
конкурентная вставка гарантированно даёт одну успешную запись и одну ошибку
`23505` независимо от уровня изоляции. Более строгий уровень
(`REPEATABLE READ`/`SERIALIZABLE`) добавил бы риск serialization failure и
необходимость ретраев, не давая здесь дополнительных гарантий.

### Менеджер транзакций

`internal/infra/postgres/transaction.Manager.Do(ctx, fn)`:

- открывает транзакцию (`BeginTx` с `IsoLevel: ReadCommitted`) и кладёт её в
  контекст под приватным ключом;
- вызывает `fn` с этим контекстом; вложенный вызов `Do` находит транзакцию в
  контексте и переиспользует её, не открывая вторую;
- `fn` вернула `nil` → `COMMIT`; вернула ошибку → `ROLLBACK`, ошибка
  пробрасывается вызывающему; паника → `ROLLBACK`, паника пробрасывается
  дальше;
- репозитории (`internal/repository/trip`) достают исполнителя
  запроса через `transaction.QuerierFromContext(ctx, pool)`: есть транзакция в
  контексте — работают через неё, нет — через пул напрямую. Транзакция не
  передаётся аргументом метода репозитория, бизнес-код про `pgx` не знает.

Создание поездки (usecase `internal/usecase/trip/create`) выполняется одним `Do`: `INSERT`
в `trips` и `INSERT` в `trip_status_history`. Проверено вручную: если уронить
второй `INSERT`, откатывается и первый — в `trips` ничего не остаётся.

### Запрет двух активных поездок у одного водителя

Ограничение закреплено в БД, а не в коде:

```sql
CREATE UNIQUE INDEX trips_one_active_per_driver_idx
    ON trips (driver_id)
    WHERE status = 'active';
```

Нарушение уникальности при конкурентной вставке возвращает код ошибки
PostgreSQL `23505`, который `Repository.Save` превращает в доменную
ошибку `trip.ErrDriverBusy` → HTTP `409 driver_busy`. При 20 параллельных
попытках создать поездку одному водителю ровно одна получает `201`, остальные
— `409`.

### Защита от гонки при завершении поездки


`Finish` использует условный `UPDATE`:

```sql
UPDATE trips SET status = 'completed', finished_at = $1, updated_at = now()
WHERE id = $2 AND status = 'active';
```

Если `RowsAffected() == 0`, это либо «поездки нет», либо «уже завершена» — эти
случаи различаются дополнительным чтением по `id`, чтобы вернуть правильный
код (`404 trip_not_found` или `409 trip_completed`). При двух конкурентных
`finish` только один запрос находит строку в статусе `active` и обновляет её
(PostgreSQL держит блокировку строки и переоценивает `WHERE` после её
получения); второй видит 0 затронутых строк и получает `409`. `finished_at`
не перезаписывается.
## Архитектура

- Слои и направление зависимостей (`http`/`repository` → `usecase` → `domain`):
  схема текстом или картинкой.
- Где объявлены интерфейсы и почему именно там (`contract.go` пакета-потребителя).
- Где собираются зависимости (`internal/container`) и порядок инициализации и остановки.
- Как устроены ошибки: доменные ошибки в `domain/trip`, перевод в HTTP — в `internal/http/domainerror`.
- Принятые решения и отступления от задания (например, `cmd/trip-service` вместо `cmd/service`,
  где лежит сущность `Trip`) с обоснованием.

## Тесты и линтер

- `make test` — unit-тесты (`domain`, `usecase`, хендлеры, `domainerror`, middleware).
- `make test-integration` — репозиторий и менеджер транзакций на реальной БД
  (перед запуском: `make migrate`, `.env` от `tripgoctl`).
- `make lint` — golangci-lint v2, конфиг `.golangci.yml` (включает правила слоёв `depguard`).
