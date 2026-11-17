# Trip Service

Сервис поездок (лабораторная работа 1): HTTP API на `chi`, данные хранятся в
PostgreSQL. Создание, получение и завершение поездки, проверки `/health` и
`/ready`.

## Требования

- Go (версия из `go.mod`)
- `tripgoctl` — локальное окружение курса (PostgreSQL)
- `goose` и `oapi-codegen` отдельно ставить не нужно: они подключены как
  `tool`-зависимости модуля и вызываются через `go tool`

## Запуск

```bash
tripgoctl cluster start        # один раз на машине
tripgoctl environment start    # поднимает PostgreSQL и создаёт .env
tripgoctl connect              # подключаемся
make migrate                   # применяет миграции
make run                       # запускает сервис на HTTP_ADDR (по умолчанию :8080)
```

`Makefile` сам подхватывает `.env` и передаёт переменные в команды, отдельно
экспортировать их не нужно. Если `.env` нет, `make migrate` и `make run`
остановятся с сообщением, что `DATABASE_URL` не задан. Значения из `.env`
важнее переменных окружения; разово заменить можно так:
`make migrate DATABASE_URL=postgres://...`.

=======
Остановка: `Ctrl+C` или `SIGTERM`. Сервис дожидается активных запросов, но не
дольше `SHUTDOWN_TIMEOUT`. Если HTTP-сервер не смог стартовать (например, порт
занят) или БД недоступна при запуске, процесс завершается с кодом 1.

## Переменные окружения

Образец — `.env.example`. Значения проверяются при старте, при ошибке сервис не
запускается и выводит все найденные проблемы сразу.

| Переменная | Описание | Правило |
|---|---|---|
| `HTTP_ADDR` | адрес HTTP-сервера | по умолчанию `:8080` |
| `LOG_LEVEL` | уровень логов | `debug`, `info`, `warn`, `error`; по умолчанию `info` |
| `SHUTDOWN_TIMEOUT` | время на graceful shutdown | обязательна, > 0 |
| `DATABASE_URL` | строка подключения к PostgreSQL | обязательна |
| `DATABASE_MAX_CONNS` | максимум соединений в пуле | обязательна, > 0 |
| `DATABASE_MIN_CONNS` | минимум соединений в пуле | обязательна, 0 ≤ min ≤ max |
| `DATABASE_MAX_CONN_LIFETIME` | максимальное время жизни соединения | обязательна, > 0 |
| `DATABASE_CONNECT_TIMEOUT` | таймаут подключения и проверки БД при старте | обязательна, > 0 |
| `DATABASE_QUERY_TIMEOUT` | таймаут одного SQL-запроса (`statement_timeout` в PostgreSQL) | обязательна, > 0 |

## Команды Makefile

| Команда | Что делает |
|---|---|
| `make migrate` (`migrate-up`) | применить миграции |
| `make migrate-down` | откатить последнюю миграцию |
| `make migrate-status` | показать состояние миграций |
| `make run` | запустить сервис |
| `make generate` | перегенерировать `internal/generated/api.gen.go` из OpenAPI |
| `make test` | запустить тесты с `-race` |

## API

Контракт — `contracts/openapi/trip-service.openapi.yaml`.

| Метод и путь | Успех | Ошибки |
|---|---|---|
| `POST /api/v1/trips` | `201`, заголовок `Location` | `400 invalid_request`, `409 driver_busy`, `500 internal_error` |
| `GET /api/v1/trips/{id}` | `200` | `404 trip_not_found`, `500 internal_error` |
| `POST /api/v1/trips/{id}/finish` | `200` | `404 trip_not_found`, `409 trip_completed`, `500 internal_error` |
| `GET /health` | `200` | — |
| `GET /ready` | `200` | `503`, если БД недоступна |

Ошибки отдаются как `application/problem+json` с полем `code`. Тело
`POST /api/v1/trips` должно быть ровно одним JSON-объектом: обязательны
`user_id`, `driver_id`, `price`, `start_point` и `end_point` с `latitude` и
`longitude`, неизвестные поля и данные после объекта отклоняются. Явно
переданные `0` для цены и координат допустимы.

## Проверка

```bash
curl -sS -i -X POST localhost:8080/api/v1/trips \
  -H 'content-type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
       "driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",
       "start_point":{"latitude":59.9398,"longitude":30.3146},
       "end_point":{"latitude":59.9290,"longitude":30.3626},
       "price":1450}'
```

Дальше по `Location` из ответа (`TRIP_ID` — идентификатор поездки):

```bash
curl -sS -i localhost:8080/api/v1/trips/$TRIP_ID                 # 200
curl -sS -i -X POST localhost:8080/api/v1/trips/$TRIP_ID/finish  # 200
curl -sS -i -X POST localhost:8080/api/v1/trips/$TRIP_ID/finish  # 409 trip_completed
```

