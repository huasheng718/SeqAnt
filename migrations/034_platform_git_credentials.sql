-- Personal access tokens for Gitee and GitLab. Apply manually in production;
-- startup AutoMigrate also adds these columns for newly opened stores.
ALTER TABLE user_configs ADD COLUMN gitee_token TEXT NULL;
ALTER TABLE user_configs ADD COLUMN gitlab_token TEXT NULL;
