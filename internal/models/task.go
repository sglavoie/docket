package models

import (
	"time"
)

// Task represents a single task, story, epic, bug, or idea.
type Task struct {
	ID          string             `json:"id"`
	ShortID     int                `json:"short_id"`
	Type        TaskType           `json:"type"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      TaskStatus         `json:"status"`
	Priority    int                `json:"priority"`
	ParentID    *string            `json:"parent_id,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Section     string             `json:"section,omitempty"` // Section for organizing tasks (e.g., "Next", "Inbox", "Waiting", "Someday")
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
	CompletedAt *time.Time         `json:"completed_at,omitempty"`
	DueDate     *time.Time         `json:"due_date,omitempty"`
	DependsOn   []string           `json:"depends_on,omitempty"` // IDs of tasks this task depends on
	Pinned      bool               `json:"pinned,omitempty"`     // Whether the task is pinned
	PinnedAt    *time.Time         `json:"pinned_at,omitempty"`  // When the task was pinned
	Active      bool               `json:"active,omitempty"`     // Whether the task is flagged as active
	Comments    []*Comment         `json:"comments,omitempty"`   // Timestamped comments/notes
	Recurrence  *RecurrencePattern `json:"recurrence,omitempty"` // Recurrence pattern (nil = non-recurring)

	// Computed fields (not persisted)
	Children []*Task  `json:"-"`
	Parent   *Task    `json:"-"`
	Project  *Project `json:"-"`
	Level    int      `json:"-"` // Depth in hierarchy (0 for root)
}

// NewTask creates a new task with default values.
func NewTask(id string, shortID int, taskType TaskType, title string) *Task {
	now := time.Now()
	return &Task{
		ID:        id,
		ShortID:   shortID,
		Type:      taskType,
		Title:     title,
		Status:    StatusTodo,
		Priority:  PriorityDefault, // Medium priority
		Tags:      []string{},
		Section:   "", // Empty section = Active view (default for new tasks)
		DependsOn: []string{},
		CreatedAt: now,
		UpdatedAt: now,
		Children:  []*Task{},
	}
}
