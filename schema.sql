-- schema.sql is the canonical reference for the on-disk SQLite shapes used by
-- ondict. The runtime applies these via forward-only migrations (see
-- dbutil/migrate.go and the `migrations` slice in each package). Both DBs
-- carry a `meta` table so the runtime knows which migrations have already
-- been applied.
--
-- See docs/adr/0001-wordbank-history-sync.md for the design rationale,
-- timestamp / tombstone semantics, and the schema-version log.

-- ---------------------------------------------------------------------------
-- meta — bookkeeping table present in every ondict-managed SQLite database.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- ---------------------------------------------------------------------------
-- history.db — query history (one row per distinct word)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS history (
    word           TEXT     NOT NULL UNIQUE,
    `count`        INTEGER  NOT NULL DEFAULT 0,
    create_time    DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    update_time    DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    deleted_at     DATETIME,                                          -- tombstone (NULL = live row)
    server_seen_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)      -- local-write wall-clock time, used as sync delta filter
);

CREATE INDEX IF NOT EXISTS i_count                     ON history(`count`);
CREATE INDEX IF NOT EXISTS i_latest                    ON history(update_time);
CREATE INDEX IF NOT EXISTS i_history_server_seen_at    ON history(server_seen_at);

-- Append / re-query template (resurrects tombstones, increments live rows):
-- INSERT INTO history (word, `count`) VALUES (?, 1)
-- ON CONFLICT(word) DO UPDATE SET
--     `count`        = CASE WHEN deleted_at IS NULL THEN `count` + 1 ELSE 1 END,
--     update_time    = CURRENT_TIMESTAMP,
--     deleted_at     = NULL,
--     server_seen_at = CURRENT_TIMESTAMP;

-- ---------------------------------------------------------------------------
-- wordbank.db — user's saved word bank (one row per distinct word)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS words (
    word           TEXT NOT NULL PRIMARY KEY COLLATE NOCASE,
    create_time    DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    update_time    DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    deleted_at     DATETIME,                                          -- tombstone (NULL = live row)
    server_seen_at DATETIME NOT NULL DEFAULT (CURRENT_TIMESTAMP)
);

CREATE INDEX IF NOT EXISTS i_words_update_time     ON words(update_time);
CREATE INDEX IF NOT EXISTS i_words_server_seen_at  ON words(server_seen_at);

-- ---------------------------------------------------------------------------
-- vocab.db — derived dictionary headword cache (rebuilt from MDX sources;
-- not part of user state, not synced).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS vocab (
    word TEXT NOT NULL,
    src  TEXT NOT NULL DEFAULT "",
    def  TEXT NOT NULL DEFAULT ""
);

CREATE UNIQUE INDEX IF NOT EXISTS i_word_src ON vocab(word, src);
