
CREATE SEQUENCE IF NOT EXISTS seq_user_code_admin START 2;
CREATE SEQUENCE IF NOT EXISTS seq_user_code_operator START 1;

CREATE OR REPLACE FUNCTION fnc_trg_user_code()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.role = 'admin' THEN
        NEW.user_code := 'ADM-' || LPAD(nextval('seq_user_code_admin')::text, 3, '0');
    ELSIF NEW.role = 'operator' THEN
        NEW.user_code := 'OP-' || LPAD(nextval('seq_user_code_operator')::text, 3, '0');
    ELSE
        RAISE EXCEPTION 'Invalid role: %', NEW.role;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_user_code
    BEFORE INSERT
    ON users
    FOR EACH ROW
EXECUTE FUNCTION fnc_trg_user_code();
