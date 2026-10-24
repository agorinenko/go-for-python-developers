# Практика. Урок 1: инструментарий и идиоматика

В пакете `refactor` лежит код, который **компилируется**, но неидиоматичен и
содержит две скрытые ошибки поведения. Задача — не «написать с нуля», а
привести код к стандарту и попутно разобраться с тулчейном. Сигнатуры функций,
на которые опираются тесты (`FormatReport`, `SumQuantities`, `ProcessItems`),
менять нельзя; внутренности и вспомогательные функции — можно.

## Задание

1. Запустите `go mod tidy && go test ./...` — увидите красные тесты.
2. Исправьте функции в `refactor/cleanup.go` так, чтобы тесты стали зелёными,
   а код — идиоматичным (лекция, разделы «Качество кода» и «Подводные камни»):
   - конкатенация строк в цикле → `strings.Join`;
   - проглоченная ошибка в `ParseQuantity` → возвращать ошибку вызывающему коду;
   - мутация входного слайса → не мутировать входные аргументы;
   - `map[string]interface{}` → типизированный `map[string]float64`;
   - говорящие имена вместо `it`, `current`, `data`.
3. Напишите `Makefile` с целями: `fmt` (`gofmt -w ./...`), `vet` (`go vet ./...`),
   `test` (`go test ./...`), `lint` (`golangci-lint run`).
4. Напишите `.golangci.yml` с линтерами: `errcheck`, `govet`, `staticcheck`,
   `ineffassign`, `unused`, `revive`, `misspell`.

## Критерий сдачи

- `make vet` молчит;
- `make test` зелёный;
- `make lint` молчит (нужен установленный [golangci-lint](https://golangci-lint.run));
- `make fmt` ничего не меняет (код уже отформатирован).

## Перенос в капстоун

Создайте скелет модуля капстоуна по контракту [CAPSTONE.md](../../../CAPSTONE.md):

```sh
mkdir -p orders/cmd/orders orders/internal/domain
cd orders && go mod init orders
```

- перенесите написанные `Makefile` (добавьте цель `build: go build -o bin/orders ./cmd/orders`)
  и `.golangci.yml` в корень капстоуна;
- создайте минимальный `cmd/orders/main.go` (может печатать что угодно);
- добейтесь: `make vet && make lint && make test && make build` — зелёные.
