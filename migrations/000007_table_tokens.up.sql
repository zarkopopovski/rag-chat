CREATE TABLE IF NOT EXISTS tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type VARCHAR(20) NOT NULL,
    uuid VARCHAR(80) NOT NULL,
    user_id INTEGER NOT NULL,
    date_created VARCHAR(20) NOT NULL
);