-- Who wrote the link back in a Paperless document, so it is corrected or
-- removed with a connection that was allowed to write it.
CREATE TABLE paperless_backlinks (
    doc_id  INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE
);
