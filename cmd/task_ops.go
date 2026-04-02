package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/tui"
)

// FindTaskByIdentifier finds a task by short ID, UUID, or title prefix.
// Returns an error if no match is found or if a title prefix matches multiple tasks.
func FindTaskByIdentifier(project *models.Project, identifier string) (*models.Task, error) {
	// Try as short ID first (must be purely numeric)
	if shortID, err := strconv.Atoi(identifier); err == nil {
		if task := project.GetTaskByShortID(shortID); task != nil {
			return task, nil
		}
	}

	// Try as UUID
	if task := project.GetTaskByID(identifier); task != nil {
		return task, nil
	}

	// Try as title prefix (case-insensitive)
	lowerID := strings.ToLower(identifier)
	var matches []*models.Task
	for _, task := range project.Tasks {
		if strings.HasPrefix(strings.ToLower(task.Title), lowerID) {
			matches = append(matches, task)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		// Build candidate list (show up to 5)
		limit := min(len(matches), 5)
		parts := make([]string, limit)
		for i := 0; i < limit; i++ {
			parts[i] = fmt.Sprintf("#%d %s", matches[i].ShortID, matches[i].Title)
		}
		candidates := strings.Join(parts, ", ")
		if len(matches) > 5 {
			candidates += fmt.Sprintf(", ... (%d more)", len(matches)-5)
		}
		return nil, tterrors.NewAmbiguousIdentifierError(identifier, candidates)
	}

	// No match found — suggest similar tasks using fuzzy matching
	if suggestions := suggestSimilarTasks(project.Tasks, identifier, 3); len(suggestions) > 0 {
		return nil, fmt.Errorf("%w: '%s'. Did you mean: %s", tterrors.ErrTaskNotFound, identifier, strings.Join(suggestions, ", "))
	}

	return nil, tterrors.NewTaskNotFoundError(identifier)
}

// ValidatePriority checks that a priority value is in the valid range.
// If allowZero is true, 0 is accepted (meaning "use default").
func ValidatePriority(value int, allowZero bool) error {
	return models.ValidatePriority(value, allowZero)
}

// ParseTaskTypeFlags determines the task type from boolean flags or a string flag.
// Returns an error if the type flag is invalid or if multiple type flags are set.
func ParseTaskTypeFlags(epic, story, bug, idea bool, typeFlag string) (models.TaskType, error) {
	// Count how many boolean type flags are set
	flagCount := 0
	var flags []string
	if epic {
		flagCount++
		flags = append(flags, "--epic")
	}
	if story {
		flagCount++
		flags = append(flags, "--story")
	}
	if bug {
		flagCount++
		flags = append(flags, "--bug")
	}
	if idea {
		flagCount++
		flags = append(flags, "--idea")
	}
	if typeFlag != "" {
		flagCount++
		flags = append(flags, "--type="+typeFlag)
	}
	if flagCount > 1 {
		return "", fmt.Errorf("conflicting type flags: %s (specify only one)", strings.Join(flags, ", "))
	}

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
		task, err := FindTaskByIdentifier(project, args[0])
		if err != nil {
			return nil, err
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

// SelectActionTask handles the common pattern for board commands that operate on action tasks.
// If args contains an ID, finds the task directly across all loaded projects.
// Otherwise, presents an interactive selector across all action tasks.
// Returns the task, its project, and an error.
func SelectActionTask(cfg *config.Config, args []string, prompt, emptyMsg string) (*models.Task, *models.Project, error) {
	if len(args) > 0 {
		task, project := FindActionTaskByID(cfg, args[0])
		if task == nil {
			return nil, nil, fmt.Errorf("action not found: %s. Use 'tt board list' to see available actions", args[0])
		}
		return task, project, nil
	}

	var candidates []*models.Task
	taskToProject := map[string]*models.Project{}
	for _, proj := range cfg.GetAllLoadableProjects() {
		if err := storage.LoadProject(proj); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", proj.Name, err)
			continue
		}
		for _, t := range proj.Tasks {
			candidates = append(candidates, t)
			taskToProject[t.ID] = proj
		}
	}

	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("%s", emptyMsg)
	}

	items := TasksToSelectorItems(candidates)
	selected, err := tui.RunSelector(prompt, items)
	if err != nil {
		return nil, nil, err
	}

	task, err := GetTaskFromSelector(selected)
	if err != nil {
		return nil, nil, err
	}
	return task, taskToProject[task.ID], nil
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

	task, findErr := FindTaskByIdentifier(project, taskIdentifier)
	if findErr != nil {
		return nil, findErr
	}

	// Capture task states before changes for event-based workflow triggers
	prevStates := CaptureTaskStates(project.Tasks)

	// Apply the update
	if err := updateFn(task); err != nil {
		return nil, err
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Save with workflow rules evaluation and undo recording
	undoContext := fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: configManager,
		PrevStates: prevStates, UndoAction: "Edit Task", UndoContext: undoContext,
	}); err != nil {
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

	task, findErr := FindTaskByIdentifier(project, taskIdentifier)
	if findErr != nil {
		return nil, findErr
	}

	// Capture task states before changes for event-based workflow triggers
	prevStates := CaptureTaskStates(project.Tasks)

	// Apply the update
	if err := updateFn(project, task); err != nil {
		return nil, err
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Save with workflow rules evaluation and undo recording
	undoContext := fmt.Sprintf("#%d %s", task.ShortID, task.Title)
	if err := SaveProject(project, &SaveOptions{
		EvaluateRules: true, Config: cfg, ConfigManager: configManager,
		PrevStates: prevStates, UndoAction: "Edit Task", UndoContext: undoContext,
	}); err != nil {
		return nil, err
	}

	return task, nil
}

// WithTaskRead encapsulates the common Load→FindTask→Read pattern for read-only commands.
// It loads the current project, finds the task, and passes it to the read function.
// No save is performed.
func WithTaskRead(taskIdentifier string, readFn func(*models.Project, *models.Task) error) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	task, findErr := FindTaskByIdentifier(project, taskIdentifier)
	if findErr != nil {
		return findErr
	}

	return readFn(project, task)
}

// parseItemIndex parses and validates an index argument against a collection length.
// Returns the parsed index or an error with a descriptive message using itemName.
func parseItemIndex(arg string, count int, itemName string) (int, error) {
	index, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("invalid %s index: %s", itemName, arg)
	}
	if count == 0 {
		return 0, fmt.Errorf("no %ss to index", itemName)
	}
	if index < 0 || index >= count {
		return 0, fmt.Errorf("%s index %d out of range (0-%d)", itemName, index, count-1)
	}
	return index, nil
}

