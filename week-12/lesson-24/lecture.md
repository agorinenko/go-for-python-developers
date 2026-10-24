# Урок 24. Kubernetes и CI/CD

## Цели урока

- разобрать минимальный набор Kubernetes-объектов: Pod, Deployment, Service, ConfigMap,
  Secret, HPA;
- понять три пробы — liveness/readiness/startup — и чем они отличаются;
- усвоить, почему requests/limits обязательны и как их подбирать;
- освоить стратегию обновления (rolling update) и обзорно — ingress;
- собрать CI/CD для Go на GitHub Actions: lint → test → build → docker push → deploy.

## Задача из капстоуна

Финальная неделя: сервис из контейнера (урок 23) становится системой, которой управляет
Kubernetes, а изменения попадают в прод не руками, а пайплайном. Создаём
`orders/deployments/k8s/` (deployment, service, configmap, hpa) и CI в
`orders/deployments/ci/` (переносится в `.github/workflows/`). Контракт — в
[CAPSTONE.md](../../CAPSTONE.md).

## Pod, Deployment, Service: три уровня одного сервиса

- **Pod** — минимальная единица: один или несколько контейнеров с общим сетевым
  пространством. Поды смертны, их никто не «чинит» — только пересоздаёт.
- **Deployment** — желаемое состояние: «две реплики orders, образ такой-то, стратегия
  обновления такая-то». Контроллер непрерывно сводит факт к желаемому: упал под —
  поднял новый. Это и есть self-healing.
- **Service** — стабильный сетевой адрес и балансировка поверх смертных подов:
  selector выбирает поды по меткам (`app: orders`), ClusterIP остаётся постоянным,
  пока жив Service.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
spec:
  replicas: 2
  selector:
    matchLabels: { app: orders }
  template:
    metadata:
      labels: { app: orders }
    spec:
      containers:
        - name: orders
          image: orders:latest   # в проде — тег по git SHA, не latest!
          ports: [{ containerPort: 8080 }]
---
apiVersion: v1
kind: Service
metadata:
  name: orders
spec:
  selector: { app: orders }
  ports:
    - port: 80            # порт Service
      targetPort: 8080    # порт контейнера
```

Ключевая мысль: в Go-мире под обычно = один процесс = один контейнер. Масштабируется
сервис **репликами** (`replicas`), а не воркерами внутри процесса — в отличие от
uvicorn `--workers`. Один бинарь ест все ядра пода через горутины, поэтому sidecar'ы
(прокси, агенты) добавляют осознанно, а не для «распараллеливания».

## Probes: зачем три и чем отличаются

Kubernetes сам проверяет здоровье контейнера HTTP-запросами. У нас с недели 9 есть
`/healthz` и `/readyz` — осталось задекларировать.

```yaml
livenessProbe:
  httpGet: { path: /healthz, port: 8080 }
  periodSeconds: 10
  failureThreshold: 3
readinessProbe:
  httpGet: { path: /readyz, port: 8080 }
  periodSeconds: 5
  failureThreshold: 3
startupProbe:
  httpGet: { path: /healthz, port: 8080 }
  periodSeconds: 5
  failureThreshold: 12
```

| Проба | Вопрос | Что происходит при провале | Что происходит при успехе |
|---|---|---|---|
| startup | «процесс уже поднялся?» | контейнер рестартует | начинают работать остальные пробы |
| readiness | «можно ли слать трафик?» | под выводится из Service | под возвращается в балансировку |
| liveness | «не завис ли намертво?» | контейнер убивается и пересоздаётся | ничего |

Три разные роли, и путать их дорого:

- **readiness** защищает от трафика на неготовый процесс: под поднялся, но подключения
  к БД нет — `readyz` отвечает 503, и Service не шлёт туда запросы. Без readiness
  каждый деплой сопровождается 5xx (новая реплика получает трафик раньше готовности),
  а при отказе БД сервис «чёрный» вместо «медленный».
- **liveness** — последняя линия: процесс, который не отвечает вообще (deadlock,
  утечка горутин). Проверка обязана быть **дёшевой и без внешних зависимостей**:
  liveness, который ходит в БД, при деградации БД перезапустит все поды — и вы
  добавите себе каскадный отказ. Поэтому `/healthz` — «жив ли процесс», `/readyz` —
  «готов ли обслуживать».
- **startup** решает задачу медленного старта: если контейнеру нужно 60 секунд на
  подключение к Kafka (неделя 8), то liveness с failureThreshold 3 × periodSeconds 10
  убьёт его раньше. startup даёт отдельный, более щедрый бюджет на старт и
  отключается, как только поднялся.

Общий совет по числам: liveness — медленный (periodSeconds 10), readiness — быстрее
(5), startup — с запасом на худший случай холодного старта. Плюс
`initialDelaySeconds` — грубая альтернатива startup-пробы (лучше не смешивать).

## Resources: requests и limits — обязательны

```yaml
resources:
  requests: { cpu: 100m, memory: 128Mi }
  limits:   { cpu: 500m, memory: 256Mi }
