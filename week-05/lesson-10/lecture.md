# Урок 10. API-контракты: JSON, DTO, валидация, ошибки

## Цели урока

- освоить `encoding/json` на уровне продакшена: теги, потоковый декодинг,
  `DisallowUnknownFields`, кастомная (де)сериализация;
- понять, зачем и когда разводить DTO и доменные типы, как их маппить;
- построить валидацию входящих данных со внятными ошибками;
- ввести единую схему ошибок API и маппинг доменных ошибок в HTTP-статусы.

## Задача из итогового проекта

Контракт «Неделя 5» (CAPSTONE.md): `POST /v1/orders` → `201` (или `400`/`409`),
`GET /v1/orders/{id}` → `200`/`404`, `GET /v1/orders` → `200`, ошибки —
`{"error": {"code": "...", "message": "..."}}`. Хэндлеры-заглушки урока 9
получают настоящие тела: декодирование, валидацию, DTO и маппинг ошибок.

## encoding/json: не только Unmarshal

### Структурные теги

```go
type Item struct {
    ProductID string `json:"product_id"`
    Name      string `json:"name,omitempty"`
    Quantity  int    `json:"quantity"`
    Price     int64  `json:"price"` // в копейках
}
```

Теги управляют именем и присутствием поля. `omitempty` пропускает zero
value: пустую строку, `0`, `false`, nil-слайс/карту/указатель. Две
ловушки: `omitempty` **не срабатывает** для структуры (`time.Time` всегда
записывается) и не отличает «поле не прислали» от «прислали null» — для
этого нужны указатели: `CreatedAt *time.Time` — nil означает «не задано»,
а не «нулевая дата».

### Потоковый декодинг: Decoder вместо Unmarshal

`json.Unmarshal(data []byte, &v)` читает всё в память. Для HTTP-тел
идиоматичнее потоковый `json.NewDecoder(r.Body).Decode(&v)` — читает по
мере разбора. Плюс `Decoder` умеет то, чего не умеет `Unmarshal`:

```go
dec := json.NewDecoder(r.Body)
dec.DisallowUnknownFields() // неизвестные поля — ошибка
if err := dec.Decode(&req); err != nil { ... }
// тело обязано содержать ровно одно JSON-значение:
if err := dec.Decode(&struct{}{}); err != io.EOF { ... }
```

`DisallowUnknownFields` — защита от опечаток клиента: `{"costomer_id": ...}`
молча проигнорируется без неё, и заказ создастся с пустым customer. Вторая
проверка ловит мусор после JSON (склейка двух документов, классика
smuggling-попыток).

### Кастомные Marshaler/Unmarshaler

Интерфейсы `Marshaler` (`MarshalJSON() ([]byte, error)`) и `Unmarshaler`
(`UnmarshalJSON([]byte) error`) дают полный контроль. Типичный кейс —
строковый enum с проверкой на входе:

```go
func (s *OrderStatus) UnmarshalJSON(b []byte) error {
    var raw string
    if err := json.Unmarshal(b, &raw); err != nil { return err }
    switch raw {
    case "new", "validated", "paid", "cancelled":
        *s = OrderStatus(raw)
        return nil
    default:
        return fmt.Errorf("unknown status %q", raw)
    }
}
```

Клиент прислал `"stauts": "nwe"` — получил 400, а не молчаливый нулевой
статус, который всплывёт через неделю в отчётах. Ещё стандартные кейсы:
`time.Duration` как `"5s"`, `null`-чувствительные обёртки, bigint в строку.

### Числа и прочее

`Decoder.UseNumber()` отдаёт числа как `json.Number` — защита от потери
точности `float64` для int64. Деньги у нас уже `int64` копеек (урок 2),
но при разборе чужих JSON с «деньгами-строками» помните: Go json **не**
конвертирует `"100"` в число — это отдельная ошибка, и это хорошо.

## DTO против доменных типов

Соблазн — отдавать клиенту `domain.Order` напрямую с json-тегами.
Правильный ответ: **нельзя**, и вот почему.

1. **Контракт ≠ модель.** Доменный тип живёт по своим правилам (инварианты,
   статус-машина, приватность полей); контракт API эволюционирует по своим
   (`/v1` и `/v2` могут существовать одновременно). Связывать их — значит
   запретить эволюцию одного без ломки другого.
2. **Утечка деталей.** Всё, что попало в доменную структуру, рано или
   поздно утечёт в ответ (забыли `json:"-"`). DTO — это явный список того,
   что клиент видит.
