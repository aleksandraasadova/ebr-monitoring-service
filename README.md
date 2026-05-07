# iObserve EBR — Electronic Batch Records

**Система мониторинга производства косметических кремов.**

Цифровой журнал производственных серий, устраняющий человеческий фактор при документировании технологических процессов. Каждое измерение — взвешивание ингредиента, температура реактора, время операции — фиксируется автоматически, верифицируется относительно рецептуры и сохраняется с полным аудит-треком.

---

## Цели проекта

**Практическая:** обеспечить прослеживаемость данных и их достоверность на каждом этапе производства, исключив человеческий фактор при документировании.

**Учебная:** написать бэкенд на Go, организованный в соответствии с трёхслойной архитектурой — `domain` (ядро без внешних зависимостей) → `repository` → `service` → `transport`.

---

## Реализовано

- Авторизация через JWT-токен, разграничение доступа по ролям (`operator`, `admin`)
- Приём телеметрии от симулятора ПЛК через MQTT-брокер (Mosquitto)
- Валидация параметров: сравнение фактических значений с рецептурой, генерация алертов при отклонениях
- Аудит через триггерную функцию в PostgreSQL — каждое изменение строки фиксируется автоматически
- Триггер на вставку уникальных кодов, рецептур, партий, логинов
- Автоматическая сборка итогового протокола партии по завершении процесса
- HTTP-сервер на `net/http` с REST API: управление рецептурами, история партий, экспорт отчётов, управление пользователями
- WebSocket-эндпоинт для передачи телеметрии в реальном времени
- Пользовательские интерфейсы оператора и администратора (HTML / CSS / JS, без фреймворков)

**Практическая направленность:** реализован типовой сценарий промышленного IoT-мониторинга — асинхронный сбор данных с источников, их обработка и сохранение для анализа. Архитектурно это соответствует паттернам работы с потоковыми данными и демонстрирует переход от бумажного учёта к автоматизированному.

---

## Стек

| Компонент | Технология |
|-----------|-----------|
| Язык | **Go 1.25**, стандартная библиотека `net/http` |
| База данных | PostgreSQL 18 |
| Миграции | golang-migrate v4 |
| Аутентификация | JWT — `golang-jwt/jwt/v5` |
| Логирование | Uber Zap |
| MQTT | Eclipse Mosquitto 2.0 — телеметрия оборудования |
| Контейнеризация | Docker, Docker Compose |
| API | REST + WebSocket, JSON |
| Фронтенд | Vanilla HTML / CSS / JS — static files |

---

## Архитектура

### C4 Level 1 — System Context

> Кто пользуется системой и с какими внешними системами она взаимодействует.

![C4 Context](docs/C4-Context.png)

---

### C4 Level 2 — Containers

> Из каких контейнеров состоит система и как они общаются.

![C4 Container](docs/C4-Container.png)

---

### C4 Level 3 — Components

> Внутреннее устройство Go-сервиса: хендлеры, сервисы, репозитории.

![C4 Component](docs/C4-Component.png)

---

### Sequence — Регистрация партии

> Полный путь запроса `POST /api/v1/batches` через все слои.

![Sequence Register Batch](docs/seq-register-batch.png)

---

### Sequence — Приём MQTT-телеметрии

> Как данные с оборудования попадают в систему и генерируют алерты.

![Sequence MQTT](docs/seq-mqtt-telemetry.png)

---

### Жизненный цикл партии

> Все статусы партии от регистрации до завершения или отклонения.

![Batch Lifecycle](docs/batch-lifecycle.png)

---

## Скриншоты

### 1. Вход в систему
`POST /api/v1/auth/login` — проверка логина/пароля, выдача JWT с ролью

![Login](docs/screenshots/01-login.jpg)

---

### 2. Панель оператора — дашборд
Агрегация активных партий, журнал событий, быстрые действия

![Dashboard](docs/screenshots/02-dashboard.jpg)

---

### 3. Навигация — раздел «Партии»
Роутинг на клиенте, JWT передаётся в каждом запросе

![Navigation](docs/screenshots/03-nav.jpg)

---

### 4. Список партий
`GET /api/v1/batches` — фильтрация по статусу, пагинация

![Batch List](docs/screenshots/04-batch-list.jpg)

---

### 5. Регистрация партии — пустая форма
Ожидание ввода кода рецептуры

