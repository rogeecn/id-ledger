CREATE TABLE IF NOT EXISTS collected_ids (
    project_key TEXT COLLATE BINARY NOT NULL CHECK (project_key <> ''),
    remote_id TEXT COLLATE BINARY NOT NULL CHECK (remote_id <> ''),
    created_at INTEGER NOT NULL,
    PRIMARY KEY (project_key, remote_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS collected_ids_by_time
ON collected_ids(project_key, created_at DESC, remote_id DESC);
