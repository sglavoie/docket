package tui

import (
	"time"

	"charm.land/bubbles/v2/textarea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// TaskEditorModel handles editing an existing task via TUI.
type TaskEditorModel struct {
	project       *models.Project
	task          *models.Task
	config        *config.Config
	configManager *config.Manager

	// Original values for change detection
	originalTitle      string
	originalTags       string
	originalType       models.TaskType
	originalParentID   *string
	originalPriority   int
	originalSection    string
	originalDueDate    *time.Time
	originalPinned     bool
	originalActive     bool
	originalDimmed     bool
	originalRecurrence *models.RecurrencePattern

	// Form fields
	titleInput textarea.Model
	tags       []string

	// Selectable fields
	taskType         models.TaskType
	typeCursor       int
	types            []models.TaskType
	parentID         *string
	parentCursor     int
	validParents     []*models.Task
	priority         int
	section          string
	sectionCursor    int
	sections         []string
	dueDate          *time.Time
	dueDateInput     string
	pinned           bool
	active           bool
	dimmed           bool
	flagCursor       int // 0=None, 1=Pin, 2=Active, 3=Dim (mutually exclusive)
	recurrence       *models.RecurrencePattern
	recurrenceCursor int
	recurrenceOpts   []string // Display labels for recurrence options

	// URL and Source fields
	urlInput    string
	sourceInput string

	// Original URL/Source for change detection
	originalURL    string
	originalSource string

	// Type-specific attributes
	// Bug attributes
	bugReproducibleCursor int
	bugSeverityCursor     int
	bugWorkaround         string

	// Idea attributes
	ideaCategoryCursor int
	ideaEffortCursor   int
	ideaValueCursor    int
	ideaValidated      bool

	// Task attributes
	taskEffortCursor   int
	taskWorkTypeCursor int

	// Story attributes
	storyPointsCursor int
	storyPersona      string

	// Epic attributes
	epicScopeCursor    int
	epicMilestone      string
	epicSuccessMetrics string

	// Original attribute values for change detection
	originalBugAttrs   *models.BugAttrs
	originalIdeaAttrs  *models.IdeaAttrs
	originalTaskAttrs  *models.TaskAttrs
	originalStoryAttrs *models.StoryAttrs
	originalEpicAttrs  *models.EpicAttrs

	// Type change confirmation
	confirmTypeChange bool
	pendingTypeCursor int
	pendingTaskType   models.TaskType

	// Form state
	focusIndex int // 0=title, 1=type, 2=parent, 3=priority, 4=section, 5=due date, 6=flags, 7=recurrence, 8=url, 9=source, 10+=attrs
	width      int
	height     int
	err        error
	hasChanges bool

	// Discard confirmation state
	confirmDiscard bool

	// Overlay for floating confirm rendering
	confirmOverlay ConfirmDialog
}
