# Урок 23. Docker для Go

## Цели урока

- понять, почему Go-образы на два порядка меньше Python-образов и что для этого нужно
  сделать руками (`CGO_ENABLED=0`, статическая линковка);
- освоить multi-stage build: `golang:1.25-alpine` → `distroless/static` или `scratch`;
- разобрать `HEALTHCHECK` и его проблему в distroless-образах;
- собрать docker-compose всей системы итогового проекта: orders + PostgreSQL + Redpanda (Kafka) + Redis;
- научиться класть в образ только то, что нужно (.dockerignore, слои).

## Задача из итогового проекта

До сих пор сервис заказов запускался `make run` и жил на машине разработчика. Пора в
контейнер: на этом уроке создаём `orders/deployments/docker/` — Dockerfile сервиса и
docker-compose всей системы (orders, postgres, redpanda, redis) с томами, healthcheck'ами
и переменными окружения. Контракт — в [CAPSTONE.md](../../CAPSTONE.md).

## Почему Go-образы крошечные

Go компилируется в **один статический исполняемый файл**: среда выполнения (GC, планировщик, стандартная
библиотека) вшит в исполняемый файл. Образу не нужны ни интерпретатор, ни системные
библиотеки, ни виртуальное окружение. Итог:

```sh
$ docker images orders
orders  latest  9.8MB    # Go, distroless/static
orders  latest  340MB    # Python, python:3.12-slim + venv + пакеты
```

Разница в 30+ раз — это не «Go лучше», это разная архитектура: Python-образ обязан нести
интерпретатор, stdlib, скомпилированные C-расширения (numpy/pydantic-core), а часто ещё
и компилятор для сборки native wheels на этапе build. Go-образ несёт один файл.

Но статичность — не всегда автоматическая:

- **cgo включён** → исполняемый файл динамически линкуется с libc, и в scratch-образе он не
  запустится (нет `/lib`). Решение: `CGO_ENABLED=0` — чистый Go-исполняемый файл без libc.
- **DNS** без cgo: `net` использует чистый Go-резолвер (читает `/etc/resolv.conf`,
  `/etc/hosts`). В distroless/scratch этих файлов нет — Kubernetes их монтирует в под
  автоматически, в чистом `docker run` придётся позаботиться самому. Отсюда же флаг
  `-tags netgo` в старых статьях — с современными версиями Go он не нужен, резолвер
  выбирается по `CGO_ENABLED` автоматически.
- **TLS-корни**: `distroless/static-debian12` содержит CA-сертификаты и tzdata;
  голый `scratch` — ничего. Сервису, который ходит по HTTPS в интернет (или валидирует
  сертификаты), нужны корневые сертификаты — проще всего взять
  `gcr.io/distroless/static-debian12`.

Правило выбора базового образа: `scratch` — когда нужен минимальный размер и нет
требований к CA/tz; `distroless/static` — когда нужны CA-сертификаты, tzdata и
`nonroot`-пользователь из коробки; `alpine` — когда нужно что-то диагностировать внутри
контейнера (shell) — и это осознанный компромисс.

## Multi-stage build

Паттерн: собрать в одном образе (с набором инструментов), упаковать в другой (пустой):

```dockerfile
# ---------- builder: только компиляция ----------
FROM golang:1.25-alpine AS builder
WORKDIR /src

# слой кэшируется: go.mod/go.sum меняются редко
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/orders ./cmd/orders

# ---------- final: только исполняемый файл ----------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/orders /orders
EXPOSE 8080
ENTRYPOINT ["/orders"]
```

Почему каждый элемент на своём месте:

- **`go mod download` до `COPY . .`** — слой с зависимостями переиспользуется между
  сборками: при каждом изменении кода заново качаются только изменённые слои. Без этого
  `go mod download` выполняется при каждом билде.
- **`-trimpath`** — вырезает абсолютные пути сборочной машины из исполняемого файла (стек-трейсы,
  отладочная информация): воспроизводимые сборки и меньше утечек внутренней раскладки.
- **`-ldflags="-s -w"`** — убирает symbol table и DWARF: минус 20–30% размера; для
  продакшена стек-трейсы всё равно собираются через pprof/otel, а не из исполняемого файла.
- **`distroless/static-debian12:nonroot`** — пользователь `nonroot` уже объявлен:
  контейнер не работает от root. Если возьмёте `scratch`, добавьте `USER 65532:65532`
  сами.
- **`ENTRYPOINT`, а не `CMD`** — исполняемый файл — единственное, что умеет этот контейнер.

`GOOS=linux` в builder нужен, если сборка может идти на Mac/Windows (образ builder —
linux, так что формально можно опустить; оставьте для явности и воспроизводимости).
`GOARCH` понадобится для мультиплатформенных образов (arm64-раннеры).

