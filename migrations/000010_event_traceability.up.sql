ALTER TABLE events
    DROP CONSTRAINT IF EXISTS events_type_check;

ALTER TABLE events
    ADD CONSTRAINT events_type_check
    CHECK (type IN ('alarm', 'deviation', 'operator_action', 'system', 'system_error', 'rate_violation'));

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS sensor_code VARCHAR(30),
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS ended_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS start_value DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS end_value DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS min_value DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS max_value DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS avg_value DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS sample_count INT;
