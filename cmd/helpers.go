package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
	"github.com/sglavoie/tt/internal/workflow"
)

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
func LoadCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return nil, nil, nil, fmt.Errorf("no current project selected. Select one with: tt select")
	}

	if err := storage.LoadProject(project); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}

	return project, cfg, configManager, nil
}

// SaveProjectWithBackup saves the project after creating a backup.
// It also runs auto-unblock checks and reorders root tasks by completion status.
func SaveProjectWithBackup(project *models.Project) error {
	project.CheckAndAutoUnblock()
	project.ReorderRootTasksByCompletion()

	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return storage.SaveProject(project)
}

// SaveProjectWithRules saves the project after evaluating workflow rules.
// It runs auto-unblock checks, evaluates rules, creates a backup, and saves.
// Prints warnings to stderr if there are rule conflicts.
// If configManager is provided, trigger counts are persisted.
// This version does not support event-based triggers (use SaveProjectWithRulesAndChanges).
func SaveProjectWithRules(project *models.Project, cfg *config.Config, configManager *config.Manager) error {
	return SaveProjectWithRulesAndChanges(project, cfg, configManager, nil)
}

// SaveProjectWithRulesAndChanges saves the project after evaluating workflow rules.
// If prevStates is provided, it enables event-based triggers by comparing current
// state against previous state. Pass nil for prevStates to disable event triggers.
func SaveProjectWithRulesAndChanges(project *models.Project, cfg *config.Config, configManager *config.Manager, prevStates map[string]*models.Task) error {
	project.CheckAndAutoUnblock()

	// Evaluate workflow rules
	rules := cfg.GetEnabledWorkflowRules()
	if len(rules) > 0 {
		var result *workflow.EvaluationResult

		if prevStates != nil {
			// Use context-aware evaluation with event triggers
			evalCtx := BuildEvaluationContext(project.Tasks, prevStates)
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
		if len(result.RuleTriggerCounts) > 0 && configManager != nil {
			cfg.IncrementRuleTriggerCounts(result.RuleTriggerCounts)
			if err := configManager.Save(cfg); err != nil {
				fmt.Fprintln(os.Stderr, "Warning: failed to save rule trigger counts:", err)
			}
		}
	}

	project.ReorderRootTasksByCompletion()

	if err := storage.BackupProject(project); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return storage.SaveProject(project)
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
