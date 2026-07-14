package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func repairConfigTaskRefsToLoadedProjects(runtime *config.RuntimeState, projects []*models.Project) error {
	if runtime == nil || len(projects) == 0 {
		return nil
	}

	taskProjects := make(map[string]string)
	for _, project := range projects {
		projectID := restoreProjectID(project)
		if projectID == "" {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.ID != "" {
				taskProjects[task.ID] = projectID
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
	planningUpdated := 0
	for i := range domains.Planning.FocusQueue {
		if projectID, ok := taskProjects[domains.Planning.FocusQueue[i].TaskID]; ok && domains.Planning.FocusQueue[i].ProjectID != projectID {
			domains.Planning.FocusQueue[i].ProjectID = projectID
			planningUpdated++
		}
	}
	for i := range domains.Planning.DailyPlan {
		if projectID, ok := taskProjects[domains.Planning.DailyPlan[i].TaskID]; ok && domains.Planning.DailyPlan[i].ProjectID != projectID {
			domains.Planning.DailyPlan[i].ProjectID = projectID
			planningUpdated++
		}
	}
	machineUpdated := 0
	for i := range domains.Machine.RecentTaskVisits {
		if projectID, ok := taskProjects[domains.Machine.RecentTaskVisits[i].TaskID]; ok && domains.Machine.RecentTaskVisits[i].ProjectID != projectID {
			domains.Machine.RecentTaskVisits[i].ProjectID = projectID
			machineUpdated++
		}
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
