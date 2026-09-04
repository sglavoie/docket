package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
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

func repairRuntimeTaskRefsAfterRestore(runtime *config.RuntimeState) func([]*models.Project) error {
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
		planning := coordinator.ClonePlanningState(runtime.Planning())
		planningChanged := false
		for i := range planning.Next {
			if id, ok := restoredTaskProject(taskProjects, planning.Next[i].TaskID, planning.Next[i].ProjectID); ok && planning.Next[i].ProjectID != id {
				planning.Next[i].ProjectID = id
				planningChanged = true
			}
		}
		planning.SetNext(planning.Next)
		for i := range planning.Today {
			if id, ok := restoredTaskProject(taskProjects, planning.Today[i].TaskID, planning.Today[i].ProjectID); ok && planning.Today[i].ProjectID != id {
				planning.Today[i].ProjectID = id
				planningChanged = true
			}
		}
		if runtime.DataDir() == "" {
			if planningChanged {
				*runtime.Planning() = *planning
			}
			return nil
		}
		if planningChanged {
			if err := runtime.CommitPlanning(planning); err != nil {
				return err
			}
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
