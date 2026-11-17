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
| `POSITION_CHECK_INTERVAL` | нет | `30s` | период тикера сканера устаревших поездок |
| `POSITION_STALE_AFTER` | нет | `60s` | после какого простоя без координат поездка считается устаревшей |
| `POSITION_WORKERS` | нет | `4` | число воркеров, опрашивающих Push Service |
| `POSITION_QUEUE_SIZE` | нет | `100` | размер буфера очереди опроса |
| `PUSH_HTTP_URL` | да | — | адрес Push Service, подставляется `tripgoctl` |
| `PUSH_REQUEST_TIMEOUT` | нет | `1s` | таймаут одного запроса к Push Service (без ретраев) |

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

### Сохранение координаты: атомарность и устойчивость к опозданиям

`AdvanceLastPositionAt` (`internal/repository/trip/positions.go`) и
`Repository.Save` (`internal/repository/position`) выполняются одним `Do`
(usecase `internal/usecase/trip/position/save`):

```sql
UPDATE trips
SET last_position_at = CASE
        WHEN last_position_at IS NULL OR last_position_at < $2 THEN $2
        ELSE last_position_at
    END,
    updated_at = now()
WHERE id = $1 AND status = 'active';
```

Это условный `UPDATE` того же вида, что и у `Finish`: `WHERE status = 'active'`
за один запрос отличает «поездки нет» от «поездка завершена» (0 затронутых
строк → дополнительное чтение по `id`, как в `Finish`, различает `404` и
`409`), а сравнение времени — `CASE WHEN ... < $2` — сделано в самом запросе,
а не чтением значения в Go с последующей записью. Поэтому опоздавшая
координата, пришедшая позже уже сохранённой свежей, не может «состарить»
поездку: проверено интеграционным тестом
(`TestRepository_AdvanceLastPositionAt_IgnoresOlderTimestamp`). `UPDATE`
удерживает блокировку строки `trips` до конца транзакции, поэтому
конкурентный `Finish` той же поездки ждёт её коммита — `INSERT` в
`trip_positions`, который выполняется следом в той же транзакции, не может
попасть в уже завершённую поездку (а при откате транзакции не остаётся и
сама координата — `TestSavePosition_RolledBackWhenTripCompleted`).

### Опрос устаревших поездок (сканер + worker pool)

`internal/worker/positions.Pool` — отдельный компонент с собственным
жизненным циклом (`Start`/`Stop`), без зависимости от `pgx`/`chi`/HTTP:
интерфейсы `lister` (выборка устаревших поездок) и `pusher` (гейтвей в Push
Service) объявлены в его `contract.go`, конкретные реализации подставляет
`container.go`.

```
        ┌──────────┐ тик раз в POSITION_CHECK_INTERVAL
        │  сканер   │─────────────────────────────┐
        └────┬─────┘                              │
             │ ListStaleActive(StaleAfter, now)    │
             ▼                                     ▼
   ┌───────────────────────┐             ┌──────────────────┐
   │ trips (частичный      │             │  dedup-структура   │
   │ индекс по             │             │  (map + mutex):     │
   │ COALESCE(last_        │             │  кто сейчас в       │
   │ position_at,          │             │  очереди/обработке  │
   │ started_at))          │             └─────────┬──────────┘
   └───────────────────────┘                       │ не дубликат?
                                                     ▼
                                          ┌─────────────────────┐
                                          │ очередь (буфер.      │
                                          │ канал, QueueSize)     │
                                          └──────────┬────────────┘
                                                     │
                                     POSITION_WORKERS воркеров
                                                     ▼
                                          ┌─────────────────────┐
                                          │ gateway/push →        │
                                          │ Push Service (HTTP,   │
                                          │ 1 попытка, свой       │
                                          │ timeout)               │
                                          └─────────────────────┘
```

Ключевые решения:

- **Выборка.** `SELECT ... WHERE status = 'active' AND COALESCE(last_position_at,
  started_at) < now() - StaleAfter ORDER BY since ASC` — покрыта частичным
  индексом `trips_active_stale_since_idx` (миграция `00003`, по тому же
  выражению `COALESCE(...)`, `WHERE status = 'active'`), так что фильтр и
  сортировка используют один index scan, а завершённые поездки индекс вообще
  не видит. Первыми возвращаются поездки, которые молчат дольше всех.
  Проверено `EXPLAIN (ANALYZE, BUFFERS)` на тестовых данных — план содержит
  `Bitmap Index Scan on trips_active_stale_since_idx`.