3. **Разные жизненные циклы.** Домену не нужны теги, DTO не нужна логика.
   Поля `ID`/`CreatedAt` проставляет сервис — в запросе их нет вообще:
   отдельный `CreateOrderRequest` не даст клиенту их «прислать».

Когда разводить не обязательно: маленький внутренний сервис, контракт и
модель совпадают один в один, версия API одна. Правило большого пальца:
**граница системы = DTO**. Внутри — доменные типы, наружу — только DTO.

Маппинг — скучные функции перекладывания полей:

```go
func (req CreateOrderRequest) ToDomain() (domain.Order, error) { ... }
func EncodeOrder(w io.Writer, o *domain.Order) error { ... } // DTO + json
```

Никаких «магических» библиотек (mapstructure и т.п.): рефлексивный маппинг
экономит строки и прячет ошибки. Явный маппинг — это место, где компилятор
ловит изменения модели.

## Валидация входящих

Порядок всегда один: **синтаксис → семантика**. Сначала JSON разобрался
(декодер вернул ошибку — это plain error), потом проверяются правила
предметной области, и их ошибки — уже с доменной семантикой.

**Ручная валидация** (в практике — она):

```go
if req.CustomerID == "" {
    return req, fmt.Errorf("%w: customer_id is required", domain.ErrEmptyCustomer)
}
for i, it := range req.Items {
    if it.Quantity <= 0 {
        return req, fmt.Errorf("%w: items[%d]: quantity must be > 0",
            domain.ErrInvalidItem, i)
    }
}
```

Два принципа: (1) сообщение называет поле и правило, (2) ошибка обёрнута
в доменный sentinel через `%w` — тогда маппинг в 400 делает один код
(`errors.Is`), а не куча строковых сравнений. Для нашего контракта ручной
валидации достаточно: три правила на структуру.

**Когда нужен go-playground/validator**: десятки DTO, правила-теги
(`validate:"required,min=1"`), вложенные структуры, локализация сообщений.
Плата: рефлексия в среде выполнения (теги не проверяются компилятором — опечатка
`minn=1` молча не работает), зависимость, порядок ошибок неочевиден.
Решение: на старте — ручная валидация, validator — когда боль от
дублирования превысит боль от рефлексии.

## Единая схема ошибок

Контракт итогового проекта: `{"error": {"code": "...", "message": "..."}}`.
Клиент разбирает ошибку одним кодом; `code` — машинный (switch у клиента),
`message` — человеческий (показывается пользователю).

```go
func WriteError(w http.ResponseWriter, err error) {
    status, code, msg := http.StatusInternalServerError, "internal", "internal error"
    switch {
    case errors.Is(err, domain.ErrOrderNotFound):
        status, code, msg = http.StatusNotFound, "not_found", err.Error()
    case errors.Is(err, domain.ErrInvalidItem),
        errors.Is(err, domain.ErrEmptyCustomer),
        errors.Is(err, domain.ErrNoItems):
        status, code, msg = http.StatusBadRequest, "bad_request", err.Error()
    case errors.Is(err, domain.ErrInvalidTransition):
        status, code, msg = http.StatusConflict, "conflict", err.Error()
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(ErrorResponse{Error: ErrorBody{Code: code, Message: msg}})
}
```

Ключевые решения:

- **4xx — детали наружу.** Клиент должен понять, что поправить:
  `"items[2]: quantity must be > 0"` — это UX.
- **5xx — только generic.** В `err.Error()` может быть всё: адрес БД,
  пароль, внутренние имена. Отдаём `internal error`, детали — в лог
  (middleware Logging с уроком 9 дают id запроса для сопоставления).
- **`errors.Is`, не строки.** Обёртки `%w` проходят сквозь цепочку; `==`
  сломается на первом `fmt.Errorf("...: %w", err)`.
- `WriteHeader` строго до записи тела — после первого байта статус не
  поменять.

Версионирование: `/v1` в пути — дешёвая страховка. Правила: никогда не
менять смысл существующих полей, только добавлять новые (add-only);
ломка контракта — только вместе с новой мажорной версией пути.

## Go vs Python: pydantic

```python
# Python: модель = и DTO, и валидатор, и сериализатор
class CreateOrderRequest(BaseModel):
    customer_id: str
    items: list[Item] = Field(min_length=1)

    @field_validator("customer_id")
    def non_empty(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("customer_id is required")
        return v
```

