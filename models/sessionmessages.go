package models

import "time"

type SessionMessage struct {
	ID           int64     `json:"id" db:"id"`
	SessionID    string    `json:"session_id" db:"session_id"`
	Message      string    `json:"message" db:"message"`
	DateCreated  time.Time `json:"date_created" db:"date_created"`
	DateModified time.Time `json:"date_modified" db:"date_modified"`
}
