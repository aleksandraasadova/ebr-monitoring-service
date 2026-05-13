# Модуль контроля и анализа отклонений (EBR Monitoring Service)

## Содержание

1. [Обзор и назначение](#1-обзор-и-назначение)
2. [Архитектура модуля](#2-архитектура-модуля)
3. [Транспортный слой: MQTT → TelemetryService](#3-транспортный-слой-mqtt--telemetryservice)
4. [Пороговые значения и детектор отклонений](#4-пороговые-значения-и-детектор-отклонений)
5. [Условия подписи стадии](#5-условия-подписи-стадии)
6. [Доставка данных на фронтенд](#6-доставка-данных-на-фронтенд)
7. [Автоматические события и их жизненный цикл](#7-автоматические-события-и-их-жизненный-цикл)
8. [Хранение телеметрии в PostgreSQL](#8-хранение-телеметрии-в-postgresql)
9. [Данные в протоколе партии](#9-данные-в-протоколе-партии)
10. [Тестовые сценарии отклонений](#10-тестовые-сценарии-отклонений)
11. [Схема таблиц](#11-схема-таблиц)

---

## 1. Обзор и назначение

Модуль реализует три взаимосвязанные задачи:

1. **Real-time мониторинг** — получение показаний с датчиков VEH-001 и сравнение с технологическими допусками во время активной стадии партии.
2. **Контроль подтверждения стадий** — оператор не может подписать стадию, пока показания датчиков не попадают в допустимый диапазон, что предотвращает нарушение технологического регламента.
3. **Автоматическая фиксация отклонений** — при устойчивом нарушении допуска (≥ 30 сек) система создаёт событие в БД, которое затем попадает в протокол EBR.

---

## 2. Архитектура модуля

```
PLC-симулятор (cmd/plc-app)
      │  publishes every 2s via MQTT
      ▼
Eclipse Mosquitto (:1883)
      │  topic: ebr/equipment/VEH-001/sensor/*
      ▼
mqtt.Client → mqtt.TopicRegistry.dispatch()
      │
      ▼
TelemetryService.ProcessRawTelemetry()
      │
      ├─── normalizeNumericTelemetry()   // string → float64, validate
      │
      ├─── s.latest[sensorCode] = *reading  // in-memory cache
      │
      ├─── broadcaster.Broadcast(json)   // WebSocket Hub → все браузеры
      │
      ├─── TelemetryRepo.SaveReading()   // persist to telemetry table (каждые 5с при активной партии)
      │
      └─── checkDeviations()             // threshold check → auto event
                │
                └─── EventCreator.CreateEventRaw()  // → ProcessService → EventRepo → DB

Браузер
  ├── WebSocket /ws/telemetry (primary)
  └── GET /api/v1/telemetry/all (REST fallback, poll 2s)
        → telemetryCache[sensorCode] = msg
        → updateStageMonitoring()   // обновить sensor-chip + condition-row + sign button
```

---

## 3. Транспортный слой: MQTT → TelemetryService

### MQTT-топики датчиков VEH-001

Маппинг топиков → метаданные датчика задан в `buildSensorMap()` (`service/telemetry.go`):

| MQTT-топик | Код датчика | Тип | Единица |
|-----------|------------|-----|---------|
| `ebr/equipment/VEH-001/sensor/water_pot_temp` | WP-TEMP-01 | temperature | °C |
| `ebr/equipment/VEH-001/sensor/oil_pot_temp` | OP-TEMP-02 | temperature | °C |
| `ebr/equipment/VEH-001/sensor/main_pot_temp` | MP-TEMP-03 | temperature | °C |
| `ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm` | WP-MIXER-01 | mixer_rpm | rpm |
| `ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm` | OP-MIXER-02 | mixer_rpm | rpm |
| `ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm` | MP-HOMOG-01 | homogenizer_rpm | rpm |
| `ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm` | MP-SCRAPER-01 | mixer_rpm | rpm |
| `ebr/equipment/VEH-001/sensor/main_pot_vacuum` | MP-VACUUM-01 | vacuum | MPa |
| `ebr/equipment/VEH-001/sensor/main_pot_weight` | MP-WEIGHT-03 | weight | g |
| `ebr/equipment/VEH-001/sensor/water_pot_weight` | WP-WEIGHT-01 | weight | g |
| `ebr/equipment/VEH-001/sensor/oil_pot_weight` | OP-WEIGHT-02 | weight | g |
| `ebr/sensor/weighing_scale_01` | SCALE-WEIGHT-01 | weight | g |

### Нормализация

`normalizeNumericTelemetry()` принимает сырой MQTT-payload (строка с числом), парсит `strconv.ParseFloat`, применяет `Scale` и `Offset` из метаданных датчика. Отрицательные значения разрешены (вакуумные датчики публикуют значения типа `−0.05 МПа`).

### Персистентность

Запись в таблицу `telemetry` происходит не при каждом MQTT-сообщении, а **не чаще 1 раза в 5 секунд** на датчик (`lastSaved[sensorCode]`). Условия записи:

- Партия активна (`activeBatch != nil`)
- Прошло ≥ 5 секунд с последней записи для данного датчика

Это снижает нагрузку на БД с ~6 записей/сек до ~0.2 записи/сек при 12 активных датчиках.

### Статус оборудования

Heartbeat от PLC (`ebr/equipment/VEH-001/status`) обрабатывается отдельным методом `ProcessEquipmentStatus()`. Обновляет `equipment["VEH-001"]` в памяти. `StartProcess` проверяет `status.Ready == true` перед запуском.

---

## 4. Пороговые значения и детектор отклонений

### Таблица допусков

Определена в `service/telemetry.go`, переменная `thresholds []TelemetryThreshold`:

```go
type TelemetryThreshold struct {
    SensorCode string
    StageKeys  []string   // стадии, для которых применяется допуск
    Min        *float64   // nil = нет нижней границы
    Max        *float64   // nil = нет верхней границы
    Severity   string     // "warning" | "critical"
    Label      string     // человекочитаемое описание нарушения
}
```

| Датчик | Применимые стадии | Мин | Макс | Severity | Технологическое обоснование |
|--------|------------------|-----|------|----------|----------------------------|
| WP-TEMP-01 | water_pot_heating, oil_pot_heating | 75 °C | 85 °C | critical | Водная фаза при < 75°C не обеспечивает гомогенного смешения; при > 85°C деградирует ТЭА |
| OP-TEMP-02 | oil_pot_heating, main_pot_oil_feeding | 75 °C | 85 °C | critical | Воск должен быть расплавлен (> 75°C); при > 85°C начинается окисление ненасыщенных жиров |
| MP-TEMP-03 | emulsifying_speed_2, emulsifying_speed_3 | 75 °C | 85 °C | critical | Оптимальный диапазон эмульгирования; выход ведёт к расслоению эмульсии |
| MP-TEMP-03 | additive_feeding | — | 40 °C | critical | Добавки (консерванты, ароматика) термолабильны выше 40 °C |
| MP-HOMOG-01 | emulsifying_speed_2, emulsifying_speed_3 | 1800 rpm | — | warning | Скорость ниже 1800 об/мин не обеспечивает дисперсность частиц < 5 мкм |

### Алгоритм детектора (`checkDeviations`)

```
вызывается из ProcessRawTelemetry() если activeBatch != nil && currentStage != ""

key = sensorCode + ":" + stageKey

violations = CheckThresholds(reading, stageKey)

if len(violations) == 0:
    delete(deviations[key])       // нарушение устранено — сбросить state
    return

if key not in deviations:
    deviations[key] = {startedAt: now(), eventFired: false}

if !deviations[key].eventFired && time.Since(startedAt) >= 30s:
    deviations[key].eventFired = true
    EventCreator.CreateEventRaw(batchID, stageKey, "alarm", severity, label)
    // одно событие на непрерывное нарушение — повторно НЕ создаётся
```

**Cooldown**: `eventFired = true` предотвращает спам событиями при длительном нарушении. При возврате в норму `deviations[key]` удаляется → следующее нарушение снова создаст событие.

**Thread safety**: доступ к `deviations` защищён `sync.RWMutex` (`s.mu`).

### Декаплинг EventCreator

`TelemetryService` не импортирует `ProcessService` напрямую (цикл зависимостей). Используется интерфейс:

```go
// service/telemetry.go
type EventCreator interface {
    CreateEventRaw(ctx context.Context, batchID int, stageKey, eventType, severity, description string) error
}
```

`ProcessService` реализует его методом `CreateEventRaw`. Связь устанавливается после инициализации обоих сервисов в `cmd/ebr-app/main.go`:

```go
processService := service.NewProcessService(processRepo, eventRepo, userRepo, telemetryService)
telemetryService.SetEventCreator(processService) // late binding — разрывает цикл
```

---

## 5. Условия подписи стадии

### Структура условия

```go
// internal/domain/process.go
type StageSignCondition struct {
    SensorCode string   // код датчика из s.latest
    MinValue   *float64 // nil = нет нижней границы
    MaxValue   *float64 // nil = нет верхней границы
    Unit       string
    Label      string   // "WP-TEMP-01 ≥ 75 °C"
}
```

Каждая из 18 стадий в `domain.AllStages` содержит `[]StageSignCondition`. Пустой срез = стадия подтверждается только оператором (без проверки датчиков — загрузка фаз, операции с клапанами).

### Проверка при подписи (серверная валидация)

`ProcessService.SignStageTransition` → `checkConditions(stageKey)`:

```go
for _, cond := range stageDef.Conditions {
    reading, err := s.telemetry.GetLatestBySensorCode(ctx, cond.SensorCode)
    if err != nil {
        return fmt.Errorf("%w: %s — нет данных с датчика", ErrConditionNotMet, cond.Label)
    }
    if cond.MinValue != nil && reading.Value < *cond.MinValue {
        return fmt.Errorf("%w: %s (текущее: %.2f)", ErrConditionNotMet, cond.Label, reading.Value)
    }
    if cond.MaxValue != nil && reading.Value > *cond.MaxValue {
        return fmt.Errorf("%w: %s (текущее: %.2f)", ErrConditionNotMet, cond.Label, reading.Value)
    }
}
```

HTTP-ответ при невыполнении: **422 Unprocessable Entity**:
```json
{"error": "stage conditions not met: WP-TEMP-01 ≥ 75 °C (текущее: 63.40 C)"}
```

### Условия для каждой стадии

| Стадия | Условие подписи | Датчик |
|--------|----------------|--------|
| water_pot_feeding | operator-confirm | — |
| oil_pot_feeding | operator-confirm | — |
| main_pot_vacuumize | MP-VACUUM-01 ≤ −0.05 МПа | MP-VACUUM-01 |
| water_pot_heating | WP-TEMP-01 ≥ 75 °C | WP-TEMP-01 |
| oil_pot_heating | OP-TEMP-02 ≥ 75 °C | OP-TEMP-02 |
| main_pot_preheating | MP-TEMP-03 ≥ 70 °C | MP-TEMP-03 |
| main_pot_water_feeding | operator-confirm | — |
| main_pot_pre_blending | MP-HOMOG-01 ≥ 100 об/мин | MP-HOMOG-01 |
| main_pot_vacuum_drawing_1 | MP-VACUUM-01 ≤ −0.05 МПа | MP-VACUUM-01 |
| main_pot_oil_feeding | OP-TEMP-02 ≥ 75 °C | OP-TEMP-02 |
| main_pot_vacuum_drawing_2 | MP-VACUUM-01 ≤ −0.05 МПа | MP-VACUUM-01 |
| emulsifying_speed_2 | MP-HOMOG-01 ≥ 1800 об/мин | MP-HOMOG-01 |
| emulsifying_speed_3 | MP-HOMOG-01 ≥ 1800 об/мин | MP-HOMOG-01 |
| cooling_start | operator-confirm | — |
| cooling_blending | MP-TEMP-03 ≤ 40 °C | MP-TEMP-03 |
| additive_feeding | MP-TEMP-03 ≤ 35 °C | MP-TEMP-03 |
| final_blending | MP-HOMOG-01 ≥ 100 об/мин | MP-HOMOG-01 |
| cooling_finish | MP-TEMP-03 ≤ 30 °C | MP-TEMP-03 |

---

## 6. Доставка данных на фронтенд

### Двухканальная стратегия

Из-за ограничений среды (reverse-proxy, браузерные ограничения WebSocket) реализована fallback-схема:

```
Канал 1 (primary):   WebSocket /ws/telemetry?token=<jwt>
                     → push каждые 2 сек при поступлении MQTT-данных
                     → горутина hub.Run() рассылает всем подключённым клиентам

Канал 2 (fallback):  GET /api/v1/telemetry/all (REST, poll каждые 2 сек)
                     → возвращает map[sensorCode → NormalizedTelemetry]
                     → startTelemetryPoll() запускается при login, работает всегда
```

Оба канала пишут в единый кэш `telemetryCache[sensorCode]` в JS, откуда читают функции обновления UI.

### Обновление UI при получении данных

```js
// web/login.html — вызывается для каждого входящего сообщения
function updateStageMonitoring(msg) {
    // 1. Обновить sensor-chip в активной стадии
    const chip = document.getElementById('sc-' + msg.sensor_code)
    chip.className = isThresholdAlert(msg.sensor_code, msg.value, currentProcessStage)
        ? 'sensor-chip sensor-alert'   // мигающий красный
        : 'sensor-chip sensor-ok'      // зелёный

    // 2. Обновить condition-row (✓ / ✗)
    document.querySelectorAll('[data-condition-sensor="' + msg.sensor_code + '"]')
        .forEach(row => {
            const met = checkConditionMet(row.dataset.min, row.dataset.max, msg.value)
            row.className = 'cond-row ' + (met ? 'cond-met' : 'cond-unmet')
        })

    // 3. Разблокировать/заблокировать кнопку «Подтвердить стадию»
    const unmet = document.querySelectorAll('.cond-unmet').length
    signBtn.disabled = total > 0 && unmet > 0
}
```

### Состояния sensor-chip

| CSS-класс | Цвет | Условие |
|-----------|------|---------|
| `sensor-waiting` | серый | данных в кэше нет |
| `sensor-ok` | зелёный | значение в норме |
| `sensor-alert` | красный + мигание (`@keyframes pulse-alert`) | выход за допуск |

### GetStageConditions — серверная поддержка

`GET /api/v1/batches/{code}/process/current` возвращает расширенный DTO с live-условиями:

```json
{
  "stage_key": "water_pot_heating",
  "instructions": ["1. Включите нагрев...", "2. Установите мешалку..."],
  "stage_sensors": ["WP-TEMP-01", "WP-MIXER-01"],
  "conditions": [
    {
      "sensor_code": "WP-TEMP-01",
      "label": "WP-TEMP-01 ≥ 75 °C",
      "current": 63.4,
      "unit": "C",
      "met": false,
      "has_reading": true
    }
  ],
  "can_sign": false
}
```

Это позволяет фронтенду инициализировать корректное состояние UI без ожидания первого MQTT-сообщения.

---

## 7. Автоматические события и их жизненный цикл

### Типы событий

| type | severity | Источник | Смысл |
|------|----------|----------|-------|
| `alarm` | warning / critical | TelemetryService (авто) | Нарушение технологического допуска |
| `deviation` | любой | Оператор (вручную) | Задокументированное отклонение |
| `operator_action` | info | Оператор (вручную) | Действие, выходящее за рамки инструкции |
| `system` | critical | ProcessService (авто) | Отмена партии, критическая ситуация |

### Жизненный цикл события

```
[создание] events.occurred_at = NOW(), resolved_by = NULL
    ↓
[фронтенд] красный алерт-баннер, событие видно в журнале
    ↓
[оператор] нажимает «Разрешить» → POST /api/v1/events/{id}/resolve
    Body: {"comment": "объяснение оператора"}
    ↓
[бэкенд] UPDATE events SET resolved_by = $userID, comment = $comment
    ↓
[протокол] событие с комментарием фиксируется в HTML-отчёте партии
```

### Событие при отмене партии

`ProcessService.CancelBatch()`:
1. `UPDATE batches SET status = 'cancelled', cancel_reason = $reason`
2. `TelemetryService.SetActiveBatch(nil)` — остановить сохранение телеметрии
3. `EventRepo.CreateEvent()` — системное событие `severity=critical`, `description = "Партия отменена оператором. Причина: " + reason`
4. Фронтенд перенаправляет на экран Протоколов и автоматически генерирует HTML-отчёт

---

## 8. Хранение телеметрии в PostgreSQL

```sql
CREATE TABLE telemetry (
    id          BIGSERIAL PRIMARY KEY,
    batch_id    INT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    sensor_id   INT NOT NULL REFERENCES sensors(id),
    stage_key   VARCHAR(50),     -- текущая стадия в момент записи
    value       DECIMAL(10,4) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_telemetry_batch ON telemetry(batch_id, recorded_at);
```

`sensor_id` разрешается через `TelemetryRepo.GetSensorIDByCode()` с кэшированием в памяти (`sensorIDs map[string]int`) — исключает N+1 запросов к БД при каждом MQTT-сообщении.

### Агрегаты для отчёта

`TelemetryRepo.GetStageAggregates()` — оконный агрегат per (batch, stage, sensor):

```sql
SELECT s.sensor_code, s.name, s.unit,
       AVG(t.value)::DECIMAL(10,2) AS avg_val,
       MIN(t.value)                AS min_val,
       MAX(t.value)                AS max_val,
       COUNT(*)                    AS reading_count
FROM telemetry t
JOIN sensors s ON s.id = t.sensor_id
WHERE t.batch_id = $1 AND t.stage_key = $2
GROUP BY s.sensor_code, s.name, s.unit
```

---

## 9. Данные в протоколе партии

HTML-протокол генерируется `ReportService.GenerateAndSave()` и содержит для каждой стадии:

- Время начала и завершения (`started_at`, `completed_at`)
- Оператор, подписавший стадию (`signed_by` → ФИО из `users`)
- Комментарий оператора (`batch_stages.comment`)
- Агрегированная телеметрия (avg/min/max по каждому датчику стадии)
- Список событий стадии (тип, severity, описание, комментарий при разрешении)

Для отменённых партий в начале отчёта выводится блок с причиной отмены (`batches.cancel_reason`).

---

## 10. Тестовые сценарии отклонений

### Сценарий 1: Кратковременный сбой датчика (предупреждение)

**Ситуация**: Во время `oil_pot_heating` датчик OP-TEMP-02 показывает 92 °C из-за электрической помехи. Температура в котле физически нормальная.

**Воспроизведение**:
```
Терминал PLC:  4   # SimulateOilOverheat → OP-TEMP-02 = 92°C на 2 мин
```

**Поведение системы**:
1. OP-TEMP-02 > 85 °C → нарушение допуска
2. TelemetryService фиксирует `deviations["OP-TEMP-02:oil_pot_heating"].startedAt`
3. Через 30 сек: `EventCreator.CreateEventRaw(batchID, "oil_pot_heating", "alarm", "critical", "OP-TEMP-02: 92.xx °C > max 85...")`
4. Фронтенд: датчик OP-TEMP-02 мигает красным, алерт-баннер внизу экрана

**Действия оператора**:
```
Терминал PLC:  5   # SimulateOilRecovery → OP-TEMP-02 → 80°C
```
Оператор нажимает «Разрешить» на событии → вводит комментарий:
> *«Помеха в кабеле датчика OP-TEMP-02. Физическая температура котла в норме (~80°C). Продукт не пострадал, контроль по OP-TEMP-01 подтверждён.»*

Событие фиксируется в протоколе с комментарием.

---

### Сценарий 2: Критическое перегревание — отмена партии

**Ситуация**: Во время `emulsifying_speed_2` нагрев основного котла вышел из-под контроля. Температура MP-TEMP-03 поднялась выше 85 °C и удерживается. Риск: окисление ненасыщенных жиров (масло виноградных косточек, кокосовое масло), изменение запаха, снижение срока годности.

**Воспроизведение**:
```
Терминал PLC:  3   # запустить симуляцию
               4   # через несколько секунд — перегрев масляного котла
```

**Критическая цепочка**:
```
MP-TEMP-03 > 85°C → checkDeviations() → 30 сек → alarm/critical event
                                                 → алерт-баннер на фронте
                                                 → датчик MP-TEMP-03 мигает красным
```

**Действия оператора**:
1. Нажать **«✕ Отменить партию»** (кнопка в шапке панели процесса)
2. Ввести причину:
   > *«Критическое превышение температуры MP-TEMP-03 > 85°C в течение > 5 мин на стадии эмульгирования. Высокий риск окисления масел. Партия не соответствует требованиям по качеству. Необходима проверка системы охлаждения.»*

**Что делает система**:
```sql
UPDATE batches
SET status = 'cancelled',
    completed_at = NOW(),
    cancel_reason = 'Критическое превышение MP-TEMP-03...'
WHERE batch_code = $1 AND status = 'in_process'
```
- Создаётся системное событие `type=system, severity=critical`
- `TelemetryService.SetActiveBatch(nil)` — остановить сохранение в БД
- Автоматически генерируется HTML-протокол с причиной отмены

---

## 11. Схема таблиц

```sql
-- Показания датчиков (сохраняются каждые 5с при активной партии)
CREATE TABLE telemetry (
    id          BIGSERIAL PRIMARY KEY,
    batch_id    INT  NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    sensor_id   INT  NOT NULL REFERENCES sensors(id),
    stage_key   VARCHAR(50),       -- стадия в момент записи
    value       DECIMAL(10,4) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Отклонения и действия оператора
CREATE TABLE events (
    id          SERIAL PRIMARY KEY,
    batch_id    INT  NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    stage_key   VARCHAR(50),
    type        VARCHAR(30) CHECK (type IN ('alarm','deviation','operator_action','system')),
    severity    VARCHAR(20) CHECK (severity IN ('info','warning','critical')),
    description TEXT NOT NULL,
    comment     TEXT,              -- оператор пишет при разрешении
    resolved_by INT  REFERENCES users(id),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Стадии с подписью и комментарием оператора
CREATE TABLE batch_stages (
    id           SERIAL PRIMARY KEY,
    batch_id     INT  NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    stage_number INT  NOT NULL,
    stage_key    VARCHAR(50) NOT NULL,
    stage_name   VARCHAR(100),
    comment      TEXT,              -- необязательный комментарий при подписи
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL,
    signed_by    INT  REFERENCES users(id),
    signed_at    TIMESTAMPTZ NULL,
    UNIQUE(batch_id, stage_key)
);

-- Партии с причиной отмены
CREATE TABLE batches (
    ...
    status        VARCHAR(30) CHECK (status IN (
                      'waiting_weighing','weighing_in_progress',
                      'ready_for_process','in_process',
                      'completed','cancelled')),
    cancel_reason TEXT        -- заполняется при status='cancelled'
);
```
