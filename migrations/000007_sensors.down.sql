DELETE FROM sensors
WHERE mqtt_topic IN (
    'ebr/equipment/VEH-001/sensor/water_pot_weight',
    'ebr/equipment/VEH-001/sensor/water_pot_temp',
    'ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm',
    'ebr/equipment/VEH-001/sensor/oil_pot_weight',
    'ebr/equipment/VEH-001/sensor/oil_pot_temp',
    'ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm',
    'ebr/equipment/VEH-001/sensor/main_pot_vacuum',
    'ebr/equipment/VEH-001/sensor/main_pot_temp',
    'ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm',
    'ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm',
    'ebr/equipment/VEH-001/sensor/main_pot_weight',
    'ebr/sensor/weighing_scale_01'
);
