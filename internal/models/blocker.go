package models

import (
	"time"

	"github.com/google/uuid"
)

// Blocker represents a project-level impediment that blocks progress.
type Blocker struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"` // nil = active
}

// NewBlocker creates a new blocker with the given title.
func NewBlocker(title string) *Blocker {
	return &Blocker{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: time.Now(),
	}
}

// Resolve marks the blocker as resolved.
func (b *Blocker) Resolve() {
	now := time.Now()
	b.ResolvedAt = &now
}

// Unresolve removes the resolved status from a blocker.
func (b *Blocker) Unresolve() {
	b.ResolvedAt = nil
}

// IsResolved returns true if the blocker has been resolved.
func (b *Blocker) IsResolved() bool {
	return b.ResolvedAt != nil
}
