
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