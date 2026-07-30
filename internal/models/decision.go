package models

import "time"

// Decision represents a timestamped decision captured on a task.
type Decision struct {
	Text      string    `json:"text"`
	Rationale string    `json:"rationale,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
