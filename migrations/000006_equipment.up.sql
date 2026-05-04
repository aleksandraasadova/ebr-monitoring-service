
CREATE TABLE equipment (
    id             SERIAL PRIMARY KEY,
    equipment_code VARCHAR(30) UNIQUE NOT NULL,  -- VEH-500L-2024-001
    name           VARCHAR(100) NOT NULL,
    type          VARCHAR(30) NOT NULL,   -- VEH, SCALE
    capacity_l/kg INT,          
    status         VARCHAR(20) DEFAULT 'offline'
                   CHECK (status IN ('available', 'occupied', 'offline')),
    last_seen_at   TIMESTAMPTZ NULL,
    created_by INT REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO equipment (equipment_code, name, type, capacity_l, status, created_by) VALUES
('VEH-001', 'Вакуумный эмульгатор-гомогенизатор', 'VEH', 500, 'offline', 1),
('SCALES-001', 'Весы платформенные', 'scale', 60, 'offline', 1);

-- vacuum, mixer_rpm, homogenizer_rpm, weight, temperature
CREATE TABLE sensors (
    id           SERIAL PRIMARY KEY,
    equipment_id INT NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    sensor_code  VARCHAR(30) NOT NULL,   -- TEMP-R1 
    name         VARCHAR(100),
    type         VARCHAR(30) 
                 CHECK (type IN ('temperature', 'vacuum', 'mixer_rpm', 'homogenizer_rpm', 'weight')),
    unit         VARCHAR(20),            -- °C, bar, rpm, 
    mqtt_topic   VARCHAR(200),           -- plc/reactor1/temperature
    UNIQUE(equipment_id, sensor_code),
    UNIQUE(mqtt_topic)
);