```

- **requests** — обещание поду: планировщик размещает поды так, чтобы сумма requests
  умещалась на узле. Без requests планировщик размещает «вслепую» — узел
  перегружается, соседи страдают (noisy neighbor).
- **limits** — потолок: CPU сверх лимита **throttle'ится** (не убивает, но режет
  производительность), память сверх лимита — **OOMKill** и рестарт.

Почему это обязательно, а не «потом»: без requests/limits вы не знаете ни сколько
реально ест сервис, ни где предел; HPA (см. ниже) вообще не заработает без requests на
CPU. Подбор: запустить под нагрузкой (урок 18, k6), снять фактические значения через
метрики, requests = p99 реального потребления, limits = requests × 2–4 с запасом на
всплески. Память для Go-сервиса: не жалейте — GC любит буфер; ориентируйтесь на
`GOMEMLIMIT` (Go 1.19+) вместо того, чтобы давать GC впритык.

**GOMAXPROCS и cgroups**: Go читает число ядер из ОС, а контейнер видит все ядра
узла. Под с лимитом 500m получит GOMAXPROCS = числу ядер машины → планировщик Go
создаст горы потоков, а ядро их всех throttle'ит — деградация «на ровном месте».
Лечится `automaxprocs` (uber-go/automaxprocs) — однострочный импорт в `main`, который
выставляет GOMAXPROCS по cgroup-лимиту. Для продакшена в контейнере — фактически
обязателен.

## HPA: горизонтальное масштабирование по метрикам

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: orders
spec:
  scaleTargetRef: { apiVersion: apps/v1, kind: Deployment, name: orders }
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target: { type: Utilization, averageUtilization: 60 }
```

HPA каждые ~15 секунд смотрит метрику (CPU-utilization = фактическое использование /
requests) и меняет `replicas` в обе стороны, с периодами стабилизации против
дребезга. Три вещи, которые часто упускают:

1. **Без `requests` HPA не работает** (делить не на что).
2. **CPU — метрика грубая.** Для Go-сервиса на I/O (а orders — это БД, Redis, Kafka)
   CPU может болтаться на 10% при полностью забитых очередях. Точнее —
   кастомные метрики: RPS, глубина очереди, latency p99 (Prometheus-адаптер,
   урок 20). CPU-метрика — стартовая точка, не финальная.
3. **Потолок упирается не только в HPA** — 10 реплик сервиса, упёршегося в пул БД,
   ничего не решат (урок 21: сначала внутренние лимиты, потом реплики).

## ConfigMap и Secret

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders-config
data:
  HTTP_ADDR: ":8080"
  DB_DSN: "postgres://orders:orders@postgres:5432/orders?sslmode=disable"  # плейсхолдер!
---
apiVersion: v1
kind: Secret
metadata:
  name: orders-secrets
type: Opaque
stringData:
  DB_PASSWORD: orders
