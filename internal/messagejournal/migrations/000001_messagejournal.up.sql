CREATE TABLE IF NOT EXISTS journal (
    id INTEGER NOT NULL PRIMARY KEY,
    message_id TEXT NOT NULL,
    sent DATETIME NOT NULL,
    worker_name TEXT NOT NULL,
    response_to TEXT,
    worker_event INTEGER,
    worker_data TEXT
);