## HEALTHCHECK и проблема distroless

`HEALTHCHECK` в Dockerfile исполняет команду **внутри контейнера**. Для postgres/redis
это естественно: `pg_isready` и `redis-cli ping` — исполняемые файлы самих сервисов:

```dockerfile
# внутри официального образа postgres
HEALTHCHECK CMD pg_isready -U postgres -d orders
```

Для Go-сервиса на distroless это проблема: в образе **нет shell, нет curl, нет wget** —
нечем выполнить HTTP-запрос к собственному `/healthz` (эндпоинт у нас есть с недели 9,
но вызвать его нечем). Варианты решения:

1. **Никакого HEALTHCHECK в Dockerfile** — здоровье проверяют Kubernetes-пробы
   (урок 24): kubelet делает HTTP-запрос к поду **снаружи**, шелл внутри не нужен. В
   компоузе для orders healthcheck тоже можно опустить, а в `depends_on` опираться на
   healthcheck'и postgres/redis.
2. **Отдельный healthcheck-исполняемый файл**: рядом с основным собрать крошечный статический
   `healthcheck` (wget-подобный), положить в тот же образ и объявить
   `HEALTHCHECK CMD ["/healthcheck", "http://127.0.0.1:8080/healthz"]` — образ растёт на
   ~1 МБ, но healthcheck работает и в `docker run`, и в compose.
3. **Добавить busybox-слой** с `wget` — «грязно», но быстро; размер вырастает на ~2 МБ
   и в образе появляется шелл (лишняя поверхность атаки).

Канонический ответ для Go+distroless — вариант 1: HEALTHCHECK/Docker-уровня —
пережиток одиночных `docker run`; в оркестраторе здоровье — задача probes. Но в
docker-compose (без оркестратора) вариант 2/3 даёт честный `depends_on: condition:
service_healthy`. В итоговом проекте: для postgres и redis — их штатные команды, для orders —
проверка readiness снаружи (curl с хоста) или проброс к k8s-пробам на следующем уроке.

## docker-compose всей системы

Собираем локальное окружение итогового проекта целиком (фрагменты):

```yaml
services:
  orders:
    build:
      context: ..        # каталог orders/ — там go.mod
      dockerfile: deployments/docker/Dockerfile
    ports: ["8080:8080"]
    environment:
      HTTP_ADDR: ":8080"
      DB_DSN: postgres://orders:orders@postgres:5432/orders?sslmode=disable
      KAFKA_BROKERS: redpanda:9092
      REDIS_ADDR: redis:6379
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: orders
      POSTGRES_PASSWORD: orders
      POSTGRES_DB: orders
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U orders -d orders"]
      interval: 5s
      timeout: 3s
      retries: 10

  redis:
    image: redis:7-alpine
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  pgdata:
```

Разбор ключевых решений:

- **`depends_on: condition: service_healthy`** — orders стартует только когда БД отвечает
  на `pg_isready`. Простой `depends_on` (без condition) ждёт лишь **запуска процесса**,
  а не готовности — классическая гонка «миграции опередили postgres».
- **volume для данных** — без него `docker compose down -v` (или падение контейнера)
  стирает БД. Для кэша (redis) volume необязателен: данные пересоздаваемы (урок 22).
- **`environment` — секреты как env**: для локальной разработки допустимо
  (`POSTGRES_PASSWORD`). Для продакшена — отдельный разговор (Kubernetes Secrets,
  внешние секрет-менеджеры; не коммитьте реальные пароли в compose-файл — используйте
  `env_file` с `.env` в `.gitignore`).
- **redpanda** — Kafka-совместимая платформа: один исполняемый файл вместо Zookeeper+Kafka,
  быстрый старт; с недели 8 итоговый проект работает с Kafka-протоколом, redpanda совместима с
  ним. В compose-блоке redpanda свои переменные (`--advertise-kafka-addr` и др.) —
  точный набор из его документации.

## .dockerignore

Контекст сборки — всё, что `docker build` отправляет демону. Без `.dockerignore` в
образ (точнее, в контекст) попадают `.git/`, исполняемые файлы `bin/`, тестовые данные,
`README.md`, а в худшем случае — `.env` с секретами. Минимум для Go-репозитория:

```
.git
bin/
*.md
.env
**/*_test.go
```

Обратите внимание: `**/*_test.go` исключает тесты из **контекста**, а не из сборки —
`go build ./...` их и так не собирает, но `COPY . .` не потащит их в слой builder.
Главный эффект .dockerignore — скорость и безопасность контекста, а не размер
финального образа.