- **Очередь и дедупликация.** Горутины на поездку не создаются: одна
  горутина-сканер раскладывает задачи по буферизованному каналу
  (`POSITION_QUEUE_SIZE`), фиксированное число воркеров (`POSITION_WORKERS`)
  его разбирает. `map[uuid.UUID]struct{}` под мьютексом хранит, какие
  поездки сейчас в очереди или в обработке — повторный тик по той же
  поездке, пока предыдущая попытка не завершилась, даёт `duplicate`, а не
  вторую задачу в канале (`tripgo.stale_checks{result=duplicate}`). Если
  канал переполнен, задача не теряется — её просто отбрасывает текущий тик
  (`result=dropped`, уровень `Warn`): `last_position_at` не менялся, и
  поездка снова попадёт в выборку на следующем тике.
- **Пауза между запросами по одной поездке** — это и есть
  `POSITION_CHECK_INTERVAL`: отдельного `sleep` после отправки нет, период
  тикера сам ограничивает частоту повторных попыток по одной и той же
  поездке (следующий тик её просто не возьмёт, пока предыдущая обработка не
  освободила запись в dedup-структуре).
- **Отмена.** Пришедшая координата (`usecase/trip/position/save`) вызывает
  `Pool.Cancel(tripID)` — поездка убирается из dedup-структуры. Если задача
  уже лежит в канале, воркер при обработке видит, что поездки там больше
  нет, и в Push Service не идёт (`TestProcess_CancelledBeforeDequeue_SkipsPush`,
  белый ящик без гонки с реальным воркером).
- **Тикер за интерфейсом.** `positions.Ticker` — обёртка над `time.Ticker` в
  проде, управляемая `fakeTicker` в тестах (`pool_test.go`): тесты шлют тики
  вручную, не дожидаясь реального `POSITION_CHECK_INTERVAL`.