Pydantic за один `BaseModel` делает валидацию, парсинг, сериализацию и
JSON Schema. В Go эти обязанности разложены: структура с тегами (парсинг
+ сериализация), отдельная функция (валидация), отдельная функция
(маппинг). Сравнение честное в обе стороны:

- Pydantic сильнее на **алгебре типов**: `Union`, `Literal`, discriminated
  unions — один компактный тип выражает варианты, которые в Go
  превращаются в явные интерфейсы или несколько структур плюс свитч.
- Pydantic выигрывает на скорости прототипа; Go выигрывает на гарантиях:
  теги и маппинг проверяются компилятором, невалидное состояние DTO в Go
  часто физически невыразимо, в Python — только через runtime-валидацию.
- Ошибки: pydantic's `ValidationError` уже структурирован (список ошибок
  с путями) — Go эту структуру вы строите сами (отсюда `items[2]: ...`
  в сообщениях).
- Производительность и memory: pydantic v2 на Rust быстр, но `BaseModel`
  остаётся Python-объектом; Go-структура — компактная память без
  боксинга, сериализация без аллокаций-каскадов.

**FastAPI** собирает из pydantic-моделей ещё и OpenAPI-схему; в Go эта
генерация — отдельный шаг (openapi-генераторы из комментариев, или proto
как источник истины — неделя 7). **C#**: System.Text.Json с source
generators близок к Go-подходу (компилируемые сериализаторы), а
DataAnnotations — аналог validator-тегов с теми же плюсами/минусами.

## Подводные камни и анти-паттерны

1. **`omitempty` на структуре.** `CreatedAt time.Time` с `omitempty`
   пишется всегда. Для «не задано» — указатель или кастомный тип.
2. **Чтение `r.Body` дважды.** Тело — поток, второй `Decode` вернёт EOF.
   Если тело нужно нескольким потребителям — читайте в `[]byte` и
   разбирайте `json.Unmarshal`.
3. **Без `DisallowUnknownFields`.** Молчаливое игнорирование чужих полей —
   источник «у меня не работает, хотя поле я послал».
4. **Числа в float64.** `json.Unmarshal` по умолчанию кладёт числа в
   `float64`; большие int64 теряют точность. Держите деньги в `int64` и
   не маршрутизируйте их через `interface{}`.
5. **Ошибки-строки в маппинге статусов.** `switch err.Error() { case
   "orders: order not found" ... }` — ломается при первом `%w`. Только
   `errors.Is`.
6. **Детали в 5xx.** `http.Error(w, err.Error(), 500)` утекает наружу
   внутренности. Generic-сообщение + полный лог на сервере.
7. **Content-Type забыт.** JSON в ответе без `application/json` — клиенты
   сломаются тихо.
8. **Encoder и HTML.** `json.NewEncoder` экранирует `<`, `>`, `&` в
   `\u003c` и т.п. Для JSON API это обычно лишний шум:
   `enc.SetEscapeHTML(false)` — решение осознанное, не забытое.
9. **Статус после записи.** `WriteHeader`/`Header().Set` после первого
   `Write` — молчаливый no-op или мусор в ответе. Считайте всё до записи.
10. **Валидация только на клиенте.** Сервер обязан валидировать сам:
    клиент бывает любой (curl, чужая интеграция).

## Что делаем в итоговом проекте

1. `orders/internal/httpapi/api`: DTO (`CreateOrderRequest`,
   `OrderResponse`, `ErrorResponse`), `DecodeCreateOrder`, `ToDomain`,
   `EncodeOrder`, `WriteError` — из практики, импорт домена
   `orders/internal/domain`.
2. Хэндлеры урока 9 переписываются на эти функции: `POST` создаёт заказ
   через сервис и отвечает `201`; `GET` мапит `ErrOrderNotFound` в `404`;
   ошибки — только через `WriteError`.
3. Проверка контракта: `curl` с валидным/невалидным телом — `201`/`400`
   и схема ошибок; тесты `httptest` на каждый код ответа.

## Материалы для изучения

- [encoding/json](https://pkg.go.dev/encoding/json) — теги, Marshaler, Decoder.
- [JSON and Go](https://go.dev/blog/json) — официальный блог.
- [pydantic](https://docs.pydantic.dev/) — модели, валидация, error structure.
- [go-playground/validator](https://github.com/go-playground/validator) — когда теговая валидация оправдана.
- [RFC 7807 Problem Details](https://www.rfc-editor.org/rfc/rfc7807) — альтернативная схема ошибок (стандарт).

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
