# Практика. Урок 24: Kubernetes-манифесты и CI/CD

Критерий приёмки — готовый тест `validate/validate_test.go`: он парсит
YAML-файлы из этого каталога и проверяет контракт. Ваша задача — написать
манифесты и пайплайн так, чтобы тест стал зелёным.

## Задание

1. **`deployment.yaml`** — Deployment `orders` (лекция, разделы «Pod/Deployment/
   Service», «Probes», «Resources»): `replicas: 2`; контейнер с
   `livenessProbe` (`httpGet /healthz`, `periodSeconds: 10`), `readinessProbe`
   (`httpGet /readyz`), `startupProbe` (`httpGet /healthz`); `resources` с
   requests и limits; `securityContext` (не root); `containerPort: 8080`.
2. **`service.yaml`** — Service ClusterIP: selector по меткам пода
   (`app: orders`), порт 80 → `targetPort: 8080`.
3. **`configmap.yaml`** — ConfigMap `orders-config` с `data`: `HTTP_ADDR`,
   `DB_DSN` (плейсхолдер; реальные секреты — в Secret, лекция, раздел
   «ConfigMap и Secret»).
4. **`hpa.yaml`** — HPA: `scaleTargetRef` на Deployment `orders`,
   `minReplicas: 2`, `maxReplicas: 10`, метрика Resource/cpu с
   `averageUtilization: 60`.
5. **`.github/workflows/ci.yml`** — GitHub Actions (лекция, раздел «CI/CD для
   Go»): триггеры `push` (main) и `pull_request`; job со шагами
   `setup-go` (go-version `"1.25"`, кэш модулей) → `lint` (golangci-lint) →
   `test` (`go test -race ./...`) → `build` (`CGO_ENABLED=0 go build`) →
   `docker` (build/push, только на main, тег по `GITHUB_SHA`, не `latest`).

Тест ищет шаги lint/test/build/docker по содержимому `name`/`uses`/`run`
шагов — называйте шаги осмысленно. Для проверки YAML локально:

```sh
kubectl apply --dry-run=client -f .        # если есть kubectl
```

## Критерий сдачи

```sh
go mod tidy
go vet ./...
go test ./...
```

Всё зелёное (validate_test.go — критерий). Плюс `go vet` и `go test` без
паник: тест читает файлы из каталога практики, все проверки — ассерты.

## Перенос в капстоун

1. Скопируйте манифесты в `orders/deployments/k8s/` (deployment, service,
   configmap, hpa); поправьте `image` на тег по git SHA и добавьте Secret
   для пароля БД (не коммитьте реальные значения).
2. Перенесите `ci.yml` в `.github/workflows/` репозитория капстоуна;
   поправьте пути сборки (`./cmd/orders`) и имя образа.
3. Добавьте `automaxprocs` в `cmd/orders/main.go` (лекция, раздел «Resources»).
4. Финальная проверка курса: CI зелёный (lint → test -race → build → docker
   push на main), манифесты валидны (`kubectl apply --dry-run=client`),
   deployment поднимается с readiness до того, как получит трафик.
