CREATE TABLE session_messages (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    session_id VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    message_role VARCHAR(10) NOT NULL,
    date_created  DATETIME NOT NULL,
    date_modified DATETIME NOT NULL,
);