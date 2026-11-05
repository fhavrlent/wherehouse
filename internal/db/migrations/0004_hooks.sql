-- An app's token from connecting remembers the host the person approved
-- (from its redirect address). The app may then ask to be pinged there
-- when something changes: hook_url, signed with hook_secret.
ALTER TABLE api_tokens ADD COLUMN app_host TEXT NOT NULL DEFAULT '';
ALTER TABLE api_tokens ADD COLUMN hook_url TEXT NOT NULL DEFAULT '';
ALTER TABLE api_tokens ADD COLUMN hook_secret TEXT NOT NULL DEFAULT '';
