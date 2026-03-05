## Описание проекта (что должно получиться)



**Temperature Hub** — учебный pet-проект на Go: небольшой backend для сбора и анализа температурных данных. Проект развивается по шагам: сначала простые конструкции (структуры, тесты, хранение в памяти), затем добавляются многопоточность и сеть, после чего подключаются реальные технологии (**PostgreSQL, Kafka, Prometheus, gRPC**). В финале симуляция заменяется на реальные данные с **ESP32**.


```mermaid
flowchart LR
  ESP[ESP32 / Simulator] --> API[HTTP/gRPC API]
  API --> Q[Queue / Pipeline]
  Q --> W[Workers]
  W --> PG[(PostgreSQL)]
  API --> ST[Stats Service]
  ST --> PG
  API --> M[metrics]
  M --> PR[Prometheus]
  PR --> G[Grafana]
  ```

### Что должен уметь итоговый сервис
К концу проекта должна получиться система, которая:

- **Собирает показания температуры** от нескольких источников:
  - на ранних этапах — симулятор (несколько “датчиков” в отдельных горутинах),
  - на финальном этапе — **ESP32**, отправляющая реальные измерения.
- Обрабатывает данные через **многопоточный pipeline**:
  - входящие измерения попадают в очередь,
  - несколько воркеров параллельно валидируют данные и сохраняют их,
  - при перегрузке работает **backpressure** (если очередь заполнена — новые данные временно не принимаются).
- **Хранит данные**:
  - сначала в памяти (для быстрого старта),
  - затем в **PostgreSQL** с миграциями и индексами.
- Предоставляет **API**:
  - **HTTP API** для ingest/чтения/статистики,
  - **gRPC API** для тех же операций.
- **Считает статистику и агрегаты**:
  - по последним N измерениям (LastN),
  - за интервал времени (Window),
  - временные ряды (Series) для графиков.
- Работает в **event-driven стиле**:
  - сначала через in-memory bus,
  - затем через **Kafka** (producer/consumer group), отделяя приём данных от записи и обработки.
- Имеет **надёжность уровня “почти прод”**:
  - защита от дублей (idempotency) через `reading_id`,
  - retry/backoff на временные ошибки (например, временная недоступность БД).
- Экспортирует **метрики Prometheus**:
  - количество принятых/отклонённых событий,
  - время обработки запросов и запись в БД,
  - ошибки Kafka/consumer,
  - (опционально) Grafana dashboard.
- Корректно ведёт себя как сервис:
  - конфигурация через env,
  - структурированные логи,
  - middleware (request id, recover),
  - **graceful shutdown**.

### Что будет получено в результате
- Репозиторий, который можно использовать как полноценный pet-проект:
  - понятная структура,
  - тесты на каждом этапе (unit + integration + e2e),
  - docker-compose для Postgres/Kafka/Prometheus/Grafana,
  - документация и примеры запросов,
  - proto-контракты для gRPC и (в финале) Protobuf payload для Kafka.
- Практическое закрепление ключевых тем Go и backend:
  - горутины/каналы/worker pool и backpressure,
  - проектирование интерфейсов и слоёв,
  - HTTP и тестирование handlers,
  - Postgres + миграции + индексы + интеграционные тесты,
  - Kafka producer/consumer + commit после успешной обработки,
  - наблюдаемость через Prometheus,
  - gRPC и Protobuf.

Итог: должен получиться **IoT-backend**, который принимает данные (симуляция/ESP32), надёжно обрабатывает их через pipeline и Kafka, хранит в Postgres, отдаёт статистику и метрики, и покрыт тестами.
<details>
  <summary><strong>Оглавление (нажми, чтобы раскрыть)</strong></summary>

