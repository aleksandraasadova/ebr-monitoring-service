
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