![Register Empty](docs/screenshots/05-register-empty.jpg)

---

### 6. Регистрация партии — рецептура найдена
`GET /api/v1/recipes/{code}` — frontend получает название, версию, описание, тип оборудования.  
`min_volume_l` / `max_volume_l` сохраняются локально для валидации объёма до отправки формы.

![Register Filled](docs/screenshots/06-register-filled.jpg)

---

### 7. Партия зарегистрирована
`POST /api/v1/batches` — валидация объёма в сервисном слое, создание в транзакции, `registered_by` из JWT-клеймов

![Register Result](docs/screenshots/07-register-result.jpg)

---

### 8. Взвешивание ингредиентов
`POST /api/v1/batches/{id}/weighing` — пофазовая фиксация масс, сравнение с нормой рецептуры, алерт при отклонении > допуска

![Weighing](docs/screenshots/08-weighing.jpg)

---

### 9. Аналитика
`GET /api/v1/analytics` — KPI за период, объём по неделям, распределение по статусам и рецептурам, производительность операторов

![Analytics](docs/screenshots/09-analytics.jpg)

---

### 10. База рецептур
`GET /api/v1/recipes` — список с версиями, допустимыми объёмами, типом оборудования и статусом архива

![Recipes](docs/screenshots/10-recipes.jpg)

---

### 11. Эмульгирование — пошаговый процесс
`POST /api/v1/batches/{id}/steps` — регистрация каждого технологического шага (нагрев, гомогенизация, охлаждение, pH), реальные данные из MQTT-телеметрии

![Emulsification](docs/screenshots/11-emulsification.jpg)

---

### 12. Панель уведомлений
`GET /api/v1/notifications` — отклонения при взвешивании, требуемые подтверждения, завершённые партии

![Notifications](docs/screenshots/12-notifications.jpg)

---

## API

Все защищённые эндпоинты требуют:
```
Authorization: Bearer <JWT>
```

### Аутентификация

#### `POST /api/v1/auth/login`

**Request:**
```json
{ "username": "ivanova_sv", "password": "secret" }
```
**Response `200`:**
```json
{
  "role": "operator", "token": "eyJ...",
  "user_code": "OP-001", "user_name": "ivanova_sv",
  "full_name": "Иванова Светлана Викторовна", "is_active": true
}
```
| Код | Причина |
|-----|---------|
| `401` | Неверный логин или пароль |
| `403` | Аккаунт деактивирован |

---

### Пользователи

#### `POST /api/v1/users` — роль: `admin`

Система автоматически генерирует `user_code` и `user_name` (транслитерация ФИО).

**Request:**
```json
{ "role": "operator", "surname": "Иванова", "name": "Светлана", "father_name": "Викторовна" }
```
**Response `201`:**
```json
{ "user_code": "OP-004", "user_name": "ivanova_sv" }
```

---

### Рецептуры

#### `GET /api/v1/recipes/{code}` — роли: `admin`, `operator`

Архивированные рецептуры возвращают `404` — статус архива скрыт от клиента.

**Response `200`:**
```json
{
  "name": "Крем увлажняющий Aqua Plus", "version": "3.2",
  "min_volume_l": 50, "max_volume_l": 300,
  "description": "Лёгкий увлажняющий крем для всех типов кожи",
  "required_equipment_type": "Гомогенизатор GH-500"
}
```

---

### Партии

#### `POST /api/v1/batches` — роль: `operator`

`registered_by` — из JWT, не от клиента. Создание в транзакции.

**Request:**
```json
{ "recipe_code": "REC-001", "target_volume_l": 100 }
```
**Response `201`:**
```json
{
  "batch_code": "ПР-2026-016", "batch_status": "ЗАРЕГИСТРИРОВАНА",
  "created_at": "2026-05-06T23:38:23Z", "registered_by": 12
}
```
| Код | Причина |
|-----|---------|
| `400` | Объём вне диапазона `[min_volume_l, max_volume_l]` |
| `404` | Рецептура не найдена или в архиве |
| `401` | Невалидный токен |
| `403` | Роль не `operator` |

#### `GET /api/v1/batches` — роли: `admin`, `operator`
Список партий. Query params: `?status=ЗАРЕГИСТРИРОВАНА&limit=20&offset=0`

