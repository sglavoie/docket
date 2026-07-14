package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/selector"
)

// GetProjectFromSelector safely extracts a Project from a selector.Item.
// Returns an error if the data is not a *models.Project.
func GetProjectFromSelector(item *selector.Item) (*models.Project, error) {
	project, ok := item.Data.(*models.Project)
	if !ok {
		return nil, fmt.Errorf("internal error: expected Project, got %T", item.Data)
	}
	return project, nil
}

// ProjectsToSelectorItems converts projects to selector items.
// If includeActive is true, includes the "Active" column.
func ProjectsToSelectorItems(projects []*models.Project, includeActive bool) []selector.Item {
	var headers []string
	if includeActive {
		headers = []string{"Project", "Context", "Tasks", "Active"}
	} else {
		headers = []string{"Project", "Context", "Tasks"}
	}

	items := make([]selector.Item, len(projects))
	for i, p := range projects {
		columns := []string{
			p.Name,
			p.Context,
			fmt.Sprintf("%d tasks", p.CountTasks()),
		}
		if includeActive {
			columns = append(columns, fmt.Sprintf("%d active", p.CountActiveTasks()))
		}
		items[i] = selector.Item{
			ID:      p.ID,
			Data:    p,
			Headers: headers,
			Columns: columns,
		}
	}
	return items
}
