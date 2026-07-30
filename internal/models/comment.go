package models

import "time"

// Comment represents a timestamped note on a task.
type Comment struct {
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}
