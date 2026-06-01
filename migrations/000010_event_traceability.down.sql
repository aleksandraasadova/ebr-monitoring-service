ALTER TABLE events
    DROP CONSTRAINT IF EXISTS events_type_check;

ALTER TABLE events
    ADD CONSTRAINT events_type_check
    CHECK (type IN ('alarm', 'deviation', 'operator_action', 'system'));

ALTER TABLE events
    DROP COLUMN IF EXISTS sample_count,
    DROP COLUMN IF EXISTS avg_value,
    DROP COLUMN IF EXISTS max_value,
    DROP COLUMN IF EXISTS min_value,
    DROP COLUMN IF EXISTS end_value,
    DROP COLUMN IF EXISTS start_value,
    DROP COLUMN IF EXISTS ended_at,
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS sensor_code;