```

Использование — env из ConfigMap/Secret в контейнере (`valueFrom: configMapKeyRef` /
`secretKeyRef`). Правила:

- ConfigMap — **не секретное** конфигурационное состояние; Secret — пароли, токены.
  Честное ограничение: Secret в etcd не шифруется по умолчанию и легко читается тем,
  у кого есть доступ к namespace — для реальных секретов используйте внешние
  менеджеры (Vault, Sealed Secrets, cloud KMS); в капстоуне Secret — учебный.
- Секреты и конфиги **как файлы**, а не только env: Go-приложениям часто удобнее
  читать конфиг из файла (`volumeMount`) — так не нужно перезапускать под при
  изменении (ConfigMap обновляется, приложение перечитывает или
  рестартует по `config-reloader`).

## Стратегии обновления: rolling update

```yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0     # ни одного неготового пода в каждый момент
      maxSurge: 1           # можно один лишний
```

Rolling update поднимает новые поды и гасит старые порциями. Два параметра —
компромисс скорости и доступности:

- `maxUnavailable: 0` + `maxSurge: 1` — «ни одного запроса мимо»: сначала поднимаем
  новый под, ждём readiness, гасим один старый. Дороже по ресурсам (нужен запас), но
  деплой без просадки.
- `maxUnavailable: 1` + `maxSurge: 0` — экономнее, но на пике на один под меньше.

Именно здесь readiness-проба (выше) включается в игру: без неё Kubernetes считает под
готовым, как только процесс стартовал — и rolling update становится «rolling
outage». Readiness — не галочка, а часть стратегии деплоя.

**Ingress — обзорно**: маршрутизация внешнего HTTP-трафика по host/path к Service'ам
(один вход вместо тысячи `LoadBalancer`). Для капстоуна достаточно `ingress-nginx` с
правилом `orders.example.com → orders:80`. TLS, rate limiting на уровне ingress —
темы отдельного курса; здесь важен сам факт: вход в кластер один, а не у каждого
сервиса.

## CI/CD для Go: GitHub Actions

Пайплайн для Go-сервиса — четыре шага, каждый быстрее, чем вы привыкли:

```yaml
name: ci
on:
  push: { branches: [main] }
  pull_request:

jobs:
  ci:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
          cache-dependency-path: go.sum   # кэш модулей между запусками

      - name: lint
        uses: golangci/golangci-lint-action@v6
        with: { version: v1.64 }

      - name: test
        run: go test -race ./...

      - name: build
        run: CGO_ENABLED=0 go build -trimpath -o bin/orders ./cmd/orders

      - name: docker
        if: github.ref == 'refs/heads/main'
        run: |
          docker build -t ghcr.io/you/orders:${GITHUB_SHA} .
          docker push ghcr.io/you/orders:${GITHUB_SHA}
