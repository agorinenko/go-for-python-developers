# Урок 19. Структурное логирование с slog

## Цели урока

- понять, зачем логи структурировать для машинной обработки, а не для человека;
- освоить `slog`: уровни, атрибуты, `With`/`WithGroup`, JSON/TextHandler;
- написать кастомный Handler: маскирование секретов;
- научиться request-scoped логированию и ленивым значениям (`slog.LogValuer`);
- сформулировать, что в логи НЕ пишут никогда.

## Задача из итогового проекта

Неделя 10 итогового проекта — observability: `internal/telemetry/log`. Сервис заказов
уже логирует кое-как (`fmt.Println` и россыпь форматов), и разбирать его логи
в агрегаторе нельзя. Задача: единый структурный формат, request ID во всех
записях запроса и гарантия, что токены и ключи не попадут в хранилище логов.

## Зачем структурные логи

Логи читают три потребителя: человек при разборе инцидента (реже всего),
лог-агрегатор (Loki/ELK/ClickHouse) и алерты (чаще всего). Для машин формат
должен быть **стабильным и типизированным**: `{"time", "level", "msg"}` +
ваши атрибуты. Текстовые логи с вкраплениями переменных — это grep и
регулярки, которые ломаются при первом рефакторинге сообщения.

```go
// Плохо: данные в тексте, машина не разберёт
log.Printf("order %s created for customer %s, total=%d", o.ID, o.CustomerID, o.Total)

// Хорошо: данные — атрибуты, msg — неизменное событие
slog.Info("order created", "order_id", o.ID, "customer_id", o.CustomerID, "total", o.Total)
```

Правило: **msg — константа**, по которой ищется событие; всё переменное —
атрибуты. Тогда «все создания заказов» — это фильтр `msg="order created"`,
а не три regex-паттерна.

## slog: базовый набор

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
logger.Info("order created", "order_id", "o-1", "total", 2250)
logger.Warn("slow request", "duration_ms", 1240)
logger.Error("payment failed", "err", err)  // err — атрибут "err", не в msg
```

- **Уровни**: Debug < Info < Warn < Error. `HandlerOptions.Level` — порог.
- **`With`** — логгер с предварительно прикреплёнными атрибутами (child);
  **`WithGroup`** — атрибуты оборачиваются в объект: `{"http": {"method": ...}}`.
- **Два handler'а из коробки**: `JSONHandler` (для машин) и `TextHandler`
  (для разработки). Один и тот же код, разный формат на выходе.
- **Ключи** атрибутов — snake_case или иной ЕДИНЫЙ стандарт команды; в
  итоговом проекте — snake_case.
- `slog.SetDefault(logger)` — логгер по умолчанию для пакетов, где логгер
  не прокинут явно (стандартная библиотека `http.Server.ErrorLog` и пр.).

Один и тот же вызов в двух форматах:

```go
l := slog.New(slog.NewJSONHandler(os.Stdout, nil))
l.Info("request handled", slog.Group("http",
    slog.String("method", "POST"), slog.Int("status", 201),
    slog.Duration("took", 42*time.Millisecond)))
```

```text
# JSONHandler
{"time":"2026-10-24T10:00:00Z","level":"INFO","msg":"request handled",
 "http":{"method":"POST","status":201,"took":42000000}}
# TextHandler
time=2026-10-24T10:00:00.000Z level=INFO msg="request handled" http.method=POST http.status=201 http.took=42ms
```

`Duration` — типизированный атрибут: JSON даёт наносекунды, Text — «42ms»;
машина и человек видят одно и то же значение без ручного форматирования.

## Request-scoped логгер

Все записи одного запроса должны собираться в агрегаторе по одному ключу.
Логгер с request ID создаётся в middleware и едет вниз по вызовам:

```go
type ctxKey struct{}

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
    return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *slog.Logger {
    if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
        return l
    }
    return slog.Default()
}

