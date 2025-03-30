package models

import "time"

type Document struct {
	ID           int       `json:"id" db:"id"`
	UserID       int       `json:"-" db:"user_id"`
	CollectionID int       `json:"collection_id" db:"collection_id"`
	FileName     string    `json:"file_name" db:"file_name"`
	IsIndexed    bool      `json:"is_indexed" db:"is_indexed"`
	DateCreated  time.Time `json:"date_created" db:"date_created"`
	DateModified time.Time `json:"-" db:"date_modified"`
}
