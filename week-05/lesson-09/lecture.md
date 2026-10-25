# Урок 9. HTTP-сервер на Go: net/http, chi, middleware, graceful shutdown

## Цели урока

- собрать production-grade HTTP-сервер на `net/http`: таймауты, роутер, middleware;
- понять, почему конфигурация `http.Server` по умолчанию опасна и что настраивать обязательно;
- освоить маршрутизацию с параметрами (chi) и цепочки middleware;
- реализовать graceful shutdown без потери in-flight запросов;
- тестировать HTTP-слой через `httptest`.

## Задача из итогового проекта

По контракту [CAPSTONE.md](../../CAPSTONE.md) (неделя 5) сервис `orders`
обретает транспортный слой: `internal/httpapi` с маршрутами `POST /v1/orders`,
`GET /v1/orders/{id}`, `GET /v1/orders` и точку входа `cmd/orders`. Сегодня
мы делаем скелет: роутер, middleware и корректный запуск/остановку сервера.
JSON-контракты и коды ошибок — следующий урок.

## net/http: сервер одним экраном

В Go HTTP-сервер — часть стандартной библиотеки, и он годится для продакшена:

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /v1/orders", createOrder)

srv := &http.Server{
    Addr:              ":8080",
    Handler:           mux,
    ReadHeaderTimeout: 5 * time.Second,
}

if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    log.Fatal(err)
}
```

Ключевая абстракция — интерфейс из одного метода:

```go
type Handler interface {
    ServeHTTP(http.ResponseWriter, *http.Request)
}
```

Всё, что умеет отвечать на запрос, — `Handler`. Адаптер `http.HandlerFunc`
превращает обычную функцию в обработчик: `http.HandlerFunc(fn)` — это
функциональный тип с методом `ServeHTTP`. Отсюда две идиомы:

```go
// 1) функция напрямую
mux.HandleFunc("GET /v1/orders/{id}", getOrder)

// 2) тип с состоянием (зависимостями)
type api struct { svc OrderService }
func (a *api) getOrder(w http.ResponseWriter, r *http.Request) { ... }
```

Второй вариант — основной для сервисов: обработчики держат зависимости
(сервисный слой, клиенты, конфиг), а не берут их из глобальных переменных.

Каждое соединение обслуживается **отдельной горутиной** — net/http сам
поднимает goroutine per connection. Модель конкурентности встроена:
обработчик пишется как последовательный код, параллелизм обеспечивает сервер.
Ваше дело — не разделять состояние между запросами без синхронизации.

## Конфигурация http.Server: значения по умолчанию опасны

`&http.Server{Addr: ..., Handler: ...}` **собирается и работает**. Проблема:
все таймауты равны нулю, то есть «бесконечность». Продакшен-сервер с такой
конфигурацией убивается одной классической атакой.

**Slowloris.** Атакующий открывает сотни соединений и шлёт по байту в
несколько секунд. Каждое соединение держит горутину и буфер; встроенный
лимит соединений отсутствует. Без `ReadHeaderTimeout` сервер молча копит
полуоткрытые соединения, пока не кончатся память или файловые дескрипторы.

Четыре таймаута, которые нужно осознанно выбрать:

| Поле | Что ограничивает | Разумный значение по умолчанию |
|---|---|---|
| `ReadHeaderTimeout` | чтение **заголовков** запроса | 5s |
| `ReadTimeout` | чтение всего запроса (включая тело) | 15s |
| `WriteTimeout` | запись ответа | 30s |
| `IdleTimeout` | keep-alive соединение без запросов | 60s |

Важные детали:

- `ReadHeaderTimeout` — минимально необходимый минимум против Slowloris;
  ставьте его всегда, даже если всё остальное не трогаете.
- `ReadTimeout` покрывает чтение тела **после** заголовков. Медленный
  аплоад большого файла с 15s-лимитом оборвётся. Если у вас большие тела —
  ставьте лимит на размер, а не на время, или считайте таймаут от
  последнего прочитанного байта.
- `WriteTimeout` с 0 = «записывай сколько угодно». Для server-sent events
  и стримов это осознанное решение, но тогда придётся управлять
  соединением самому (`http.ResponseController` в Go 1.20+).
- `IdleTimeout` должен быть **меньше** таймаута балансировщика/прокси:
  если LB рвёт idle-соединение раньше сервера, получите `connection reset`
  в логи — безобидно, но шумно.

Ещё два поля, которые стоит знать: `MaxHeaderBytes` (лимит размера
заголовков, значение по умолчанию 1MB) и `BaseContext` (базовый контекст для запросов —
место, куда кладут общий `slog.Logger` и трейс-провайдер).

## Роутинг: ServeMux или chi

**Стандартный `http.ServeMux`** с Go 1.22 умеет методы и wildcard-параметры:

```go
mux.HandleFunc("POST /v1/orders", createOrder)
mux.HandleFunc("GET /v1/orders/{id}", getOrder)      // r.PathValue("id")
mux.HandleFunc("GET /v1/orders", listOrders)
```

Для небольшого API этого достаточно. Чего в ServeMux нет: middleware
(стандартных обёрток нет — только свой код), группировки маршрутов,
частичных wildcard'ов вида `/files/{path...}` (появились в 1.22, но
ограниченно), reverse routing.

**chi** — самый популярный роутер, «идиоматичный поверх net/http»:
`chi.Router` — это `http.Handler`, совместимый со всей экосистемой.

```go
r := chi.NewRouter()
r.Route("/v1/orders", func(r chi.Router) {
    r.Post("/", createOrder)        // POST /v1/orders
    r.Get("/", listOrders)          // GET  /v1/orders
    r.Get("/{id}", getOrder)        // GET  /v1/orders/{id}
})

