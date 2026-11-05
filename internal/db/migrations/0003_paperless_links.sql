-- Whether documents someone links from their Paperless get a link back to
-- the things they're on.
ALTER TABLE user_settings ADD COLUMN paperless_backlinks INTEGER NOT NULL DEFAULT 0;
