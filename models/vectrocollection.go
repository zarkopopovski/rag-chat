package models

import "time"

type VectorCollection struct {
	ID             int64     `json:"id" db:"id"`
	UserID         int64     `json:"user_id" db:"user_id"`
	Name           string    `json:"name" db:"name"`
	CollectionHash string    `json:"collection_hash" db:"collection_hash"`
	DateCreated    time.Time `json:"date_created" db:"date_created"`
	DateModified   time.Time `json:"date_modified" db:"date_modified"`
}