// в middleware:
logger := logger.With("request_id", requestID)
ctx = WithLogger(r.Context(), logger)
```

Альтернатива — не задачать логгер по всем слоям, а прокидывать request ID
как обычный контекст и логировать `slog.Default()`; но тогда каждый вызов —
`logger.With("request_id", id)` заново, и легко забыть. Логгер в контексте —
стандарт индустрии, в итоговом проекте так и делаем.

## Кастомный Handler: маскирование секретов

Интерфейс `slog.Handler` — три метода:

```go
type Handler interface {
    Enabled(ctx context.Context, level slog.Level) bool
    Handle(ctx context.Context, r slog.Record) error
    WithAttrs(attrs []slog.Attr) slog.Handler
    WithGroup(name string) slog.Handler
}
```

Обёртка (middleware над handler'ом, как в практике) — канонический способ
менять записи по пути к выводу. Маскирующий handler:

```go
func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
    r2 := slog.NewRecord(r.Time, r.Level, h.redactString(r.Message), r.PC)
    r.Attrs(func(a slog.Attr) bool {
        r2.AddAttrs(slog.Attr{Key: a.Key, Value: h.redactValue(a.Value)})
        return true
    })
    return h.next.Handle(ctx, r2)
}
```

Три ловушки, проверенные в практике:

1. **`r.Clone()` вместо `slog.NewRecord`** — Clone копирует старые атрибуты,
   `AddAttrs` добавит новые рядом: в сыром JSON останутся и секрет, и маска.
2. **Группы** (`slog.KindGroup`) — рекурсия обязательна: секрет на третьем
   уровне вложенности — всё ещё секрет.
3. **`WithAttrs`** — предварительно прикреплённые атрибуты проходят мимо
   `Handle`; маскировать нужно и там, иначе `logger.With("api_key", ...)`
   обойдёт защиту.

То же касается `WithGroup`: обёртка должна пробрасывать группу в `next` и
сохранять собственные секреты — рекурсия в `Handle` накроет вложенность.

## Ленивые значения и производительность

Атрибут вычисляется **до** вызова, даже если уровень отфильтрует запись:

```go
logger.Debug("state", "orders", expensiveSnapshot()) // посчитали и выбросили
```

`expensiveSnapshot` выполнится всегда. Лечение — `slog.LogValuer`:

```go
type lazyOrders struct{ repo OrderRepository }

func (l lazyOrders) LogValue() slog.Value {
    return slog.AnyValue(l.repo.Snapshot()) // вызывается ТОЛЬКО если запись пройдёт
}

logger.Debug("state", "orders", lazyOrders{repo})
```

`LogValuer` вычисляется лениво — на этапе сериализации, уже после фильтра
уровня. Не злоупотребляйте: замыкание с доступом к живым структурам — это
потенциальная гонка, если сериализация случится позже в другой горутине.

Если уровней недостаточно и Debug нужен в проде — `Enabled` перед дорогим
кодом:

```go
if logger.Enabled(ctx, slog.LevelDebug) {
    logger.Debug("state", "orders", snapshot)
}
```

## Что НЕ логировать никогда

- **Пароли, токены, ключи API, номера карт, cookies** — ни в атрибутах, ни в
  сообщениях, ни в ошибках (`err` может содержать DSN с паролем!).
  Маскирование — последняя линия обороны, а не разрешение писать секреты.
- **PII** сверх операционной необходимости: email-ы и телефоны — под
  минимизацию; лог-хранилище — это ещё одна база персональных данных со
  своими сроками хранения и аудитом доступа.
- **Тела запросов/ответов целиком** — по умолчанию нет; для отладки —
  отдельный механизм с ограничением размера и ротации.
- **Секреты в `With`** — атрибут из `With` размножается на все дочерние
  записи; если это токен — он теперь в каждой строке.

Отдельно про **ошибки как атрибут**: `slog.Error("save failed", "err", err)`
пишет сообщение ошибки. Обёртки из `fmt.Errorf("%w")` в него входят — по
логам видна вся цепочка контекста. Не пишите ошибку в `msg` (теряется
структура) и не пишите в атрибуты `err.Error()` вручную (теряется тип и
`errors.Is`-цепочка — агрегатор не сможет сгруппировать ошибки по классу).

## Go vs Python

**Python stdlib `logging`** — разброс форматов: `%`-style, `f-string`,
`extra=`; структура держится на дисциплине. Классика:

```python
logger.info("order %s created", order.id)              # %-style, лениво
logger.info(f"order {order.id} created")               # f-string: НЕ лениво, eval всегда
logger.info("order created", extra={"order_id": order.id})  # структура через extra
```

`f-string` вычисляется до фильтра уровня — то же самое, от чего в Go спасает
`LogValuer`. **`structlog`** — фактический стандарт структурного логирования
в Python: связывание контекста (`bind`), процессоры, рендереры в JSON —
концептуально тот же `slog` с `With` и handler'ами, только библиотекой.
**C#**: `ILogger` + `Serilog` — шаблоны сообщений (`"Order {OrderId}
created"`) вместо `extra`; структурные свойства из шаблона — тот же принцип
«msg константа, данные — поля».

