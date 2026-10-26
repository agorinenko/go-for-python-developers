# Урок 4. Тестирование в Go как в промышленности

## Цели урока

- освоить `go test`: подтесты, `-v`, `-run`, `-cover`, table-driven как стандарт;
- понять разницу фейков, стабов и моков — и когда что применять;
- научиться golden files (`testdata/`, флаг `-update`) и AAA-структуре тестов;
- увидеть место testify (`assert`/`require`) в индустриальном Go;
- относиться к покрытию как к инструменту, а не к цели.

## Задача из итогового проекта

`OrderService` написан на прошлом уроке поверх интерфейсов — теперь он должен
быть **покрыт тестами как продакшен-код**: полное покрытие веток, фейковый
репозиторий, читаемые тесты-документацию. Практика этого урока необычная:
пакет `discount` дан готовым, а пишете вы **тесты** — как при вливании в
проект, где код уже есть, а тестов нет.

## go test: инструмент, а не фреймворк

В stdlib — полноценный тестовый раннер, без внешних зависимостей:

```go
func TestCreateOrder(t *testing.T) {
    cases := []struct {
        name string
        customerID string
        wantErr error
    }{
        {"пустой клиент", "", domain.ErrEmptyCustomer},
        {"нет позиций", "c-1", domain.ErrNoItems},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {        // подтест
            _, err := svc.CreateOrder(context.Background(), tc.customerID, validItems())
            if !errors.Is(err, tc.wantErr) {
                t.Fatalf("CreateOrder error = %v, want errors.Is %v", err, tc.wantErr)
            }
        })
    }
}
```

Table-driven — не «паттерн», а культурная норма: таблица кейсов + один
`t.Run` на строку. Каждый подтест изолирован по `Fatal` (падает только его
поддерево) и адресуется через `-run`:

```sh
go test -run 'TestCreateOrder/пустой_клиент' -v   # один подтест
go test -count=10 ./...                            # прогон 10 раз (флаки)
go test -cover ./...                               # % покрытия
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
```

`t.Fatal`/`t.FailNow` обязаны вызываться **из горутины самого теста** —
внутри `go func()` допустимы только `t.Error`/`t.Errorf` (иначе паника
`FailNow called from non-test goroutine`).

## testify: assert и require

`github.com/stretchr/testify` — де-факто стандарт ассертов в индустрии.
Два семейства, разница в одном: **что делать после провала**:

```go
got, err := svc.GetOrder(ctx, id)
require.NoError(t, err)         // при ошибке тест ОСТАНОВИТСЯ здесь
assert.Equal(t, "o-1", got.ID)  // при несовпадении — продолжит, отметит провал

require.ErrorIs(t, err, domain.ErrOrderNotFound) // errors.Is как ассерт
require.Len(t, orders, 3)
```

Правило: `require` — для предусловий, без которых дальнейший код не имеет
смысла (иначе nil-pointer в тесте и невнятный стек); `assert` — для проверок,
которые можно накапливать. Чем раньше `require` обрывает тест, тем короче
диагностика. `assert`/`require` принимают сообщение последним аргументом и
сами форматируют diff — руками `t.Errorf("got %v want %v")` писать не нужно.

## Фейки, стабы, моки: терминология

