package models

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxTaskTitleLength is the maximum allowed length for a task title.
const MaxTaskTitleLength = 700

// MaxProjectNameLength is the maximum allowed length for a project name.
const MaxProjectNameLength = 100

// Attachment represents a file or URL attached to a task.
type Attachment struct {
	Filename     string    `json:"filename"`      // filename (local) or full URL
	IsURL        bool      `json:"is_url"`        // true for URLs (not stored locally)
	IsCodeRef    bool      `json:"is_code_ref"`   // true for code references (file:line format)
	LineNumber   int       `json:"line_number"`   // line number for code references (0 if none)
	OriginalPath string    `json:"original_path"` // original filesystem path for files, URL for URLs
	AddedAt      time.Time `json:"added_at"`
}

// ChecklistItem represents a single item in a task's checklist.
type ChecklistItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// Task is the target generic work record.
type Task struct {
	ID          string `json:"id"`
	ShortID     int    `json:"short_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// WorkflowState is the single lifecycle source of truth.
	WorkflowState WorkflowState `json:"workflow_state,omitempty"`
	Priority      int           `json:"priority"`
	ParentID      *string       `json:"parent_id,omitempty"`
	Tags          []string      `json:"tags,omitempty"`
	Context       string        `json:"context,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	// StateChangedAt is the state clock: the instant WorkflowState last became
	// its current value. UpdatedAt cannot answer "in this state since" because
	// comments, tags, attachments, reparenting, and manual reordering bump it.
	StateChangedAt time.Time       `json:"state_changed_at"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	DueDate        *time.Time      `json:"due_date,omitempty"`
	Pinned         bool            `json:"pinned,omitempty"`    // Whether the task is pinned
	PinnedAt       *time.Time      `json:"pinned_at,omitempty"` // When the task was pinned
	Comments       []*Comment      `json:"comments,omitempty"`  // Timestamped comments/notes
	Attachments    []Attachment    `json:"attachments,omitempty"`
	DeferredUntil  *time.Time      `json:"deferred_until,omitempty"` // Task hidden until this date
	Checklist      []ChecklistItem `json:"checklist,omitempty"`      // Checklist items for subtasks

	// Computed fields (not persisted)
	Children []*Task  `json:"-"`
	Parent   *Task    `json:"-"`
	Project  *Project `json:"-"`
	Level    int      `json:"-"` // Depth in hierarchy (0 for root)
}

// ValidateTaskTitle returns an error if the title is empty, exceeds MaxTaskTitleLength,
// or contains markdown meta-characters that would corrupt the storage format.
func ValidateTaskTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("task title cannot be empty")
	}
	if n := utf8.RuneCountInString(title); n > MaxTaskTitleLength {
		return fmt.Errorf("task title too long (%d chars, max %d)", n, MaxTaskTitleLength)
	}
	// Newlines in the title would cause data loss: the markdown heading format
	// is a single line ("## Type: Title"), so anything after the first newline
	// would be parsed as description content on the next load.
	if strings.ContainsAny(title, "\n\r") {
		return fmt.Errorf("task title must not contain newlines")
	}
	if strings.HasPrefix(title, "##") {
		return fmt.Errorf("task title must not start with '##' (reserved for markdown headings)")
	}
	if strings.Contains(title, "<!--") {
		return fmt.Errorf("task title must not contain '<!--' (reserved for metadata comments)")
	}
	if strings.Contains(title, "-->") {
		return fmt.Errorf("task title must not contain '-->' (reserved for metadata comments)")
	}
	return nil
}

// NewGenericTask creates a target generic Task with default values.
func NewGenericTask(id string, shortID int, title string) (*Task, error) {
	return newTask(id, shortID, title)
}

func newTask(id string, shortID int, title string) (*Task, error) {
	if err := ValidateTaskTitle(title); err != nil {
		return nil, err
	}
	now := time.Now()
	return &Task{
		ID:             id,
		ShortID:        shortID,
		Title:          title,
		WorkflowState:  StateBacklog,
		Priority:       PriorityUnset,
		Tags:           []string{},
		CreatedAt:      now,
		UpdatedAt:      now,
		StateChangedAt: now,
		Children:       []*Task{},
	}, nil
}
