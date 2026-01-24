package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/tui"
	"github.com/sglavoie/tt/internal/workflow"
)

// Dependencies holds injectable dependencies for commands.
// This allows swapping implementations for testing.
type Dependencies struct {
	Selector   external.FuzzySelector
	Renderer   external.MarkdownRenderer
	Editor     external.Editor
	Repository storage.ProjectRepository
}

// Deps is the global dependencies instance used by commands.
// Tests can replace this with mock implementations.
var Deps = Dependencies{
	Selector:   external.DefaultSelector,
	Renderer:   external.DefaultRenderer,
	Editor:     external.DefaultEditor,
	Repository: storage.DefaultRepository,
}

// Debugf prints a formatted message to stderr if verbose mode is enabled.
func Debugf(format string, args ...any) {
	if IsVerbose() {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}

// LoadConfig loads the configuration file.
// Returns the config, manager, and any error encountered.
func LoadConfig() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, configManager, nil
}

// LoadCurrentProject loads the config and current project with its tasks.
// Returns an error if no project is selected or if loading fails.
// Parser warnings are printed to stderr (always for data corrections, verbose mode for others).
func LoadCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return nil, nil, nil, fmt.Errorf("%w. Select one with: tt select", tterrors.ErrNoCurrentProject)
	}

	Debugf("loading project: %s (%s)", project.Name, project.Path)

	result, err := storage.LoadProjectWithWarnings(project)
	if err != nil {
		if errors.Is(err, tterrors.ErrParseError) {
			return nil, nil, nil, fmt.Errorf("corrupted tasks file in %s: %w", project.Name, err)
		}
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}

	Debugf("loaded %d tasks", len(project.Tasks))

	// Print warnings - always show data correction warnings, others only in verbose mode
	for _, w := range result.Warnings {
		// Data correction warnings (like cleared parent references) are always shown
		if strings.Contains(w.Message, "clearing") || strings.Contains(w.Message, "corrected") {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w.Message)
		} else {
			Debugf("parser warning: %s", w.Message)
		}
	}

	return project, cfg, configManager, nil
}

// SaveOptions configures the behavior of SaveProject.
type SaveOptions struct {
	// EvaluateRules enables workflow rule evaluation before saving.
	EvaluateRules bool
	// PrevStates provides previous task states for event-based triggers.
	// If nil, event triggers are disabled.
	PrevStates map[string]*models.Task
	// Config is required when EvaluateRules is true.
	Config *config.Config
	// ConfigManager is required when EvaluateRules is true.
	ConfigManager *config.Manager
}

// SaveProject saves the project with optional workflow rule evaluation.
// If opts is nil, performs basic backup and save with auto-unblock and reordering.
// If opts.EvaluateRules is true, evaluates workflow rules before saving.
// Prints warnings to stderr if there are rule conflicts.
func SaveProject(project *models.Project, opts *SaveOptions) error {
	project.CheckAndAutoUnblock()

	// Evaluate workflow rules if requested
	if opts != nil && opts.EvaluateRules {
		if opts.Config == nil || opts.ConfigManager == nil {
			return fmt.Errorf("config and ConfigManager required when EvaluateRules is true")
		}

		rules := opts.Config.GetEnabledWorkflowRules()
		if len(rules) > 0 {
			Debugf("evaluating %d workflow rules", len(rules))
			var result *workflow.EvaluationResult

			if opts.PrevStates != nil {
				// Use context-aware evaluation with event triggers
				evalCtx := BuildEvaluationContext(project.Tasks, opts.PrevStates)
				result = workflow.EvaluateRulesWithContext(evalCtx, rules)
			} else {
				// Use standard evaluation (no event triggers)
				result = workflow.EvaluateRules(project.Tasks, rules)
			}

			if len(result.Conflicts) > 0 {
				fmt.Fprintln(os.Stderr, "Warning: workflow rule conflicts detected:")
				fmt.Fprintln(os.Stderr, workflow.FormatConflicts(result.Conflicts))
			}
			// Persist trigger counts if any rules fired
			if len(result.RuleTriggerCounts) > 0 {
				Debugf("workflow rules triggered: %v", result.RuleTriggerCounts)
				opts.Config.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
				if err := opts.ConfigManager.Save(opts.Config); err != nil {
					fmt.Fprintln(os.Stderr, "Warning: failed to save rule trigger counts:", err)
				}
			}
		}
	}

	project.ReorderRootTasksByCompletion()

	Debugf("saving project: %s", project.Name)
	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return storage.SaveProject(project)
}

