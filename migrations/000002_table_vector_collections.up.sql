CREATE TABLE IF NOT EXISTS vector_collections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    collection_hash VARCHAR(50) NOT NULL,
    date_created  DATETIME NOT NULL,
    date_modified DATETIME NOT NULL
);