
CREATE TABLE batches (
    id                SERIAL PRIMARY KEY,
    batch_number      VARCHAR(50) UNIQUE NOT NULL,      -- например: BATCH-2026-001
    recipe_id         INT NOT NULL REFERENCES recipes(id),
    target_volume_l   DECIMAL(10,2) NOT NULL,           -- целевой объём партии в литрах
    status            VARCHAR(30) DEFAULT 'waiting_weighing' 
                      CHECK (status IN ('waiting_weighing', 'weighing_in_progress', 'ready_for_process', 'in_process', 'completed', 'cancelled')),
    registered_by     INT REFERENCES users(id),         -- кто создал партию
    operator_id       INT REFERENCES users(id),
    created_at        TIMESTAMPTZ DEFAULT NOW(),
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ
);

CREATE INDEX idx_batches_status ON batches(status);


-- Требования к сырью должны быть зафиксированы до начала выполнения.
--  Поэтому при создании партии мы рассчитываем требуемые количества каждого ингредиента в граммах и сохраняем их в отдельной таблице.
-- actual_qty, container_code, weighed_by, weighed_at - заполняются по мере выполнения взвешивания и могут быть обновлены в случае ошибок.

CREATE TABLE weighing_log (
    id             SERIAL PRIMARY KEY,
    batch_id       INT NOT NULL REFERENCES batches(id) ON DELETE CASCADE,
    ingredient_id  INT NOT NULL REFERENCES ingredients(id),
    stage_key      VARCHAR(50) NOT NULL,               -- water_phase, oil_phase, additive
    required_qty DECIMAL(10,2) NOT NULL,             -- рассчитано при создании партии
    actual_qty   DECIMAL(10,2),                      -- вводится оператором / приходит с весов
    container_code VARCHAR(20),
    weighed_by     INT REFERENCES users(id) ,         -- кто взвесил
    weighed_at     TIMESTAMPTZ,
    UNIQUE(batch_id, ingredient_id, stage_key)
);

CREATE INDEX idx_weighing_log_batch ON weighing_log(batch_id);