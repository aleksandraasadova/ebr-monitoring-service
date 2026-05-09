# Рефакторинг архитектуры

Документ описывает изменения, внесённые в проект `ebr-monitoring-service`, объясняет причины каждого изменения и указывает применённые паттерны.

---

## Содержание

1. [Вынос Dependency Injection из транспорта в `main.go`](#1-вынос-di-в-maingo)
2. [Потребительские интерфейсы для сервисов](#2-потребительские-интерфейсы-для-сервисов)
3. [Удаление `*sql.DB` из сервиса, перенос транзакции в репозиторий](#3-перенос-транзакции-в-репозиторий)
4. [Типизированные Claims в middleware](#4-типизированные-claims-в-middleware)
5. [Унификация именования интерфейсов](#5-унификация-именования)
6. [Сопутствующие баг-фиксы](#6-баг-фиксы)
7. [Маршрутизация в транспортном слое](#7-маршрутизация-в-транспортном-слое)
8. [OpenAPI / Swagger документация](#8-openapi--swagger-документация)
9. [Handler Struct паттерн](#9-handler-struct-паттерн)
10. [Сервисы без DTO — примитивы и доменные сущности](#10-сервисы-без-dto--примитивы-и-доменные-сущности)
11. [DTO вынесены из domain в transport](#11-dto-вынесены-из-domain-в-transport)

---

## 1. Вынос DI в `main.go`

### Что было

Файл `internal/transport/wsserver/server.go` выполнял три роли одновременно:
- HTTP-сервер (lifecycle)
- Сборка графа зависимостей (`UserRepo → UserService → handler`)
- Регистрация роутов

```go
// БЫЛО: wsserver/server.go
func NewServer(addr string, db *sql.DB) WSServer {
    m := http.NewServeMux()
    userRepo := repository.NewUserRepo(db)
    userService := service.NewUserService(userRepo)
    createUserHandler := transport.CreateUserHandler(userService)
    m.Handle("POST /api/v1/users", middleware.JWT(...))
    // ... ещё 5 сервисов и репо
    return &wsSrv{srv: &http.Server{Handler: m}}
}
```

### Почему это плохо

**1. Нарушение Single Responsibility Principle.** Один пакет отвечал за HTTP-инфраструктуру И за сборку приложения. Изменение конструктора любого сервиса заставляло править транспортный слой.

**2. Циклическая зависимость по смыслу.** Транспорт (внешний слой) импортировал `repository` (нижний слой), который сам по себе не нужен транспорту — он нужен только для сборки.

**3. Невозможно использовать сервер с другим набором роутов.** Например, нельзя поднять `wsserver` для тестов с моками — `NewServer` всегда сам создаёт реальные репозитории.

**4. Тестирование без БД невозможно.** Чтобы создать `wsSrv`, нужен живой `*sql.DB`.

### Что стало

```go
// СТАЛО: cmd/ebr-app/main.go
func main() {
    db, _ := sql.Open("postgres", os.Getenv("DB_URL"))
    defer db.Close()

    userRepo := repository.NewUserRepo(db)
    recipeRepo := repository.NewRecipeRepo(db)
    batchRepo := repository.NewBatchRepo(db)

    userService := service.NewUserService(userRepo)
    authService := service.NewAuthService(userRepo)
    recipeService := service.NewRecipeService(recipeRepo)
    batchService := service.NewBatchService(batchRepo, recipeRepo)

    mux := setupRouter(userService, authService, recipeService, batchService)
    srv := wsserver.NewServer(":8081", mux)
    srv.Start()
}

// СТАЛО: wsserver/server.go
func NewServer(addr string, handler http.Handler) WSServer {
    return &wsSrv{srv: &http.Server{Addr: addr, Handler: handler}}
}
```

### Применённый паттерн: **Composition Root**

> **Composition Root** — единственная точка в приложении, где собирается весь граф зависимостей. Должна находиться как можно ближе к точке входа (entry point) — обычно в `main`.

Цитата автора паттерна Mark Seemann (книга *Dependency Injection in .NET*):

> *"A Composition Root is a (preferably) unique location in an application where modules are composed together."*

### Почему это правильно

1. **Изолированность точки сборки.** Все остальные пакеты ничего не знают о том, как собирается приложение — они принимают зависимости через конструкторы.
2. **`wsserver` стал переиспользуемым.** Теперь он принимает любой `http.Handler` — можно подменять для тестов.
3. **Видимый граф зависимостей.** В `main.go` сразу видно: что от чего зависит, в каком порядке создаётся.
4. **Соответствует Go-идиоме.** В стандартной библиотеке и большинстве open-source Go проектов сборка идёт в `main`.

---

## 2. Потребительские интерфейсы для сервисов

### Что было

Хэндлеры зависели от **конкретных структур** сервисов:

```go
// БЫЛО: transport/http/user.go
func CreateUserHandler(svc *service.UserService) http.HandlerFunc { ... }

// БЫЛО: transport/http/batch.go
func CreateBatchHandler(bs *service.BatchService) http.HandlerFunc { ... }
func ListBatchesByStatusHandler(bs *service.BatchService) http.HandlerFunc { ... }
```

### Почему это плохо

**1. Невозможно тестировать хэндлер без реального сервиса.** Чтобы вызвать `CreateUserHandler`, надо создать настоящий `*service.UserService`, для которого нужен `domain.UserRepo`, для которого нужен `*sql.DB`. То есть юнит-тест хэндлера превращается в интеграционный.

**2. Нарушение Dependency Inversion Principle.** Высокоуровневый модуль (хэндлер) зависит от низкоуровневого (конкретный сервис) вместо того, чтобы оба зависели от абстракции.

**3. Нарушение Interface Segregation Principle.** `ListBatchesByStatusHandler` зависел от полного `*service.BatchService`, хотя ему нужен только метод `GetByStatus`. Изменение `CreateBatch` могло сломать этот хэндлер на уровне типов.

### Что стало

Каждый хэндлер объявляет **минимальный интерфейс** с теми методами, которые ему нужны:

```go
// СТАЛО: transport/http/user.go
type userCreator interface {
    Create(ctx context.Context, req domain.CreateUserRequest) (*domain.CreateUserResponse, error)
}
func CreateUserHandler(svc userCreator) http.HandlerFunc { ... }

// СТАЛО: transport/http/batch.go
type batchCreator interface {
    CreateBatch(ctx context.Context, req domain.CreateBatchRequest, registeredByID int) (*domain.CreateBatchResponse, error)
}
type batchLister interface {
    GetByStatus(ctx context.Context, status string) ([]domain.Batch, error)
}
func CreateBatchHandler(svc batchCreator) http.HandlerFunc { ... }
func ListBatchesByStatusHandler(svc batchLister) http.HandlerFunc { ... }
```

В `main.go` ничего не изменилось — `*service.UserService` автоматически удовлетворяет интерфейсу `userCreator` (Go duck typing).

### Применённые паттерны:

#### **Accept Interfaces, Return Structs** (Go-идиома)

Сформулирована Jack Lindamood в [статье 2016 года](https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8) и закреплена в Go-сообществе как стандарт. Ключевая идея:

> *Принимай интерфейсы, возвращай конкретные типы. Это позволяет потребителю заменять реализацию, а производителю — не быть привязанным к чужим интерфейсам.*

Сервисы продолжают возвращать `*UserService`, `*BatchService` (конкретные типы), но потребители (хэндлеры) принимают только нужные им интерфейсы.

#### **Interface Segregation Principle (ISP)** — буква **I** в SOLID

Robert C. Martin:

> *"Clients should not be forced to depend upon interfaces that they do not use."*

`batchCreator` и `batchLister` разделены, потому что это два независимых клиента, и им не нужны лишние методы.

#### **Dependency Inversion Principle (DIP)** — буква **D** в SOLID

> *"High-level modules should not depend on low-level modules. Both should depend on abstractions."*

Хэндлер (high-level — описывает HTTP-протокол) больше не зависит от конкретного `*service.UserService` (low-level — содержит бизнес-логику). Оба зависят от абстракции `userCreator`.

### Почему интерфейсы названы с маленькой буквы

`userCreator`, `batchCreator` — package-private. Это намеренно:

- Интерфейсы определены в пакете-потребителе и используются только внутри него
- Они не предназначены для импорта другими пакетами
- Так выражается намерение: "это чисто внутренняя абстракция данного пакета"

Это противоположно репозиторным интерфейсам (`domain.UserRepo`), которые экспортируются — их реализуют в другом пакете (`repository`).

---

## 3. Перенос транзакции в репозиторий

### Что было

Сервис содержал инфраструктурный код и сырой SQL:

```go
// БЫЛО: service/batch.go
type BatchService struct {
    db         *sql.DB                    // ← инфраструктура в сервисе
    batchRepo  domain.BatchRepo
    recipeRepo domain.RecipeRepo
}

func (bs *BatchService) CreateBatch(ctx, req, registeredByID) {
    recipe, err := bs.recipeRepo.GetByCode(ctx, req.RecipeCode)
    // валидация...

    tx, err := bs.db.BeginTx(ctx, nil)   // ← сервис управляет транзакцией
    defer tx.Rollback()

    bs.batchRepo.Create(ctx, tx, batch)  // ← *sql.Tx протекает в domain

    _, err = tx.ExecContext(ctx, `       // ← СЫРОЙ SQL В СЕРВИСЕ
        INSERT INTO weighing_log ...
        SELECT $1::INT, ri.ingredient_id, ...
        FROM recipe_ingredients ri WHERE ri.recipe_id = $3
    `, batch.ID, req.TargetVolumeL, recipe.ID)

    return tx.Commit()
}
```

И самое страшное — интерфейс `domain.BatchRepo` импортировал `database/sql`:

```go
// БЫЛО: domain/batch.go
import "database/sql"

type BatchRepo interface {
    Create(ctx context.Context, db *sql.Tx, batch *Batch) error  // ← *sql.Tx в domain
    GetByStatus(ctx context.Context, status string) ([]Batch, error)
}
```

### Почему это плохо

**1. Утечка инфраструктуры в domain.** `domain` — самый внутренний слой, не должен знать ни о SQL, ни о HTTP, ни о JWT. Импорт `database/sql` в `domain/batch.go` — грубейшее нарушение Clean Architecture.

**2. Сервис знает о технологии хранения.** `BeginTx`, `Rollback`, `Commit` — детали реляционной БД. Если завтра данные переедут в Mongo или distributed event store, сервис придётся переписать целиком, хотя бизнес-логика не изменилась.

**3. Сервис пишет SQL.** Двойное нарушение слоёв — сервисный слой содержит знание о схеме таблиц (`weighing_log`, `recipe_ingredients`).

**4. Невозможно мокать `BatchRepo` для тестов.** Мок должен принимать `*sql.Tx`, а это конкретный SQL-тип — приходится поднимать настоящую БД.

**5. В SQL был баг** — `PaymentInfo41::INT` вместо `$1::INT` (опечатка / последствие неудачного автокомплита). Запрос вообще не работал.

### Что стало

**Domain очищен от инфраструктуры:**

```go
// СТАЛО: domain/batch.go
import (
    "context"
    "errors"
    "time"
)
// никакого database/sql

type BatchRepo interface {
    Create(ctx context.Context, batch *Batch, recipeID int) error  // ← чистая абстракция
    GetByStatus(ctx context.Context, status string) ([]Batch, error)
}
```

**Сервис стал тонким — только бизнес-логика:**

```go
// СТАЛО: service/batch.go
type BatchService struct {
    batchRepo  domain.BatchRepo
    recipeRepo domain.RecipeRepo
    // никакого *sql.DB
}

func (bs *BatchService) CreateBatch(ctx, req, registeredByID) {
    recipe, err := bs.recipeRepo.GetByCode(ctx, req.RecipeCode)
    if err != nil { return nil, err }

    if req.TargetVolumeL < recipe.MinVolumeL || req.TargetVolumeL > recipe.MaxVolumeL {
        return nil, domain.ErrInvalidBatchVolume
    }

    batch := &domain.Batch{...}
    return bs.batchRepo.Create(ctx, batch, recipe.ID)  // ← один атомарный вызов
}
```

**Репозиторий владеет транзакцией внутри себя:**

```go
// СТАЛО: repository/batch.go
func (br *BatchRepo) Create(ctx, batch, recipeID) error {
    tx, err := br.db.BeginTx(ctx, nil)
    if err != nil { return err }
    defer tx.Rollback()

    // INSERT batch...
    // INSERT weighing_log...

    return tx.Commit()
}
```

### Применённые паттерны:

#### **Aggregate Root** (DDD — Domain-Driven Design)

Eric Evans, *Domain-Driven Design*:

> *"An Aggregate is a cluster of associated objects that we treat as a unit for the purpose of data changes. Each Aggregate has a root and a boundary."*

`Batch` — корень агрегата, который включает в себя `weighing_log` записи. Они создаются и существуют только вместе с batch. Поэтому **транзакционная граница совпадает с границей агрегата**, а владельцем границы является один репозиторий.

Когда транзакция охватывает только один агрегат — её должен инкапсулировать репозиторий этого агрегата. Сервис не должен знать, что под капотом происходит несколько SQL-запросов.

#### **Repository Pattern** (Martin Fowler, *Patterns of Enterprise Application Architecture*)

> *"A Repository mediates between the domain and data mapping layers, acting like an in-memory collection of domain objects."*

Хороший репозиторий выглядит как коллекция в памяти: `repo.Create(batch)`, `repo.GetByStatus(status)`. Никаких транзакций, курсоров, SQL — это технические детали реализации, которые скрыты от потребителя.

#### **Dependency Inversion Principle**

Domain определяет интерфейс (`BatchRepo`), который реализует repository. Зависимость направлена внутрь: `repository` зависит от `domain`, но не наоборот.

```
domain.BatchRepo (interface)
        ↑ реализует
repository.BatchRepo (struct)
        ↑ использует
service.BatchService
```

### Почему НЕ Transactor

Альтернативный паттерн — Unit of Work / Transactor:

```go
type Transactor interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Когда он нужен:
- Транзакция охватывает **несколько разных репозиториев**
- Бизнес-операция требует атомарной работы между агрегатами

В нашем случае операция атомарна **внутри одного агрегата** (`Batch`). Введение `Transactor` ради единственного места было бы over-engineering — лишняя абстракция без выгоды. Когда появится кросс-агрегатная транзакция — введём `Transactor` тогда.

---

## 4. Типизированные Claims в middleware

### Что было

```go
// БЫЛО: middleware/auth.go
const TokenKey Key = "token_claims"

func JWT(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w, r) {
        // парсинг токена...
        claims, _ := token.Claims.(jwt.MapClaims)
        ctx := context.WithValue(r.Context(), TokenKey, claims)  // ← в ctx кладётся jwt.MapClaims
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// БЫЛО: transport/http/batch.go
func CreateBatchHandler(bs *service.BatchService) http.HandlerFunc {
    return func(w, r) {
        raw := r.Context().Value(middleware.TokenKey)
        claims, ok := raw.(jwt.MapClaims)              // ← хэндлер знает про jwt
        if !ok { ... }
        registeredBy, ok := claims["user_id"].(float64) // ← ручной cast float64
        if !ok { ... }
        // вызов сервиса с int(registeredBy)
    }
}
```

### Почему это плохо

**1. Хэндлер знает о JWT-библиотеке.** Импортирует `github.com/golang-jwt/jwt/v5`, разбирает `jwt.MapClaims`. Если завтра перейдём на Paseto, OAuth2 introspection или session-cookie — придётся править все хэндлеры.

**2. Дублирование логики.** Каждый хэндлер, которому нужен `user_id`, повторяет один и тот же код извлечения и cast'а. DRY-нарушение.

**3. Магические строки.** `claims["user_id"]`, `claims["role"]` — литералы, разбросанные по коду. Опечатка в одном месте даст runtime-ошибку.

**4. Слабая типизация.** `claims["user_id"].(float64)` — почему `float64`? Потому что JSON-числа в Go всегда `float64` после парсинга. Это деталь реализации JWT-библиотеки, протекающая в код приложения.

**5. Экспортированный ключ контекста.** `middleware.TokenKey` доступен извне — любой пакет может прочитать или подменить значение, нарушая инкапсуляцию.

### Что стало

```go
// СТАЛО: middleware/auth.go
type Claims struct {
    UserID int
    Role   string
}

type ctxKey struct{}                      // ← приватный тип ключа
var claimsCtxKey = ctxKey{}                // ← приватная переменная

func UserFromContext(ctx context.Context) (Claims, bool) {  // ← единственный API доступа
    c, ok := ctx.Value(claimsCtxKey).(Claims)
    return c, ok
}

func JWT(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w, r) {
        // ... парсинг и валидация JWT здесь, вся работа со стрингами в одном месте ...
        userID, _ := mapClaims["user_id"].(float64)
        role, _ := mapClaims["role"].(string)

        claims := Claims{UserID: int(userID), Role: role}
        ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// СТАЛО: transport/http/batch.go
func CreateBatchHandler(svc batchCreator) http.HandlerFunc {
    return func(w, r) {
        user, ok := middleware.UserFromContext(r.Context())  // ← одна строка, типизированно
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        resp, err := svc.CreateBatch(r.Context(), req, user.UserID)
    }
}
```

### Применённые паттерны:

#### **Context Value with Unexported Key** (Go-идиома)

Зафиксирована в [официальном Go blog](https://go.dev/blog/context):

> *"The provided key must be comparable and should not be of type string or any other built-in type to avoid collisions between packages using context."*

Использование `type ctxKey struct{}` гарантирует:
- Никакой другой пакет не сможет случайно прочитать или перезаписать значение
- Нет коллизий по строковому ключу

#### **Tell, Don't Ask** (Andy Hunt, Dave Thomas)

Хэндлер больше не "спрашивает": *"дай мне map claims, найди в нём user_id, скастуй float64 в int"*. Он "говорит": *"дай мне юзера из контекста"*.

#### **Anti-Corruption Layer** (DDD)

> *"An Anti-Corruption Layer translates between two models, preventing the foreign model from leaking into your domain."*

`middleware` — anti-corruption layer между внешним протоколом аутентификации (JWT) и внутренней моделью приложения (`Claims`). Если в будущем JWT заменится на любой другой механизм — менять надо только middleware, а хэндлеры остаются нетронутыми.

#### **Single Responsibility Principle**

`middleware/auth.go` — единственное место, которое знает про JWT. Все остальные слои работают с типизированной моделью `Claims`.

---

## 5. Унификация именования

### Что было

```go
// domain/user.go
type UserRepository interface { ... }   // ← полное "Repository"

// domain/batch.go
type BatchRepo interface { ... }        // ← короткое "Repo"

// domain/recipe.go
type RecipeRepo interface { ... }       // ← короткое "Repo"
```

### Почему это плохо

**1. Когнитивный шум.** Читая код, надо постоянно помнить какой суффикс у какого типа. Лишняя нагрузка на восприятие без какой-либо смысловой пользы.

**2. Несогласованность как симптом.** Когда код растёт, разные авторы добавляют новые сущности кто как привык — `OrderRepo`, `PaymentRepository`, `UserDAO`. Хороший проект задаёт **один** стиль.

**3. Не соответствует Go-идиоме.** В Go ценится краткость без потери ясности. `Repo` достаточно ясно, чтобы понять — это репозиторий.

### Что стало

```go
// domain/user.go, domain/batch.go, domain/recipe.go
type UserRepo interface { ... }
type BatchRepo interface { ... }
type RecipeRepo interface { ... }
```

### Применённый принцип: **Consistency / Principle of Least Astonishment**

> *"Cодержание похожих сущностей следует именовать единообразно — иначе читатель будет тратить силы на анализ различий, которых на самом деле нет."*

В *The Go Programming Language* (Donovan, Kernighan):

> *"Naming conventions should be consistent across the codebase. A reader of one file should be able to predict the names used in another."*

---

## 6. Баг-фиксы

Сопутствующие исправления, обнаруженные в процессе рефакторинга:

### 6.1. SQL-опечатка в `weighing_log`

```sql
-- БЫЛО (не компилировалось в Postgres):
SELECT
PaymentInfo51::INT,
        ri.ingredient_id, ...

-- СТАЛО:
SELECT
    $1::INT,
    ri.ingredient_id, ...
```

Запрос вообще никогда не выполнялся успешно. Опечатка скорее всего была результатом неправильного автокомплита.

### 6.2. Отсутствующий `return` в `ListBatchesByStatusHandler`

```go
// БЫЛО:
batches, err := bs.GetByStatus(r.Context(), status)
if err != nil {
    http.Error(w, "failed to list batches", http.StatusInternalServerError)
    // ← НЕТ return! код продолжается дальше
}
// и кодирует nil как []
resp := make([]domain.GetBatchesByStatusResponse, len(batches))

// СТАЛО:
if err != nil {
    http.Error(w, "failed to list batches", http.StatusInternalServerError)
    return
}
```

При ошибке хэндлер сначала писал в ответ `500`, а потом ещё пытался закодировать пустой массив — двойная запись в `ResponseWriter` это runtime warning + сломанный protocol.

### 6.3. `GetRecipeByCodeHandler` падал при неизвестных ошибках

```go
// БЫЛО:
if err != nil {
    if err == domain.ErrRecipeNotFound { ... return }
    if err == domain.ErrRecipeArchived { ... return }
}
// если err не один из этих двух — падаем сюда с err != nil И пишем StatusOK с nil resp
w.WriteHeader(http.StatusOK)
json.NewEncoder(w).Encode(resp)

// СТАЛО:
if err != nil {
    switch err {
    case domain.ErrRecipeNotFound: ...
    case domain.ErrRecipeArchived: ...
    default:
        http.Error(w, "internal server error", http.StatusInternalServerError)
    }
    return
}
```

### 6.4. Удалены отладочные `fmt.Println`

Из `transport/http/batch.go` и `service/batch.go` удалены принты вида:
```go
fmt.Println("CreateBatchHandler called")
fmt.Printf("User ID from token: %v\n", registeredBy)
fmt.Printf("Before transaction\n")
fmt.Printf("Transaction started\n")
```

Их место — структурированный логгер (`slog`) с уровнями `debug` / `info`. Сейчас они просто выкинуты как технический мусор.

---

## 7. Маршрутизация в транспортном слое

### Что было (после первой итерации)

После шага 1 функция `setupRouter` лежала прямо в `main.go`:

```go
// БЫЛО: cmd/ebr-app/main.go
func main() {
    // ... db, repos, services ...
    mux := setupRouter(userService, authService, recipeService, batchService)
    srv := wsserver.NewServer(":8081", mux)
    srv.Start()
}

func setupRouter(
    userService *service.UserService,
    authService *service.AuthService,
    recipeService *service.RecipeService,
    batchService *service.BatchService,
) *http.ServeMux {
    m := http.NewServeMux()
    // ... статика, /swagger/ ...
    m.Handle("POST /api/v1/users", middleware.JWT(...))
    m.HandleFunc("POST /api/v1/auth/login", ...)
    // и так далее — 50+ строк
    return m
}
```

### Почему это плохо

**1. Смешение ответственностей в `main.go`.** Composition Root должен только **собирать граф зависимостей**. Описание HTTP-протокола (URL, методы, цепочки middleware) — это уже семантика **транспортного слоя**, не сборки.

**2. `main.go` раздуется при росте API.** При 30+ эндпоинтах файл превратится в скролл-простыню, где сложно найти DI среди роутов.

**3. Невозможно протестировать роутер изолированно.** Чтобы дёрнуть `setupRouter` в тесте, надо тащить за собой `func main` и его глобальные импорты.

**4. Описание API размазано по двум пакетам.** Хэндлеры, middleware, OpenAPI-аннотации — в `internal/transport/http`. А роуты, которые их склеивают — в `cmd/ebr-app`. Чтобы понять "какой URL дёрнет какой хэндлер" — надо прыгать между пакетами.

### Что стало

**`internal/transport/http/router.go`** — новый файл рядом с хэндлерами:

```go
package transport

type RouterDeps struct {
    WebDir        string
    UserService   *service.UserService
    AuthService   *service.AuthService
    RecipeService *service.RecipeService
    BatchService  *service.BatchService
}

func NewRouter(d RouterDeps) *http.ServeMux {
    m := http.NewServeMux()

    // статика, /swagger/, и все 5 роутов с middleware
    m.Handle("POST /api/v1/users",
        middleware.JWT(middleware.RequireRole("admin")(CreateUserHandler(d.UserService))))
    // ...

    return m
}
```

**`main.go` остаётся чистым composition root:**

```go
func main() {
    // db, repos, services...

    mux := transport.NewRouter(transport.RouterDeps{
        WebDir:        filepath.Join(wd, "web"),
        UserService:   userService,
        AuthService:   authService,
        RecipeService: recipeService,
        BatchService:  batchService,
    })

    srv := wsserver.NewServer(":8081", mux)
    srv.Start()
}
```

### Применённые паттерны:

#### **Single Responsibility Principle** (на уровне файла)

`main.go` делает **одну вещь** — собирает приложение. Описание HTTP-API живёт в транспортном слое. Каждый файл отвечает за одну ответственность.

#### **Parameter Object** (Martin Fowler, *Refactoring*)

> *"Replace Long Parameter List with Parameter Object — when methods take many parameters that conceptually belong together, group them into a struct."*

Вместо позиционного списка `NewRouter(webDir, userSvc, authSvc, recipeSvc, batchSvc)` использован `RouterDeps{...}`. Преимущества:
- Имена параметров читаются на стороне вызова (`UserService:`, `AuthService:`)
- Добавление новой зависимости (`Logger`, `Metrics`) **не ломает существующих вызовов**
- Невозможно перепутать порядок аргументов одного типа

### Граница, которую важно не пересечь

`NewRouter` **принимает только готовые сервисы** — никаких `*sql.DB`, `repository.*`, `*sql.Tx`. Если эта граница нарушится — мы вернёмся к исходной проблеме шага 1: транспорт превратится в "тот, кто всё собирает". Текущий код это запрещает: для построения графа `repos → services` нужны импорты `repository` и `service`, а конструкторы сервисов требуют `domain.*Repo` интерфейсы — всё это работа `main.go`.

### Что мы получили в сумме

| Где живёт | До рефакторинга | После шага 1 | После шага 7 |
|-----------|----------------|--------------|--------------|
| Открытие БД | wsserver | main.go | main.go |
| Создание repos/services | wsserver | main.go | main.go |
| Регистрация роутов | wsserver | main.go (`setupRouter`) | **transport/router.go** |
| HTTP-сервер lifecycle | wsserver | wsserver | wsserver |

---

## 8. OpenAPI / Swagger документация

### Что было

Документации API не было. Контракт между фронтом и бэком существовал только в голове разработчика и в коде хэндлеров. Любое изменение DTO или эндпоинта могло сломать клиентов незаметно.

### Что стало

Подключён **`swaggo/swag`** — code-first генератор OpenAPI 2.0 спецификации из специальных Go-комментариев.

**Зависимости в `go.mod`:**
- `github.com/swaggo/swag` — парсер аннотаций и CLI
- `github.com/swaggo/http-swagger/v2` — `http.Handler` для UI
- `github.com/swaggo/files/v2` — статика Swagger UI

**Аннотации в `main.go` — общая информация об API:**

```go
// @title           EBR Monitoring Service API
// @version         1.0
// @description     API сервиса мониторинга процесса эмульсификации.
// @host            localhost:8081
// @BasePath        /
// @schemes         http
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
func main() { ... }
```

**Аннотации над каждым хэндлером — описание эндпоинта:**

```go
// CreateBatchHandler godoc
// @Summary      Создать партию (batch)
// @Description  Создаёт партию по рецепту. registered_by берётся из JWT.
// @Tags         batches
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body domain.CreateBatchRequest true "данные партии"
// @Success      201 {object} domain.CreateBatchResponse
// @Failure      400 {string} string "invalid json or invalid batch volume"
// @Failure      401 {string} string "unauthorized"
// @Failure      404 {string} string "recipe not found or archived"
// @Failure      500 {string} string "failed to create batch"
// @Router       /api/v1/batches [post]
func CreateBatchHandler(svc batchCreator) http.HandlerFunc { ... }
```

**Сгенерировано** командой `swag init -g cmd/ebr-app/main.go -o docs/swagger --parseInternal`:

```
docs/swagger/
├── docs.go         ← Go-пакет, который импортируется в main.go как side-effect:
│                     _ "github.com/.../docs/swagger"
├── swagger.json    ← OpenAPI 2.0 спецификация (для интеграций)
└── swagger.yaml    ← она же в YAML
```

**UI** примонтирован в `router.go`:

```go
m.Handle("/swagger/", httpSwagger.Handler(
    httpSwagger.URL("/swagger/doc.json"),
))
```

Открывается по `http://localhost:8081/swagger/index.html` — интерактивная документация: можно нажать "Authorize", вставить JWT, и дёргать эндпоинты прямо из браузера.

### Применённые паттерны:

#### **Code-first API documentation**

Источник истины — Go-код. Аннотации лежат **рядом** с хэндлером, а не в отдельном YAML, который легко забыть синхронизировать. При изменении DTO достаточно перегенерировать спеку.

Альтернативы (которые не выбрали):
- **Contract-first** (написать `openapi.yaml`, генерить Go-типы из неё) — строже, но требует переделки хэндлеров под сгенерированные интерфейсы.
- **Рукописный YAML** — полный контроль, но спека и код разъезжаются на любом изменении.

#### **Documentation as a build artefact**

Спецификация **генерируется**, не пишется руками. Это значит:
- Спека всегда отражает реальный код (если перегенерировали)
- В CI можно добавить `swag init` + `git diff` — если разработчик забыл регенерить, билд упадёт
- Для разных версий API — разные генерации

### Команда регенерации после правок аннотаций:

```bash
swag init -g cmd/ebr-app/main.go -o docs/swagger --parseInternal
```

Флаг `--parseInternal` нужен потому, что DTO-структуры лежат в `internal/domain/` — без него парсер их не найдёт.

---

---

## 9. Handler Struct паттерн

### Что было

Каждый хэндлер — отдельная функция, возвращающая `http.HandlerFunc` через замыкание:

```go
// БЫЛО: отдельные функции-фабрики
func CreateBatchHandler(svc batchCreator) http.HandlerFunc { ... }
func ListBatchesByStatusHandler(svc batchLister) http.HandlerFunc { ... }
```

Для каждой функции свой интерфейс (`batchCreator`, `batchLister`), хотя оба живут в одном файле и работают с одним доменом.

### Почему это неоптимально

**1. Искусственное дробление связанных обработчиков.** `CreateBatch` и `ListByStatus` относятся к одному ресурсу. Разделять их на независимые функции без общего состояния — лишнее усложнение.

**2. Два интерфейса вместо одного.** `batchCreator` и `batchLister` — это просто половинки одного сервиса. При добавлении третьего endpoint'а для батчей появится третий интерфейс.

**3. Нет явного конструктора.** Нельзя посмотреть "что нужно BatchHandler" без чтения сигнатур всех функций.

### Что стало

```go
// СТАЛО: структура с конструктором и методами
type batchService interface {
    CreateBatch(ctx, recipeCode string, targetVolumeL, registeredByID int) (*domain.Batch, error)
    GetByStatus(ctx, status string) ([]domain.Batch, error)
}

type BatchHandler struct {
    svc batchService
}

func NewBatchHandler(svc batchService) *BatchHandler {
    return &BatchHandler{svc: svc}
}

func (h *BatchHandler) Create(w http.ResponseWriter, r *http.Request)      { ... }
func (h *BatchHandler) ListByStatus(w http.ResponseWriter, r *http.Request) { ... }
```

В роутере:
```go
batchH := NewBatchHandler(d.BatchService)
m.Handle("POST /api/v1/batches", middleware.JWT(...)(http.HandlerFunc(batchH.Create)))
m.Handle("GET /api/v1/batches",  middleware.JWT(...)(http.HandlerFunc(batchH.ListByStatus)))
```

Все четыре хэндлера приведены к единому виду: `AuthHandler`, `UserHandler`, `RecipeHandler`, `BatchHandler`.

### Применённые паттерны

#### **Object-Oriented Handler** (Go-идиома)

Стандартная практика в Go HTTP-приложениях — группировать связанные обработчики в структуру. Это то, что делают большинство фреймворков (`gin`, `echo`), но применимо и к stdlib `net/http`.

#### **Interface Segregation Principle — прагматичный баланс**

Мы объединили `batchCreator` + `batchLister` в один `batchService`. Это отход от строгого ISP (нарушение: `Create` видит метод `GetByStatus`). Однако здесь оправдано:

- Оба метода принадлежат одному ресурсу (batches)
- Тестирование одного метода с mock всего `batchService` — не проблема
- Дробление ради ISP здесь — over-engineering

Для однометодных хэндлеров (`AuthHandler.Login`, `RecipeHandler.GetByCode`) интерфейсы тоже остаются минимальными — ровно один метод.

---

## 10. Сервисы без DTO — примитивы и доменные сущности

### Что было

Сервисы принимали и возвращали DTO-структуры:

```go
// БЫЛО
func (us *UserService) Create(ctx, req domain.CreateUserRequest) (*domain.CreateUserResponse, error)
func (rs *RecipeService) GetByCode(ctx, code) (*domain.GetRecipeByCodeResponse, error)
func (as *AuthService) Login(ctx, req domain.LoginRequest) (*domain.LoginResponse, error)
```

### Почему это плохо

**1. Сервис привязан к форме HTTP-запроса.** `CreateUserRequest` — это структура, описывающая поля JSON-тела. Если завтра тот же сервис вызывается из CLI или очереди сообщений — поля могут называться иначе. Сервис не должен знать о JSON.

**2. Сервис знает что хочет видеть транспорт.** Возвращая `*domain.CreateUserResponse`, сервис диктует транспорту форму ответа. Если API меняется (добавляется поле, переименовывается) — трогаем сервис, хотя бизнес-логика не изменилась.

**3. `domain/dto.go` импортируется в сервис.** Пакет `domain` засорён HTTP-контрактами: `json:"user_code"`, `json:"batch_status"` — это не доменные концепции.

**4. Избыточный параметр `recipeID` в `BatchRepo.Create`.** Сигнатура `Create(ctx, batch *Batch, recipeID int)` принимала `recipeID`, хотя `batch.RecipeID` уже содержит то же значение:

```go
// Оба значения всегда равны — один лишний параметр
bs.batchRepo.Create(ctx, batch, recipe.ID)  // recipe.ID == batch.RecipeID
```

**5. `UserRole` не использовался как тип.** Тип `UserRole string` был определён, но `User.Role` оставался `string`. Результат — ручные кастования:
```go
if req.Role != string(domain.Admin) && ...  // зачем тип, если всё равно кастуем в string?
```

**6. Дублирование валидации** между хэндлером и сервисом. Хэндлер проверял `req.Role == ""`, сервис проверял то же самое через `domain.ErrInvalidRole`.

### Что стало

**Сервисы принимают примитивы, возвращают доменные сущности:**

```go
// UserService — роль теперь типизирована, сервис работает с User напрямую
func (us *UserService) Create(ctx context.Context, role domain.UserRole, surname, name, fatherName string) (*domain.User, error)

// AuthService — возвращает сущность + токен
func (as *AuthService) Login(ctx context.Context, username, password string) (*domain.User, string, error)

// RecipeService — возвращает сущность
func (rs *RecipeService) GetByCode(ctx context.Context, code string) (*domain.Recipe, error)

// BatchRepo — recipeID убран, repo читает batch.RecipeID сам
func (br *BatchRepo) Create(ctx context.Context, batch *domain.Batch) error
```

**`User.Role` стал строго типизирован:**

```go
type User struct {
    Role UserRole  // было string
}

// Сравнение без кастования:
if role != domain.Admin && role != domain.Operator { ... }
```

**Хэндлеры маппят сами:**

```go
// transport/http/auth.go
user, token, err := h.svc.Login(r.Context(), req.Username, req.Password)
// ...
json.NewEncoder(w).Encode(LoginResponse{
    Token:    token,
    Role:     string(user.Role),
    UserCode: user.UserCode,
    ...
})
```

**Дублирующая валидация удалена из хэндлеров.** Хэндлер только декодирует JSON. Если поле пустое или недопустимое — сервис вернёт `domain.ErrInvalidRole` / `domain.ErrFullNameRequired`, хэндлер их обработает.

### Применённые паттерны

#### **Tell, Don't Ask** (обратная сторона)

Теперь хэндлер получает доменную сущность и сам решает какие поля и в каком порядке включить в ответ. Транспорт отвечает за транспортный формат, сервис — за бизнес-результат.

#### **Single Responsibility на уровне метода**

Каждый метод сервиса отвечает за одну бизнес-операцию и ничего не знает о том, кто и зачем его вызывает.

#### **Type Driven Design**

`UserRole` как отдельный тип позволяет компилятору отлавливать ошибки. Передать произвольную строку вместо роли теперь требует явного `domain.UserRole("что-то")` — это сигнал разработчику, что он делает что-то нестандартное.

---

## 11. DTO вынесены из domain в transport

### Что было

```
internal/domain/
    dto.go          ← LoginRequest, LoginResponse, CreateUserRequest,
                       CreateUserResponse, GetRecipeByCodeResponse,
                       CreateBatchRequest, CreateBatchResponse,
                       GetBatchesByStatusResponse
```

Все структуры с `json`-тегами лежали в `domain`. Пакет domain импортировал стандартную библиотеку `time` ради этих DTO, а сами структуры имели поля типа `json:"batch_status"` — что является HTTP-концерном.

### Почему это плохо

Domain-пакет должен описывать **бизнес-сущности и правила**. `json:"user_code"` — это соглашение HTTP API, а не бизнес-правило. Загрязнение domain слоя техническими деталями сериализации нарушает принцип Clean Architecture: *domain не должен знать о механизмах доставки данных*.

Кроме того, поскольку эти DTO использовались как параметры сервисов (до рефакторинга §10), любое изменение API-контракта требовало правок в domain и сервисе одновременно.

### Что стало

```
internal/domain/
    dto.go          ← УДАЛЁН. Больше не существует.

internal/transport/http/
    dto.go          ← LoginRequest, LoginResponse, CreateUserRequest,
                       CreateUserResponse, GetRecipeByCodeResponse,
                       CreateBatchRequest, CreateBatchResponse,
                       GetBatchesByStatusResponse
```

Domain теперь содержит только:
- **Сущности** (`User`, `Batch`, `Recipe`) — без json-тегов
- **Ошибки** (`ErrInvalidRole`, `ErrRecipeNotFound`, ...)
- **Интерфейсы** (`UserRepo`, `BatchRepo`, `RecipeRepo`)
- **Типы** (`UserRole`)

**Правило:** добавляешь новое поле в API-ответ → меняешь `transport/http/dto.go`. Domain не трогаешь.

### Применённый принцип

#### **Clean Architecture — Dependency Rule**

Robert C. Martin:

> *"Source code dependencies must point only inward, toward higher-level policies."*

Внутренние слои (domain) не должны зависеть от внешних (transport). `json`-теги — это зависимость от конкретного формата сериализации, внешнего для бизнес-логики.

#### **Separation of Concerns**

| Пакет | Отвечает за |
|-------|------------|
| `domain` | бизнес-сущности, правила, контракты |
| `transport/http` | HTTP-шейп запросов/ответов, сериализация |
| `service` | оркестрация бизнес-логики |
| `repository` | SQL, транзакции |

---

## Итоговая архитектура

```
┌──────────────────────────────────────────────────────────┐
│  cmd/ebr-app/main.go            (Composition Root)       │
│  открывает БД → создаёт repos → создаёт services         │
│  → transport.NewRouter(RouterDeps) → запускает srv       │
└──────────────────────┬───────────────────────────────────┘
                       │
        ┌──────────────┼──────────────────┐
        ↓              ↓                  ↓
┌──────────────┐ ┌─────────────┐ ┌─────────────────┐
│  transport   │ │   service   │ │  repository     │
│              │ │             │ │                 │
│  dto.go      │ │  принимает  │ │  SQL +          │
│  AuthHandler │ │  примитивы  │ │  транзакции     │
│  UserHandler │ │  возвращает │ │                 │
│  BatchHandler│ │  *domain.X  │ │  реализует      │
│  RecipeHandlr│ │             │ │  domain.*Repo   │
│  router.go   │ │  НЕТ DTO    │ │                 │
│  middleware  │ │  НЕТ JSON   │ │                 │
│  + Swagger   │ │             │ │                 │
└──────┬───────┘ └──────┬──────┘ └────────┬────────┘
       │                │                 │
       └────────────────┼─────────────────┘
                        ↓
              ┌──────────────────┐
              │     domain       │
              │                  │
              │  User, Batch,    │
              │  Recipe          │
              │  UserRole        │
              │  sentinel errors │
              │  repo interfaces │
              │                  │
              │  ZERO json-теги  │
              │  ZERO infra      │
              └──────────────────┘

   wsserver  ─────  тонкая обёртка lifecycle.
                    NewServer(addr, http.Handler)
```

**Правила слоёв:**

| Слой | Может импортировать | НЕ может импортировать |
|------|--------------------|------------------------|
| `domain` | std lib (`context`, `errors`, `time`) | ничего инфраструктурного, никаких `json`-тегов |
| `repository` | `domain`, `database/sql`, драйверы БД | `service`, `transport` |
| `service` | `domain`, `golang.org/x/crypto`, `jwt` (только в auth) | `database/sql`, `transport`, DTO |
| `transport/middleware` | `domain`, `net/http`, `jwt` | `service`, `repository` |
| `transport/http` | `domain`, `service`, `net/http`, `swaggo` | `repository`, `database/sql` |
| `transport/wsserver` | `net/http` | всё доменное |
| `cmd/*/main.go` | всё | — (точка сборки) |

**Где что живёт:**

| Концепция | Пакет |
|-----------|-------|
| Бизнес-сущности, ошибки, интерфейсы репо | `domain` |
| SQL, транзакции | `repository` |
| Бизнес-логика (примитивы in, entity out) | `service` |
| HTTP-шейп запросов/ответов (DTO с `json`-тегами) | `transport/http` |
| JWT-валидация, типизированные Claims | `transport/middleware` |
| HTTP-сервер lifecycle | `transport/wsserver` |
| Граф зависимостей | `cmd/ebr-app/main.go` |

---

## Что не сделано (на будущее)

1. **`os.Getenv("JWT_SECRET")` в `middleware` и `service/auth.go`.** Конфигурация должна инжектиться через структуру `Config`, а не читаться из окружения по месту.
2. **Смена пароля.** Пользователь не может сменить дефолтный пароль (логин == пароль при создании). Нужен endpoint `PATCH /api/v1/users/me/password`.
3. **Тесты.** Код стал полностью тестируемым — handler struct с интерфейсом, сервисы с примитивами, чистый domain — но реальных тестов пока нет.
4. **Структурированное логирование.** `slog` есть только в `main.go`. Хэндлеры глушат реальные ошибки в `default → 500` без деталей. Нужен middleware-логгер.
5. **`errors.Is` везде.** В `AuthHandler` и `UserHandler` ошибки сравниваются через `==` вместо `errors.Is`. Некритично для sentinel-ошибок сейчас, но сломается при обёртке через `fmt.Errorf("...: %w", err)`.
6. **Swagger нужно регенерировать** после этого рефакторинга — аннотации хэндлеров ссылались на `domain.LoginRequest` и др., теперь DTO в пакете `transport`, имена схем изменились.
7. **CI-проверка свежести Swagger.** `swag init && git diff --exit-code docs/swagger/`.
8. **Автоматизация регенерации диаграмм.** Вынести в Makefile цель `make docs`.

---

## Источники

- Mark Seemann. *Dependency Injection in .NET* — паттерн Composition Root
- Robert C. Martin. *Clean Architecture* — Dependency Rule, layered architecture
- Eric Evans. *Domain-Driven Design* — Aggregate Root, Anti-Corruption Layer
- Martin Fowler. *Patterns of Enterprise Application Architecture* — Repository pattern
- Martin Fowler. *Refactoring: Improving the Design of Existing Code* — Parameter Object, Extract Function
- Alan Donovan, Brian Kernighan. *The Go Programming Language* — Go-идиомы
- Jack Lindamood. *Accept Interfaces, Return Structs* (2016)
- [Go Blog: Context](https://go.dev/blog/context) — best practices для context.Context
- [swaggo/swag](https://github.com/swaggo/swag) — code-first OpenAPI 2.0 генератор для Go
- [OpenAPI Specification](https://swagger.io/specification/) — стандарт описания REST API
