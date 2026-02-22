package state

const schema = `
CREATE TABLE IF NOT EXISTS services (
    name         TEXT PRIMARY KEY,
    repo         TEXT NOT NULL,
    version      TEXT NOT NULL,
    prev_version TEXT,
    port         INTEGER NOT NULL UNIQUE,
    route_type   TEXT NOT NULL,
    route_value  TEXT NOT NULL,
    db_name      TEXT NOT NULL,
    db_user      TEXT NOT NULL,
    extra_env    TEXT,
    deployed_at  INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    service     TEXT NOT NULL,
    action      TEXT NOT NULL,
    version     TEXT,
    timestamp   INTEGER NOT NULL,
    detail      TEXT
);
`
