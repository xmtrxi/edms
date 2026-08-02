package database

import "time"

type Tag struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
