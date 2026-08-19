# tinder_api

Учебный pet-project: backend в духе Tinder, без лишней сложности.
A learning pet-project: a Tinder-like backend, kept intentionally simple.

- **go-core** — Gin, основное ядро (users, profiles, swipes, matches, feed). Go core owns all write-path business logic.
- **python-analytics** — FastAPI, внутренний аналитический сервис. Consumes events from the core asynchronously, no impact on request latency.
- **Postgres** — источник правды для core (users/profiles/swipes/matches). Source of truth.
- **Redis** — (1) кэш фида/сессий для go-core, (2) событийная шина (Redis Streams) между go-core и python-analytics.
- **ClickHouse** — analytics storage: сырые события (swipe/match/view), удобно для агрегаций по времени.

## Как связаны Go-ядро и Python-сервис / How core and analytics are wired

Это не синхронный REST-вызов на каждый чих — event-driven связь через Redis Streams,
plus REST в обратную сторону, когда python-у есть что отдать ядру.

```
 client
   |
   v
[go-core / Gin] --XADD tinder:events--> [Redis Stream] --XREADGROUP--> [python-analytics]
   |                                                                         |
   | (Postgres: users/profiles/swipes/matches, Redis: cache)                v
   |                                                                   [ClickHouse]
   |                                                                         |
   `-------------------- GET /analytics/users/{id}/hot-score <-------------- FastAPI
                          (used later to rank the discovery feed)
```

1. `go-core` publishes domain events (`swipe`, `match`, ...) into a Redis
   Stream — non-blocking, via a small worker-pool
   ([internal/events/publisher.go](go-core/internal/events/publisher.go)).
   HTTP-хендлер вызывает `Publish()` и не ждёт сети — событие просто падает в буферизованный канал.
2. `python-analytics` — consumer group на том же стриме
   ([app/services/redis_consumer.py](python-analytics/app/services/redis_consumer.py)),
   пишет каждое событие в ClickHouse
   ([app/services/clickhouse_client.py](python-analytics/app/services/clickhouse_client.py)).
3. `python-analytics` отдаёт агрегаты обратно через свой REST API
   ([app/api/routes/analytics.py](python-analytics/app/api/routes/analytics.py)) —
   go-core может дёргать `GET /analytics/users/{id}/hot-score`, чтобы улучшить ранжирование ленты (ticket 9-10).

Так go остаётся быстрым синхронным ядром, а вся "тяжёлая" аналитика уходит в отдельный async-сервис,
который её не тормозит.

## Как работает backend

Обычный запрос пользователя обрабатывает только `go-core`:

1. Клиент вызывает HTTP API, например `POST /api/v1/swipes`.
2. Gin направляет запрос в handler. После тикета 3 middleware проверит JWT и
   положит текущего пользователя в контекст запроса.
3. Handler валидирует входные данные и вызывает код предметной области и
   репозитории.
4. Репозитории читают или изменяют данные в Postgres — это главный и
   надёжный источник данных приложения.
5. Для ленты кандидатов `go-core` сначала смотрит Redis-кэш; при отсутствии
   данных читает Postgres, сохраняет результат в Redis и возвращает ответ.
6. После действий вроде свайпа или матча ядро отправляет событие в Redis
   Stream. Это не задерживает HTTP-ответ: `Publisher` кладёт событие в
   буфер, а worker goroutine отправляет его в Redis в фоне.
7. `python-analytics` читает события из Stream и сохраняет их в ClickHouse.
   Когда ядру нужен аналитический показатель для ленты, оно запрашивает его
   по REST у Python-сервиса.

Таким образом, Go-сервис отвечает за пользовательские операции и данные,
а Python-сервис — за асинхронную аналитику. Падение аналитики не должно
мешать регистрации, просмотру профиля или свайпу.

## Папки `go-core`

```
go-core/
  cmd/server/                 точка запуска приложения
  internal/config/            чтение настроек из переменных окружения
  internal/domain/            структуры предметной области: User, Profile и др.
  internal/repository/        слой работы с Postgres (появится в тикете 2)
  internal/platform/postgres/ создание и проверка пула подключений Postgres
  internal/platform/redis/    создание и проверка Redis-клиента
  internal/events/            публикация событий в Redis Streams
  internal/transport/http/    HTTP-слой: роуты, middleware и handlers Gin
  migrations/                 SQL-миграции, создающие и меняющие таблицы