Индустрия (и Фаулеровская статья Mocks Aren't Stubs) различает три подмены:

- **Fake** — рабочая реализация с коротким путём: репозиторий в памяти
  (`map[string]*Order`), который действительно хранит и фильтрует. Пишется
  один раз, живёт в тестах, не требует библиотек. Именно его мы используем
  для `OrderService`.
- **Stub** — заранее заготовленные ответы: `fakeValidator{reject: {...}}`
  возвращает заданную ошибку на заданный вход. Проверяется не «как вызвали»,
  а «что вернули при таком ответе».
- **Mock** — запись вызовов и проверка **взаимодействия**: «UpdateStatus
  вызван ровно один раз с такими аргументами». В Go это обычно ручной
  «spy» (`f.updates = append(...)`), а не генераторы вроде mockery/gomock —
  они нужны, когда контракт большой и его дорого писать руками.

Иерархия предпочтений: fake > stub > mock. Проверять взаимодействие стоит,
только когда это суть контракта (статус сохранён через UpdateStatus), иначе
тесты прибиваются к реализации и ломаются при любом рефакторинге.

## Golden files

Для детерминированного текстового вывода (отчёты, сериализация, CLI) тест
сравнивает вывод с эталонным файлом в `testdata/`:

```go
var update = flag.Bool("update", false, "перезаписать golden-файлы")

func TestFormatBill(t *testing.T) {
    got := discount.FormatBill(bill)
    golden := filepath.Join("testdata", "TestFormatBill.golden")
    if *update {
        err := os.MkdirAll("testdata", 0o755)
        require.NoError(t, err)
        require.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
        return
    }
    want, err := os.ReadFile(golden)
    require.NoError(t, err)
    assert.Equal(t, string(want), got)
}
```

```sh
go test ./discount -run TestFormatBill -update   # обновить эталоны
```

Соглашения:

- каталог `testdata/` Go-инструменты игнорируют целиком (`go build ./...` его
  не трогает) — это его официальное назначение;
- эталон — **код**: изменения вывода ревьюятся в диффе, `-update` не должен
  стать способом «починить» красный тест без понимания, что изменилось;
- флаг `-update` объявляется в тесте через `flag.Bool` — и ставится **после**
  имени пакета: `go test ./pkg -run X -update` (иначе `go test` сочтёт его
  своим флагом и не передаст тесту).

## Тесты как документация: AAA

Тест читается чаще, чем выполняется, поэтому структура «Arrange / Act /
Assert» — с комментариями-маркерами в нетривиальных тестах:

```go
func TestCancelOrder(t *testing.T) {
    // Arrange
    repo := newFakeRepo()
    svc := newService(repo, nil)
    o, err := svc.CreateOrder(context.Background(), "c-1", validItems())
    require.NoError(t, err)

    // Act
    err = svc.CancelOrder(context.Background(), o.ID)
    require.NoError(t, err)

    // Assert
    got, err := svc.GetOrder(context.Background(), o.ID)
    require.NoError(t, err)
    assert.Equal(t, domain.StatusCancelled, got.Status)
}
```

Имена тестов — английские (инструменты и соглашения), но сценарии в
подтестах можно описывать по-русски, если команда русскоязычная. Хорошее имя
теста отвечает «что проверяем и при каком условии»: `TestCancelOrder`,
`TestGetOrder/неизвестный ID`. Тест-кейсы — это и есть живая спецификация
сервиса: новый разработчик читает `service_test.go` раньше, чем `service.go`.

## Покрытие: инструмент, а не цель

`go test -cover` показывает процент покрытых **выражений** (не веток!).
Смысл метрики не в «100%», а в карте непокрытого кода: `go tool cover -html`
подсвечивает, какие ветки не проверены, — это вопросы к качеству тестов, а
не цифры для KPI. Гонка за процентами порождает тесты-пустышки (вызвали
функцию, ничего не проверили) и подталкивает к правке кода ради метрики.

Здоровый минимум для сервисного слоя — 80–90% с акцентом на **сложные ветки**
(машина статусов, фильтры), а не на геттеры. Покрытие считается по пакету:
`go test -cover ./...` в CI, порог — на ваше усмотрение (не встраивается в
`go test` штатно).

## Go vs Python: pytest

```python
@pytest.mark.parametrize("customer_id,items,want_err", [
    ("", items(), ErrEmptyCustomer),
    ("c-1", [], ErrNoItems),
])
def test_create_order_errors(customer_id, items, want_err):
    with pytest.raises(...):
        service.create_order(customer_id, items)
```

```go
cases := []struct{ customerID string; wantErr error }{...}
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) { ... })
}
```

Что где сильнее:

| Аспект | pytest | Go |
|---|---|---|
| Параметризация | `@pytest.mark.parametrize` | table-driven вручную (тело цикла) |
| Фикстуры | fixtures с scope, autouse, yield | нет; общие хелперы — обычные функции (`newFakeRepo`) |
| Временные файлы | `tmp_path` | `t.TempDir()` |
| Ассерты | встроенные `assert`, подробные repr | testify assert/require, diff структур |
| Изоляция | процесс на тест-функцию | общий процесс, изоляция на подтест |
| Параллелизм | pytest-xdist (процессы) | `t.Parallel()` + `-race` в одном процессе |

Python-фикстуры с автоочисткой — то, чего в Go нет как механизма: их роль
играют хелперы (`newService`, `validItems`) и `t.Cleanup`. Зато Go даёт то,
чего нет у pytest: тесты — обычный код в том же пакете (доступ к
неэкспортируемому), статическая типизация кейсов и **race detector** в том же
запуске (`go test -race`). Аналог `pytest.raises` в Go не нужен — ошибки
возвращаются значениями: `require.ErrorIs(t, err, ErrX)`.

**C#:** NUnit/xUnit с теориями (`[Theory] [InlineData]`) — ближе к
table-driven; `Assert.Throws<T>` ↔ проверка `errors.Is`; Moq ↔ ручные
fakes/spies.

## Подводные камни и анти-паттерны

1. **`t.Fatal` из горутины** — паника. В `go func` только `t.Error`.
2. **Хрупкие тайминги.** `time.Sleep` в тесте, чтобы «дождаться» — флак.
   Договариваться нужно каналами/контекстами; таймер допустим только как
   guard «не завис» с большим запасом.
3. **Тесты, зависящие от порядка карты.** Итерация `map` случайна — сравнение
   результатов по спискам даст флаки. Сравнивайте множества (по ID) или
   сортируйте.
4. **Golden без `-update` в CI и `-update` «чтобы зазеленело»** — эталон
   становится бессмысленным. Обновление — осознанное действие с ревью диффа.
5. **`assert.Equal` на ошибках вместо `ErrorIs`** — тест сломается при первом
   `%w`-оборачивании.
6. **Тест мутирует общий объект.** `t.Parallel()` + общая переменная — гонка;
   кейсы должны владеть своими данными (копии слайсов/структур).
7. **Покрытие ради покрытия** — тесты без ассертов и «вызвать функцию, чтобы
   строка стала зелёной». Покрытие — карта слепых зон, не метрика качества.
8. **Моки, проверяющие каждое движение** — тест прибит к реализации; рефактор
   ломает всё. Проверяйте поведение, взаимодействие — только по контракту.

## Что делаем в итоговом проекте

1. Подключаем `testify` в модуль `orders`.
2. Переписываем тесты `OrderService` (урок 3) в стиле практики: table-driven,
   AAA, `require`/`assert`, `require.ErrorIs`.
3. Доводим покрытие `internal/service` до ≥ 90%: кейсы пустого клиента,
   пустых позиций, лимита в `ListOrders`, повторной отмены.

## Материалы для изучения

- [testing package](https://pkg.go.dev/testing) — t.Run, t.Parallel, t.Cleanup, T.TempDir.
- [Add a test (go.dev tutorial)](https://go.dev/doc/tutorial/add-a-test).
- [Table driven tests (wiki)](https://go.dev/wiki/TableDrivenTests).
- [testify documentation](https://pkg.go.dev/github.com/stretchr/testify) — assert/require.
- [Martin Fowler: Mocks Aren't Stubs](https://martinfowler.com/articles/mocksArentStubs.html).
- [Coverage profiling](https://go.dev/blog/cover) — coverprofile, html-отчёты.

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
