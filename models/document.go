package models

import "time"

type Document struct {
	ID           int64     `json:"id" db:"id"`
	UserID       int64     `json:"-" db:"user_id"`
	CollectionID int64     `json:"collection_id" db:"collection_id"`
	FileName     string    `json:"file_name" db:"file_name"`
	IsIndexed    bool      `json:"is_indexed" db:"is_indexed"`
	DateCreated  time.Time `json:"date_created" db:"date_created"`
	DateModified time.Time `json:"-" db:"date_modified"`
}
