-- Personal Codeup credentials. Apply manually in production; startup
-- AutoMigrate also adds these columns for newly opened stores.
ALTER TABLE user_configs ADD COLUMN codeup_token TEXT NULL;
