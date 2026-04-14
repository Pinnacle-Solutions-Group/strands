CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    started_at  TEXT NOT NULL,
    ended_at    TEXT,
    workdir     TEXT NOT NULL,
    summary     TEXT
);

CREATE TABLE IF NOT EXISTS strands (
    id          TEXT PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    topic       TEXT NOT NULL,
    body        TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS strands_session_idx ON strands(session_id);
CREATE INDEX IF NOT EXISTS strands_created_idx ON strands(created_at DESC);

CREATE TABLE IF NOT EXISTS strand_tags (
    strand_id   TEXT NOT NULL REFERENCES strands(id) ON DELETE CASCADE,
    tag_type    TEXT NOT NULL CHECK (tag_type IN ('read','user','corrected','inferred','tested','narrative')),
    tag_value   TEXT,
    PRIMARY KEY (strand_id, tag_type, tag_value)
);

CREATE TABLE IF NOT EXISTS strand_bead_links (
    strand_id   TEXT NOT NULL REFERENCES strands(id) ON DELETE CASCADE,
    bead_id     TEXT NOT NULL,
    relation    TEXT NOT NULL CHECK (relation IN ('produced','discussed','blocked-on','discovered')),
    PRIMARY KEY (strand_id, bead_id, relation)
);

CREATE INDEX IF NOT EXISTS strand_bead_links_bead_idx ON strand_bead_links(bead_id);

CREATE TABLE IF NOT EXISTS private_flags (
    strand_id   TEXT PRIMARY KEY REFERENCES strands(id) ON DELETE CASCADE,
    reason      TEXT NOT NULL,
    flagged_at  TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS strands_fts USING fts5(
    topic,
    body,
    content='strands',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS strands_ai AFTER INSERT ON strands BEGIN
    INSERT INTO strands_fts(rowid, topic, body) VALUES (new.rowid, new.topic, new.body);
END;

CREATE TRIGGER IF NOT EXISTS strands_ad AFTER DELETE ON strands BEGIN
    INSERT INTO strands_fts(strands_fts, rowid, topic, body) VALUES ('delete', old.rowid, old.topic, old.body);
END;

CREATE TRIGGER IF NOT EXISTS strands_au AFTER UPDATE ON strands BEGIN
    INSERT INTO strands_fts(strands_fts, rowid, topic, body) VALUES ('delete', old.rowid, old.topic, old.body);
    INSERT INTO strands_fts(rowid, topic, body) VALUES (new.rowid, new.topic, new.body);
END;
