package models

import "time"

type ChatSession struct {
	ID           int64     `json:"id" db:"id"`
	UserID       int64     `json:"user_id" db:"user_id"`
	SessionID    string    `json:"session_id" db:"session_id"`
	DateCreated  time.Time `json:"date_created" db:"date_created"`
	DateModified time.Time `json:"date_modified" db:"date_modified"`
}