// SaveProjectWithBackup saves the project after creating a backup.
// It also runs auto-unblock checks and reorders root tasks by completion status.
// Deprecated: Use SaveProject with nil opts instead.
func SaveProjectWithBackup(project *models.Project) error {
	return SaveProject(project, nil)
}

// SaveProjectWithRules saves the project after evaluating workflow rules.
// It runs auto-unblock checks, evaluates rules, creates a backup, and saves.
// Prints warnings to stderr if there are rule conflicts.
// If configManager is provided, trigger counts are persisted.
// This version does not support event-based triggers (use SaveProjectWithRulesAndChanges).
// Deprecated: Use SaveProject with SaveOptions instead.
func SaveProjectWithRules(project *models.Project, cfg *config.Config, configManager *config.Manager) error {
	return SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
	})
}

// SaveProjectWithRulesAndChanges saves the project after evaluating workflow rules.
// If prevStates is provided, it enables event-based triggers by comparing current
// state against previous state. Pass nil for prevStates to disable event triggers.
// Deprecated: Use SaveProject with SaveOptions instead.
func SaveProjectWithRulesAndChanges(project *models.Project, cfg *config.Config, configManager *config.Manager, prevStates map[string]*models.Task) error {
	return SaveProject(project, &SaveOptions{
		EvaluateRules: true,
		Config:        cfg,
		ConfigManager: configManager,
		PrevStates:    prevStates,
	})
}

// CaptureTaskStates creates a snapshot of task states for event trigger detection.
// Call this before making changes to tasks to capture their previous state.
func CaptureTaskStates(tasks []*models.Task) map[string]*models.Task {
	states := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		states[task.ID] = copyTaskState(task)
	}
	return states
}

// copyTaskState creates a shallow copy of task fields relevant for event detection.
func copyTaskState(task *models.Task) *models.Task {
	if task == nil {
		return nil
	}
	// Copy tags slice
	tagsCopy := make([]string, len(task.Tags))
	copy(tagsCopy, task.Tags)

	return &models.Task{
		ID:       task.ID,
		Status:   task.Status,
		Priority: task.Priority,
		Section:  task.Section,
		Tags:     tagsCopy,
		Type:     task.Type,
		// Preserve parent/child relationships for hierarchical triggers
		Parent:   task.Parent,
		Children: task.Children,
		Level:    task.Level,
	}
}

// BuildEvaluationContext creates an evaluation context for workflow rules.
func BuildEvaluationContext(tasks []*models.Task, prevStates map[string]*models.Task) *workflow.EvaluationContext {
	changes := make([]workflow.TaskChangeContext, len(tasks))
	taskMap := make(map[string]*models.Task, len(tasks))

	for i, task := range tasks {
		taskMap[task.ID] = task
		changes[i] = workflow.TaskChangeContext{
			Task:     task,
			Previous: prevStates[task.ID], // nil if task is new
		}
	}

	return &workflow.EvaluationContext{
		Changes:  changes,
		AllTasks: tasks,
		TaskMap:  taskMap,
	}
}

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
func ParseTaskTypeFlags(epic, story, bug, idea, note bool, typeFlag string) (models.TaskType, error) {
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
	if note {
		return models.TaskTypeNote, nil
	}
	if typeFlag != "" {
		t, ok := models.ParseTaskType(typeFlag)
		if !ok {
			return "", fmt.Errorf("invalid task type: %s", typeFlag)
		}
		return t, nil
	}
	return models.TaskTypeTask, nil
}