## Go vs Python

Один и тот же сервис в двух мирах:

```dockerfile
# Python: slim-образ + venv + gunicorn/uvicorn
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000", "--workers", "4"]
```

Сравнение по существу:

| | Go | Python |
|---|---|---|
| Финальный образ | исполняемый файл ~10–20 МБ, без ОС-зависимостей | интерпретатор + venv + wheels, 150–400 МБ |
| База образа | scratch/distroless (пусто) | нужен минимум debian/alpine |
| Запуск | один процесс, миллисекунды | uvicorn + N исполнителей, warmup импортов |
| Масштабирование внутри контейнера | горутины в одном процессе | исполнители-процессы (`--workers`) |
| Build | компиляция, кросс-компиляция тривиальна | установка пакетов, native wheels |

Ключевое следствие: в Go-контейнере **нет смысла в исполнитель-процессах** — один исполняемый файл
использует все ядра через горутины, а количество реплик задаёт оркестратор (урок 24).
В Python `--workers 4` — вынужденная мера (GIL), и её приходится согласовывать с
ресурсами контейнера. В C# картина ближе к Go: `dotnet publish` + self-contained —
тоже один артефакт, но среда выполнения .NET в образе обычно всё же присутствует.

## Подводные камни и анти-паттерны

1. **CGO включён, образ scratch** — исполняемый файл не запустится (`not found` на линковщике
   libc). Симптом коварный: ошибка выглядит как «файл не найден», а не «нет libc».
2. **Alpine и cgo** — если cgo всё же нужен (например, C-библиотека), alpine линкует
   musl, а не glibc: пакеты, собранные под debian-базы, несовместимы. cgo + alpine —
   отдельная боль, проще взять debian-slim.
3. **`HEALTHCHECK` в distroless** — `CMD curl ...` там просто нет; см. раздел выше.
4. **Сборка без `-trimpath`** — в образах оказываются пути `/home/you/projects/...`;
   стек-трейсы в проде раскрывают раскладку машины разработчика.
5. **Запуск от root** — забыли `USER nonroot` в scratch: контейнер работает с правами
   root, а это лишняя поверхность атаки и нарушение security-политик (k8s это
   подсветит, урок 24).
6. **`latest` в продакшене** — невоспроизводимая сборка; фиксируйте версии образов
   (postgres:16-alpine — уже фиксация major/minor; для своих образов — тег по git SHA).
7. **Копирование `go.sum` мимо** — слой `COPY go.mod go.sum ./` без go.sum ломает
   воспроизводимость; оба файла копируются вместе, всегда.
8. **Секреты в `ENV` образа** — `ENV DB_PASSWORD=...` вшивает пароль в слои образа
   (виден в `docker history`). Секреты — только в среде выполнения (env при запуске, k8s
   Secrets), не на этапе build.
9. **Забытый `.dockerignore`** — контекст в сотни мегабайт, `.env` с секретами внутри
   слоя builder (а если слой попал в финальный образ — секрет «развёрнут»).
10. **Compose без healthcheck/volumes** — `depends_on` без condition гоняется со
    стартом БД; отсутствие volume = потеря данных при каждом пересоздании.

## Что делаем в итоговом проекте

1. Создаём `orders/deployments/docker/Dockerfile` (multi-stage из практики,
   `cmd/orders` как точка сборки, порт 8080).
2. Пишем `orders/deployments/docker/docker-compose.yml` всей системы: orders,
   postgres:16-alpine (volume + `pg_isready`), redpanda, redis:7-alpine
   (`redis-cli ping`), `depends_on: condition: service_healthy`.
3. Добавляем `.dockerignore` в корень `orders/`.
4. Проверяем: `docker compose config` (валидация), `docker compose up --build` —
   миграции (неделя 6) применяются, `POST /v1/orders` работает, outbox-реле (неделя 8)
   доставляет события в redpanda.
5. Образ помечаем тегом по git SHA — готово к CI (урок 24).

## Материалы для углубления

- [Docker: Multi-stage builds](https://docs.docker.com/build/building/multi-stage/) —
  официальная документация.
- [distroless](https://github.com/GoogleContainerTools/distroless) — базовые образы без
  шелла.
- [Dockerfile reference: HEALTHCHECK](https://docs.docker.com/reference/dockerfile/#healthcheck).
- [Compose: depends_on / healthcheck](https://docs.docker.com/compose/compose-file/) —
  спецификация compose-файла.
- [Redpanda quickstart (Docker)](https://docs.redpanda.com/current/get-started/quick-start/) —
  переменные окружения redpanda для compose.
- [dockerignore reference](https://docs.docker.com/build/concepts/context/#dockerignore-files).

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
