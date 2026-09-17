CREATE TABLE boards (
    id         INTEGER PRIMARY KEY,
    code       TEXT    NOT NULL UNIQUE,
    chat_id    INTEGER NOT NULL,
    thread_id  INTEGER NOT NULL,
    title      TEXT    NOT NULL,
    pin_msg_id INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL,
    UNIQUE (chat_id, thread_id)
);

CREATE TABLE users (
    id         INTEGER PRIMARY KEY,
    username   TEXT    NOT NULL DEFAULT '',
    first_name TEXT    NOT NULL DEFAULT '',
    last_name  TEXT    NOT NULL DEFAULT '',
    dm_chat_id INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT    NOT NULL
);

CREATE TABLE tasks (
    id            INTEGER PRIMARY KEY,
    board_id      INTEGER NOT NULL REFERENCES boards (id),
    num           INTEGER NOT NULL,
    title         TEXT    NOT NULL,
    description   TEXT    NOT NULL DEFAULT '',
    raw_text      TEXT    NOT NULL DEFAULT '',
    status        TEXT    NOT NULL,
    priority      INTEGER NOT NULL DEFAULT 0,
    author_id     INTEGER NOT NULL,
    assignee_id   INTEGER NOT NULL DEFAULT 0,
    assignee_name TEXT    NOT NULL DEFAULT '',
    card_msg_id   INTEGER NOT NULL DEFAULT 0,
    source_msg_id INTEGER NOT NULL,
    media_kind    TEXT    NOT NULL DEFAULT '',
    media_file_id TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    closed_at     TEXT,
    deleted_at    TEXT,
    UNIQUE (board_id, num),
    UNIQUE (board_id, source_msg_id)
);

CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    tag     TEXT    NOT NULL,
    PRIMARY KEY (task_id, tag)
);

CREATE TABLE task_events (
    id          INTEGER PRIMARY KEY,
    task_id     INTEGER NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    actor_id    INTEGER NOT NULL,
    kind        TEXT    NOT NULL,
    from_status TEXT    NOT NULL DEFAULT '',
    to_status   TEXT    NOT NULL DEFAULT '',
    payload     TEXT    NOT NULL DEFAULT '',
    created_at  TEXT    NOT NULL
);

CREATE INDEX tasks_card   ON tasks (board_id, card_msg_id);
CREATE INDEX tasks_status ON tasks (board_id, status);
CREATE INDEX events_task  ON task_events (task_id, id);
