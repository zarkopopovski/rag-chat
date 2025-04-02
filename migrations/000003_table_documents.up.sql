CREATE TABLE IF NOT EXISTS documents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    collection_id INTEGER NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    is_indexed INTEGER NOT NULL,
    date_created  DATETIME NOT NULL,
    date_modified DATETIME NOT NULL
);