id := chi.URLParam(r, "id") // параметр из URL
```

Маршруты с параметрами: `{id}` — сегмент, `{id:regexp}` — с валидацией
по регулярному выражению (например `{id:[a-z0-9-]+}`), `{path...}` — остаток
пути. Группировка через `Route` позволяет повесить middleware на группу:

```go
r.Route("/v1/orders", func(r chi.Router) {
    r.Use(RequireAuth) // только для этой группы
    r.Post("/", createOrder)
})
```

Поведение при ошибке маршрутизации: неизвестный путь — `404`, известный
путь с неверным методом — `405` с заголовком `Allow` (chi делает это сам,
писать код не нужно — тесты практики это проверяют).

**Почему не gorilla/mux**: проект заархивирован. Почему не gin/echo:
фреймворки со своим контекстом, своей системой middleware и своим способом
держать зависимости — всё это несовместимо с `http.Handler`, а мы хотим,
чтобы любой код (`httptest`, линтеры, инструменты) работал без адаптеров.

## Middleware: цепочки и порядок

Middleware — функция, которая оборачивает `http.Handler`:

```go
type Middleware func(http.Handler) http.Handler

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" { id = newID() }
        w.Header().Set("X-Request-ID", id)
        next.ServeHTTP(w, r)
    })
}
```

Обёртывание применяется снаружи внутрь: `chain := Wrap(h, A, B)` означает
`A(B(h))` — код A выполняется до B, и порядок **влияет на наблюдаемое
поведение**:

```go
func Wrap(h http.Handler, mws ...Middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- {
        h = mws[i](h)
    }
    return h
}
```

Три обязательных middleware любого сервиса:

**Request ID.** Идентификатор запроса, сквозной между сервисами. Если
клиент прислал свой — используем его (доверие к границе, не к каждому
прыжку); если нет — генерируем. Генерация: `crypto/rand` → 16 байт → hex
(32 символа), либо `github.com/google/uuid`. Заголовок проставляется **до**
вызова `next` — поэтому он попадает в ответ, даже если обработчик паникует.
Это и есть тест на правильный порядок: `RequestID` обязан стоять **снаружи**
`Recover`.

**Recovery.** net/http по умолчанию при панике в обработчике ловит её сам,
логирует и **рвёт соединение без ответа** — клиент получает не 500, а
обрыв. Восстанавливаем на границе:

```go
func Recover(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                slog.ErrorContext(r.Context(), "panic recovered", "panic", rec)
                http.Error(w, "internal error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

Это тот самый случай «recover только на границе» из урока 2: паника —
не механизм ошибок, но изолировать её нужно на внешнем слое. Нюанс: если
обработчик уже начал писать ответ (заголовки ушли), `http.Error` добавит
мусор в тело; продвинутый вариант проверяет через
`http.ResponseController`/`Unwrap() http.Flusher`, начался ли ответ.

**Logging.** Статус и длительность известны только после выполнения
обработчика, а `http.ResponseWriter` не отдаёт записанный статус. Решение —
обёртка:

```go
type statusWriter struct {
    http.ResponseWriter
    status int
}

func (w *statusWriter) WriteHeader(status int) {
    if w.status == 0 { w.status = status } // запоминаем только первый
    w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
    if w.status == 0 { w.status = http.StatusOK } // неявный 200
    return w.ResponseWriter.Write(b)
}
```

После `next.ServeHTTP` логируем структурно: method, path, status,
duration, request_id. Статус берём из обёртки, значение по умолчанию 200. Логгер
передаётся как зависимость — middleware с зависимостями делают
фабриками (`func Logging(log *slog.Logger) Middleware`), как chi's
`middleware.RequestLogger`.

## Graceful shutdown

Наивный `ListenAndServe` + `os.Exit` по Ctrl+C теряет in-flight запросы.
Правильная последовательность: поймать сигнал → перестать принимать новые
соединения → дать текущим завершиться → выйти.

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

if err := Run(ctx, ":8080", handler); err != nil {
    slog.Error("server stopped", "err", err)
}
```

`Run` — наша обёртка (реализуете в практике):

```go
func Run(ctx context.Context, addr string, h http.Handler) error {
    srv := &http.Server{ /* таймауты выше */ Addr: addr, Handler: h }
    errCh := make(chan error, 1)
    go func() { errCh <- srv.ListenAndServe() }()

    select {
    case err := <-errCh:
        if errors.Is(err, http.ErrServerClosed) { return nil } // норма
        return err
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        return srv.Shutdown(shutdownCtx)
    }
}
```

Ключевые решения:

- `server.Shutdown` мягко останавливает: закрывает listener, **ждёт**
  завершения активных запросов. `Close` — жёсткий вариант (рвёт всё),
  его используйте только в самом крайнем случае.
- Таймаут у Shutdown обязателен: вечно висящий запрос (клиент читает
  медленно) не должен блокировать развёртывание. По истечении таймаута —
  `Close` и выход.
- `ListenAndServe` при Shutdown возвращает `http.ErrServerClosed` — это
  не ошибка, а сигнал «остановились штатно», сравнивайте через
  `errors.Is`.
- Канал ошибок буферизованный (`cap 1`): горутина должна иметь
  возможность отправить ошибку и завершиться, даже если мы уже ушли в
  ветку `ctx.Done()` — иначе утечка горутины.
- `signal.NotifyContext` останавливает «глотание» сигналов: первый сигнал
  отменяет контекст (мягкая остановка), повторный — по вашему коду (обычно
  `stop()` + жёсткий выход). Не зовите `os.Exit` в defer — он не выполнит
  defers, включая ваш собственный.

**Порядок важен**: сначала `Shutdown` HTTP-сервера (перестаём принимать),
потом остановка исполнителей, потом закрытие зависимостей (БД, брокеры) —
иначе обработчик отменённого запроса обратится к уже закрытому пулу.

## Тестирование: httptest

`httptest.NewRecorder()` — `ResponseWriter` в памяти, `httptest.NewRequest`
строит запрос. Хэндлер тестируется без сети:

```go
rec := httptest.NewRecorder()
h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/orders/o-1", nil))
if rec.Code != http.StatusOK { t.Fatalf(...) }
```

Для интеграционных проверок (`http.Client`, redirects, таймауты) —
`httptest.NewServer`, он поднимает настоящий сервер на случайном порту.
Запуск/остановку `Run` тестируем по-настоящему: свободный порт через
`net.Listen("tcp", "127.0.0.1:0")`, горутина с `Run`, отмена контекста,
проверка, что сервер перестал принимать соединения.

## Go vs Python: FastAPI/uvicorn/gunicorn

**Python.** FastAPI описывает маршруты и Pydantic-модели; **исполняет** их
uvicorn (ASGI-сервер: event loop, libuv) или gunicorn (WSGI-исполнители).
Отдельный процесс, отдельный менеджер исполнителей, отдельная настройка
таймаутов на каждом слое. У Uvicorn по умолчанию: `timeout_keep_alive=5` —
каждые 5 секунд исполнитель прерывается циклом обслуживания keep-alive (кроме
идеальных условий — систематические ошибки 502 под балансировщиком);
graceful shutdown ограничен `timeout_graceful_shutdown`, и запрос, который
не успел, просто обрывается.

```python
# FastAPI: декоратор + аннотации, исполнение — у uvicorn
@app.post("/v1/orders")
async def create_order(req: CreateOrderRequest) -> OrderResponse:
    ...
```

**Go.** Один процесс, один исполняемый файл: `http.Server` **и есть** production-
сервер — аналог связки gunicorn+uvicorn, только внутри стандартной
библиотеки. Модель конкуренции — goroutine per connection (потоки, но
лёгкие): не нужно думать про event loop и блокирующие вызовы, которые
замораживают весь исполнитель (классическая ловушка FastAPI: `time.sleep` в
async-обработчике вешает весь event loop). Обратная сторона: масштабирование
«на сервер» и прерывание запросов — забота процесса, а не библиотеки.

**C#**: `HttpClient`/`Kestrel` близки по духу (асинхронный сервер в
среде выполнения), но ASP.NET Core — фреймворк с конвейером middleware, тогда как
в Go вы собираете конвейер из функций сами; связка Go+chi похожа на
минимальный ASP.NET Core без DI-контейнера.

**JS/Node**: `express` + middleware-цепочки — ближайшая аналогия паттерна;
в Node сервер тоже сам по себе, но модель «один процесс — один поток» и
вся хрупкость вокруг этого — обратная.

## Подводные камни и анти-паттерны

1. **Нулевые таймауты.** `&http.Server{}` без `ReadHeaderTimeout` — открытая
   дверь для Slowloris. Это не «тюнинг на потом», это минимум для запуска.
2. **`WriteTimeout` и потоковая передача.** SSE/веб-сокеты с ненулевым `WriteTimeout`
   будут рваться. Для стримовых маршрутов нужен отдельный сервер или
   управление через `ResponseController`.
3. **Запись после recover.** Если обработчик успел отправить заголовки и
   паникует, дописывать 500 в тело нельзя — проверяйте, начался ли ответ.
4. **`os.Exit` в main.** Не выполняет `defer`, теряет in-flight запросы и
   не флашит логи. Останавливайтесь через отмену контекста.
5. **Shutdown без таймаута.** Один медленный клиент — и развёртывание висит вечно.
6. **`defer r.Body.Close()` с полным чтением.** Если тело прочитано не до
   конца, соединение не вернётся в пул; при потоковой обработке читайте до
   `io.EOF` или закрывайте принудительно.
7. **Мидлварь после роутера/порядок.** `Recover` снаружи `Logging` —
   логируем всё, включая паники; `RequestID` снаружи `Recover` — заголовок
   в ответе даже при 500. Порядок — часть контракта, тестируйте его.
8. **Магические тайминги в тестах.** HTTP-тесты с `time.Sleep` — флаки;
   используйте `httptest`, отмену контекста и опрос с крайним сроком.
9. **Ответ после первого `Write`.** После `WriteHeader` заголовки ушли —
   менять статус и заголовки поздно. Вычисляйте всё до первой записи.
10. **Хэндлеры без таймаутов.** `ctx` из запроса уже отменяется при обрыве
    соединения — передавайте его вниз (в сервис, в БД), не игнорируйте.

## Что делаем в итоговом проекте

1. `orders/internal/httpapi`: `NewRouter` (chi, маршруты `/v1/orders`),
   middleware `RequestID`/`Recover`/`Logging`, `Wrap`, `Run` — из практики.
2. `orders/cmd/orders/main.go`: slog-логгер, `signal.NotifyContext`,
   `httpapi.Run` с адресом из `ORDERS_ADDR` (значение по умолчанию `:8080`).
3. `Makefile`: цель `run: go run ./cmd/orders` + smoke-тест `curl`.
4. Дальше (урок 10): обработчики получат настоящие DTO, валидацию и
   маппинг ошибок — каркас уже готов, правится только `httpapi`.

## Материалы для углубления

- [net/http](https://pkg.go.dev/net/http) — Server, Handler, таймауты.
- [Cloudflare: The Complete Guide to Go net/http Timeouts](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/) — канонический разбор таймаутов.
- [chi](https://go-chi.io/#/) — роутер, middleware, документация.
- [Go 1.22 routing enhancements](https://go.dev/blog/routing-enhancements) — методы и wildcard в ServeMux.
- [Graceful shutdown](https://pkg.go.dev/net/http#Server.Shutdown) — документация Shutdown.
- [signal.NotifyContext](https://pkg.go.dev/os/signal#NotifyContext).
- [httptest](https://pkg.go.dev/net/http/httptest).

## Домашнее задание

[Практика к уроку — шаблон с тестами](practice/README.md)
