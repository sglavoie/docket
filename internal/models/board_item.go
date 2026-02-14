package models

import (
	"fmt"
	"time"
)

// BoardItemStatus represents the state of a board item.
type BoardItemStatus string

const (
	BoardItemOpen      BoardItemStatus = "open"
	BoardItemDone      BoardItemStatus = "done"
	BoardItemDismissed BoardItemStatus = "dismissed"
)

// BoardItem represents an external or lightweight item on the focus board.
type BoardItem struct {
	ID           string          `yaml:"id"`
	ShortID      int             `yaml:"short_id"`
	Title        string          `yaml:"title"`
	URL          string          `yaml:"url,omitempty"`
	Source       string          `yaml:"source,omitempty"`
	Priority     int             `yaml:"priority"`
	Status       BoardItemStatus `yaml:"status"`
	Tags         []string        `yaml:"tags,omitempty"`
	LinkedTaskID string          `yaml:"linked_task_id,omitempty"`
	ProjectID    string          `yaml:"project_id,omitempty"`
	ProjectName  string          `yaml:"project_name,omitempty"`
	CreatedAt    time.Time       `yaml:"created_at"`
	UpdatedAt    time.Time       `yaml:"updated_at"`
	DoneAt       *time.Time      `yaml:"done_at,omitempty"`

	// Filename is the on-disk filename (e.g., "1-review-pr-432.md").
	Filename string `yaml:"-"`

	// Body is the markdown content after the frontmatter.
	Body string `yaml:"-"`
}

// BoardItemPriorityDefault is the default priority for new board items.
const BoardItemPriorityDefault = 3

// NewBoardItem creates a new board item with default values.
func NewBoardItem(id string, shortID int, title string) *BoardItem {
	now := time.Now()
	return &BoardItem{
		ID:        id,
		ShortID:   shortID,
		Title:     title,
		Priority:  BoardItemPriorityDefault,
		Status:    BoardItemOpen,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsOpen returns true if the item is in open state.
func (b *BoardItem) IsOpen() bool {
	return b.Status == BoardItemOpen
}

// IsClosed returns true if the item is done or dismissed.
func (b *BoardItem) IsClosed() bool {
	return b.Status == BoardItemDone || b.Status == BoardItemDismissed
}

// MarkDone marks the item as done.
func (b *BoardItem) MarkDone() {
	now := time.Now()
	b.Status = BoardItemDone
	b.DoneAt = &now
	b.UpdatedAt = now
}

// MarkDismissed marks the item as dismissed.
func (b *BoardItem) MarkDismissed() {
	now := time.Now()
	b.Status = BoardItemDismissed
	b.DoneAt = &now
	b.UpdatedAt = now
}

// ClearLink removes the linked task reference and associated project fields.
func (b *BoardItem) ClearLink() {
	b.LinkedTaskID = ""
	b.ProjectID = ""
	b.ProjectName = ""
	b.UpdatedAt = time.Now()
}

// Reopen resets the item to open status.
func (b *BoardItem) Reopen() {
	b.Status = BoardItemOpen
	b.DoneAt = nil
	b.UpdatedAt = time.Now()
}

// GenerateBoardItemFilename creates a filename from shortID and title slug.
func GenerateBoardItemFilename(shortID int, title string) string {
	slug := slugify(title)
	return fmt.Sprintf("%d-%s.md", shortID, slug)
}
