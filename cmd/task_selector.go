package cmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
)

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