Повторный `POST /api/v1/trips` на того же водителя, пока поездка активна, даёт
`409 driver_busy`. Неполное тело (например, только `user_id` и `driver_id`)
даёт `400 invalid_request`.

Гонка при создании (`trip.json` — тело запроса из примера выше, `driver_id`
должен быть свободен): должен получиться ровно один `201`, остальные `409`.

```bash
seq 20 | xargs -P20 -I{} curl -s -o /dev/null -w '%{http_code}\n' \
  -X POST localhost:8080/api/v1/trips -H 'content-type: application/json' \
  -d @trip.json | sort | uniq -c
```

## Принятые решения

### Уровень изоляции транзакций: Read Committed

Это уровень по умолчанию в PostgreSQL. Корректность здесь обеспечивают не
уровень изоляции, а ограничения на уровне БД: частичный уникальный индекс и
условие `status = 'active'` в `UPDATE`. Более строгие уровни
(`Repeatable Read`, `Serializable`) добавили бы ошибки сериализации и повторные
попытки, но ничего не дали бы для этих двух сценариев. В `Read Committed`
конкурентный `UPDATE` ждёт снятия блокировки строки и после этого заново
проверяет условие `WHERE` на актуальной версии строки — на этом построена защита
от двойного завершения.

### Менеджер транзакций (`internal/txmanager`)

- `Do(ctx, fn)` открывает транзакцию и кладёт её в контекст под приватным
  ключом; `fn` получает этот контекст.
- Репозитории вызывают `txmanager.QuerierFromContext(ctx, pool)`: если в
  контексте есть транзакция, запрос идёт через неё, иначе напрямую через пул.
  Репозиторий вне `Do` не падает, а работает без транзакции, каждый запрос сам
  по себе.
- `pgx.Tx` не передаётся параметрами и не попадает в сервис и хендлеры: они
  видят только `Do` и репозитории.
- Если `fn` вернула ошибку или случилась паника, выполняется `Rollback` (после
  паники она пробрасывается дальше), иначе `Commit`.
- Вложенный `Do` обнаруживает транзакцию в контексте и просто вызывает `fn`, а
  не открывает вторую транзакцию.
- Создание поездки (`INSERT` в `trips` и запись в `trip_status_history`) и
  завершение (`UPDATE` и запись в историю) выполняются в одной транзакции:
  либо оба изменения, либо ни одного.

### Запрет двух активных поездок у водителя

Частичный уникальный индекс:

```sql
CREATE UNIQUE INDEX trips_one_active_per_driver_idx
    ON trips (driver_id)
    WHERE status = 'active';
```

Проверка `SELECT` перед `INSERT` не подходит: два параллельных запроса оба
увидели бы свободного водителя. Индекс работает на уровне БД: из двух
одновременных `INSERT` проходит один, второй получает ошибку `23505`
(`unique_violation`), которую репозиторий превращает в доменную
`ErrDriverBusy`, а хендлер — в `409 driver_busy`. После завершения поездки
статус меняется на `completed`, строка выпадает из индекса, и водитель снова
свободен.

### Защита от двойного завершения

```sql
UPDATE trips SET status = 'completed', ...
WHERE id = $1 AND status = 'active'
```

При двух одновременных `finish` второй `UPDATE` ждёт первого и затем не находит
подходящей строки (`RowsAffected = 0`). Тогда репозиторий отдельным чтением
выясняет причину: поездки нет — `404`, поездка завершена — `409 trip_completed`,
сбой БД при чтении — `500`. Без условия `status = 'active'` оба запроса
выполнились бы, перезаписав `finished_at` и добавив дубль в историю статусов.

### `/health` и `/ready`

`/health` отвечает, что процесс жив, и не обращается к БД: недоступная база не
должна приводить к перезапуску самого сервиса. `/ready` проверяет БД (`Ping` с
таймаутом) и отдаёт `503`, пока сервис не может обслуживать запросы.

## Структура проекта

```
cmd/trip-service/            точка входа: конфиг, логгер, сборка зависимостей, запуск и остановка
internal/config/             чтение и проверка переменных окружения
internal/domain/             доменные типы и ошибки
internal/generated/          код из OpenAPI (не править руками, make generate)
internal/repository/         SQL через squirrel и pgx
internal/service/            бизнес-логика
internal/storage/postgres/   пул соединений pgxpool
internal/transport/http/     HTTP-хендлеры (реализация api.ServerInterface)
internal/txmanager/          менеджер транзакций
migrations/                  миграции goose
contracts/                   контракты курса (не редактируются)
>>>>>>> 404c384 (Fix after reviews)
```
