
DROP TRIGGER IF EXISTS trg_user_code ON users;
DROP FUNCTION IF EXISTS fnc_trg_user_code();

DROP SEQUENCE IF EXISTS seq_user_code_admin;
DROP SEQUENCE IF EXISTS seq_user_code_operator;