```

- `cmd/server/main.go` собирает зависимости, запускает HTTP-сервер и корректно
  завершает его по `SIGINT`/`SIGTERM`.
- `internal` нельзя импортировать из другого Go-модуля: здесь живёт только
  внутренняя логика ядра.
- `domain` не должен знать о Gin, Postgres или Redis; он описывает данные и
  правила предметной области.
- `repository` изолирует SQL от HTTP-кода: handler не должен сам писать SQL.
- `transport/http` превращает HTTP-запрос в вызов кода приложения и формирует
  HTTP-ответ.

## Concurrency in go-core

`internal/events/publisher.go` содержит worker pool: HTTP-handler быстро
кладёт событие в буферизированный канал, а несколько goroutine отправляют
его в Redis. Этот паттерн пригодится в тикетах 5 и 6.

## Запуск / Running locally

```bash
docker compose up --build
```

- go-core: http://localhost:8080/health
- python-analytics: http://localhost:8000/health
- ClickHouse HTTP: http://localhost:8123
- Postgres: localhost:5432 (tinder/tinder)
- Redis: localhost:6379

Локальная разработка без Docker: смотри `go-core/.env.example` и
`python-analytics/.env.example`.

## Авторизация

Авторизация реализована через `github.com/meysam81/go-auth` и использует
существующую таблицу `users`: пароль хранится только как bcrypt-хеш в
`users.password_hash`. Дополнительные таблицы для текущего режима не нужны,
потому что сервис выдаёт только короткоживущий access JWT (15 минут). Таблица
для refresh-токенов понадобится только при добавлении logout/refresh/revocation.

| Маршрут | Назначение |
| --- | --- |
| `POST /api/v1/auth/register` | Создать пользователя и вернуть access token |
| `POST /api/v1/auth/login` | Проверить email и пароль, вернуть access token |
| `GET /api/v1/auth/me` | Проверить работу защищённого маршрута |

Тело register/login: `{"email":"user@example.com","password":"минимум 8 символов"}`.
Для защищённого маршрута передайте `Authorization: Bearer <access_token>`.

`internal/transport/http/middleware.RequireJWT` использует middleware из
`go-auth`, проверяет токен и кладёт в Gin context `user_id` (`int64`) и
`user_email` (`string`). Поэтому handler не читает заголовок и не валидирует
JWT самостоятельно: например, `userID := c.GetInt64("user_id")`.

`JWT_SIGNING_KEY` обязателен и должен быть не короче 32 символов. В
`docker-compose.yml` есть локальное development-значение; перед любым
развёртыванием его нужно заменить через переменную окружения.

## Реализованные Go API

Все маршруты ленты находятся под JWT middleware: middleware кладёт `user_id`
в Gin context, а handler передаёт его в сервис. Лента получает профиль и
фильтры, предварительно отбирает кандидатов SQL-запросом по bounding box,
возрасту, полу, верификации, ранее поставленным лайкам и исключениям в обе
стороны; затем Go применяет точную Haversine-дистанцию.

| Маршрут | Назначение |
| --- | --- |
| `GET /api/v1/ribbon` | Лента кандидатов; поддерживает `limit` и `cursor` |
| `GET/PUT /api/v1/ribbon/preferences` | Прочитать или заменить фильтры поиска |
| `GET /api/v1/ribbon/likes` | Входящие лайки с курсором |
| `POST /api/v1/ribbon/likes` | Like; создаёт чат при взаимном like |
| `POST /api/v1/ribbon/dislikes` | Скрыть пользователя из своей ленты |
| `POST/DELETE /api/v1/ribbon/blocks` | Заблокировать или разблокировать пользователя |
| `POST /api/v1/ribbon/reports` | Отправить жалобу |
| `GET/PUT/DELETE /api/v1/profiles/me` | Получить, сохранить или деактивировать свой профиль |
| `GET/POST/DELETE /api/v1/profiles/me/photos` | Список, загрузка в MinIO и удаление фотографий |
| `GET /api/v1/chats` | Список активных матчей/чатов |
| `GET/POST /api/v1/chats/:chat_id/messages` | История или отправка сообщения |
| `POST /api/v1/chats/:chat_id/read` | Отметить сообщения собеседника прочитанными |

Все маршруты из таблицы, кроме register/login, требуют
`Authorization: Bearer <access_token>`.

Миграции встроены в Go-бинарник и безопасно проверяются при каждом запуске:
информация о выполненных SQL-файлах хранится в `schema_migrations`. Для
локальных тестовых данных запустите:

```bash
cd go-core
go run ./cmd/seed
```

Команда идемпотентна: создаёт аккаунт `demo@example.com` и тестовые профили,
не дублируя их при следующем запуске. Пароль всех seed-пользователей:
`Password123!`.

## Первые 10 тикетов / First 10 tickets

Первые шесть Go-задач ниже теперь реализованы в текущем проекте. Тикеты 7–10
остаются работой интеграции Go ↔ Python и аналитики.

### 1. Проверить запуск — готово
Поднять `docker compose up --build` и убедиться, что оба `/health` отвечают
`200`. Посмотреть логи контейнеров и понять, где искать ошибки.

### 2. Профиль и репозиторий (Go) — готово
Создать таблицу `profiles`, структуры `User`/`Profile` и слой
`internal/repository` с методами создания, чтения и обновления профиля.

### 3. Регистрация и вход (Go) — готово
Сделать `POST /auth/register` и `POST /auth/login`: хранить хеш пароля,
выдавать JWT и закрыть будущие приватные маршруты middleware-ом.

### 4. Управление своим профилем (Go) — готово
Есть `GET/PUT/DELETE /profiles/me`, а фотографии загружаются в MinIO с
метаданными в PostgreSQL.

### 5. Свайпы и мэтчи (Go) — готово
Like/pass и блокировки работают через `/ribbon`; взаимный like создаёт чат и
публикует аналитические события в Redis Stream.

### 6. Лента кандидатов и кэш (Go) — готово без Redis-кэша
Реализована SQL-фильтрация и точная геопроверка. Redis-кэш можно добавить
отдельно после измерения реальной нагрузки.

### 7. Формат событий (Go ↔ Python) — базовая часть готова
Go публикует `swipe`, `match`, `block`, `report` с `user_id`,
`target_user_id` и `occurred_at`; Python consumer сохраняет их как JSON в
ClickHouse.

### 8. Устойчивый consumer (Python)
Невалидные события логировать и подтверждать (`XACK`), чтобы очередь не
застревала. Добавить счётчики успешно обработанных и ошибочных событий.

### 9. Hot score и ранжирование (Python + Go)
Посчитать в ClickHouse показатель популярности пользователя, отдать его через
`GET /analytics/users/{id}/hot-score` и использовать для сортировки ленты.

### 10. Отчёты и сквозная проверка (Python + Go)
Добавить `retention` и `matches-per-day`. Написать один сценарий проверки:
свайп в Go → событие в Redis → запись в ClickHouse → результат в Python API.

---

Дальше (не в первые 10, но держите в уме): rate limiting на свайпы, unmatch,
soft-delete аккаунта, geo-фильтрация через PostGIS или просто lat/lon +
bounding box, WebSocket для realtime "it's a match" уведомлений.
