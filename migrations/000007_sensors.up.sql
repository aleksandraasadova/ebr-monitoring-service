INSERT INTO sensors (equipment_id, sensor_code, name, type, unit, mqtt_topic)
SELECT e.id, data.sensor_code, data.name, data.type, data.unit, data.mqtt_topic
FROM equipment e
JOIN (VALUES
    ('VEH-001', 'WP-WEIGHT-01', 'Water Pot тензодатчик массы', 'weight', 'g', 'ebr/equipment/VEH-001/sensor/water_pot_weight'),
    ('VEH-001', 'WP-TEMP-01', 'Water Pot температура', 'temperature', 'C', 'ebr/equipment/VEH-001/sensor/water_pot_temp'),
    ('VEH-001', 'WP-MIXER-01', 'Water Pot скорость мешалки', 'mixer_rpm', 'rpm', 'ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm'),

    ('VEH-001', 'OP-WEIGHT-02', 'Oil Pot тензодатчик массы', 'weight', 'g', 'ebr/equipment/VEH-001/sensor/oil_pot_weight'),
    ('VEH-001', 'OP-TEMP-02', 'Oil Pot температура', 'temperature', 'C', 'ebr/equipment/VEH-001/sensor/oil_pot_temp'),
    ('VEH-001', 'OP-MIXER-02', 'Oil Pot скорость мешалки', 'mixer_rpm', 'rpm', 'ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm'),

    ('VEH-001', 'MP-VACUUM-01', 'Main Pot абсолютное давление/вакуум', 'vacuum', 'MPa', 'ebr/equipment/VEH-001/sensor/main_pot_vacuum'),
    ('VEH-001', 'MP-TEMP-03', 'Main Pot температура продукта', 'temperature', 'C', 'ebr/equipment/VEH-001/sensor/main_pot_temp'),
    ('VEH-001', 'MP-HOMOG-01', 'Main Pot скорость гомогенизатора', 'homogenizer_rpm', 'rpm', 'ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm'),
    ('VEH-001', 'MP-SCRAPER-01', 'Main Pot скорость скребковой мешалки', 'mixer_rpm', 'rpm', 'ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm'),
    ('VEH-001', 'MP-WEIGHT-03', 'Main Pot тензодатчик массы', 'weight', 'g', 'ebr/equipment/VEH-001/sensor/main_pot_weight'),

    ('SCALES-001', 'SCALE-WEIGHT-01', 'Платформенные весы', 'weight', 'g', 'ebr/sensor/weighing_scale_01')
) AS data(equipment_code, sensor_code, name, type, unit, mqtt_topic)
    ON e.equipment_code = data.equipment_code
ON CONFLICT (mqtt_topic) DO UPDATE
SET
    sensor_code = EXCLUDED.sensor_code,
    name = EXCLUDED.name,
    type = EXCLUDED.type,
    unit = EXCLUDED.unit;
