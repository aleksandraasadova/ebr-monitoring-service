# EBR Monitoring Service — Архитектура и документация

## Содержание

1. [Что это такое](#1-что-это-такое)
2. [Стек технологий](#2-стек-технологий)
3. [Архитектурная схема](#3-архитектурная-схема)
4. [Слои приложения](#4-слои-приложения)
5. [Схема базы данных](#5-схема-базы-данных)
6. [Роли и права доступа](#6-роли-и-права-доступа)
7. [Поток данных: от датчика до браузера](#7-поток-данных-от-датчика-до-браузера)
8. [Жизненный цикл партии](#8-жизненный-цикл-партии)
9. [API Reference](#9-api-reference)
10. [Запуск и настройка](#10-запуск-и-настройка)
11. [Тестирование](#11-тестирование)

---

## 1. Что это такое

**EBR Monitoring Service** — это система электронного производственного регламента (Electronic Batch Record) для мониторинга и документирования технологического процесса производства косметических кремов на вакуумном эмульгаторе-гомогенизаторе (VEH-001).

Система решает задачи:
- Регистрация производственных партий по рецептуре
- Электронный журнал взвешивания ингредиентов с e-подписью
- Ведение оператора через 18 стадий технологического процесса
- Телеметрия с датчиков в реальном времени (WebSocket)
- Фиксация отклонений и аварий с комментариями оператора
- Генерация HTML-протокола партии с экспортом в PDF
- Аналитика по партиям и качеству процесса

---

## 2. Стек технологий

| Компонент | Технология |
|-----------|-----------|
| Язык бэкенда | Go 1.25 |
| База данных | PostgreSQL 18 |
| MQTT брокер | Eclipse Mosquitto 2.0 |
| PLC симулятор | Go (отдельный бинарник) |
| WebSocket | gorilla/websocket |
| Аутентификация | JWT (HS256) + bcrypt |
| Фронтенд | Vanilla JS SPA (один HTML-файл) |
| Графики | Chart.js |
| PDF экспорт | html2pdf.js (клиентский) |
| Документация API | Swagger (swaggo) |
| Деплой | Docker Compose |

---

## 3. Архитектурная схема

```
┌─────────────────────────────────────────────────────────────────┐
│                        БРАУЗЕР (SPA)                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌───────────────┐  │
│  │  Логин   │  │ Партии   │  │ Процесс  │  │  Аналитика    │  │
│  │  /auth   │  │ /batches │  │ /stages  │  │  /analytics   │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────┬───────┘  │
│       │             │             │                  │          │
│       └─────────────┴─────────────┴──────────────────┘         │
│                    REST API (HTTP/JSON)          WebSocket /ws   │
└────────────────────────────┬────────────────────────┬───────────┘
                             │ HTTP                    │ WS
┌────────────────────────────▼────────────────────────▼───────────┐
│                     EBR APP (:8080)                              │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               HTTP Transport Layer                       │   │
│  │  AuthH  UserH  RecipeH  BatchH  ProcessH  ReportH  ...  │   │
│  │                 JWT Middleware                            │   │
│  └──────────────────────┬──────────────────────────────────┘   │
│                          │                                       │
│  ┌───────────────────────▼──────────────────────────────────┐  │
│  │                  Service Layer                            │  │
│  │  AuthSvc  UserSvc  RecipeSvc  BatchSvc  ProcessSvc        │  │
│  │  TelemetrySvc  ReportSvc                                  │  │
│  └───────────────────────┬──────────────────────────────────┘  │
│                           │                                      │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │                Repository Layer                           │  │
│  │  UserRepo  RecipeRepo  BatchRepo  ProcessRepo  EventRepo  │  │
│  │  TelemetryRepo  ReportRepo  AnalyticsRepo                 │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                            │ SQL                                 │
│  ┌─────────────────────────▼────────────────────────────────┐  │
│  │                  WebSocket Hub                            │  │
│  │         (broadcast телеметрии всем клиентам)             │  │
│  └──────────────────────────────────────────────────────────┘  │
└──────────────────────┬─────────────────────────────────────────┘
                       │ SQL
           ┌───────────▼──────────┐
           │    PostgreSQL         │
           │    ebr_db (:5433)    │
           └──────────────────────┘

                    MQTT Pipeline
┌───────────────┐          ┌───────────────┐       ┌──────────────┐
│  PLC App      │  MQTT    │   Mosquitto   │  MQTT │  EBR App     │
│  (симулятор)  │─────────►│   Broker      │──────►│  MQTT Client │
│  :1880        │  publish │   (:1883)     │ subs  │  → TelSvc    │
└───────────────┘          └───────────────┘       └──────────────┘
```

---

## 4. Слои приложения

### 4.1 Domain (`internal/domain/`)

Центральный слой — не зависит ни от чего. Содержит бизнес-сущности и ошибки.

```
domain/
  batch.go      — Batch, WeighingLogItem, BatchStage, статусы, ошибки партии
  process.go    — BatchStage, Event, ProcessStage (18 стадий), ошибки процесса
  recipe.go     — Recipe, ошибки рецептуры
  telemetry.go  — NormalizedTelemetry, EquipmentStatus, SensorMeta
  user.go       — User, UserRole (admin/operator), ошибки пользователей
```

Ключевые ошибки домена:

| Ошибка | Смысл |
|--------|-------|
| `ErrRecipeNotFound` | Рецептура не найдена |
| `ErrRecipeArchived` | Рецептура архивирована |
| `ErrInvalidBatchVolume` | Объём партии вне допустимого диапазона |
| `ErrInvalidBatchStatus` | Операция недопустима в текущем статусе |
| `ErrInvalidSignature` | Неверный пароль электронной подписи |
| `ErrEquipmentOffline` | Оборудование не подключено к сети |
| `ErrBatchCompleted` | Партия завершена (все стадии пройдены) |
| `ErrNotProcessOperator` | Только оператор, запустивший процесс, может подписывать стадии |
| `ErrStageAlreadySigned` | Стадия уже подписана |
| `ErrEventNotFound` | Событие не найдено |

### 4.2 Repository (`internal/repository/`)

SQL-запросы к PostgreSQL. Каждый репозиторий — отдельная структура с `*sql.DB`.

```
repository/
  user.go       — Create, GetByID, GetByUserName
  recipe.go     — GetByCode, GetAll, Create, Archive, GetIngredients
  batch.go      — Create, GetByStatus, GetWeighingLog, StartWeighing, ConfirmWeighingItem
  process.go    — CreateStage, GetStagesByBatchID, SignAndCompleteStage, GetBatchIDByCode,
                  StartProcess, CheckProcessOperator, BatchBelongsToUser, CompleteBatch
  event.go      — CreateEvent, GetEventsByBatchID, ResolveEvent
  telemetry.go  — SaveReading, GetStageAggregates, GetLatestByEquipment
  report.go     — SaveReport, GetReport, ListReports, ListReportsByOperator,
                  GetBatchParticipants, GetBatchEquipment, GetUsersByIDs
  analytics.go  — Summary, BatchCountByPeriod, CycleTimes, StatusBreakdown,
                  EventsByStage, EventsPerBatch, AvgHomogenizerTemp
```

### 4.3 Service (`internal/service/`)

Бизнес-логика. Каждый сервис определяет собственные интерфейсы к репозиториям (dependency inversion).

```
service/
  auth.go       — Login (GetByUserName + bcrypt + JWT)
  user.go       — Create (транслитерация ФИО → username, bcrypt пароль)
  recipe.go     — GetByCode, GetAll, Create, Archive
  batch.go      — CreateBatch, GetByStatus, GetWeighingLog, StartWeighing,
                  ConfirmWeighingItem (bcrypt e-подпись)
  process.go    — StartProcess (e-подпись + проверка оборудования + создание стадии),
                  SignStageTransition (e-подпись + только свой оператор + следующая стадия),
                  GetAllStages, GetCurrentStage, CreateEvent, GetEvents, ResolveEvent
  telemetry.go  — ProcessRawTelemetry (MQTT → нормализация → WS broadcast → DB persist),
                  GetEquipmentStatus, GetLatestBySensorCode, SetActiveBatch
  report.go     — GenerateAndSave (сбор всех данных + HTML шаблон), GetReport, ListReports
```

### 4.4 Transport (`internal/transport/`)

```
transport/
  http/
    router.go       — регистрация всех маршрутов + middleware
    auth.go         — POST /api/v1/auth/login
    user.go         — POST /api/v1/users
    recipe.go       — GET/POST/DELETE /api/v1/recipes
    batch.go        — CRUD партий, взвешивание
    process.go      — старт/подпись стадий, события
    report.go       — генерация и список протоколов
    analytics.go    — GET /api/v1/analytics
    telemetry.go    — текущие показания датчиков, статус оборудования
    dto.go          — все Request/Response структуры
  middleware/
    auth.go         — JWT из Authorization header или ?token= query
  mqtt/
    client.go       — MQTT подключение, подписка, диспетчеризация сообщений
    registry.go     — маппинг MQTT-топиков на обработчики
  wsserver/
    hub.go          — WebSocket Hub (broadcast всем подключённым клиентам)
    server.go       — HTTP-сервер с поддержкой WebSocket
```

---

## 5. Схема базы данных

```
┌──────────────────┐      ┌──────────────────────┐
│     users        │      │      ingredients      │
├──────────────────┤      ├──────────────────────┤
│ id               │      │ id                   │
│ user_code (OP-*) │      │ name                 │
│ username         │      │ unit (г)             │
│ password_hash    │      └──────────────────────┘
│ role             │              │
│ full_name        │              │ N
│ is_active        │      ┌───────▼──────────────┐
└──────┬───────────┘      │  recipe_ingredients  │
       │                  ├──────────────────────┤
       │ registered_by    │ recipe_id ──────────────────┐
       │ operator_id      │ ingredient_id               │
       │                  │ stage_key                   │
┌──────▼───────────┐      │ percentage                  │
│     batches      │      └──────────────────────┘      │
├──────────────────┤                              ┌──────▼────────┐
│ id               │                              │    recipes    │
│ batch_code (auto)│◄─────────────────────────────┤ id            │
│ recipe_id        │                              │ recipe_code   │
│ target_volume_l  │                              │ name, version │
│ equipment_id     │                              │ min/max_volume│
│ status           │                              │ is_active     │
│ registered_by    │                              └───────────────┘
│ operator_id      │
│ created_at       │
│ started_at       │    ┌─────────────────┐
│ completed_at     │    │    equipment    │
└──────┬───────────┘    ├─────────────────┤
       │ batch_id       │ equipment_code  │
       │                │ name, type      │
       ├──────────────► │ status          │◄─── batch.equipment_id
       │                │ last_seen_at    │
       │                └────────┬────────┘
       │                         │
       │                ┌────────▼────────┐
       │                │    sensors      │
       │                ├─────────────────┤
       │                │ equipment_id    │
       │                │ sensor_code     │
       │                │ name, type      │
       │                │ mqtt_topic      │
       │                └─────────────────┘
       │
       ├──── weighing_log (ингредиент + фактическое кол-во + подпись)
       ├──── batch_stages (стадия + кто/когда подписал)
       ├──── events (отклонения + комментарии оператора)
       ├──── telemetry (показания датчиков с временной меткой)
       ├──── batch_reports (HTML-снимок протокола)
       └──── audit_log (кто что изменил)
```

### Статусы партии

```
waiting_weighing
      │
      ▼ StartWeighing (оператор)
weighing_in_progress
      │
      ▼ ConfirmWeighingItem × N (e-подпись, последний элемент)
ready_for_process
      │
      ▼ StartProcess (e-подпись + оборудование online)
in_process
      │
      ▼ SignStageTransition × 18 (e-подпись, последняя стадия)
completed
```

---

## 6. Роли и права доступа

### Таблица прав

| Действие | admin | operator |
|----------|:-----:|:--------:|
| **Аутентификация** | | |
| POST /auth/login | ✅ | ✅ |
| **Пользователи** | | |
| POST /users (создать оператора) | ✅ | ❌ |
| **Рецептуры** | | |
| GET /recipes (список всех) | ✅ | ❌ |
| GET /recipes/{code} (по коду) | ✅ | ✅ |
| POST /recipes (создать) | ✅ | ❌ |
| DELETE /recipes/{code} (архивировать) | ✅ | ❌ |
| GET /ingredients (справочник) | ✅ | ❌ |
| **Партии** | | |
| POST /batches (создать) | ❌ | ✅ |
| GET /batches?status=... (список) | ✅ | ✅ |
| GET /batches/{code}/weighing | ✅ | ✅ |
| POST /batches/{code}/weighing/start | ❌ | ✅ |
| POST /batches/{code}/weighing/{id}/confirm | ❌ | ✅ |
| **Технологический процесс** | | |
| POST /batches/{code}/process/start | ❌ | ✅ |
| POST /batches/{code}/process/sign | ❌ | ✅* |
| GET /batches/{code}/process/stages | ✅ | ✅ |
| GET /batches/{code}/process/current | ✅ | ✅ |
| **События** | | |
| POST /batches/{code}/events | ✅ | ✅ |
| GET /batches/{code}/events | ✅ | ✅ |
| POST /events/{id}/resolve | ✅ | ✅ |
| **Протоколы** | | |
| GET /batches/{code}/report | ✅ | ✅** |
| GET /reports (список) | ✅ | ✅** |
| **Аналитика** | | |
| GET /analytics | ✅ (все партии) | ✅ (свои партии) |
| **Телеметрия** | | |
| GET /telemetry/sensor/{code}/current | ✅ | ✅ |
| GET /equipment/{code}/status | ✅ | ✅ |
| GET /ws/telemetry (WebSocket) | ✅ | ✅ |

> **\*** Подписывать стадии может только **тот оператор, который запустил процесс** (поле `batches.operator_id`).
>
> **\*\*** Оператор видит только протоколы партий, в которых он участвовал (`registered_by` или `operator_id`).

### Электронная подпись (e-signature)

Следующие операции требуют повторного ввода пароля:

```
ConfirmWeighingItem   — подтверждение взвешивания ингредиента
StartProcess          — запуск технологического процесса
SignStageTransition   — переход к следующей стадии
```

Механизм: пользователь вводит пароль → сервис запрашивает хэш из БД → `bcrypt.CompareHashAndPassword`.

### Генерация учётных данных

При создании оператора через `POST /api/v1/users`:

```
Входные данные:   Иванов Иван Иванович
↓
username:         ivanov.iv.iv   (транслитерация: фамилия.2б-имени.2б-отчества)
password:         ivanov.iv.iv   (= username, устанавливается администратором)
user_code:        OP-001         (автоматически, sequence в БД)
```

Оператор обязан сменить пароль при первом входе (это доработка будущих версий).

---

## 7. Поток данных: от датчика до браузера

```
PLC Симулятор (cmd/plc-app)
│
│  Публикует данные по MQTT-топикам каждые ~1с:
│  ebr/equipment/VEH-001/sensor/main_pot_temp  → {"value": 78.3}
│  ebr/equipment/VEH-001/sensor/homogenizer_rpm → {"value": 2000}
│  ebr/equipment/VEH-001/heartbeat             → {"status": "available"}
│
▼
Eclipse Mosquitto (mqtt-broker :1883)
│
▼
EBR App — MQTT Client (transport/mqtt/)
│
│  TopicRegistry.dispatch(topic, payload)
│  → TelemetryService.ProcessRawTelemetry(topic, payload)
│
▼
TelemetryService
│  1. Нормализует значение (определяет SensorMeta по топику)
│  2. Обновляет in-memory кэш latest[sensorCode]
│  3. Если активная партия (activeBatch != nil) и прошло ≥5с:
│     → TelemetryRepo.SaveReading() → запись в telemetry (PostgreSQL)
│  4. Если heartbeat: обновляет equipment["VEH-001"].Status
│  5. broadcaster.Broadcast(json) → WebSocket Hub
│
▼
WebSocket Hub (wsserver/)
│  Рассылает JSON всем подключённым браузерам:
│  {
│    "sensor_code": "MP-TEMP-03",
│    "parameter_type": "temperature",
│    "value": 78.3,
│    "unit": "C",
│    "equipment_code": "VEH-001",
│    "measured_at": "2026-05-13T10:00:00Z"
│  }
│
▼
Браузер (SPA)
│  ws.onmessage → updateTelemetryCard(data)
│  → Отображает карточку датчика
│  → checkClientThreshold(data) → showAlertBar() при отклонении
```

### Датчики VEH-001

| Топик | Код | Тип | Единица |
|-------|-----|-----|---------|
| `.../water_pot_temp` | WP-TEMP-01 | temperature | °C |
| `.../oil_pot_temp` | OP-TEMP-02 | temperature | °C |
| `.../main_pot_temp` | MP-TEMP-03 | temperature | °C |
| `.../water_pot_mixer_rpm` | WP-MIXER-01 | mixer_rpm | rpm |
| `.../oil_pot_mixer_rpm` | OP-MIXER-02 | mixer_rpm | rpm |
| `.../main_pot_mixer_rpm` | MP-MIXER-03 | mixer_rpm | rpm |
| `.../main_pot_homogenizer_rpm` | MP-HOMOG-01 | homogenizer_rpm | rpm |
| `.../main_pot_vacuum` | MP-VACUUM-01 | vacuum | MPa |
| `.../water_pot_weight` | WP-WEIGHT-01 | weight | kg |
| `.../oil_pot_weight` | OP-WEIGHT-02 | weight | kg |
| `.../main_pot_weight` | MP-WEIGHT-03 | weight | kg |
| `ebr/equipment/SCALES-001/sensor/weight` | SCALE-WEIGHT-01 | weight | g |

---

## 8. Жизненный цикл партии

### 18 стадий эмульгирования

```
  №   Ключ                      Название                    Датчики
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  1   water_pot_feeding          Загрузка водной фазы        WP-WEIGHT-01
  2   oil_pot_feeding            Загрузка масляной фазы      OP-WEIGHT-02
  3   main_pot_vacuumize         Вакуумирование              MP-VACUUM-01
  4   water_pot_heating          Нагрев водной фазы (→80°C)  WP-TEMP-01
  5   oil_pot_heating            Нагрев масляной фазы (→80°C)OP-TEMP-02
  6   main_pot_preheating        Предварительный нагрев      MP-TEMP-03
  7   main_pot_water_feeding     Подача водной фазы          WP-WEIGHT-01
  8   main_pot_pre_blending      Предварительное смешение    MP-HOMOG-01
  9   main_pot_vacuum_drawing_1  Вакуумирование 1            MP-VACUUM-01
  10  main_pot_oil_feeding       Подача масляной фазы        MP-TEMP-03
  11  main_pot_vacuum_drawing_2  Вакуумирование 2            MP-VACUUM-01
  12  emulsifying_speed_2        Эмульгирование — скорость 2 MP-HOMOG-01 (2000 rpm)
  13  emulsifying_speed_3        Эмульгирование — скорость 3 MP-HOMOG-01, MP-TEMP-03
  14  cooling_start              Начало охлаждения           MP-TEMP-03
  15  cooling_blending           Охлаждение при перемешивании MP-TEMP-03
  16  additive_feeding           Внесение добавок (≤35°C)    MP-TEMP-03
  17  final_blending             Финальное перемешивание     MP-HOMOG-01
  18  cooling_finish             Завершение, контроль pH     MP-TEMP-03
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Пороговые значения (критические алерты)

| Датчик | Мин | Макс | Стадии | Тип |
|--------|-----|------|--------|-----|
| WP-TEMP-01 | 75°C | 85°C | 4, 5 | critical |
| OP-TEMP-02 | 75°C | 85°C | 4, 5 | critical |
| MP-TEMP-03 | 75°C | 85°C | 12, 13 | critical |
| MP-TEMP-03 | — | 35°C | 16 | critical |
| MP-HOMOG-01 | 1800 rpm | — | 12, 13 | warning |
| MP-VACUUM-01 | -0.05 MPa | — | 3, 9, 11 | warning |

### Рецептура крема (тестовая партия 2000г = 100%)

| Ингредиент | % | Фаза |
|-----------|---|------|
| Масло виноградных косточек | 5.0 | oil_phase |
| Кокосовое масло | 5.0 | oil_phase |
| МГД (моноглицериды) | 5.0 | oil_phase |
| Эмульсионный воск | 4.0 | oil_phase |
| Ланолин безводный | 3.0 | oil_phase |
| Кремофор А25 | 2.0 | oil_phase |
| Глицерин | 3.0 | water_phase |
| ТЭА | 0.5 | water_phase |
| Вода очищенная | 65.6 | water_phase |
| Бадана экстракт | 2.0 | additive |
| Салициловая кислота | 2.0 | additive |
| Диметикон | 2.0 | additive |
| Ментол | 0.1 | additive |
| Октопирокс | 0.1 | additive |
| Эуксил 9010 | 0.5 | additive |
| Эфирное масло | 0.2 | additive |

---

## 9. API Reference

Полная документация доступна в Swagger UI: `http://localhost:8080/swagger/`

### Аутентификация

```http
POST /api/v1/auth/login
Content-Type: application/json

{"username": "admin01", "password": "admin01"}

→ 200 {"token": "eyJ...", "role": "admin", "user_code": "ADM-001", ...}
```

### Пользователи

```http
POST /api/v1/users                  (только admin)
{"role": "operator", "surname": "Иванов", "name": "Иван", "father_name": "Иванович"}
→ 201 {"user_code": "OP-002", "user_name": "ivanov.iv.iv"}
```

### Рецептуры

```http
GET  /api/v1/recipes                (admin)        → список всех
GET  /api/v1/recipes/{code}         (admin+operator)→ по коду
POST /api/v1/recipes                (admin)        → создать
DELETE /api/v1/recipes/{code}       (admin)        → архивировать
GET  /api/v1/ingredients            (admin)        → справочник ингредиентов
```

### Партии

```http
POST /api/v1/batches                (operator)     → создать партию
GET  /api/v1/batches?status=...     (admin+operator)→ список по статусу
GET  /api/v1/batches/{code}/weighing               → журнал взвешивания
POST /api/v1/batches/{code}/weighing/start (operator)→ начать взвешивание
POST /api/v1/batches/{code}/weighing/{id}/confirm  → подтвердить (e-подпись)
    Body: {"actual_qty": 100.5, "signature_password": "ivanov.iv.iv"}
```

### Процесс

```http
POST /api/v1/batches/{code}/process/start (operator)
    Body: {"password": "ivanov.iv.iv"}
    → 204 OK | 409 equipment offline | 403 wrong password

POST /api/v1/batches/{code}/process/sign  (operator, только тот кто запустил)
    Body: {"password": "ivanov.iv.iv"}
    → 204 стадия подписана | 200 {"completed":true} последняя стадия

GET  /api/v1/batches/{code}/process/stages → все стадии
GET  /api/v1/batches/{code}/process/current → текущая стадия
```

### События

```http
POST /api/v1/batches/{code}/events
    Body: {"type": "deviation", "severity": "warning", "description": "..."}
    → 201 {"id": 1, "stage_key": "water_pot_heating", ...}

GET  /api/v1/batches/{code}/events → список событий

POST /api/v1/events/{id}/resolve
    Body: {"comment": "Объяснение оператора"}
    → 204
```

### Протоколы

```http
GET /api/v1/batches/{code}/report   → HTML протокол (генерирует если нет)
GET /api/v1/reports                 → список (admin=все, operator=свои)
```

### Аналитика

```http
GET /api/v1/analytics?days=30
→ {
    "summary": {"total_batches": 10, "completed_batches": 7, "avg_cycle_hours": 3.2},
    "batch_by_day": [...],
    "cycle_times": [...],
    "status_breakdown": [...],
    "events_by_stage": [...],
    "events_per_batch": [...],
    "avg_homog_temp": [...]
  }
```

### Телеметрия и оборудование

```http
GET /api/v1/telemetry/sensor/{code}/current → последнее показание датчика
GET /api/v1/equipment/{code}/status        → статус оборудования (online/offline)
GET /ws/telemetry?token={jwt}              → WebSocket поток телеметрии
```

---

## 10. Запуск и настройка

### Требования

- Go 1.25+
- Docker + Docker Compose
- Make (опционально)

### Быстрый старт

```bash
# 1. Запустить PostgreSQL и MQTT брокер
docker compose up -d ebr-postgres mqtt-broker

# 2. Применить миграции (из корня проекта)
docker run --rm -v $(pwd)/migrations:/migrations \
  --network host migrate/migrate:v4.19.1 \
  -path=/migrations \
  -database "$DB_URL" \
  up

# 3. Запустить основное приложение
go run ./cmd/ebr-app/

# 4. (Опционально) Запустить PLC-симулятор
go run ./cmd/plc-app/
# В терминале PLC:
#   1 — симуляция весов (SCALE-WEIGHT-01)
#   2 — heartbeat оборудования VEH-001 (equipment = available)
#   3 — симуляция 18 стадий эмульгирования (5 мин каждая)
```

### Переменные окружения (.env)

```env
DB_URL=postgres://ebr_user:ebr_pass@localhost:5433/ebr_db?sslmode=disable
JWT_SECRET=your-secret-key
MQTT_BROKER=tcp://localhost:1883
CLIENT_ID=ebr-app
```

### Миграции

```
migrations/
  000001_init.up.sql          — таблица users
  000002_admin.up.sql         — начальный admin01
  000003_usercode_index.up.sql— индекс на user_code
  000004_ingredients_recipes.up.sql — ингредиенты, рецепты
  000005_batches.up.sql       — partii, weighing_log, equipment, batch_code trigger
  000006_equipment.up.sql     — оборудование VEH-001, SCALES-001
  000007_sensors.up.sql       — 12 датчиков VEH-001 + SCALES-001
  000008_process.up.sql       — batch_stages, telemetry, events, audit_log, batch_reports
```

---

## 11. Тестирование

### E2E тесты

Тесты в `test/e2e/` — честные интеграционные тесты через реальный HTTP-сервер и реальную БД (без моков).

```bash
# Убедиться что DB запущена и мигрирована, потом:
DB_URL="$DB_URL" \
go test ./test/e2e/ -v -count=1 -timeout 60s
```

Что тестируется:

| Файл | Тест-кейсы |
|------|-----------|
| `auth_test.go` | Логин успешный, неверный пароль, неизвестный юзер, пустое тело, невалидный токен, поля ответа |
| `user_test.go` | Создание оператора, запрет для оператора, вход с автогенерированными кредами, запрет на список рецептур |
| `recipe_test.go` | Получение по коду, несуществующая рецептура, список для admin, запрет для operator, создание+архивирование, справочник ингредиентов |
| `batch_test.go` | Создание партии, неверный объём, несуществующая рецептура, запрет для admin, список по статусу, полный жизненный цикл взвешивания с e-подписью, неверный пароль при взвешивании |
| `process_test.go` | Старт без оборудования (→409), неверный пароль (→403), запрет для admin, стадии до старта (пусто), события: создание+список+разрешение |
| `analytics_test.go` | Admin видит всё, оператор видит своё, структура ответа, 401 без токена |
| `report_test.go` | Список (может быть пуст), генерация HTML, кеширование, доступ оператора только к своим партиям, admin доступ к любой |

### Логика cleanup

После всех тестов `cleanupTestData()`:
1. Удаляет партии по test recipe (cascade: weighing_log, batch_stages, events, telemetry, reports)
2. Удаляет саму test recipe
3. Удаляет всех пользователей с `full_name LIKE 'E2E %'`

### Почему нет моков

Тесты намеренно работают с реальной БД и реальными HTTP-вызовами, потому что:
- Мок-тесты могут пройти, а реальное взаимодействие — нет (как это и случалось)
- SQL-триггеры (batch_code, recipe_code), CASCADE DELETE, FOR UPDATE блокировки — всё это проверяется только с реальной БД
- Bcrypt e-подпись, JWT генерация и парсинг — проверяется реальным стеком
