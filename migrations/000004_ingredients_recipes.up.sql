-- общие данные (Master Data) 
CREATE TABLE ingredients (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(100) UNIQUE NOT NULL,  -- "Вода очищенная"
    unit    VARCHAR(20) NOT NULL DEFAULT 'г'          
);

INSERT INTO ingredients (name, unit) VALUES
('Масло виноградных косточек', 'г'),
('Кокосовое масло', 'г'),
('МГД (моноглицериды)', 'г'),
('Эмульсионный воск', 'г'),
('Ланолин безводный', 'г'),
('Глицерин', 'г'),
('Экстракт бадана толстолистного сухой', 'г'),
('Кремофор А25', 'г'),
('Салициловая кислота', 'г'),
('Диметикон', 'г'),
('Триэтаноламин (ТЭА)', 'г'),
('Эуксил 9010', 'г'),
('Эфирное масло розового дерева', 'г'),
('Ментол', 'г'),
('Октопирокс', 'г'),
('Вода очищенная', 'г')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE recipes (
    id          SERIAL PRIMARY KEY,
    recipe_code VARCHAR(30) UNIQUE NOT NULL,
    name        VARCHAR(100) NOT NULL,
    version     VARCHAR(20) DEFAULT '1.0',
    description TEXT,
    required_equipment_type VARCHAR(30) CHECK (required_equipment_type IN ('VEH')),
    created_by  INT REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    is_active   BOOLEAN DEFAULT TRUE,
    CONSTRAINT unique_recipe_version UNIQUE (name, version)
);

-- триггер на вставку

CREATE SEQUENCE IF NOT EXISTS seq_recipe_code START 1;

CREATE OR REPLACE FUNCTION fnc_trg_recipe_code()
RETURNS TRIGGER AS $$
BEGIN
    NEW.recipe_code := 'RC-' || TO_CHAR(CURRENT_DATE, 'YYYY') || ('') || LPAD(nextval('seq_recipe_code')::text, 3, '0');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_recipe_code
    BEFORE INSERT 
    ON recipes
    FOR EACH ROW
EXECUTE FUNCTION fnc_trg_recipe_code();

INSERT INTO recipes (name, version, description, required_equipment_type, created_by) VALUES
('Косметический эмульсионный крем увлажняющий', '1.0',
 'Макроэмульсия типа М/В. Водная фаза ~69.1%, масляная ~22.0%, добавки ~8.9%.', 'VEH', 1)
ON CONFLICT (name, version) DO NOTHING; 

CREATE TABLE recipe_ingredients (
    id             SERIAL PRIMARY KEY,
    recipe_id      INT NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id  INT NOT NULL REFERENCES ingredients(id), 
    stage_key      VARCHAR(50) NOT NULL,          -- группа ингредиентов 
    percentage     DECIMAL(5,2) NOT NULL CHECK (percentage > 0 AND percentage <= 100),
    UNIQUE(recipe_id, ingredient_id, stage_key)
);

INSERT INTO recipe_ingredients (recipe_id, ingredient_id, stage_key, percentage)
SELECT 
    r.id AS recipe_id, 
    i.id AS ingredient_id,
    data.stage_key AS stage_key,
    data.pct AS percentage
FROM recipes r
CROSS JOIN 
(VALUES 
    ('oil_phase',   'Масло виноградных косточек',          5.0),
    ('oil_phase',   'Кокосовое масло',                     5.0),
    ('oil_phase',   'МГД (моноглицериды)',                 5.0),
    ('oil_phase',   'Эмульсионный воск',                   4.0),
    ('oil_phase',   'Ланолин безводный',                   3.0),
    ('water_phase', 'Глицерин',                            3.0),
    ('additive',    'Экстракт бадана толстолистного сухой', 2.0),
    ('oil_phase',   'Кремофор А25',                        2.0),
    ('additive',    'Салициловая кислота',                 2.0),
    ('additive',    'Диметикон',                           2.0),
    ('water_phase', 'Триэтаноламин (ТЭА)',                 0.5),
    ('additive',    'Эуксил 9010',                         0.5),
    ('additive',    'Эфирное масло розового дерева',       0.2),
    ('additive',    'Ментол',                              0.1),
    ('additive',    'Октопирокс',                          0.1),
    ('water_phase', 'Вода очищенная',                     65.6)
) AS data(stage_key, name, pct)
JOIN ingredients i ON i.name = data.name
WHERE 
    r.name = 'Косметический эмульсионный крем увлажняющий' AND 
    r.version = '1.0';