#### `POST /api/v1/batches/{id}/weighing` — роль: `operator`
Фиксация взвешивания ингредиентов пофазово. Валидация отклонения от нормы.

#### `POST /api/v1/batches/{id}/steps` — роль: `operator`
Регистрация шага технологического процесса.

#### `PATCH /api/v1/batches/{id}/status` — роль: `operator`, `admin`
Смена статуса партии.

#### `GET /api/v1/batches/{id}/report` — роль: `admin`
Экспорт итогового протокола партии.

---

## База данных

| Миграция | Содержание |
|----------|-----------|
| `000001_init` | `users`: роли `admin`/`operator`, bcrypt-хэш, `is_active` |
| `000002_admin` | Seed первого администратора |
| `000003_usercode_index` | Уникальный индекс по `user_code` |
| `000004_ingredients_recipes` | `recipes`, `ingredients` |
| `000005_batches` | `batches`, статусы, FK на `recipes` и `users` |
| `000006_equipment` | `equipment`, поверки, привязка к партиям |

Аудит реализован через триггерную функцию PostgreSQL — каждое изменение строки автоматически записывается в таблицу аудита с временной меткой и идентификатором пользователя.

---

## TODO

### Бэкенд
- [ ] `GET /api/v1/batches/{id}/report` — экспорт итогового протокола
- [ ] `GET /api/v1/analytics` — KPI и агрегаты
- [ ] `GET /api/v1/notifications` — уведомления оператора
- [ ] Audit trigger — реализация триггерной функции

### Инфраструктура
- [ ] Health-check эндпоинты `/health`, `/ready`
- [ ] Structured logging Zap во все слои


---

## Runbook

### Требования
- Docker + Docker Compose
- Go 1.25+
- `.env` файл в корне проекта

### `.env`
```env
PROJECT_ROOT=/абсолютный/путь/до/проекта

POSTGRES_DB=ebr
POSTGRES_USER=ebr_user
POSTGRES_PASSWORD=ebr_pass
DATABASE_URL=ebr_pass@localhost4:ebr_pass@localhost5:ebr_pass@localhost6:ebr_pass@localhost7/ebr?sslmode=disable

JWT_SECRET=your-secret-key-min-32-chars
SERVER_ADDR=:8080
```

### Запуск

```bash
# 1. Поднять PostgreSQL и MQTT-брокер
docker compose up -d ebr-postgres mqtt-broker

# 2. Применить миграции
docker compose run --rm ebr-postgres-migrate \
  -path=/migrations \
  -database "${DATABASE_URL}" \
  up

# 3. Запустить сервис
go run ./cmd/...
```

Сервис: `http://localhost:8080` — страница входа.

### Утилиты

```bash
# Откатить последнюю миграцию
docker compose run --rm ebr-postgres-migrate \
  -path=/migrations -database "${DATABASE_URL}" down 1

# Подключиться к БД
docker exec -it ebr-env-postgres psql -U ebr_user -d ebr

# Логи сервиса
go run ./cmd/... 2>&1 | tee service.log
```

---

## Структура проекта

```
ebr-monitoring-service/
├── cmd/      
│   ├── ebr-app/     # main.go — точка входа      
│   └── plc-app/     # main.go — точка входа
│       └── internal
│           ├── plc
│           ├── sensor
│           └── simulations 
├── config/mqtt/            # mosquitto.conf
├── docs/                   # Диаграммы и скриншоты
│   ├── arch-system.png
│   ├── arch-layers.png
│   ├── arch-flow.png
│   └── screenshots/
├── internal/
│   ├── domain/             # Сущности, интерфейсы, ошибки — без внешних зависимостей
│   ├── repository/         # SQL-реализации интерфейсов domain
│   ├── service/            # Бизнес-логика, оркестрация
│   └── transport/
│       ├── http/           # HTTP-хендлеры (один файл — один домен)
│       ├── middleware/     # JWT-валидация + RBAC
│       └── wsserver/       # Сборка роутера, graceful shutdown
├── migrations/             # SQL up/down
├── web/                    # login.html, operator.html, admin.html
├── docker-compose.yaml
└── go.mod
```

---

## Роли

| Роль | Доступ |
|------|--------|
| `admin` | `POST /api/v1/users`, все `GET`-эндпоинты, экспорт отчётов |
| `operator` | `GET /api/v1/recipes/{code}`, `POST /api/v1/batches`, регистрация шагов |