- [Описание проекта (что должно получиться)](#описание-проекта-что-должно-получиться)

- [Этап 0 — Каркас репозитория](#этап-0--каркас-репозитория)
- [Этап 1 — Domain + MemoryStore (первые unit-тесты)](#этап-1--domain--memorystore-первые-unit-тесты)
- [Этап 2 — Многопоточность v1: Pipeline (очередь + пул воркеров)](#этап-2--многопоточность-v1-pipeline-очередь--пул-воркеров)

- [Этап 3 — Симулятор v1: один датчик → Pipeline](#этап-3--симулятор-v1-один-датчик--pipeline)
  - [Этап 3.1 — Один датчик](#этап-31--один-датчик)
  - [Этап 3.2 — Несколько датчиков](#этап-32--несколько-датчиков)

- [Этап 4 — Stats v1: LastN (без сети)](#этап-4--stats-v1-lastn-без-сети)
- [Этап 5 — Latest + Window stats (без сети)](#этап-5--latest--window-stats-без-сети)

- [Этап 6 — HTTP](#этап-6)
  - [Этап 6.1 — HTTP скелет: роутер + /health](#этап-61--http-скелет-роутер--health)
  - [Этап 6.2 — JSON helpers + единые ошибки](#этап-62--json-helpers--единые-ошибки)
  - [Этап 6.3 — HTTP ingest v0 (учебный): синхронно пишет в store](#этап-63--http-ingest-v0-учебный-синхронно-пишет-в-store)
  - [Этап 6.4 — HTTP ingest v1: через Pipeline (асинхронно + backpressure)](#этап-64--http-ingest-v1-через-pipeline-асинхронно--backpressure)
  - [Этап 6.5 — Read + Stats endpoints](#этап-65--read--stats-endpoints)

- [Этап 7](#этап-7)
  - [Этап 7.1 — Конфиг через env](#этап-71--конфиг-через-env)
  - [Этап 7.2 — Логи (slog) + access log middleware](#этап-72--логи-slog--access-log-middleware)
  - [Этап 7.3 — Request ID + Recover middleware](#этап-73--request-id--recover-middleware)
  - [Этап 7.4 — Graceful shutdown](#этап-74--graceful-shutdown)

- [Этап 8 PostgreSQL](#этап-8-postgresql)
  - [Этап 8.1 — Compose Postgres + миграции goose + тест “таблица существует”](#этап-81--compose-postgres--миграции-goose--тест-таблица-существует)
  - [Этап 8.2 — PostgresStore v1: подключение + Health/Close](#этап-82--postgresstore-v1-подключение--healthclose)
  - [Этап 8.3 — PostgresStore v2: Append/List](#этап-83--postgresstore-v2-appendlist)
  - [Этап 8.4 — PostgresStore v3: Latest + индексы](#этап-84--postgresstore-v3-latest--индексы)
  - [Этап 8.5 — Retention: DeleteBefore + job](#этап-85--retention-deletebefore--job)

- [Этап 9 — Bus (подготовка к Kafka): интерфейсы + InMemory bus](#этап-9--bus-подготовка-к-kafka-интерфейсы--inmemory-bus)
  - [Этап 9.1 — Bus интерфейсы](#этап-91--bus-интерфейсы)
  - [Этап 9.2 — InMemory Publisher/Consumer](#этап-92--inmemory-publisherconsumer)

- [Этап 10 — Kafka](#этап-10--kafka)
  - [Этап 10.1 — Kafka v1: минимальный publish/consume (JSON)](#этап-101--kafka-v1-минимальный-publishconsume-json)
  - [Этап 10.2 — Kafka v2: consumer group + commit после успеха](#этап-102--kafka-v2-consumer-group--commit-после-успеха)

- [Этап 11 — Надёжность (idempotency + retry/backoff)](#этап-11--надёжность-idempotency--retrybackoff)
  - [Этап 11.1 — Idempotency (дубликаты)](#этап-111--idempotency-дубликаты)
  - [Этап 11.2 — Retry/backoff на временные ошибки](#этап-112--retrybackoff-на-временные-ошибки)

- [Этап 12 — Prometheus (минимум → расширение → Grafana)](#этап-12--prometheus-минимум--расширение--grafana)
  - [Этап 12.1 — /metrics минимально](#этап-121--metrics-минимально)
  - [Этап 12.2 — Метрики “как в проде”: duration + ошибки](#этап-122--метрики-как-в-проде-duration--ошибки)
  - [Этап 12.3 — Prometheus + Grafana в compose + dashboard](#этап-123--prometheus--grafana-в-compose--dashboard)

- [Этап 13 — gRPC (Ping → Ingest → Query)](#этап-13--grpc-ping--ingest--query)
  - [Этап 13.1 — gRPC smoke: proto + Ping](#этап-131--grpc-smoke-proto--ping)
  - [Этап 13.2 — gRPC ingest: SendReading → Publish](#этап-132--grpc-ingest-senderroring--publish)
  - [Этап 13.3 — gRPC query: GetLatest + stats](#этап-133--grpc-query-getlatest--stats)

- [Этап 14 — Protobuf end-to-end (конвертеры → Kafka payload Protobuf)](#этап-14--protobuf-end-to-end-конвертеры--kafka-payload-protobuf)
  - [Этап 14.1 — Конвертеры domain ↔ proto](#этап-141--конвертеры-domain--proto)
  - [Этап 14.2 — Kafka payload = Protobuf](#этап-142--kafka-payload--protobuf)

- [Этап 15 — Series (time-series): Go → SQL оптимизация](#этап-15--series-time-series-go--sql-оптимизация)
  - [Этап 15.1 — Series в Go (простая версия)](#этап-151--series-в-go-простая-версия)
  - [Этап 15.2 — Series в SQL (оптимизация)](#этап-152--series-в-sql-оптимизация)

- [Этап 16 — ESP32 (финал)](#этап-16--esp32-финал)
  - [Этап 16.1 — ESP32 через HTTP](#этап-161--esp32-через-http)
  - [Этап 16.2 — ESP32 через gRPC (опционально)](#этап-162--esp32-через-grpc-опционально)

- [Итоговые артефакты проекта](#итоговые-артефакты-проекта)

</details>
---

<details>
  <summary><strong>Зачем нужны файлы на каждом этапе (нажми, чтобы раскрыть)</strong></summary>

## Этап 0 — Каркас репозитория
- `cmd/server/main.go` — точка входа приложения (сборка зависимостей и запуск сервиса).
- `README.md` — описание проекта, как запускать и как тестировать.
- `Makefile` — короткие команды для запуска/линта/тестов/compose, чтобы не помнить длинные команды.
- `.golangci.yml` — конфиг линтера, чтобы стиль и ошибки проверялись одинаково.
- `internal/` (папка) — место для внутреннего кода приложения (не экспортируется наружу как библиотека).

---

## Этап 1 — Domain + MemoryStore
- `internal/domain/reading.go` — доменная модель `Reading` и `ReadingFilter`, чтобы все слои работали с одним форматом данных.
- `internal/store/store.go` — интерфейс `ReadingStore`, чтобы можно было позже заменить MemoryStore на PostgresStore без переписывания логики.
- `internal/store/memory_store.go` — реализация хранения в памяти для быстрого старта без БД.
- `internal/store/memory_store_test.go` — unit-тесты хранилища (добавление, чтение, лимит), чтобы уверенно развивать проект дальше.

---

## Этап 2 — Pipeline (многопоточность)
- `internal/pipeline/errors.go` — общие ошибки pipeline (например `ErrQueueFull`), чтобы HTTP/gRPC могли правильно реагировать (429/503).
- `internal/pipeline/validator.go` — единая функция `Validate`, чтобы правила проверки данных были в одном месте и использовались всеми источниками.
- `internal/pipeline/pipeline.go` — реализация очереди + worker pool (ядро многопоточности и backpressure).
- `internal/pipeline/pipeline_test.go` — тесты pipeline (валидные проходят, невалидные отбрасываются, переполнение очереди), чтобы не ловить гонки и флаки позже.

---

## Этап 3.1 — Симулятор (один датчик)
- `internal/sim/generator.go` — генерация температуры (`NextTemp`), чтобы получать поток данных без железа.
- `internal/sim/sim_test.go` — тесты генератора (диапазон + детерминизм через seed), чтобы симуляция не делала тесты “случайными”.

---

## Этап 3.2 — Симулятор (несколько датчиков)
- `internal/sim/runner.go` — запуск нескольких “датчиков” в горутинах, чтобы моделировать реальные параллельные источники данных.
- `internal/sim/runner_test.go` — тест, что симулятор реально генерирует поток и есть readings от разных `sensor_id`.

---

## Этап 4 — Stats v1 (LastN)
- `internal/domain/stats.go` — структура `Stats`, общий формат результата статистики.
- `internal/stats/service.go` — реализация `StatsService` (логика расчёта статистики, независимая от HTTP/БД).
- `internal/stats/stats_test.go` — unit-тесты математики (min/max/avg/count), чтобы не допускать тихих ошибок.

---

## Этап 5 — Latest + Window (без сети)
- `internal/domain/reading.go` (изменение) — добавление `From/To` в `ReadingFilter`, чтобы уметь выбирать данные за период.
- `internal/store/store.go` (изменение) — добавление `Latest`, чтобы быстро получать текущее значение датчика.
- `internal/store/memory_store.go` (изменение) — реализация `Latest` и фильтрации по времени для MemoryStore.
- `internal/stats/service.go` (изменение) — добавление `Window`, чтобы считать статистику за интервал времени.
- `internal/stats/window_test.go` — тесты статистики за окно (важно для корректности фильтрации по времени).
- `internal/store/memory_store_test.go` (дополнение) — тесты для `Latest` и фильтрации по `From/To`.

---

## Этап 6.1 — HTTP скелет: роутер + /health
- `internal/httpapi/router.go` — сборка роутов в одном месте (удобно подключать middleware и версии API).
- `internal/httpapi/health.go` — handler `/health` (проверка, что процесс жив).
- `internal/httpapi/health_test.go` — handler-тест, чтобы гарантировать стабильный ответ.

---

## Этап 6.2 — JSON helpers + единые ошибки
- `internal/httpapi/json.go` — `writeJSON`, чтобы все ответы в JSON писались одинаково.
- `internal/httpapi/errors.go` — `APIError` + `writeError`, чтобы ошибки были единообразные и предсказуемые для клиента.
- `internal/httpapi/errors_test.go` — тесты на формат ошибок, чтобы случайно не сломать контракт API.

---

## Этап 6.3 — HTTP ingest v0 (учебный, синхронно в store)
- `internal/httpapi/dto.go` — структуры запросов/ответов (DTO) для JSON, чтобы не “светить” доменные структуры напрямую.
- `internal/httpapi/ingest.go` — handler `POST /ingest` (decode JSON → validate → store.Append) для освоения HTTP POST.
- `internal/httpapi/ingest_test.go` — тесты ingest (201/400), чтобы закрепить контракт и обработку ошибок.

---

## Этап 6.4 — HTTP ingest v1 (через Pipeline)
- `internal/httpapi/ingest.go` (изменение) — вместо `store.Append` вызывает `pipeline.Submit`, чтобы HTTP не блокировался записью.
- `internal/httpapi/ingest_test.go` (дополнение) — тест на `202` и на переполнение очереди (429/503).

---

## Этап 6.5 — Read + Stats endpoints
- `internal/httpapi/readings.go` — handlers чтения (`/readings`, `/readings/latest`) поверх `store.List/Latest`.
- `internal/httpapi/stats.go` — handlers статистики (`/stats/last`, `/stats/window`) поверх `StatsService`.
- `internal/httpapi/readings_test.go` — тесты чтения (параметры, 404 для latest без данных).
- `internal/httpapi/stats_test.go` — тесты статистики (параметры, 400 на ошибки, корректный JSON).

---

## Этап 7.1 — Конфиг через env
- `internal/config/config.go` — `Config` + `Load()`, чтобы управлять портом, воркерами и очередью без правки кода.

---

## Этап 7.2 — Логи (slog) + access log middleware
- `internal/logging/logging.go` — настройка `slog` (уровень/формат), чтобы логи были структурированными.
- `internal/httpapi/middleware/access_log.go` — логирует каждый HTTP запрос (метод/путь/статус/latency) для диагностики.

---

## Этап 7.3 — Request ID + Recover middleware
- `internal/httpapi/middleware/request_id.go` — добавляет request id в запрос/ответ, чтобы связывать логи одной операции.
- `internal/httpapi/middleware/recover.go` — ловит panic и возвращает 500, чтобы сервис не падал из-за ошибки в handler.

---

## Этап 7.4 — Graceful shutdown
- `cmd/server/main.go` (изменение) — обработка сигналов, `Shutdown` HTTP сервера и остановка воркеров, чтобы завершаться без потери данных и “висящих” горутин.

---

## Этап 8.1 — Postgres в compose + миграции + тест “таблица существует”
- `deploy/docker-compose.yml` — поднять Postgres локально одной командой.
- `migrations/0001_create_readings.sql` — создать таблицы/индексы для readings (источник правды о схеме).
- `internal/migrate/migrate.go` — код, который умеет применять миграции из тестов/утилит.
- `internal/migrate/migrate_test.go` — проверяет, что миграции реально создают таблицу (ловит ошибки схемы рано).

---

## Этап 8.2 — PostgresStore v1: подключение + Health/Close
- `internal/store/postgres_store.go` — реализация `ReadingStore` поверх Postgres (пул соединений, health, close).
- `internal/store/postgres_store_test.go` — интеграционные тесты с Postgres.

---

## Этап 8.3 — PostgresStore v2: Append/List
- `internal/store/postgres_store.go` (изменение) — добавить SQL для `Append` и `List`.
- `internal/store/postgres_store_test.go` (дополнение) — тест “append → list нашёл”.

---

## Этап 8.4 — PostgresStore v3: Latest + индексы
- `internal/store/postgres_store.go` (изменение) — добавить `Latest`.
- `migrations/...sql` (изменение/новая миграция при необходимости) — индексы по `(sensor_id, ts)`.
- `internal/store/postgres_store_test.go` (дополнение) — тест latest.

---

## Этап 8.5 — Retention: DeleteBefore + job
- `internal/retention/job.go` — фоновая задача, которая периодически чистит старые данные.
- (изменения в store интерфейсе и postgres store) — метод `DeleteBefore`.

---

## Этап 9.1 — Bus интерфейсы
- `internal/bus/bus.go` — интерфейсы Publisher/Consumer, чтобы отделить бизнес-логику от транспорта.

---

## Этап 9.2 — InMemory Publisher/Consumer
- `internal/bus/inmem_pub.go` — publisher на канале.
- `internal/bus/inmem_cons.go` — consumer читает канал и вызывает handler.
- `internal/bus/inmem_test.go` — unit-тест “publish N → handler N”.

---

## Этап 10.1 — Kafka v1 (JSON)
- `deploy/docker-compose.yml` (изменение) — добавить Kafka/Redpanda.
- `internal/bus/kafka_pub.go` — publisher в Kafka.
- `internal/bus/kafka_cons.go` — consumer из Kafka.
- `internal/bus/kafka_integration_test.go` — интеграционный тест “publish → запись в Postgres”.

---

## Этап 10.2 — Kafka v2 (commit после успеха)
- `internal/bus/kafka_cons.go` (изменение) — commit offset после успешной обработки.
- `internal/bus/kafka_integration_test.go` (дополнение) — тест “ошибка handler → сообщение не считается обработанным”.

---

## Этап 11.1 — Idempotency
- `internal/domain/reading.go` (изменение) — `ReadingID`.
- `migrations/...sql` — уникальный индекс по `reading_id`.
- integration test — “2 одинаковых ReadingID → 1 строка”.

---

## Этап 11.2 — Retry/backoff
- `internal/retry/backoff.go` — retry helper.
- `internal/retry/backoff_test.go` — unit-тесты retry.

---

## Этап 12.1 — /metrics минимально
- `internal/metrics/metrics.go` — регистрация метрик.
- `internal/httpapi/metrics.go` — endpoint `/metrics`.
- `internal/httpapi/metrics_test.go` — тест `/metrics`.

---

## Этап 12.2 — Метрики “как в проде”
- `internal/metrics/metrics.go` (изменение) — duration histograms + error counters.
- unit tests — инкременты метрик на ошибках.

---

## Этап 12.3 — Prometheus + Grafana
- `deploy/prometheus.yml` — конфиг Prometheus.
- `deploy/grafana/dashboard.json` — дашборд.
- `deploy/docker-compose.yml` (изменение) — добавить Prometheus/Grafana.

---

## Этап 13.1 — gRPC smoke
- `proto/temperature.proto` — контракт.
- `internal/grpcapi/server.go` — сервер.
- `internal/grpcapi/server_test.go` — bufconn тест.

---

## Этап 13.2 — gRPC ingest
- `proto/temperature.proto` (изменение) — `SendReading`.
- `internal/grpcapi/server.go` (изменение) — реализовать publish.
- `internal/grpcapi/server_test.go` — тест с mock publisher.

---

## Этап 13.3 — gRPC query
- `proto/temperature.proto` (изменение) — query методы.
- `internal/grpcapi/server.go` — реализовать query.
- `internal/grpcapi/server_test.go` — тесты с mock store/stats.

---

## Этап 14.1 — Конвертеры domain ↔ proto
- `internal/codec/convert.go` — конвертация.
- `internal/codec/convert_test.go` — тесты конвертации.

---

## Этап 14.2 — Kafka payload Protobuf
- `internal/bus/codec_proto.go` — encode/decode protobuf bytes.

---

## Этап 15.1 — Series в Go
- `internal/stats/series.go` — бакетирование в Go.
- `internal/stats/series_test.go` — unit-тесты series.

---

## Этап 15.2 — Series в SQL
- `internal/stats/service.go` или `internal/store/postgres_store.go` (изменение) — SQL агрегации.
- integration test — проверка результата.

---

## Этап 16.1 — ESP32 HTTP
- `docs/esp32.md` — инструкция по отправке данных.
- `internal/e2e/e2e_test.go` — e2e smoke тест.

---

## Этап 16.2 — ESP32 gRPC (опционально)
- `docs/esp32.md` (обновление) — gRPC вариант.
- e2e тест — gRPC send → latest/stats.

</details>

---
# Этап 0 — Каркас репозитория

**Цель:** подготовить репо, ничего “умного” ещё нет.

### Добавить файлы

- `cmd/server/main.go`
- `README.md`
- `Makefile` (run/lint/test/compose-up/compose-down)
- `.golangci.yml`
- папка `internal/` (пустая)

### Итог

- `go run ./cmd/server` запускается.

---

# Этап 1 — Domain + MemoryStore (первые unit-тесты)

На этом этапе создаётся **модель данных** (`Reading`) и **простое хранилище в памяти** (`MemoryStore`), которое умеет:
- **добавлять** показания (`Append`)
- **возвращать** список показаний по простому фильтру (`List`)

Хранилище ограничивается по размеру (хранит **последние N** записей), чтобы память не росла бесконечно.  
Этап считается завершённым, когда написаны и проходят **unit-тесты** на `Append`, `List` и работу лимита.

**Итог:** есть проверенное тестами in-memory хранилище, на которое дальше опирается pipeline, симулятор и API.

**Цель:** научиться хранить показания в памяти.

### Новые файлы

- `internal/domain/reading.go`
- `internal/store/store.go`
- `internal/store/memory_store.go`
- `internal/store/memory_store_test.go`

### Вводим интерфейс `ReadingStore` (минимум)

```go
type ReadingStore interface {
    Append(ctx context.Context, r domain.Reading) error
    List(ctx context.Context, f domain.ReadingFilter) ([]domain.Reading, error)
}
```
## Метод: `Append(ctx, r)`

- **Что делает:** сохраняет одно показание.
- **Зачем:** базовая операция “мы собираем данные”.

## Метод: `List(ctx, f)`

- **Что делает:** отдаёт список показаний по фильтру (пока `sensor_id`, `limit`).
- **Зачем:** проверять, что данные копятся, и использовать для статистики.

---

## Тесты

- `Append + List`
- лимит хранения (последние `N`)

---

## Мини-скелет

```go
type Reading struct {
    SensorID  string
    Timestamp time.Time
    TempC     float64
}
```
## Итог

- Рабочее **in-memory** хранение + тесты.

**Переход → Этап 2:** строим многопоточность (до сети).

---

# Этап 2 — Многопоточность v1: Pipeline (очередь + пул воркеров)

На этом этапе появляется **многопоточное ядро обработки данных**: `Pipeline`.  
Он принимает показания через `Submit`, складывает их в **очередь (канал)** и обрабатывает параллельно с помощью **пула воркеров (горутины)**. Каждый воркер валидирует данные (`Validate`) и сохраняет их в `store.Append`.

Также вводится **backpressure**: если очередь переполнена, `Submit` возвращает `ErrQueueFull`, чтобы система не “легла” под нагрузкой.

Этап считается завершённым, когда тестами подтверждено:
- валидные readings записываются в store,
- невалидные отбрасываются,
- переполнение очереди корректно возвращает ошибку.

**Итог:** готовый многопоточный pipeline, к которому дальше подключается симулятор, HTTP/gRPC и Kafka consumer.

**Цель:** ядро проекта: “входящие события” → очередь → несколько воркеров → store.

## Новые файлы

- `internal/pipeline/errors.go` (ошибки pipeline, например `ErrQueueFull`)
- `internal/pipeline/validator.go`
- `internal/pipeline/pipeline.go`
- `internal/pipeline/pipeline_test.go`

## Вводим интерфейс `Pipeline`

```go
type Pipeline interface {
    Start(ctx context.Context)
    Submit(ctx context.Context, r domain.Reading) error
}
```
## Pipeline (очередь + воркеры)

### Метод: `Start(ctx)`
**Что делает:** запускает `N` воркеров (горутины), которые читают очередь и пишут в `store`.  
**Зачем:** получаем параллельность и отделяем “приём” от “обработки”.

### Метод: `Submit(ctx, r)`
**Что делает:** кладёт `reading` в очередь; если очередь заполнена — возвращает ошибку.  
**Зачем:** backpressure (не уронить систему при нагрузке). Позже это будут вызывать HTTP/gRPC/Kafka consumer.

### Функция: `Validate(r)`
```go
func Validate(r domain.Reading) error
```

## Validate + Pipeline: ожидаемое поведение и тесты

### `Validate(r domain.Reading) error`
**Что делает:** проверяет корректность:
- `sensor_id` не пустой
- `temp` в допустимом диапазоне
- `timestamp` валиден

**Зачем:** единые правила валидации для всех источников данных.

### Что должно получиться
- Если отправить 100 `readings` через `Submit`, воркеры параллельно запишут их в `store.Append`.
- Невалидные `readings` не попадут в `store`.
- Если очередь переполнена — `Submit` возвращает `ErrQueueFull`.

### Тесты (важно: без “sleep наугад”)
- **валидные события:** “10 отправил → 10 появилось”
- **невалидные:** “отправил битые → в store не попали”
- **переполнение очереди:** маленькая очередь + быстрые `submit` → получаем `ErrQueueFull`

### Мини-скелет (идея backpressure)
```go
select {
case q <- r:
	return nil
default:
	return ErrQueueFull
}
```

## Итог (после Pipeline) → Переход к симулятору

**Итог:** многопоточный pipeline + тесты.  
**Переход → Этап 3:** добавляем производителей данных (симулятор).

---

# Этап 3 — Симулятор v1: один датчик → Pipeline

## Этап 3.1 — Один датчик

На этом этапе добавляется **простейшая симуляция датчика**: функция `NextTemp(base)` генерирует температуру с небольшим шумом вокруг базового значения. По тикеру (например, раз в секунду) создаётся `Reading` и отправляется в `pipeline.Submit`, чтобы система начала получать реальные “живые” данные без железа.

Этап считается завершённым, когда тестами подтверждено, что генератор:
- выдаёт значения в заданном диапазоне,
- ведёт себя детерминированно при фиксированном seed (тесты не флакают).

**Итог:** один виртуальный датчик стабильно генерирует readings и кормит pipeline.

**Цель:** получить поток данных без железа.

### Новые файлы
- `internal/sim/generator.go`
- `internal/sim/sim_test.go`

### Вводим функцию генерации
```go
func NextTemp(base float64) float64
```
**Что делает:** генерирует температуру вокруг `base` с шумом.
**Зачем:** имитация реального датчика.

### Тесты 
- генератор в диапазоне
- детерминизм через seed

### Мини-пример
```go
t := time.NewTicker(time.Second)
defer t.Stop()
```

### Итог
- Датчик раз в секунду генерит reading и отправляет в pipeline.

## Этап 3.2 

На этом этапе симулятор расширяется до **нескольких датчиков**, работающих параллельно: вводится интерфейс `Simulator` с методом `Start(ctx)`, который запускает 3–5 горутин. Каждая горутина по тикеру генерирует `Reading` со своим `sensor_id` и отправляет его в `pipeline.Submit`. Это имитирует реальную ситуацию, когда данные приходят одновременно от разных источников.

Этап считается завершённым, когда тестами подтверждено, что:
- за короткое время появляется как минимум N записей,
- присутствуют readings от разных `sensor_id`.

**Итог:** многопоточный симулятор стабильно генерирует поток данных от нескольких датчиков и наполняет store через pipeline.

### Новые файлы
- `internal/sim/runner.go`
- `internal/sim/runner_test.go`

### Интерфейс `Simulator`

```go
type Simulator interface {
    Start(ctx context.Context)
}
```
- `Start(ctx)`
- - **Что делает:** запускает несколько датчиков (горутины), каждый по тикеру отправляет readings в pipeline.
- - **Зачем** реальный сценарий: много источников данных.

### Тесты 
- за короткое время появляется >= N readings
- сть readings от разных `sensor_id`

### Мини-пример
```go
go func() {
    // ticker → pipeline.Submit(...)
}()
```
### Итог 
- Запускается 3–5 датчиков, readings копятся в store.

# Этап 4 — Stats v1: LastN (без сети)

На этом этапе добавляется первая “аналитика”: вводится сервис `StatsService` с методом `LastN`, который считает **min/max/avg/count** по последним N измерениям выбранного датчика. Реализация опирается на данные из `ReadingStore` (через `List`) и выполняет расчёты в коде на Go.

Этап считается завершённым, когда unit-тестами подтверждено, что `LastN` корректно считает статистику на заранее известном наборе данных.

**Итог:** сервис умеет выдавать базовую статистику по последним N readings и покрыт тестами.

**Цель:** первая аналитика.
### Новые файлы
- internal/domain/stats.go
- internal/stats/service.go
- internal/stats/stats_test.go

### Вводим `StatsService` (минимум)
```go 
type StatsService interface {
    LastN(ctx context.Context, sensorID string, n int) (domain.Stats, error)
}
```

### Метод `LastN(...)`
- **Что делает:** min/max/avg/count по последним N readings.
- **Зачем:** самая простая полезная статистика.

### Тесты

- unit: известный набор → известный результат

### Мини-пример
```go
sum := 0.0
for _, r := range readings { sum += r.TempC }
avg := sum / float64(len(readings))
```

### Итог 
- LastN корректно считает статистику.

# Этап 5 — Latest + Window stats (без сети)

На этом этапе добавляются два важных сценария работы с данными:  
1) **текущее значение датчика** (`Latest`),  
2) **статистика за интервал времени** (`Window`).

Для этого расширяется фильтр чтения (`ReadingFilter` получает `From/To`), `MemoryStore` учится выбирать записи по времени и отдавать последнее значение, а `StatsService` получает метод `Window`, который считает min/max/avg/count за заданный период.

Этап считается завершённым, когда тестами подтверждено, что:
- `Latest` корректно возвращает “нет данных” для пустого хранилища и реальное последнее значение для заполненного,
- `Window` корректно фильтрует измерения по времени и считает статистику.

**Итог:** доступны “latest” и статистика по временному окну, всё покрыто тестами.

**Цель:** добавить “текущее значение” и статистику за период.

### Изменяем/добавляем файлы
- `internal/domain/reading.go` — расширить ReadingFilter: From, To
- `internal/store/store.go` — добавить Latest
- `internal/store/memory_store.go` — реализовать Latest и фильтр по времени
- `internal/stats/service.go` — добавить Window
- обновить тесты:
- - `internal/store/memory_store_test.go`
- - `internal/stats/window_test.go`

### Новый метод store: `Latest`
```go
Latest(ctx context.Context, sensorID string) (domain.Reading, bool, error)
```
- **Что делает:** возвращает самое новое reading по датчику; bool=false если нет данных.
- **Зачем:** дашборды и “проверить, что датчик жив”.

### Новый метод stats: `Window`
```go
Window(ctx context.Context, sensorID string, from, to time.Time) (domain.Stats, error)
```

- **Что делает:** статистика по временному окну.
- **Зачем:** реальная аналитика “за 10 минут/час”.

### Тесты
- `Latest:` пусто/не пусто
- `Window:` фильтр по времени корректен

### Мини-пример
```go
from := time.Now().UTC().Add(-10 * time.Minute)
```

### Итог
- `Latest` и `Window` работают.

# Этап 6
## Этап 6.1 — HTTP скелет: роутер + /health

На этом этапе поднимается **самый простой HTTP сервер** без бизнес-логики: настраивается роутер (например, `chi`) и добавляется endpoint `GET /health`, который возвращает `200 OK`. 
Это нужно, чтобы убедиться, что сеть и базовая маршрутизация работают, прежде чем подключать ingest и остальные методы.

Этап считается завершённым, когда есть handler-тест, подтверждающий, что `/health` стабильно отвечает `200` и возвращает ожидаемое тело ответа.

**Итог:** сервис доступен по HTTP, `GET /health` работает и протестирован.

**Цель:** запустить сеть без бизнес-логики.

### Новые файлы

- `internal/httpapi/router.go`
- `internal/httpapi/health.go`
- `internal/httpapi/health_test.go`

### Тесты

- `handler test /health`

### Мини-пример
```go
r := chi.NewRouter()
r.Get("/health", h.Health)
```

### Итог 
- `GET /health` → `200 ok`

## Этап 6.2 — JSON helpers + единые ошибки

На этом этапе вводится единый “стиль” HTTP-ответов: добавляются небольшие helper-функции для отправки JSON (`writeJSON`) и для ошибок (`writeError`) с общей структурой `APIError`. 
Это нужно, чтобы все обработчики отвечали одинаково и предсказуемо: клиенту проще обрабатывать ответы, а тесты становятся стабильнее.

Этап считается завершённым, когда тестами подтверждено, что `writeError` формирует корректный JSON (код + сообщение), и обработчики начинают использовать эти helpers.

**Итог:** сервис возвращает ответы и ошибки в едином JSON-формате.

**Цель:** единый формат ответов/ошибок.
### Новые файлы

- `internal/httpapi/json.go`
- `internal/httpapi/errors.go`
- `internal/httpapi/errors_test.go`

### Тесты
- `writeError` возвращает корректный JSON

### Мини-пример

```go 
type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### Итог
- `writeJSON` и `writeError` используются в обработчиках.

## Этап 6.3 — HTTP ingest v0 (учебный): синхронно пишет в store

На этом этапе появляется первый “настоящий” API для приёма данных: endpoint `POST /api/v1/ingest`. 
Он принимает JSON, декодирует его, делает базовую валидацию и **сразу** сохраняет показание через `store.Append`. 
Это учебная версия ingest — она помогает спокойно разобраться с основами HTTP POST, JSON decode и обработкой ошибок, без очередей и асинхронности.

Этап считается завершённым, когда тестами подтверждено:
- валидный запрос возвращает `201 Created`,
- плохой JSON или невалидные поля дают `400 Bad Request`,
- запись реально попадает в store.

**Итог:** сервис умеет принимать измерения по HTTP и синхронно сохранять их в хранилище.

**Цель:** закрепить POST + JSON decode.

### Новые файлы
- `internal/httpapi/dto.go`
- `internal/httpapi/ingest.go`
- `internal/httpapi/ingest_test.go`

### Endpoint
- POST /api/v1/ingest → store.Append → 201 Created

### Используем метод
- `store.Append`
- - `что делает:` сохраняет reading
- - `зачем здесь: `самая простая версия ingest

### Тесты
- 201 на валидный JSON
- 400 на плохой JSON/валидацию

### Мини-пример
```go
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    writeError(w, 400, "BAD_JSON", "cannot parse")
    return
}
```

### Итог
- POST принимает JSON и сохраняет запись.

## Этап 6.4 — HTTP ingest v1: через Pipeline (асинхронно + backpressure)

На этом этапе ingest переводится на “правильную” архитектуру: `POST /api/v1/ingest` больше **не пишет напрямую** в хранилище, а кладёт событие в многопоточный `Pipeline` через `pipeline.Submit`. 
В ответ сервер возвращает `202 Accepted` — это означает, что данные **приняты в очередь** и будут обработаны воркерами чуть позже.

Также появляется поведение при перегрузке: если очередь переполнена, `Submit` возвращает `ErrQueueFull`, а endpoint отвечает `429` или `503`. 
Это и есть **backpressure**, который защищает сервис от “падения” под нагрузкой.

Этап считается завершённым, когда тестами подтверждено:
- валидный ingest возвращает `202`,
- переполнение очереди даёт `429/503`,
- принятые события реально появляются в store после обработки воркерами.

**Итог:** ingest стал неблокирующим и устойчивым к нагрузке.

**Цель:** правильная архитектура: HTTP не блокируется записью.

### Изменяем файлы
- `internal/httpapi/ingest.go, ingest_test.go`

### Endpoint
- `POST /api/v1/ingest` → `pipeline.Submit` → `202 Accepted`
- `ErrQueueFull` → `429/503`

### Используем метод
- `pipeline.Submit`

- - `что делает:` кладёт в очередь или возвращает ошибку

- - `зачем:` backpressure и многопоточность

### Тесты
- 202 success
- 429/503 на переполненной очереди

### Мини-пример
```go
if errors.Is(err, pipeline.ErrQueueFull) {
    writeError(w, 429, "QUEUE_FULL", "try later")
}
```
### Итог
- POST отдаёт 202, данные появляются в store после обработки воркерами.

## Этап 6.5 — Read + Stats endpoints

На этом этапе добавляются HTTP endpoints для **чтения данных** и **получения статистики**: список измерений, последнее значение датчика, статистика по последним N и за интервал времени. 
Это нужно, чтобы сервис стал **полезным снаружи**: можно не только отправлять данные, но и **проверять, что они сохранились**, и **сразу получать аналитику** для графиков/дашборда. 
Все endpoints покрываются handler-тестами (корректные ответы, 400 на плохие параметры, 404 для `latest`, если данных нет).

**Цель:** полноценное HTTP API.

### Новые файлы
- `internal/httpapi/readings.go`
- `internal/httpapi/stats.go`
- `internal/httpapi/readings_test.go`
- `internal/httpapi/stats_test.go`

### Endpoints
- `GET /api/v1/readings?sensor_id=&limit=`
- `GET /api/v1/readings/latest?sensor_id=`
- `GET /api/v1/stats/last?sensor_id=&n=`
- `GET /api/v1/stats/window?sensor_id=&from=&to=`

### Используем методы
- `store.List`, `store.Latest`
- `stats.LastN`, `stats.Window`

### Тесты
- параметры/ошибки 400
- 404 для latest без данных
- корректные JSON ответы

### Мини-пример
```go
n, err := strconv.Atoi(q.Get("n"))
```
### Итог
- Все endpoints работают и возвращают JSON.

# Этап 7
## Этап 7.1 — Конфиг через env

На этом этапе добавляется централизованная конфигурация сервиса: создаётся `Config` и функция `config.Load()`, которая читает значения из переменных окружения (например, порт HTTP, количество воркеров и размер очереди) и валидирует их. 
Это нужно, чтобы управлять поведением приложения без изменения кода и удобно запускать его в разных средах (локально, в Docker, на сервере). 
Этап закрывается unit-тестами на корректную загрузку и ошибки при неверных значениях.

### Новые файлы
- `internal/config/config.go`

### Метод `config.Load() (Config, error)`
- **Что делает:** читает env и валидирует конфиг (порт, workers, queue size).
- **Зачем:** менять поведение без правки кода.

### Тесты
- unit test на `Load`

### Мини-пример
```go
type Config struct {
    HTTPPort  int
    Workers   int
    QueueSize int
}
```
## Этап 7.2 — Логи (slog) + access log middleware

На этом этапе добавляются структурированные логи: создаётся `logging.New(cfg)` для настройки `slog` (уровень логирования, формат) и подключается middleware `access_log`, которое записывает в лог каждый HTTP-запрос (метод, путь, статус, время обработки). 
Это нужно для быстрой диагностики и понимания поведения сервиса в реальном запуске. 
Этап закрывается простым smoke-тестом, что middleware корректно пропускает запрос и не ломает ответы.

### Новые файлы
- `internal/logging/logging.go`
- `internal/httpapi/middleware/access_log.go`

### Метод `logging.New(cfg) *slog.Logger`
- **Что делает:** создаёт logger с уровнем.
- **Зачем:** диагностика и прод-практика.

### Тесты
- middleware smoke test

### Мини-пример
```go
logger.Info("request", "path", r.URL.Path, "status", status)
```
## Этап 7.3 — Request ID + Recover middleware

На этом этапе добавляются два важных middleware для “продового” поведения сервиса:  
- **Request ID** — присваивает каждому запросу уникальный идентификатор и возвращает его в header, чтобы удобно искать все логи конкретного запроса.  
- **Recover** — перехватывает паники в обработчиках и вместо падения сервера возвращает `500`, сохраняя сервис живым.

Этап закрывается тестами: проверяется наличие request id в ответе и то, что panic превращается в `500`.

### Новые файлы

- `internal/httpapi/middleware/request_id.go`

- `internal/httpapi/middleware/recover.go`

### Тесты
- request id в ответе
- recover возвращает 500

### Итог
- каждый запрос получает request id в header
- паника не валит сервер, а отдаёт 500

## Этап 7.4 — Graceful shutdown

На этом этапе добавляется корректное завершение сервиса: при `Ctrl+C` (или сигнале остановки) сервер перестаёт принимать новые запросы, аккуратно завершает текущие, останавливает pipeline-воркеры и закрывает ресурсы. 
Это нужно, чтобы сервис не терял данные, не оставлял “висящие” горутины и не портил состояние при остановке. 
Этап закрывается коротким integration smoke-тестом: запуск → остановка → проверка, что нет дедлоков и процесс завершается нормально.

### Изменяем
- `cmd/server/main.go`

### Тесты
- короткий integration smoke (старт/стоп без дедлока)

### Итог
- Ctrl+C завершает HTTP и воркеры корректно.

# Этап 8 PostgreSQL
## Этап 8.1 — Compose Postgres + миграции goose + тест “таблица существует”

На этом этапе поднимается настоящая база данных **PostgreSQL** через `docker-compose` и добавляются **миграции** (goose), которые создают таблицу для хранения измерений. 
Параллельно добавляется небольшой тест, который поднимает Postgres (например, через testcontainers), применяет миграции и проверяет, что нужная таблица действительно появилась. 
Это нужно, чтобы окружение и схема БД всегда поднимались одинаково и не зависели от ручных действий.

**Итог:** Postgres и миграции работают воспроизводимо и подтверждены тестом.

### Новые файлы
- `deploy/docker-compose.yml`
- `migrations/0001_create_readings.sql`
- `internal/migrate/migrate.go`
- `internal/migrate/migrate_test.go`

### Итог
- миграции применяются воспроизводимо.

## Этап 8.2 — PostgresStore v1: подключение + Health/Close

На этом этапе добавляется реализация хранилища на **PostgreSQL**: создаётся `PostgresStore`, который умеет подключаться к базе и управлять жизненным циклом соединений. 
В интерфейс хранилища добавляются методы `Health` (проверка доступности БД) и `Close` (корректное закрытие пула), чтобы сервис мог честно отвечать readiness-проверкам и правильно завершаться. 
Этап закрывается интеграционными тестами, подтверждающими, что подключение работает и `Health` возвращает успех на живой базе.

**Итог:** сервис умеет подключаться к Postgres и проверять/закрывать соединения.

### Новые файлы
- `internal/store/postgres_store.go`
- `internal/store/postgres_store_test.go`

### Новые методы в `ReadingStore`
```go
Health(ctx context.Context) error
Close() error
```

- `Health`
- - `что делает:` ping/SELECT 1
- - `зачем:` readiness endpoint (/ready)

- `Close`
- - `что делает:` закрывает пул
- - `зачем:` graceful shutdown

### Тесты
- integration: `Health()` ok

## Этап 8.3 — PostgresStore v2: Append/List

На этом этапе `PostgresStore` получает основную функциональность хранения: реализуются методы `Append` (вставка нового измерения в таблицу) и `List` (выборка измерений по фильтрам). 
Это нужно, чтобы заменить in-memory хранение на реальную БД без изменения остальной логики сервиса. 
Этап закрывается интеграционным тестом: запись через `Append` должна затем находиться через `List`.

**Итог:** измерения сохраняются и читаются из PostgreSQL.

### Тесты
- integration: append → list

### Итог
- Append/List работают с Postgres.

## Этап 8.4 — PostgresStore v3: Latest + индексы

На этом этапе добавляется быстрый запрос “последнее значение”: реализуется метод `Latest`, который возвращает самое свежее измерение по `sensor_id`. 
Одновременно настраиваются/проверяются индексы по `sensor_id` и времени, чтобы запрос работал быстро даже при большом объёме данных. Этап закрывается интеграционным тестом: после вставки нескольких измерений `Latest` должен вернуть самое новое.

**Итог:** `Latest` корректен и рассчитан на рост данных.

### Тесты
- integration: latest возвращает самое новое

### Итог
- Latest работает быстро и корректно.

## Этап 8.5 — Retention: DeleteBefore + job

На этом этапе добавляется механизм “ретеншена” (очистки старых данных): в хранилище появляется метод `DeleteBefore`, который удаляет записи старше заданной даты и возвращает количество удалённых строк. 
Поверх этого делается простая фоновая job (или отдельный запуск), которая периодически чистит данные по правилу retention. 
Это нужно, чтобы база данных не росла бесконечно и сервис оставался управляемым. 
Этап закрывается интеграционным тестом: старые записи должны удаляться, а новые — оставаться.

**Итог:** есть контролируемое хранение данных с автоматической очисткой.

### Новые файлы
- internal/retention/job.go

### Новый метод
```go
DeleteBefore(ctx context.Context, t time.Time) (int64, error)
```
- `Что делает:` удаляет старые записи, возвращает количество удалённых.

- `Зачем:` контроль роста БД.

### Тесты
- integration: старые удалились, новые остались

# Этап 9 — Bus (подготовка к Kafka): интерфейсы + InMemory bus
## Этап 9.1 — Bus интерфейсы

На этом этапе вводится абстракция “шины событий”: определяются интерфейсы `ReadingPublisher` и `ReadingConsumer`. 
Они отделяют бизнес-логику (приём/обработка измерений) от конкретной технологии транспорта (сначала in-memory, позже Kafka). 
Благодаря этому дальше можно будет переключиться на Kafka без переписывания HTTP/gRPC и обработчиков — изменится только реализация publisher/consumer. 
Этап закрывается простыми unit-тестами/моками, подтверждающими, что интерфейсы удобно подменяются.

**Итог:** есть контракт для event-driven архитектуры, готовый для in-memory и Kafka реализаций.

### Новые файлы
- `internal/bus/bus.go`

- Новый интерфейс `ReadingPublisher`
```go
type ReadingPublisher interface {
    Publish(ctx context.Context, r domain.Reading) error
    Health(ctx context.Context) error
    Close() error
}
```
- **Publish** — публикует reading в шину
- - **зачем:** API не блокируется обработкой/БД

- **Health** — проверка шины
- - **зачем**: readiness

- **Close** — закрыть producer
- - **зачем**: graceful shutdown

### Новый интерфейс `ReadingConsumer`
```go
type ReadingConsumer interface {
    Run(ctx context.Context, handler func(context.Context, domain.Reading) error) error
    Health(ctx context.Context) error
    Close() error
}
```
- `Run` — читает сообщения и вызывает handler
- - **зачем:** обработка независима от транспорта

- `Health/Close` — readiness/shutdown

### Тесты
- unit: компиляция + простые моки (можно через hand-written mock)

### Итог
- Интерфейсы определены, зависимости можно “подменять” (in-memory/Kafka).

## Этап 9.2 — InMemory Publisher/Consumer

На этом этапе делается первая рабочая реализация шины событий без внешних зависимостей: `InMemoryPublisher` и `InMemoryConsumer` на основе каналов. 
Это нужно, чтобы отладить event-driven поток (publish → consume → handler) локально и быстро, прежде чем подключать Kafka. 
Этап закрывается unit-тестом: при публикации N сообщений consumer должен вызвать обработчик ровно N раз.

**Итог:** есть полностью рабочий in-memory bus, который ведёт себя так же, как будущая Kafka-версия, но проще для разработки и тестов.

### Новые файлы
- `internal/bus/inmem_pub.go`
- `internal/bus/inmem_cons.go`
- `internal/bus/inmem_test.go`

### Тесты
- unit: publish N → handler N раз

### Итог
- publish → consumer вызывает handler N раз.

# Этап 10 — Kafka
## Этап 10.1 — Kafka v1: минимальный publish/consume (JSON)

На этом этапе in-memory шина заменяется на настоящую **Kafka (или Redpanda)**: поднимается брокер через `docker-compose`, пишется минимальный `KafkaPublisher` (публикует сообщения в топик) и `KafkaConsumer` (читает из топика и передаёт данные обработчику). 
Для простоты на первом шаге payload остаётся в **JSON**. 
Этап закрывается интеграционным тестом: после `publish` сообщение должно быть прочитано consumer’ом и сохранено в PostgreSQL.

**Итог:** работает реальный pipeline событий: **API → Kafka → Consumer → Postgres**.

### Новые файлы
- `deploy/docker-compose.yml (+ Kafka или Redpanda)`
- `internal/bus/kafka_pub.go`
- `internal/bus/kafka_cons.go`
- `internal/bus/kafka_integration_test.go`

### Тесты
- integration: publish → запись в БД

### Итог
- API публикует → Kafka → consumer читает → данные появляются в Postgres.

## Этап 10.2 — Kafka v2: consumer group + commit после успеха

На этом этапе Kafka-часть доводится до “правильного” поведения: consumer работает как **consumer group**, а смещение (offset) коммитится **только после успешной обработки** сообщения (например, после записи в БД). 
Это нужно, чтобы при сбоях данные не терялись: если обработчик вернул ошибку, сообщение должно быть обработано повторно. 
Этап закрывается интеграционным тестом, который проверяет, что при ошибке handler сообщение не считается обработанным и остаётся доступным для повторной обработки.

**Итог:** Kafka consumer становится надёжнее — данные не теряются при ошибках.

### Тесты
- integration: ошибка handler → сообщение не считается обработанным (повторяемость)

### Итог
- offset commit делается только если обработка успешна (запись в БД прошла).

# Этап 11 — Надёжность (idempotency + retry/backoff)
## Этап 11.1 — Idempotency (дубликаты)

На этом этапе добавляется защита от повторной доставки сообщений: в модель измерения вводится `ReadingID`, а в базе создаётся уникальный индекс по этому полю. 
Теперь, если одно и то же событие придёт из Kafka повторно, система не создаст дубликат в таблице — запись либо будет вставлена один раз, либо повторная вставка будет корректно обработана. 
Этап закрывается интеграционным тестом: два сообщения с одинаковым `ReadingID` должны дать только одну строку в БД.

**Итог:** обработка становится идемпотентной — повторы не портят данные.

### Изменяем
- `internal/domain/reading.go` — добавить `ReadingID`
- миграции — уникальный индекс по reading_id

### Тесты
- integration: 2 одинаковых ReadingID → 1 строка

### Итог
- дубликаты Kafka сообщений не создают дубликаты в БД.

## Этап 11.2 — Retry/backoff на временные ошибки

На этом этапе добавляется механизм повторных попыток: создаётся helper для **retry с backoff**, который используется в обработке сообщений (например, при временной недоступности БД или сетевых сбоях). 
Это нужно, чтобы система не “падала” из-за кратковременных проблем и могла автоматически восстановиться. 
Этап закрывается unit-тестами, которые проверяют количество попыток и корректность задержек/backoff (без бесконечных ретраев).

**Итог:** при временных ошибках выполняются ограниченные retry-попытки с backoff.

### Новые файлы
- `internal/retry/backoff.go`
- `internal/retry/backoff_test.go`

## Тесты
- unit: количество попыток и backoff

### Итог
- на временных ошибках происходит retry с ограничением попыток.

# Этап 12 — Prometheus (минимум → расширение → Grafana)
## Этап 12.1 — /metrics минимально

На этом этапе добавляется базовая наблюдаемость: подключается endpoint `/metrics` в формате Prometheus и вводятся первые метрики (например, счётчики принятых событий/обработанных сообщений). 
Это нужно, чтобы начать измерять работу сервиса и видеть его состояние не только по логам. 
Этап закрывается handler-тестом: `/metrics` должен отвечать `200` и содержать имена зарегистрированных метрик.

**Итог:** сервис экспортирует метрики через `/metrics`, готов к подключению Prometheus.

### Новые файлы
- `internal/metrics/metrics.go`
- `internal/httpapi/metrics.go`
- `internal/httpapi/metrics_test.go`

### Итог
- `/metrics` отдаёт метрики.

### Тесты
- handler: 200 и содержит имена метрик

## Этап 12.2 — Метрики “как в проде”: duration + ошибки

На этом этапе метрики расширяются до уровня, близкого к реальной эксплуатации: добавляются **histogram’ы времени** (например, длительность HTTP-запросов и запись в БД) и **счётчики ошибок** для Kafka/consumer/DB. 
Это нужно, чтобы видеть не только “сколько”, но и “насколько быстро” работает сервис, и где возникают сбои. 
Этап закрывается unit-тестами, которые подтверждают, что при ошибках корректно увеличиваются соответствующие counters.

**Итог:** метрики показывают производительность и ошибки сервиса в продовом стиле.

### Итог
- есть duration histogram для HTTP/DB, counters для ошибок Kafka/consumer.

### Тесты
- unit: при ошибке инкрементится error counter

## Этап 12.3 — Prometheus + Grafana в compose + dashboard

На этом этапе поднимается полноценная система мониторинга: в `docker-compose` добавляются **Prometheus** (сбор метрик) и **Grafana** (визуализация), настраивается `prometheus.yml` и добавляется базовый dashboard для Grafana. 
Это нужно, чтобы метрики стали “живыми” — можно было смотреть графики, видеть нагрузку, задержки и ошибки в удобном интерфейсе. 
Этап считается завершённым, когда после запуска compose в Grafana отображаются графики по метрикам сервиса.

**Итог:** метрики автоматически собираются Prometheus и отображаются в Grafana.

### Новые файлы
- `deploy/prometheus.yml`
- `deploy/grafana/dashboard.json`
- обновить `deploy/docker-compose.yml`

### Итог
- Grafana показывает графики.

# Этап 13 — gRPC (Ping → Ingest → Query)
## Этап 13.1 — gRPC smoke: proto + Ping

На этом этапе добавляется базовый gRPC “скелет”: создаётся `.proto` файл, поднимается gRPC сервер и реализуется простой метод `Ping`, который нужен для проверки, что gRPC вообще настроен и работает. 
Это минимальная точка входа перед добавлением реальных методов (ingest и запросы). 
Этап закрывается тестом через `bufconn`, который вызывает `Ping` без реальной сети и проверяет корректный ответ.

**Итог:** gRPC инфраструктура поднята и подтверждена тестом.

### Новые файлы
- `proto/temperature.proto`
- `internal/grpcapi/server.go`
- `internal/grpcapi/server_test.go (bufconn)`

### Тесты
- bufconn: Ping возвращает ответ

### Итог
- gRPC сервер запускается, Ping работает.

## Этап 13.2 — gRPC ingest: SendReading → Publish

На этом этапе добавляется реальный gRPC метод приёма данных: `SendReading`. 
Он принимает измерение по gRPC и **публикует событие в bus** через `publisher.Publish`, то есть идёт тем же путём, что и HTTP ingest. 
Это нужно, чтобы поддерживать два интерфейса (HTTP и gRPC) без дублирования логики обработки. 
Этап закрывается тестом через `bufconn` с мокнутым publisher: проверяется, что при вызове `SendReading` публикация действительно происходит.

**Итог:** gRPC ingest работает и интегрирован в event-driven pipeline.

### Тесты
- bufconn + mock publisher

### Итог
- SendReading публикует событие в bus.

## Этап 13.3 — gRPC query: GetLatest + stats

На этом этапе gRPC расширяется методами чтения: добавляются запросы вроде `GetLatest` и методы для получения статистики. 
Они напрямую используют уже существующие слои `store` и `stats`, чтобы не дублировать бизнес-логику и держать поведение одинаковым для HTTP и gRPC. 
Этап закрывается тестами через `bufconn` с моками `store/stats`, подтверждающими корректные вызовы и ответы.

**Итог:** gRPC API умеет не только принимать данные, но и отдавать latest/статистику.

### Тесты
- bufconn + mock store/stats

### Итог
- GetLatest/GetStats используют store/stats.

# Этап 14 — Protobuf end-to-end (конвертеры → Kafka payload Protobuf)
## Этап 14.1 — Конвертеры domain ↔ proto

На этом этапе добавляется тонкий слой конвертации между внутренними структурами (`domain`) и gRPC/Protobuf моделями (`proto`). 
Это нужно, чтобы аккуратно и единообразно обрабатывать типы вроде времени (timestamp), чисел и идентификаторов, а также уменьшить количество ошибок и “разнобоя” в разных частях кода. 
Этап закрывается unit-тестами, которые проверяют корректность конвертации полей и времени.

**Итог:** domain и proto модели согласованы, конвертация надёжна и протестирована.

###Новые файлы
- `internal/codec/convert.go`
- `internal/codec/convert_test.go`

### Тесты
- unit: конвертация timestamp/полей

### Итог
- конвертация времени/полей корректна.

## Этап 14.2 — Kafka payload = Protobuf

На этом этапе сообщения в Kafka переводятся с JSON на **Protobuf bytes**: добавляется кодек, который сериализует измерение в Protobuf при публикации и десериализует его в consumer. 
Это нужно, чтобы использовать единый контракт данных для gRPC и Kafka, снизить вероятность ошибок формата и облегчить версионирование. 
Этап закрывается интеграционным тестом: publish Protobuf-сообщения → consumer успешно декодирует → запись появляется в PostgreSQL.

**Итог:** Kafka использует Protobuf как формат сообщений, pipeline end-to-end работает.

### Новые файлы
- `internal/bus/codec_proto.go`

## Тесты
- integration: publish protobuf → decode → запись

### Итог
- producer публикует protobuf bytes, consumer декодит и пишет в БД.


# Этап 15 — Series (time-series): Go → SQL оптимизация
## Этап 15.1 — Series в Go (простая версия)

На этом этапе добавляется поддержка временных рядов (series): реализуется расчёт точек для графика по заданному шагу времени (`step`) — данные группируются по “бакетам” (например, по минутам) и для каждого бакета формируется точка (например, средняя/минимальная/максимальная температура). 
Это нужно, чтобы строить графики и видеть динамику изменения температуры. 
Этап закрывается unit-тестами, проверяющими корректность бакетирования по времени.

**Итог:** сервис умеет отдавать series-точки для графиков, реализовано на Go и протестировано.

### Новые файлы
- `internal/stats/series.go`
- `internal/stats/series_test.go`

### Тесты
- unit: бакетирование по времени

### Итог
- endpoint возвращает точки для графика по шагу времени (step).

## Этап 15.2 — Series в SQL (оптимизация)

На этом этапе расчёт временных рядов оптимизируется: вместо того чтобы вытаскивать все readings в Go и группировать их там, агрегация переносится в **SQL** (группировка по временным бакетам и вычисление агрегатов прямо в базе). 
Это нужно, чтобы series работал быстро на больших объёмах данных и не нагружал память/CPU сервиса. 
Этап закрывается интеграционным тестом: результат SQL-агрегации должен совпадать с ожидаемым (или с эталонным расчётом).

**Итог:** series считается в базе, сервис получает уже готовые агрегированные точки.

### Тесты
- integration: результат совпадает с ожидаемым

### Итог
- агрегация делается в SQL, не тянем все readings в Go.


# Этап 16 — ESP32 (финал)
## Этап 16.1 — ESP32 через HTTP


На этом этапе симуляция дополняется (или заменяется) реальным источником данных: **ESP32** начинает отправлять измерения в сервис через `POST /api/v1/ingest`. 
Добавляется краткая документация с форматом запроса и примером отправки, а также e2e smoke-тест, который проверяет весь путь целиком: ingest → данные попали в систему → `latest/stats` возвращают ожидаемый результат. 
Это нужно, чтобы убедиться, что backend готов к реальному устройству и работает end-to-end.

**Итог:** ESP32 успешно отправляет данные, а сервис показывает их через API и статистику.

### Новые файлы
- `docs/esp32.md`
- `internal/e2e/e2e_test.go (build tag e2e)`

### Тесты
- e2e: ingest → latest/stats

### Итог
- ESP32 шлёт POST /api/v1/ingest, данные видны через latest/stats.

## Этап 16.2 — ESP32 через gRPC (опционально)

На этом этапе (по желанию) добавляется второй способ отправки данных с устройства: вместо HTTP ESP32 отправляет измерения через gRPC, вызывая метод `SendReading`. 
Это нужно, если хочется попробовать более строгий контракт и бинарный протокол Protobuf, а также приблизить проект к “индустриальному” стилю интеграции.

**Итог:** ESP32 умеет отправлять readings в сервис по gRPC.
### Итог
- ESP32 вызывает SendReading по gRPC.

## Итоговые артефакты проекта
- многопоточный pipeline (очередь + worker pool) с backpressure
- симулятор датчиков
- HTTP API: ingest/read/latest/stats + middleware + graceful shutdown
- PostgreSQL: миграции + store + retention
- Bus: in-memory → Kafka
- надёжность: idempotency + retry/backoff
- Prometheus метрики + Grafana dashboard
- gRPC API
- Kafka payload Protobuf
- time-series (series): Go → SQL оптимизация
- ESP32 интеграция + e2e smoke тесты