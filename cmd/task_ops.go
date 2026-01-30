package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/tui"
)

// FindTaskByIdentifier finds a task by short ID, UUID, or title prefix.
// Returns nil if no matching task is found.
func FindTaskByIdentifier(project *models.Project, identifier string) *models.Task {
	// Try as short ID first
	var shortID int
	if _, err := fmt.Sscanf(identifier, "%d", &shortID); err == nil {
		if task := project.GetTaskByShortID(shortID); task != nil {
			return task
		}
	}

	// Try as UUID
	if task := project.GetTaskByID(identifier); task != nil {
		return task
	}

	// Try as title prefix (case-insensitive)
	lowerID := strings.ToLower(identifier)
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), lowerID) {
			return task
		}
	}

	return nil
}

// ParseTaskTypeFlags determines the task type from boolean flags or a string flag.
// Returns an error if the type flag is invalid.
func ParseTaskTypeFlags(epic, story, bug, idea bool, typeFlag string) (models.TaskType, error) {
	if epic {
		return models.TaskTypeEpic, nil
	}
	if story {
		return models.TaskTypeStory, nil
	}
	if bug {
		return models.TaskTypeBug, nil
	}
	if idea {
		return models.TaskTypeIdea, nil
	}
	if typeFlag != "" {
		t, ok := models.ParseTaskType(typeFlag)
		if !ok {
			return "", tterrors.NewInvalidTaskTypeError(typeFlag)
		}
		return t, nil
	}
	return models.TaskTypeTask, nil
}

// TasksToSelectorItems converts tasks to selector items with standard columns.
func TasksToSelectorItems(tasks []*models.Task) []selector.Item {
	headers := []string{"ID", "Type", "Status", "Title"}
	items := make([]selector.Item, len(tasks))
	for i, t := range tasks {
		items[i] = selector.Item{
			ID:      t.ID,
			Data:    t,
			Headers: headers,
			Columns: []string{
				fmt.Sprintf("#%d", t.ShortID),
				t.Type.Icon(),
				t.Status.Icon(),
				t.Title,
			},
		}
	}
	return items
}

// GetTaskFromSelector safely extracts a Task from a selector.Item.
// Returns an error if the data is not a *models.Task.
func GetTaskFromSelector(item *selector.Item) (*models.Task, error) {
	task, ok := item.Data.(*models.Task)
	if !ok {
		return nil, fmt.Errorf("internal error: expected Task, got %T", item.Data)
	}
	return task, nil
}

// TaskFilter is a predicate function for filtering tasks.
type TaskFilter func(*models.Task) bool

// CanMoveUpFilter returns a filter for tasks that can be moved up among siblings.
func CanMoveUpFilter(project *models.Project) TaskFilter {
	return func(t *models.Task) bool {
		siblings := project.GetSiblings(t)
		for i, s := range siblings {
			if s.ID == t.ID && i > 0 {
				return true
			}
		}
		return false
	}
}

// CanMoveDownFilter returns a filter for tasks that can be moved down among siblings.
func CanMoveDownFilter(project *models.Project) TaskFilter {
	return func(t *models.Task) bool {
		siblings := project.GetSiblings(t)
		for i, s := range siblings {
			if s.ID == t.ID && i < len(siblings)-1 {
				return true
			}
		}
		return false
	}
}

// CanIndentFilter returns a filter for tasks that can be indented.
func CanIndentFilter(project *models.Project) TaskFilter {
	return func(t *models.Task) bool {
		targetParent := project.GetTaskAbove(t)
		if targetParent == nil {
			return false
		}
		ok, _ := t.CanIndent(targetParent)
		return ok
	}
}

// CanOutdentFilter returns a filter for tasks that can be outdented.
func CanOutdentFilter() TaskFilter {
	return func(t *models.Task) bool {
		ok, _ := t.CanOutdent()
		return ok
	}
}

// SelectTask handles the common pattern of selecting a task from args or interactively.
// If args contains an identifier, finds the task directly.
// Otherwise, filters tasks using the predicate and presents an interactive selector.
// Returns an error if no task is found or selection is canceled.
func SelectTask(project *models.Project, args []string, filter TaskFilter, prompt, emptyMsg string) (*models.Task, error) {
	if len(args) > 0 {
		task := FindTaskByIdentifier(project, args[0])
		if task == nil {
			return nil, fmt.Errorf("task '%s': %w", args[0], tterrors.ErrTaskNotFound)
		}
		return task, nil
	}

	var candidates []*models.Task
	for _, t := range project.Tasks {
		if filter(t) {
			candidates = append(candidates, t)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("%s", emptyMsg)
	}

	items := TasksToSelectorItems(candidates)
	selected, err := tui.RunSelector(prompt, items)
	if err != nil {
		return nil, err
	}

	return GetTaskFromSelector(selected)
}

// WithTaskUpdate encapsulates the common Load→FindTask→Update→Save pattern.
// It handles task lookup by identifier, captures state for workflow triggers,
// applies the update function, and saves with workflow rule evaluation.
// The update function receives the task and should return an error if the update fails.
// Returns the updated task on success for further inspection.
func WithTaskUpdate(taskIdentifier string, updateFn func(*models.Task) error) (*models.Task, error) {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return nil, err
	}

	task := FindTaskByIdentifier(project, taskIdentifier)
	if task == nil {
		return nil, tterrors.NewTaskNotFoundError(taskIdentifier)
	}

	// Capture task states before changes for event-based workflow triggers
	prevStates := CaptureTaskStates(project.Tasks)

	// Apply the update
	if err := updateFn(task); err != nil {
		return nil, err
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Save with workflow rules evaluation
	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: configManager, PrevStates: prevStates}); err != nil {
		return nil, err
	}

	return task, nil
}

// WithTaskUpdateInProject encapsulates the Load→FindTask→Update→Save pattern
// with access to the project. This is useful when the update function needs
// to interact with other project state (e.g., sections, other tasks).
// The update function receives the project and task and should return an error if the update fails.
// Returns the updated task on success for further inspection.
func WithTaskUpdateInProject(taskIdentifier string, updateFn func(*models.Project, *models.Task) error) (*models.Task, error) {
	project, cfg, configManager, err := LoadCurrentProject()
	if err != nil {
		return nil, err
	}

	task := FindTaskByIdentifier(project, taskIdentifier)
	if task == nil {
		return nil, tterrors.NewTaskNotFoundError(taskIdentifier)
	}

	// Capture task states before changes for event-based workflow triggers
	prevStates := CaptureTaskStates(project.Tasks)

	// Apply the update
	if err := updateFn(project, task); err != nil {
		return nil, err
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Save with workflow rules evaluation
	if err := SaveProject(project, &SaveOptions{EvaluateRules: true, Config: cfg, ConfigManager: configManager, PrevStates: prevStates}); err != nil {
		return nil, err
	}

	return task, nil
}
