package cmd

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type taskRefRepairer struct {
	runtime          *config.RuntimeState
	restoredProjects map[string]*models.Project
	taskProjects     map[string]map[string]bool
	loadedProjects   map[string]*models.Project
	missingProjects  map[string]bool
	unknownProjects  map[string]bool
}

func newTaskRefRepairer(runtime *config.RuntimeState, projects []*models.Project) *taskRefRepairer {
	repairer := &taskRefRepairer{
		runtime:          runtime,
		restoredProjects: make(map[string]*models.Project),
		taskProjects:     make(map[string]map[string]bool),
		loadedProjects:   make(map[string]*models.Project),
		missingProjects:  make(map[string]bool),
		unknownProjects:  make(map[string]bool),
	}
	for _, project := range projects {
		projectID := restoreProjectID(project)
		if projectID == "" {
			continue
		}
		repairer.restoredProjects[projectID] = project
		for _, task := range project.Tasks {
			if task == nil || task.ID == "" {
				continue
			}
			if repairer.taskProjects[task.ID] == nil {
				repairer.taskProjects[task.ID] = make(map[string]bool)
			}
			repairer.taskProjects[task.ID][projectID] = true
		}
	}
	return repairer
}

func (r *taskRefRepairer) currentOwnerRetainsReference(projectID, taskID string) bool {
	current := r.loadedProjects[projectID]
	if current == nil && !r.missingProjects[projectID] && !r.unknownProjects[projectID] {
		current = r.runtime.GetProject(projectID)
		if current == nil {
			r.missingProjects[projectID] = true
		} else if current.GetTaskByID(taskID) == nil && current.Path != "" {
			if err := storage.LoadProject(current); err != nil {
				// A failed current-owner check is uncertain, so fail closed and
				// leave every reference to that project unchanged.
				r.unknownProjects[projectID] = true
				current = nil
			}
		}
		if current != nil {
			r.loadedProjects[projectID] = current
		}
	}
	return r.unknownProjects[projectID] || current != nil && current.GetTaskByID(taskID) != nil
}

func (r *taskRefRepairer) reassignment(projectID, taskID string) (string, bool) {
	if restored := r.restoredProjects[projectID]; restored != nil {
		if restored.GetTaskByID(taskID) != nil {
			return "", false
		}
	} else if r.currentOwnerRetainsReference(projectID, taskID) {
		return "", false
	}

	owners := r.taskProjects[taskID]
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

func repairConfigTaskRefsToLoadedProjects(runtime *config.RuntimeState, projects []*models.Project) error {
	if runtime == nil || len(projects) == 0 {
		return nil
	}

	repairer := newTaskRefRepairer(runtime, projects)
	if len(repairer.taskProjects) == 0 {
		return nil
	}

	domains, err := runtime.CloneDomains()
	if err != nil {
		return err
	}
	planningUpdated := 0
	for i := range domains.Planning.FocusQueue {
		if projectID, ok := repairer.reassignment(domains.Planning.FocusQueue[i].ProjectID, domains.Planning.FocusQueue[i].TaskID); ok {
			domains.Planning.FocusQueue[i].ProjectID = projectID
			planningUpdated++
		}
	}
	for i := range domains.Planning.DailyPlan {
		if projectID, ok := repairer.reassignment(domains.Planning.DailyPlan[i].ProjectID, domains.Planning.DailyPlan[i].TaskID); ok {
			domains.Planning.DailyPlan[i].ProjectID = projectID
			planningUpdated++
		}
	}
	machineUpdated := 0
	for i := range domains.Machine.RecentTaskVisits {
		if projectID, ok := repairer.reassignment(domains.Machine.RecentTaskVisits[i].ProjectID, domains.Machine.RecentTaskVisits[i].TaskID); ok {
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
