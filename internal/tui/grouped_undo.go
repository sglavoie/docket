package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func tuiUndoProjectID(project *models.Project) string {
	if project == nil {
		return ""
	}
	if project.ID != "" {
		return project.ID
	}
	return project.Slug
}

type tuiUndoHistoryLoader func(*models.Project) ([]storage.UndoEntry, error)

func restoreProjectsForTopTUIHistoryEntry(project *models.Project, candidates []*models.Project, load tuiUndoHistoryLoader) []*models.Project {
	entries, err := load(project)
	if err != nil || len(entries) == 0 {
		return uniqueTUIUndoProjects([]*models.Project{project})
	}

	latest := entries[0]
	if latest.GroupID == "" && len(latest.GroupProjectIDs) == 0 {
		return uniqueTUIUndoProjects([]*models.Project{project})
	}

	projectsByID := make(map[string]*models.Project, len(candidates)+1)
	if id := tuiUndoProjectID(project); id != "" {
		projectsByID[id] = project
	}
	for _, candidate := range candidates {
		if id := tuiUndoProjectID(candidate); id != "" {
			projectsByID[id] = candidate
		}
	}

	projects := make([]*models.Project, 0, len(latest.GroupProjectIDs))
	seen := make(map[string]bool, len(latest.GroupProjectIDs))
	for _, id := range latest.GroupProjectIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if candidate := projectsByID[id]; candidate != nil {
			projects = append(projects, candidate)
		}
	}
	if len(projects) == 0 {
		projects = append(projects, project)
	}
	return uniqueTUIUndoProjects(projects)
}

func uniqueTUIUndoProjects(projects []*models.Project) []*models.Project {
	seen := make(map[string]bool, len(projects))
	unique := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		id := tuiUndoProjectID(project)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, project)
	}
	return unique
}

func activeUndoProjectsFromRuntime(runtime *config.RuntimeState) []*models.Project {
	if runtime == nil {
		return nil
	}
	return runtime.ActiveProjects()
}

func repairRuntimeTaskRefsAfterRestore(runtime *config.RuntimeState) postRestoreHook {
	return func(projects []*models.Project) error {
		if runtime == nil || len(projects) == 0 {
			return nil
		}
		taskProjects := make(map[string][]string)
		for _, project := range projects {
			projectID := tuiUndoProjectID(project)
			for _, task := range project.Tasks {
				if task != nil && task.ID != "" {
					taskProjects[task.ID] = append(taskProjects[task.ID], projectID)
				}
			}
		}
		planning := clonePlanningState(runtime.Planning())
		machine := cloneMachineState(runtime.Machine())
		planningChanged := false
		machineChanged := false
		for i := range planning.FocusQueue {
			if id, ok := restoredTaskProject(taskProjects, planning.FocusQueue[i].TaskID, planning.FocusQueue[i].ProjectID); ok && planning.FocusQueue[i].ProjectID != id {
				planning.FocusQueue[i].ProjectID = id
				planningChanged = true
			}
		}
		for i := range planning.DailyPlan {
			if id, ok := restoredTaskProject(taskProjects, planning.DailyPlan[i].TaskID, planning.DailyPlan[i].ProjectID); ok && planning.DailyPlan[i].ProjectID != id {
				planning.DailyPlan[i].ProjectID = id
				planningChanged = true
			}
		}
		for i := range machine.RecentTaskVisits {
			if id, ok := restoredTaskProject(taskProjects, machine.RecentTaskVisits[i].TaskID, machine.RecentTaskVisits[i].ProjectID); ok && machine.RecentTaskVisits[i].ProjectID != id {
				machine.RecentTaskVisits[i].ProjectID = id
				machineChanged = true
			}
		}
		if runtime.DataDir() == "" {
			if planningChanged {
				*runtime.Planning() = *planning
			}
			if machineChanged {
				runtime.SetMachineInMemory(machine)
			}
			return nil
		}
		if planningChanged {
			if err := runtime.CommitPlanning(planning); err != nil {
				return err
			}
		}
		if machineChanged {
			return runtime.CommitMachine(machine)
		}
		return nil
	}
}

func restoredTaskProject(taskProjects map[string][]string, taskID, currentProjectID string) (string, bool) {
	projects := taskProjects[taskID]
	for _, projectID := range projects {
		if projectID == currentProjectID {
			return currentProjectID, true
		}
	}
	if len(projects) == 1 {
		return projects[0], true
	}
	return "", false
}
