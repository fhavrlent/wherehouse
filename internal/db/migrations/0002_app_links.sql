-- Links from other apps, such as a thing's warranty in Warranteed: which
-- app (source) and what there (external_id), with a little of its data to
-- show here. A thing has at most one link per app.
ALTER TABLE thing_links ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE thing_links ADD COLUMN external_id TEXT NOT NULL DEFAULT '';
ALTER TABLE thing_links ADD COLUMN data TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX thing_links_source ON thing_links (thing_id, source) WHERE source <> '';
CREATE INDEX thing_links_external ON thing_links (source, external_id) WHERE source <> '';

-- The app whose purchase and warranty details these are ('' = this one's).
-- Those details are then changed there, not here.
ALTER TABLE things ADD COLUMN purchase_owner TEXT NOT NULL DEFAULT '';

-- Deleted things, so other apps reading the change feed hear of them.
CREATE TABLE deleted_things (
    household_id INTEGER NOT NULL REFERENCES households (id) ON DELETE CASCADE,
    thing_id     INTEGER NOT NULL,
    deleted_at   TEXT NOT NULL
);
CREATE INDEX deleted_things_household ON deleted_things (household_id, deleted_at);