Главное отличие Go: `slog` — в **стандартной библиотеке**, с единым
интерфейсом `Handler`, который встраивают в свои конвейеры (маскирование,
экспорт в OTel) без смены API логирования.

## Подводные камни и анти-паттерны

1. **Данные в msg** — «`order o-1 for customer c-1 created`»: фильтры и
   алерты по атрибутам невозможны, а каждое изменение текста ломает дашборды.
2. **`f-string`/дорогие вычисления до фильтра уровня** — Python-привычка,
   переносимая в Go через немедленные аргументы; лечится `LogValuer`/`Enabled`.
3. **`With` на каждый вызов** — `logger.With(...)` создаёт child-логгер;
   в цикле — мусор. Один раз на запрос/горутину.
4. **`Clone` в обёртках handler'а** — дубликаты атрибутов и утечка
   оригиналов (см. выше).
5. **Логировать ошибку и возвращать её же** — двойной шум: `if err != nil
   { logger.Error(...); return err }` на каждом слое. Логируйте ошибку **один
   раз** — на границе, где она обрабатывается (обычно middleware), с полным
   контекстом из обёрток.
6. **Panic-логирование без stack** — `recover` в middleware (неделя 5)
   должен писать stack trace, иначе падение неотлаживаемо.
7. **Смена формата на полпути** — TextHandler в проде «временно»; формат —
   контракт с агрегатором, меняется только осознанно.
8. **Секреты в `err`** — ошибка от драйвера БД может содержать DSN; маска
   обрабатывает и атрибут `err`, если секрет передан в списке.

## Что делаем в итоговом проекте

1. `internal/telemetry/log`: `NewLogger` (JSON), `WithRequestID`,
   `RedactHandler` со списком секретов из конфига.
2. Middleware недели 5: request ID → `WithRequestID` → логгер в `context`;
   recovery пишет stack trace.
3. Все слои логируют через `FromContext(ctx)`; msg — константы, данные —
   атрибуты (`order_id`, `customer_id`, `total`, `request_id`).
4. Проверка: агрегатор видит поля, секреты замаскированы, по `request_id`
   собирается вся цепочка запроса. Следующий урок свяжет логи с трейсами.

## Материалы для углубления

- [log/slog](https://pkg.go.dev/log/slog) — документация пакета.
- [Structured Logging with slog](https://go.dev/blog/slog) — официальный вводный материал.
- [slog: Handler interface](https://pkg.go.dev/log/slog#Handler) — контракт кастомных handler'ов.
- [12-Factor App: Logs](https://12factor.net/logs) — логи как поток событий.
- [structlog](https://www.structlog.org/en/stable/) — аналог в Python для сравнения.

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
