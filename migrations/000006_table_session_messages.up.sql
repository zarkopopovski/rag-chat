CREATE TABLE session_messages (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    date_created  DATETIME NOT NULL,
    date_modified DATETIME NOT NULL,
);