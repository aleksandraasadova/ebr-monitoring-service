CREATE TABLE batch_stages (
    id           SERIAL PRIMARY KEY,
    batch_id     INT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    stage_number INT NOT NULL,
    stage_key    VARCHAR(50) NOT NULL,
    stage_name   VARCHAR(100),
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL,
    signed_by    INT REFERENCES users(id),
    signed_at    TIMESTAMPTZ NULL,
    UNIQUE(batch_id, stage_key)
);

CREATE TABLE telemetry (
    id          BIGSERIAL PRIMARY KEY,
    batch_id    INT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    sensor_id   INT NOT NULL REFERENCES sensors(id),
    stage_key   VARCHAR(50),
    value       DECIMAL(10,4) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_telemetry_batch ON telemetry(batch_id, recorded_at);

CREATE TABLE events (
    id          SERIAL PRIMARY KEY,
    batch_id    INT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    stage_key   VARCHAR(50),
    type        VARCHAR(30) NOT NULL
                CHECK (type IN ('alarm', 'deviation', 'operator_action', 'system')),
    severity    VARCHAR(20) NOT NULL DEFAULT 'info'
                CHECK (severity IN ('info', 'warning', 'critical')),
    description TEXT NOT NULL,
    comment     TEXT,
    resolved_by INT REFERENCES users(id),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    table_name  VARCHAR(50) NOT NULL,
    record_id   INT NOT NULL,
    action      VARCHAR(20) NOT NULL
                CHECK (action IN ('INSERT', 'UPDATE', 'DELETE', 'SIGN')),
    user_id     INT REFERENCES users(id),
    old_values  JSONB,
    new_values  JSONB,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE batch_reports (
    id           SERIAL PRIMARY KEY,
    batch_id     INT NOT NULL UNIQUE REFERENCES batches(id) ON DELETE CASCADE,
    generated_by INT REFERENCES users(id),
    html_content TEXT NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
