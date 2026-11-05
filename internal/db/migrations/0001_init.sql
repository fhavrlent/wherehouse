CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    issuer        TEXT NOT NULL,
    subject       TEXT NOT NULL,
    email         TEXT NOT NULL DEFAULT '',
    name          TEXT NOT NULL DEFAULT '',
    picture       TEXT NOT NULL DEFAULT '',
    -- Profile picture downloaded at sign-in and served by the app, so the
    -- page needn't load images from the identity provider. picture_etag
    -- changes with the image ('' = none).
    picture_data  BLOB,
    picture_type  TEXT NOT NULL DEFAULT '',
    picture_etag  TEXT NOT NULL DEFAULT '',
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_login_at TEXT,
    UNIQUE (issuer, subject)
);

CREATE TABLE sessions (
    token_hash    TEXT PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    id_token      TEXT NOT NULL DEFAULT '',
    -- Encrypted OIDC refresh token, used to re-check group membership, and
    -- when that was last done ('' = never).
    refresh_token TEXT NOT NULL DEFAULT '',
    checked_at    TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    expires_at    TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions (user_id);

-- A household is whose things these are. Everyone in it sees and changes
-- all of them. A person can be in several, and works in one at a time.
CREATE TABLE households (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    -- The currency prices are in unless a thing says otherwise.
    currency   TEXT NOT NULL DEFAULT 'EUR',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Owners can rename the household, invite people and remove them.
CREATE TABLE household_members (
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role         TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    joined_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (household_id, user_id)
);
CREATE INDEX household_members_user ON household_members (user_id);

-- Invite links; only a hash of the secret in the link is kept.
CREATE TABLE invites (
    token_hash   TEXT PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    created_by   INTEGER REFERENCES users (id) ON DELETE SET NULL,
    expires_at   TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE user_settings (
    user_id           INTEGER PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    theme             TEXT NOT NULL DEFAULT 'system',
    -- The language picked in settings ('' = follow the browser), and the
    -- browser's language as last seen, which reminders use in that case.
    language          TEXT NOT NULL DEFAULT '',
    browser_language  TEXT NOT NULL DEFAULT '',
    -- The household the person is working in.
    current_household INTEGER REFERENCES households (id) ON DELETE SET NULL,
    notify_urls       TEXT NOT NULL DEFAULT '',
    -- Days before maintenance is due that reminders go out.
    notify_days       TEXT NOT NULL DEFAULT '7,0',
    paperless_token   TEXT NOT NULL DEFAULT ''
);

-- Custom icons a household uploaded for its kinds.
CREATE TABLE icons (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    file         TEXT NOT NULL,
    mime_type    TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Kinds of thing: building, room, box, drill... Things of a place kind are
-- shown as places (cards, the place picker); any thing can hold others.
-- Built-in kinds have a key and are named in the reader's language until
-- renamed. icon is the name of a built-in icon or "custom:<icon id>".
CREATE TABLE kinds (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    key          TEXT,
    name         TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL,
    is_place     INTEGER NOT NULL DEFAULT 0,
    position     INTEGER NOT NULL DEFAULT 0,
    UNIQUE (household_id, key)
);

-- Everything: buildings, rooms, boxes, and what's in them.
CREATE TABLE things (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    household_id      INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    parent_id         INTEGER REFERENCES things (id) ON DELETE RESTRICT,
    kind_id           INTEGER NOT NULL REFERENCES kinds (id) ON DELETE RESTRICT,
    name              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    quantity          INTEGER NOT NULL DEFAULT 1,
    manufacturer      TEXT NOT NULL DEFAULT '',
    model             TEXT NOT NULL DEFAULT '',
    serial            TEXT NOT NULL DEFAULT '',
    notes             TEXT NOT NULL DEFAULT '',
    -- Purchase: price is of one piece, in currency ('' = the household's).
    purchase_date     TEXT,
    purchase_price    REAL,
    purchase_from     TEXT NOT NULL DEFAULT '',
    currency          TEXT NOT NULL DEFAULT '',
    warranty_until    TEXT,
    lifetime_warranty INTEGER NOT NULL DEFAULT 0,
    warranty_details  TEXT NOT NULL DEFAULT '',
    insured           INTEGER NOT NULL DEFAULT 0,
    -- Sold or given away: such things count toward no totals.
    sold_date         TEXT,
    sold_price        REAL,
    sold_to           TEXT NOT NULL DEFAULT '',
    sold_notes        TEXT NOT NULL DEFAULT '',
    -- Templates are copied to add similar things and are listed nowhere else.
    is_template       INTEGER NOT NULL DEFAULT 0,
    archived_at       TEXT,
    created_by        INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX things_household ON things (household_id, updated_at);
CREATE INDEX things_parent ON things (parent_id);

-- Photos of a thing, in the order shown; the first is the cover.
CREATE TABLE photos (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    thing_id   INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    file       TEXT NOT NULL,
    mime_type  TEXT NOT NULL,
    size       INTEGER NOT NULL DEFAULT 0,
    position   INTEGER NOT NULL DEFAULT 0,
    created_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX photos_thing ON photos (thing_id, position);

-- Printed labels. A label's QR code opens /l/<code>, which shows the thing
-- it is on. Blank labels are printed ahead and attached when first scanned.
CREATE TABLE labels (
    code         TEXT PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    thing_id     INTEGER REFERENCES things (id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE UNIQUE INDEX labels_thing ON labels (thing_id) WHERE thing_id IS NOT NULL;

CREATE TABLE tags (
    id           INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         TEXT NOT NULL COLLATE NOCASE,
    color        TEXT NOT NULL,
    UNIQUE (household_id, name)
);
CREATE TABLE thing_tags (
    thing_id INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    tag_id   INTEGER NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (thing_id, tag_id)
);
CREATE INDEX thing_tags_tag ON thing_tags (tag_id);

-- Custom fields every thing in a household can fill in, in the order shown.
CREATE TABLE fields (
    id           INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         TEXT NOT NULL COLLATE NOCASE,
    type         TEXT NOT NULL CHECK (type IN ('text', 'number', 'date', 'url', 'bool')),
    position     INTEGER NOT NULL DEFAULT 0,
    UNIQUE (household_id, name)
);
CREATE TABLE thing_fields (
    thing_id INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    field_id INTEGER NOT NULL REFERENCES fields (id) ON DELETE CASCADE,
    value    TEXT NOT NULL,
    PRIMARY KEY (thing_id, field_id)
);

-- Documents of a thing: manuals, receipts and so on, stored here or
-- linked from Paperless-ngx.
CREATE TABLE attachments (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    thing_id         INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    kind             TEXT NOT NULL CHECK (kind IN ('manual', 'receipt', 'warranty', 'other')),
    storage          TEXT NOT NULL CHECK (storage IN ('local', 'paperless')),
    filename         TEXT NOT NULL,
    mime_type        TEXT NOT NULL DEFAULT 'application/octet-stream',
    size             INTEGER NOT NULL DEFAULT 0,
    file             TEXT NOT NULL DEFAULT '',
    paperless_doc_id INTEGER,
    added_by         INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX attachments_thing ON attachments (thing_id);

-- The maintenance log: what was done to a thing, and when it is next due.
CREATE TABLE maintenance (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    thing_id   INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    title      TEXT NOT NULL,
    done_on    TEXT,
    next_due   TEXT,
    notes      TEXT NOT NULL DEFAULT '',
    cost       REAL,
    created_by INTEGER REFERENCES users (id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX maintenance_thing ON maintenance (thing_id);
CREATE INDEX maintenance_due ON maintenance (next_due) WHERE next_due IS NOT NULL;

-- One row per (user, entry, due date, threshold), so each reminder is sent
-- once, and again if the due date changes.
CREATE TABLE reminders_sent (
    user_id        INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    maintenance_id INTEGER NOT NULL REFERENCES maintenance (id) ON DELETE CASCADE,
    due            TEXT NOT NULL,
    threshold_days INTEGER NOT NULL,
    sent_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (user_id, maintenance_id, due, threshold_days)
);

-- Links from a thing to elsewhere, such as its warranty in Warranteed.
CREATE TABLE thing_links (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    thing_id   INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    url        TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
CREATE INDEX thing_links_thing ON thing_links (thing_id);

-- API tokens for other apps, each for one person in one household. Only
-- a hash of the token is kept.
CREATE TABLE api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT
);
CREATE INDEX api_tokens_user ON api_tokens (user_id);

-- What was imported from HomeBox, by HomeBox ID, so importing again skips
-- it and links to HomeBox items can be moved here.
CREATE TABLE homebox_map (
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    homebox_id   TEXT NOT NULL,
    thing_id     INTEGER NOT NULL REFERENCES things (id) ON DELETE CASCADE,
    PRIMARY KEY (household_id, homebox_id)
);

-- Full-text search over things. ref is the thing's ID; path is the names of
-- the things it is in. The store keeps it up to date and rebuilds it at
-- startup. Diacritics are ignored, so "kuchyn" finds "Kuchyň".
CREATE VIRTUAL TABLE search USING fts5(
    household UNINDEXED,
    ref UNINDEXED,
    name,
    details,
    path,
    tokenize = 'unicode61 remove_diacritics 2'
);
