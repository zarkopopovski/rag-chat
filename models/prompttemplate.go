package models

import "time"

type PromptTemplate struct {
	ID           int64     `json:"id" db:"id"`
	UserID       int64     `json:"user_id" db:"user_id"`
	CollectionID int64     `json:"-" db:"collection_id"`
	Template     string    `json:"template" db:"template"`
	DateCreated  time.Time `json:"date_created" db:"date_created"`
	DateModified time.Time `json:"-" db:"date_modified"`
}