// resolveEditor returns the flag value if set, otherwise falls back to the config editor.
func resolveEditor(flag, cfgEditor string) string {
	if flag != "" {
		return flag
	}
	return cfgEditor
}

// WithBlockerUpdate encapsulates the common Load→Find→Update→Save pattern for blocker commands.
// It loads the current project's blockers, finds one by ID prefix, applies the update function,
// and saves the (potentially modified) blocker list back.
// The update function receives the full blocker list and the matched blocker, and returns
// the blocker list to save (allowing deletion by returning a filtered list).
func WithBlockerUpdate(idPrefix string, updateFn func([]*models.Blocker, *models.Blocker) ([]*models.Blocker, error)) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return fmt.Errorf("failed to load blockers: %w", err)
	}

	blocker := storage.FindBlockerByPrefix(blockers, idPrefix)
	if blocker == nil {
		return fmt.Errorf("blocker not found: %s", idPrefix)
	}

	blockers, err = updateFn(blockers, blocker)
	if err != nil {
		return err
	}

	if err := storage.SaveBlockers(project, blockers); err != nil {
		return fmt.Errorf("failed to save blockers: %w", err)
	}

	return nil
}

// WithBlockerCreate encapsulates the common Load→Create→Save pattern for adding a blocker.
// It loads the current project's blockers, applies the create function to produce a new blocker,
// appends it to the list, and saves.
func WithBlockerCreate(createFn func() (*models.Blocker, error)) (*models.Blocker, error) {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return nil, err
	}

	blockers, err := storage.LoadBlockers(project)
	if err != nil {
		return nil, fmt.Errorf("failed to load blockers: %w", err)
	}

	blocker, err := createFn()
	if err != nil {
		return nil, err
	}

	blockers = append(blockers, blocker)
	if err := storage.SaveBlockers(project, blockers); err != nil {
		return nil, fmt.Errorf("failed to save blockers: %w", err)
	}

	return blocker, nil
}

// ValidateParentTask finds a parent task by identifier and validates that the child type
// can be nested under it. Returns the parent task or an error.
func ValidateParentTask(project *models.Project, parentIdentifier string, childType models.TaskType) (*models.Task, error) {
	parent, findErr := FindTaskByIdentifier(project, parentIdentifier)
	if findErr != nil {
		return nil, findErr
	}

	if !childType.CanBeChildOf(parent.Type) {
		return nil, fmt.Errorf("%w: %s cannot be a child of %s", tterrors.ErrInvalidParent,
			childType.HeadingPrefix(), parent.Type.HeadingPrefix())
	}

	// Reject if adding a child would exceed the maximum nesting depth.
	// Without this check the task is created with the parent reference, but
	// BuildTaskTree silently detaches it at save time — misleading the user.
	if parent.Level+1 > models.MaxNestingDepth {
		return nil, fmt.Errorf("%w: nesting under #%d would exceed maximum depth of %d levels",
			tterrors.ErrInvalidParent, parent.ShortID, models.MaxNestingDepth)
	}

	return parent, nil
}

// suggestSimilarTasks returns formatted suggestions for tasks whose titles
// fuzzy-match the given identifier, sorted by score descending.
func suggestSimilarTasks(tasks []*models.Task, identifier string, limit int) []string {
	type scored struct {
		task  *models.Task
		score int
	}
	var candidates []scored
	for _, t := range tasks {
		if s := fuzzy.Score(t.Title, identifier); s > 0 {
			candidates = append(candidates, scored{task: t, score: s})
		}
	}
	// Sort by score descending
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	suggestions := make([]string, len(candidates))
	for i, c := range candidates {
		suggestions[i] = fmt.Sprintf("#%d %s", c.task.ShortID, c.task.Title)
	}
	return suggestions
}
