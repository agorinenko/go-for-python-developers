# Практика. Урок 23: Dockerfile и docker-compose для orders

Критерий приёмки — готовый тест `validate/validate_test.go`: он читает
`Dockerfile` и `docker-compose.yml` из этого каталога и проверяет контракт.
Ваша задача — написать оба файла так, чтобы тест стал зелёным.

## Задание

1. **`Dockerfile`** — multi-stage build сервиса (лекция, раздел
   «Multi-stage build»):
   - builder: `golang:1.25-alpine`; сначала `COPY go.mod go.sum` + `go mod download`,
     затем остальной код;
   - сборка с `CGO_ENABLED=0` (статический бинарь), рекомендуется
     `-trimpath` и `-ldflags="-s -w"`;
   - final: `gcr.io/distroless/static-debian12:nonroot` **или** `scratch`
     (в scratch не забудьте не-root пользователя);
   - `EXPOSE 8080`, `ENTRYPOINT` — бинарь `./cmd/orders`.
2. **`docker-compose.yml`** — вся система капстоуна (лекция, раздел
   «docker-compose»):
   - сервисы `orders`, `postgres` (`postgres:16-alpine`), `redpanda`,
     `redis` (`redis:7-alpine`);
   - у `postgres` — healthcheck `pg_isready`, volume для данных
     (`/var/lib/postgresql/data`);
   - у `redis` — healthcheck `redis-cli ping`;
   - `orders`: порт 8080, `environment` с `HTTP_ADDR`, `DB_DSN`
     (postgres://...@postgres:5432/...), `KAFKA_BROKERS`, `REDIS_ADDR`;
     `depends_on` с `condition: service_healthy` для postgres и redis;
   - переменные `POSTGRES_USER/PASSWORD/DB` для postgres.

Тест проверяет: два `FROM`, `CGO_ENABLED=0`, финальный образ
distroless/static или scratch, четыре сервиса в compose, healthcheck'и
postgres/redis, volume у postgres. Остальное (порты, environment, depends_on) —
по заданию выше и контракту CAPSTONE.md.

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...        # validate_test.go зелёный
```

Плюс, если установлен Docker:

```sh
docker compose config          # валидация compose-файла (без запуска)
docker compose up -d postgres redis   # поднять инфраструктуру
docker compose build orders            # собрать образ сервиса
docker compose up --build              # вся система
```

## Перенос в капстоун

1. Скопируйте `Dockerfile` в `orders/deployments/docker/Dockerfile`
   (поправьте контекст: сборка из корня `orders/`).
2. Скопируйте `docker-compose.yml` в `orders/deployments/docker/` и расширьте
   его: топики/миграции капстоуна (недели 6–8) — например, команда применения
   миграций перед стартом orders или init-контейнер.
3. Создайте `orders/.dockerignore` (`.git`, `bin/`, `*.md`, `.env`,
   `**/*_test.go`).
4. Проверьте в капстоуне: `docker compose config` без ошибок, `docker compose
   up --build` — orders стартует только после `service_healthy` postgres,
   `POST /v1/orders` отвечает 201.