```

Почему Go-пайплайны быстрые и почему этим грех не пользоваться:

- **Кэш модулей**: `setup-go` кэширует `GOMODCACHE` по `go.sum` — типичный прогон
  качает зависимости только при изменении `go.mod`/`go.sum`. Секунды вместо минут.
- **Кэш сборки**: `GOCACHE` тоже кэшируется (или отдельный action) — инкрементальная
  компиляция делает `go build` почти бесплатной на неизменённых пакетах.
- **Быстрые тесты**: юнит-тесты без сети/БД (интерфейсы, фейки — недели 2–4) летают;
  интеграционные (testcontainers, неделя 6) — отдельный job, который можно
  запускать только на PR.
- **`-race`** — обязателен в CI: race detector в проде не включают (замедление ~10×),
  а в тестах он ловит гонки недель 3–4. Дешёвая страховка.

Порядок шагов сознательный: lint и тесты **до** сборки — дешёвые проверки отсекают
дорогие. Docker build/push — только на `main` (по тегу git SHA, не `latest`);
deploy-шаг — `kubectl apply`/`helm upgrade` или gitops.

**GitOps одной строкой**: вместо «CI сам деплоит» репозиторий хранит желаемое
состояние (`deployments/k8s/`), а ArgoCD/Flux непрерывно сводит кластер к нему.
CI лишь пушит новый образ и обновляет манифест (или пишет новую версию тега). Так
деплой становится PR'ом в git — с ревью, аудитом и откатом через revert.

## Go vs Python

Манифесты Kubernetes для Python-сервиса выглядят почти так же — разница в образе
(урок 23) и в том, что происходит **внутри** пода:

| | Go | Python (uvicorn/gunicorn) |
|---|---|---|
| Процессов в поде | 1 бинарь, все ядра через горутины | 1 master + N воркеров |
| GOMAXPROCS / воркеры | automaxprocs по cgroup | `--workers` вручную под ресурсы пода |
| Старт | миллисекунды | импорты, warmup — секунды |
| startupProbe | формальность | часто обязательна |
| Масштабирование | реплики пода (HPA) | реплики + воркеры внутри |

Типичная ошибка переноса привычек: `uvicorn --workers 4` + `replicas: 3` в Python —
это 12 процессов на 3 пода; в Go те же 3 реплики — 3 процесса, каждый уже
многопоточный. Заводить в Go-поде «воркеры» — значит плодить процессы в обход
Kubernetes, который теряет контроль над их количеством и метриками.

Ещё одна разница — **C#**: там уместна прямая аналогия с Go (self-contained
deployment, один процесс, Kestrel), поэтому всё сказанное про под = процесс
переносится один в один.

## Подводные камни и анти-паттерны

1. **Пробы без эндпоинтов.** Манифест требует `/healthz` — а сервер отвечает 404:
   под в crash-loop'е без причины. Пробы и эндпоинты пишутся вместе (неделя 9).
2. **Liveness с внешними зависимостями** — каскадные рестарты при деградации БД
   (см. выше). Liveness — только про процесс.
3. **Нет requests** — HPA не работает, планировщик гадает, соседи страдают.
4. **GOMAXPROCS = все ядра узла** — throttle при лимите 500m; забытый automaxprocs
   проявляется как «в k8s всё в 5 раз медленнее».
5. **`latest`-тег и `imagePullPolicy: Always`** — невоспроизводимый прод; тег по SHA.
6. **Rolling update без readiness** — деплой с 5xx-окном.
7. **Секреты в ConfigMap** (или в git) — пароли в открытом виде там, где их
   читают все с доступом к namespace.
8. **`maxReplicas` без потолка зависимостей** — HPA нагонит реплик, БД ляжет
   (урок 21); масштабируйте после того, как сняты внутренние узкие места.
9. **CI без кэша модулей** — каждый прогон качает полмира; минуты, которые
   превращают CI в очередь.
10. **Deploy-шаг в CI на каждый push** — гонка деплоев; deploy только из main и
    идемпотентно (или gitops).

## Что делаем в капстоуне

1. Создаём `orders/deployments/k8s/`: `deployment.yaml` (replicas 2, все три пробы,
   resources, securityContext), `service.yaml` (ClusterIP 80 → 8080),
   `configmap.yaml` (HTTP_ADDR, DB_DSN-плейсхолдер), `hpa.yaml` (2–10 реплик, CPU 60%).
2. Переносим `ci.yml` из практики в `.github/workflows/` капстоуна: lint → test
   (`-race`) → build → docker build/push (только main, тег по SHA).
3. Добавляем automaxprocs в `cmd/orders` (одна строка в `main`).
4. Проверяем локально: `kubectl apply --dry-run=client -f deployments/k8s/` —
   манифесты валидны; CI зелёный; образ деплоится, readiness не пускает трафик
   до готовности.
5. Финальный аккорд курса: вся система из CAPSTONE.md — orders + catalog + payments +
   PostgreSQL + Kafka + Redis — в Kubernetes, собранная пайплайном.

## Материалы для углубления

- [Kubernetes: Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/).
- [Kubernetes: Managing Resources for Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/).
- [Kubernetes: HorizontalPodAutoscaler Walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/).
- [Kubernetes: Rolling Update (Deployment)](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment).
- [uber-go/automaxprocs](https://github.com/uber-go/automaxprocs) — GOMAXPROCS по cgroup.
- [GitHub Actions: setup-go (кэширование)](https://github.com/actions/setup-go).
- [golangci-lint-action](https://github.com/golangci/golangci-lint-action).
- [GOMEMLIMIT](https://pkg.go.dev/runtime#hdr-Environment_Variables) — лимит памяти для GC.
