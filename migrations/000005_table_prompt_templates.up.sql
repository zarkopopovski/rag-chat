CREATE TABLE prompt_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    collection_id INTEGER NOT NULL,
    template TEXT NOT NULL,
    date_created  DATETIME NOT NULL,
    date_modified DATETIME NOT NULL
);