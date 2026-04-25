
CREATE TABLE users (
    id            SERIAL PRIMARY KEY,
    user_code     VARCHAR(30) UNIQUE NOT NULL,  -- OP-001, ADM-001
    username      VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(20) NOT NULL 
                  CHECK (role IN ('admin','operator')),
    full_name     VARCHAR(100),
    is_active     BOOLEAN DEFAULT TRUE,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE equipment (
    id             SERIAL PRIMARY KEY,
    equipment_code VARCHAR(30) UNIQUE NOT NULL,  -- VEH-500L-2024-1
    name           VARCHAR(100) NOT NULL,
    plc_address    VARCHAR(50),                  -- IP адрес ПЛК
    status         VARCHAR(20) DEFAULT 'offline'
                   CHECK (status IN ('online','offline','fault')),
    last_seen_at   TIMESTAMPTZ NULL
);

CREATE TABLE sensors (
    id           SERIAL PRIMARY KEY,
    equipment_id INT NOT NULL REFERENCES equipment(id),
    sensor_code  VARCHAR(30) NOT NULL,   -- TEMP-R1, PH-E
    name         VARCHAR(100),
    type         VARCHAR(30) 
                 CHECK (type IN ('temperature','pressure',
                                 'rpm','level','vacuum','ph',
                                 'valve_state')),
    unit         VARCHAR(20),            -- °C, bar, rpm, pH
    mqtt_topic   VARCHAR(200)            -- plc/reactor1/temperature
);

CREATE TABLE ingredients (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(100) UNIQUE NOT NULL,  -- "Вода очищенная"
    unit    VARCHAR(20) NOT NULL           -- кг, л, г
);

CREATE TABLE recipes (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(100) NOT NULL,
    version     VARCHAR(20) DEFAULT '1.0',
    description TEXT,
    
    -- Стадии со своими параметрами контроля
    stages      JSONB NOT NULL DEFAULT '[]',
    
    -- Состав: соотношения ингредиентов
    components  JSONB NOT NULL DEFAULT '[]',
    
    created_by  INT REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    is_active   BOOLEAN DEFAULT TRUE
);

CREATE TABLE batches (
    id              SERIAL PRIMARY KEY,
    batch_number    VARCHAR(50) UNIQUE NOT NULL,
    operator_id     INT NOT NULL REFERENCES users(id),
    equipment_id    INT NOT NULL REFERENCES equipment(id),
    recipe_id       INT NOT NULL REFERENCES recipes(id),
    
    -- Копия рецептуры на момент старта (!)
    recipe_snapshot JSONB,
    
    volume_liters   DECIMAL(10,2) NOT NULL,
    status          VARCHAR(20) DEFAULT 'registered'
                    CHECK (status IN ('registered','in_progress',
                                      'paused','completed','aborted')),
    current_stage   INT DEFAULT 1,
    
    started_at      TIMESTAMPTZ NULL,
    completed_at    TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE batch_stages (
    id           SERIAL PRIMARY KEY,
    batch_id     INT NOT NULL REFERENCES batches(id),
    stage_number INT NOT NULL,
    stage_key    VARCHAR(50) NOT NULL,
    stage_name   VARCHAR(100),
    started_at   TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    signed_by    INT REFERENCES users(id),    -- кто подписал переход
    signed_at    TIMESTAMPTZ NULL             -- когда подписал
);

CREATE TABLE weighing_log (
    id            SERIAL PRIMARY KEY,
    batch_id      INT NOT NULL REFERENCES batches(id),
    stage_key     VARCHAR(50) NOT NULL,
    ingredient_id INT NOT NULL REFERENCES ingredients(id),
    planned_qty   DECIMAL(10,4) NOT NULL,   -- сколько должно быть
    actual_qty    DECIMAL(10,4),            -- сколько реально взвесили
    unit          VARCHAR(20) NOT NULL,
    confirmed_by  INT REFERENCES users(id),
    confirmed_at  TIMESTAMPTZ NULL
);

CREATE TABLE telemetry (
    id          BIGSERIAL PRIMARY KEY,
    batch_id    INT NOT NULL REFERENCES batches(id),
    sensor_id   INT NOT NULL REFERENCES sensors(id),
    stage_key   VARCHAR(50),              -- на какой стадии пришло
    value       DECIMAL(10,4) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE events (
    id          SERIAL PRIMARY KEY,
    batch_id    INT NOT NULL REFERENCES batches(id),
    stage_key   VARCHAR(50),
    type        VARCHAR(30)
                CHECK (type IN ('alarm','stage_change','deviation',
                                'operator_action','system')),
    severity    VARCHAR(20) DEFAULT 'info'
                CHECK (severity IN ('info','warning','critical')),
    description TEXT NOT NULL,
    comment     TEXT,
    resolved_by INT REFERENCES users(id),
    occurred_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    table_name  VARCHAR(50) NOT NULL,
    record_id   INT NOT NULL,
    action      VARCHAR(20) NOT NULL
                CHECK (action IN ('INSERT','UPDATE','DELETE','SIGN')),
    user_id     INT REFERENCES users(id),
    old_values  JSONB,
    new_values  JSONB,
    ip_address  VARCHAR(45),
    changed_at  TIMESTAMPTZ DEFAULT NOW()
);