// TasksToSelectorItems converts tasks to selector items with standard columns.
func TasksToSelectorItems(tasks []*models.Task) []tui.SelectorItem {
	headers := []string{"ID", "Type", "Status", "Title"}
	items := make([]tui.SelectorItem, len(tasks))
	for i, t := range tasks {
		items[i] = tui.SelectorItem{
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

// GetTaskFromSelector safely extracts a Task from a SelectorItem.
// Returns an error if the data is not a *models.Task.
func GetTaskFromSelector(item *tui.SelectorItem) (*models.Task, error) {
	task, ok := item.Data.(*models.Task)
	if !ok {
		return nil, fmt.Errorf("internal error: expected Task, got %T", item.Data)
	}
	return task, nil
}

// TaskFilter is a predicate function for filtering tasks.
type TaskFilter func(*models.Task) bool

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
	if err := SaveProjectWithRulesAndChanges(project, cfg, configManager, prevStates); err != nil {
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
	if err := SaveProjectWithRulesAndChanges(project, cfg, configManager, prevStates); err != nil {
		return nil, err
	}

	return task, nil
}

// GetProjectFromSelector safely extracts a Project from a SelectorItem.
// Returns an error if the data is not a *models.Project.
func GetProjectFromSelector(item *tui.SelectorItem) (*models.Project, error) {
	project, ok := item.Data.(*models.Project)
	if !ok {
		return nil, fmt.Errorf("internal error: expected Project, got %T", item.Data)
	}
	return project, nil
}

// ProjectsToSelectorItems converts projects to selector items.
// If includeActive is true, includes the "Active" column.
func ProjectsToSelectorItems(projects []*models.Project, includeActive bool) []tui.SelectorItem {
	var headers []string
	if includeActive {
		headers = []string{"Project", "Context", "Tasks", "Active"}
	} else {
		headers = []string{"Project", "Context", "Tasks"}
	}

	items := make([]tui.SelectorItem, len(projects))
	for i, p := range projects {
		columns := []string{
			p.Name,
			p.Context,
			fmt.Sprintf("%d tasks", p.CountTasks()),
		}
		if includeActive {
			columns = append(columns, fmt.Sprintf("%d active", p.CountActiveTasks()))
		}
		items[i] = tui.SelectorItem{
			ID:      p.ID,
			Data:    p,
			Headers: headers,
			Columns: columns,
		}
	}
	return items
}

// WorkflowEvalResult holds the result of evaluating workflow rules for a project.
type WorkflowEvalResult struct {
	ProjectName   string
	ModifiedTasks int
	RulesApplied  map[string]int // rule ID -> count of tasks affected
	Skipped       bool           // true if evaluation was skipped (recently evaluated)
	Error         error
}

// EvaluateProjectWorkflowRules evaluates workflow rules for a project.
// If force is false and the project was recently evaluated, evaluation is skipped.
// The project must already be loaded with tasks.
// Returns the evaluation result and updates the eval state in config.
func EvaluateProjectWorkflowRules(project *models.Project, cfg *config.Config, configManager *config.Manager, force bool) WorkflowEvalResult {
	result := WorkflowEvalResult{
		ProjectName:  project.Name,
		RulesApplied: make(map[string]int),
	}

	// Check if evaluation is needed
	if !force && !cfg.ShouldEvaluateWorkflow(project.ID) {
		result.Skipped = true
		return result
	}

	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) == 0 {
		result.Skipped = true
		return result
	}

	// Run auto-unblock before rule evaluation
	project.CheckAndAutoUnblock()

	// Evaluate rules
	evalResult := workflow.EvaluateRules(project.Tasks, rules)

	result.ModifiedTasks = len(evalResult.ModifiedTasks)
	result.RulesApplied = evalResult.RuleTriggerCounts

	// Only save if there were actual modifications
	if result.ModifiedTasks > 0 {
		project.ReorderRootTasksByCompletion()

		if err := storage.BackupProject(project); err != nil {
			result.Error = fmt.Errorf("failed to create backup: %w", err)
			return result
		}

		if err := storage.SaveProject(project); err != nil {
			result.Error = fmt.Errorf("failed to save project: %w", err)
			return result
		}

		// Update trigger counts
		if len(evalResult.RuleTriggerCounts) > 0 && configManager != nil {
			cfg.IncrementRuleTriggerCounts(evalResult.RuleTriggerCounts)
		}
	}

	// Update eval state
	cfg.SetWorkflowEvalState(project.ID, config.WorkflowEvalState{
		LastEvalTime: time.Now(),
		RulesHash:    cfg.ComputeWorkflowRulesHash(),
	})

	// Persist config changes
	if configManager != nil {
		if err := configManager.Save(cfg); err != nil {
			// Non-fatal, just log
			fmt.Fprintln(os.Stderr, "Warning: failed to save workflow eval state:", err)
		}
	}

	return result
}
