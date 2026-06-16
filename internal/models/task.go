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

// Task represents a single task, story, epic, bug, or idea.
type Task struct {
	ID            string             `json:"id"`
	ShortID       int                `json:"short_id"`
	Type          TaskType           `json:"type"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	Status        TaskStatus         `json:"status"`
	Priority      int                `json:"priority"`
	ParentID      *string            `json:"parent_id,omitempty"`
	Tags          []string           `json:"tags,omitempty"`
	Section       string             `json:"section,omitempty"` // GTD section (e.g., "Next", "Inbox", "Waiting", "Someday")
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	CompletedAt   *time.Time         `json:"completed_at,omitempty"`
	DueDate       *time.Time         `json:"due_date,omitempty"`
	HasTime       bool               `json:"has_time,omitempty"`   // true when DueDate carries a meaningful clock time
	DependsOn     []string           `json:"depends_on,omitempty"` // IDs of tasks this task depends on
	RelatedTo     []string           `json:"related_to,omitempty"` // IDs of related tasks (bidirectional)
	Pinned        bool               `json:"pinned,omitempty"`     // Whether the task is pinned
	PinnedAt      *time.Time         `json:"pinned_at,omitempty"`  // When the task was pinned
	Active        bool               `json:"active,omitempty"`     // Whether the task is flagged as active
	Dimmed        bool               `json:"dimmed,omitempty"`     // Whether the task is visually dimmed
	Comments      []*Comment         `json:"comments,omitempty"`   // Timestamped comments/notes
	Decisions     []*Decision        `json:"decisions,omitempty"`  // Design/architecture decisions
	Recurrence    *RecurrencePattern `json:"recurrence,omitempty"` // Recurrence pattern (nil = non-recurring)
	Attachments   []Attachment       `json:"attachments,omitempty"`
	SnoozedUntil  *time.Time         `json:"snoozed_until,omitempty"`  // Task hidden until this date
	Activity      []*ActivityEntry   `json:"activity,omitempty"`       // Persistent changelog of field changes
	Checklist     []ChecklistItem    `json:"checklist,omitempty"`      // Checklist items for subtasks
	Stage         string             `json:"stage,omitempty"`          // Workflow stage (e.g., "Brainstorm", "Design", "Implement")
	StartDate     *time.Time         `json:"start_date,omitempty"`     // Planned/actual start date (used for Gantt scheduling)
	EstimatedDays float64            `json:"estimated_days,omitempty"` // Estimated duration in working days (0 = unset)
	Source        string             `json:"source,omitempty"`         // Origin of the task (e.g., "github", "jira")
	URL           string             `json:"url,omitempty"`            // External reference URL
	// Scoring dimensions: manual overrides for scoring profile dimensions (e.g., "impact": 4)
	ScoreDimensions map[string]int `json:"score_dimensions,omitempty"`

	// Type-specific attributes (only one should be set based on task Type)
	BugAttrs   *BugAttrs   `json:"bug_attrs,omitempty"`
	IdeaAttrs  *IdeaAttrs  `json:"idea_attrs,omitempty"`
	TaskAttrs  *TaskAttrs  `json:"task_attrs,omitempty"`
	StoryAttrs *StoryAttrs `json:"story_attrs,omitempty"`
	EpicAttrs  *EpicAttrs  `json:"epic_attrs,omitempty"`

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

// NewTask creates a new task with default values.
// Returns an error if the title is invalid.
func NewTask(id string, shortID int, taskType TaskType, title string) (*Task, error) {
	if err := ValidateTaskTitle(title); err != nil {
		return nil, err
	}
	now := time.Now()
	return &Task{
		ID:              id,
		ShortID:         shortID,
		Type:            taskType,
		Title:           title,
		Status:          StatusTodo,
		Priority:        PriorityDefault, // Medium priority
		Tags:            []string{},
		Section:         "", // Empty section = Active view (default for new tasks)
		DependsOn:       []string{},
		RelatedTo:       []string{},
		ScoreDimensions: map[string]int{},
		CreatedAt:       now,
		UpdatedAt:       now,
		Children:        []*Task{},
	}, nil
}

// ClearTypeAttrs clears all type-specific attribute pointers.
// Called when task type changes to prevent stale attributes from persisting.
//
// Design note: each task type owns at most one Attrs pointer (e.g. a Bug has
// BugAttrs, a Story has StoryAttrs). The other pointers are always nil during
// normal operation. Clearing all of them on a type change is intentional —
// it is cheaper and safer than tracking which pointer was previously active,
// and ensures no phantom attributes leak into the new type's serialized form.
func (t *Task) ClearTypeAttrs() {
	t.BugAttrs = nil
	t.IdeaAttrs = nil
	t.TaskAttrs = nil
	t.StoryAttrs = nil
	t.EpicAttrs = nil
}
