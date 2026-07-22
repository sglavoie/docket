package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func repairConfigTaskRefsToLoadedProjects(runtime *config.RuntimeState, projects []*models.Project) error {
	if runtime == nil || len(projects) == 0 {
		return nil
	}

	restoredProjects := make(map[string]*models.Project)
	taskProjects := make(map[string]map[string]bool)
	for _, project := range projects {
		projectID := restoreProjectID(project)
		if projectID == "" {
			continue
		}
		restoredProjects[projectID] = project
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				if taskProjects[task.ID] == nil {
					taskProjects[task.ID] = make(map[string]bool)
				}
				taskProjects[task.ID][projectID] = true
			}
		}
	}
	if len(taskProjects) == 0 {
		return nil
	}

	domains, err := runtime.CloneDomains()
	if err != nil {
		return err
	}
	loadedProjects := make(map[string]*models.Project)
	missingProjects := make(map[string]bool)
	unknownProjects := make(map[string]bool)
	reassignment := func(projectID, taskID string) (string, bool) {
		if restored := restoredProjects[projectID]; restored != nil {
			if restored.GetTaskByID(taskID) != nil {
				return "", false
			}
		} else {
			current := loadedProjects[projectID]
			if current == nil && !missingProjects[projectID] && !unknownProjects[projectID] {
				current = runtime.GetProject(projectID)
				if current == nil {
					missingProjects[projectID] = true
				} else if current.GetTaskByID(taskID) == nil && current.Path != "" {
					if loadErr := storage.LoadProject(current); loadErr != nil {
						// A failed current-owner check is uncertain, so fail closed and
						// leave every reference to that project unchanged.
						unknownProjects[projectID] = true
						current = nil
					}
				}
				if current != nil {
					loadedProjects[projectID] = current
				}
			}
			if unknownProjects[projectID] {
				return "", false
			}
			if current != nil && current.GetTaskByID(taskID) != nil {
				return "", false
			}
		}

		owners := taskProjects[taskID]
		if len(owners) != 1 {
			return "", false
		}
		for owner := range owners {
			if owner != projectID {
				return owner, true
			}
		}
		return "", false
	}
	planningUpdated := 0
	for i := range domains.Planning.FocusQueue {
		if projectID, ok := reassignment(domains.Planning.FocusQueue[i].ProjectID, domains.Planning.FocusQueue[i].TaskID); ok {
			domains.Planning.FocusQueue[i].ProjectID = projectID
			planningUpdated++
		}
	}
	for i := range domains.Planning.DailyPlan {
		if projectID, ok := reassignment(domains.Planning.DailyPlan[i].ProjectID, domains.Planning.DailyPlan[i].TaskID); ok {
			domains.Planning.DailyPlan[i].ProjectID = projectID
			planningUpdated++
		}
	}
	machineUpdated := 0
	for i := range domains.Machine.RecentTaskVisits {
		if projectID, ok := reassignment(domains.Machine.RecentTaskVisits[i].ProjectID, domains.Machine.RecentTaskVisits[i].TaskID); ok {
			domains.Machine.RecentTaskVisits[i].ProjectID = projectID
			machineUpdated++
		}
	}
	if runtime.DataDir() == "" {
		if planningUpdated > 0 || machineUpdated > 0 {
			runtime.SetDomainsInMemory(domains)
		}
		return nil
	}
	if planningUpdated > 0 {
		if err := runtime.CommitPlanning(domains.Planning); err != nil {
			return fmt.Errorf("failed to repair planning references after restore: %w", err)
		}
	}
	if machineUpdated > 0 {
		if err := runtime.CommitMachine(domains.Machine); err != nil {
			return fmt.Errorf("failed to repair recent visits after restore: %w", err)
		}
	}
	return nil
}
