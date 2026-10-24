# TODO: multi-stage build для сервиса orders (лекция, раздел «Multi-stage build»).
# Контракт, который проверяет validate_test.go:
#   - минимум два FROM (builder: golang:1.25-alpine → final: distroless/static или scratch);
#   - CGO_ENABLED=0 на этапе сборки;
#   - финальный образ gcr.io/distroless/static-debian12 ИЛИ scratch;
#   - запуск не от root (например, :nonroot или USER 65532:65532);
#   - EXPOSE 8080;
#   - ENTRYPOINT — собранный бинарь ./cmd/orders.
