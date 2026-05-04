
DROP TABLE IF EXISTS recipe_ingredients;
DROP TABLE IF EXISTS recipes;
DROP TABLE IF EXISTS ingredients;

-- DROP TRIGGER IF EXISTS trg_recipe_code ON recipes; - триггер исчезнет вместе с таблицей
DROP FUNCTION IF EXISTS fnc_trg_recipe_code();
DROP SEQUENCE IF EXISTS seq_recipe_code;