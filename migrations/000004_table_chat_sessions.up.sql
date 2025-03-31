CREATE TABLE chat_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    collection_id INTEGER NOT NULL,
    session_id VARCHAR(50) NOT NULL,
    date_created  DATETIME NOT NULL,
    date_modified DATETIME NOT NULL
);