- **Остановка** (`Pool.Stop`, вызывается из `Container.Shutdown` между
  остановкой HTTP-сервера и закрытием пула БД): сначала останавливается
  сканер (новые задачи не появляются), затем закрывается канал очереди —
  воркеры дорабатывают то, что уже в буфере, в пределах `ctx`
  (`SHUTDOWN_TIMEOUT), и только потом закрываются HTTP-клиент гейтвея
  (`pushClient.Close()`) и пул БД. Проверено `TestPool_StopDrainsBufferedQueueWithinDeadline`
  и `TestMain`/`goleak.VerifyTestMain` — ни один тест пакета не оставляет
  после себя горутины.
- **Гейтвей в Push Service** (`internal/gateway/push`) — HTTP, одной
  попыткой, без ретраев (ретраи — в следующей работе), со своим
  `http.Client` и таймаутом `PUSH_REQUEST_TIMEOUT`; 5xx/429 превращаются в
  `ErrUnavailable`, остальные 4xx — в `ErrRejected`, транспортные ошибки и
  таймаут — тоже в `ErrUnavailable`.
- **Упрощение, оставленное сознательно:** сканер складывает в очередь *весь*
  результат выборки за один тик, без пакетной обработки (батчами) и без
  приоритизации внутри самой очереди сверх сортировки на уровне SQL. При
  текущих `POSITION_WORKERS`/`POSITION_QUEUE_SIZE` по умолчанию это
  достаточно; батчинг добавил бы сложности больше, чем пользы на этом
  масштабе.

## Архитектура

### Слои и направление зависимостей

```
            ┌───────────────┐      ┌───────────────────┐
            │   internal/    │      │     internal/      │
            │     http/      │      │   repository/trip  │
            │ (хендлеры,     │      │  (Postgres,         │
            │  роутер,       │      │   squirrel)         │
            │  domainerror)  │      │                     │
            └───────┬────────┘      └─────────┬───────────┘
                    │                          │
                    │   зависят от интерфейсов  │
                    ▼                          ▼
              ┌─────────────────────────────────────┐
              │          internal/usecase/trip/*      │
              │   (create, get, finish — сценарии)    │
              └───────────────────┬────────────────────┘
                                  │
                                  ▼
                       ┌─────────────────────┐
                       │  internal/domain/trip │
                       │  (Trip, Status,       │
                       │   доменные ошибки)     │
                       └─────────────────────┘
```

Зависимости направлены строго внутрь: `http` и `repository` знают про `usecase`
(точнее — про интерфейсы, которые `usecase` объявляет для себя как потребитель),
`usecase` знает только про `domain`, а `domain/trip` не импортирует ничего из
проекта вообще (`internal/domain/trip/*.go` зависит только от стандартной
библиотеки и `github.com/google/uuid`). Обратных импортов нет: домен не знает
про `usecase`, `usecase` не знает про `http`/`repository`/`pgx`/`chi` — это
закреплено не только соглашением, но и правилами `depguard` в
`.golangci.yml` (секции `domain`, `usecase`, `http`), так что нарушение
направления зависимостей падает на `make lint`.

Слой `internal/infra` (`postgres` — пул соединений, `transaction` — менеджер
транзакций, `telemetry` — трейсы/метрики/логи) — это инфраструктурные детали,
на которые ссылается только `repository` и `container`; `usecase` их не видит
напрямую, а работает через собственный интерфейс `txManager`.

### Где объявлены интерфейсы и почему

Интерфейсы объявлены **на стороне потребителя**, а не поставщика — в
`contract.go` того пакета, которому они нужны:

- `internal/usecase/trip/create/contract.go` (и аналогично у `finish`, `get`)
  объявляет `txManager`, `tripRepository`, `historyRepository`, `clock`,
  `idGenerator`, `metrics`, `logger` — минимальные интерфейсы, которые
  требуются именно этому usecase, а не весь API `*pgxpool.Pool` или
  `*repository.Repository` целиком.
- `internal/http/trip/create/contract.go` аналогично объявляет интерфейс
  usecase-слоя, который нужен хендлеру.

Такое расположение даёт две вещи: во-первых, `usecase` не зависит от
конкретной реализации репозитория (`internal/repository/trip`) и от `pgx` —
зависимость идёт от конкретного к абстрактному, а не наоборот; во-вторых,
интерфейс потребителя узкий и описывает только то, что реально вызывается
(`Save`, `AppendHistory`, `Do` и т. п.), что упрощает генерацию моков через
`go:generate go tool mockgen -source ${GOFILE}` — моки по `contract.go`
usecase-пакета используются в `*_test.go` рядом (`mocks_test.go`).

Реализации этих интерфейсов (`internal/repository/trip.Repository`,
`internal/infra/transaction.Manager`, `internal/infra/telemetry.Logger` и
т. д.) ничего не знают о том, что они удовлетворяют какому-то конкретному
интерфейсу usecase — связь проверяется только на этапе сборки в контейнере.

### Где собираются зависимости

Всё собирается в одном месте — `internal/container/container.go`
(`container.New`, вызывается из `cmd/trip-service/main.go`). Порядок
инициализации: телеметрия (`telemetry.Setup`) → пул Postgres
(`postgres.NewPool`, с пингом на старте) → менеджер транзакций
(`transaction.New`) → репозитории → инструменты метрик → usecase-ы → HTTP-
хендлеры → роутер → `http.Server`. Остановка (`Container.Shutdown`) идёт в
обратном порядке: сначала `server.Shutdown` (graceful, ждёт активные
запросы), затем закрывается пул БД, затем — `telemetry.Shutdown` (чтобы
последние трейсы/метрики/логи успели экспортироваться). Глобальных
синглтонов приложения нет: всё, что нужно компонентам, передаётся явно через
конструкторы (`New(...)`), а не достаётся из пакетных переменных.

### Ошибки

Доменные ошибки (`ErrTripNotFound`, `ErrTripCompleted`, `ErrDriverBusy`,
`ValidationError`) объявлены в `internal/domain/trip/errors.go`. Перевод
«ошибка → HTTP Problem» сделан в одном месте —
`internal/http/domainerror/domainerror.go` (`Problem()` + `Writer.Write`):
известные доменные ошибки превращаются в нужный код/статус через
`errors.Is`/`errors.As`, а всё остальное отдаётся наружу как generic
`500 internal_error` с логированием исходной ошибки внутри сервиса — детали
внутренних ошибок (например, текст ошибки Postgres) клиенту не передаются.


## Тесты и линтер

- `make test` — unit-тесты (`domain`, `usecase`, хендлеры, `domainerror`,
  `middleware`, `gateway/push`, `worker/positions`). Пакет `worker/positions`
  запускается и с `-race`: там же `goleak.VerifyTestMain` проверяет, что
  тесты не оставляют после себя горутины сканера/воркеров.
- `make test-integration` — репозиторий и менеджер транзакций на реальной БД
  (перед запуском: `make migrate`, `.env` от `tripgoctl`); включает
  `AdvanceLastPositionAt`/`ListStaleActive`/`EXPLAIN`-сценарии из раздела про
  опрос устаревших поездок выше.
- `make lint` — golangci-lint v2, конфиг `.golangci.yml` (включает правила слоёв `depguard`).