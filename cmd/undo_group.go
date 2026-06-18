package cmd

import (
	"fmt"
	"sort"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func recordGroupedUndoSnapshots(projects []*models.Project, action, context string, depth int) (string, error) {
	if depth <= 0 {
		return "", nil
	}

	projects = uniqueUndoProjects(projects)
	if len(projects) == 0 {
		return "", nil
	}

	groupID := uuid.New().String()
	groupProjectIDs := undoGroupProjectIDs(projects)
	for _, project := range projects {
		if err := storage.RecordUndoSnapshotWithOptions(project, storage.UndoSnapshotOptions{
			Action:          action,
			Context:         context,
			MaxDepth:        depth,
			GroupID:         groupID,
			GroupProjectIDs: groupProjectIDs,
		}); err != nil {
			return "", fmt.Errorf("failed to record grouped undo snapshot for %s: %w", project.Name, err)
		}
	}
	return groupID, nil
}

func uniqueUndoProjects(projects []*models.Project) []*models.Project {
	seen := make(map[string]bool, len(projects))
	unique := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		id := restoreProjectID(project)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		unique = append(unique, project)
	}
	return unique
}

func undoGroupProjectIDs(projects []*models.Project) []string {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		if id := restoreProjectID(project); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func repairConfigTaskRefsToLoadedProjects(mgr *config.Manager, cfg *config.Config, projects []*models.Project) error {
	if mgr == nil || cfg == nil || len(projects) == 0 {
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

	updated := 0
	for i := range cfg.FocusQueue {
		if projectID, ok := taskProjects[cfg.FocusQueue[i].TaskID]; ok && cfg.FocusQueue[i].ProjectID != projectID {
			cfg.FocusQueue[i].ProjectID = projectID
			updated++
		}
	}
	for i := range cfg.DailyPlan {
		if projectID, ok := taskProjects[cfg.DailyPlan[i].TaskID]; ok && cfg.DailyPlan[i].ProjectID != projectID {
			cfg.DailyPlan[i].ProjectID = projectID
			updated++
		}
	}
	for i := range cfg.RecentTaskVisits {
		if projectID, ok := taskProjects[cfg.RecentTaskVisits[i].TaskID]; ok && cfg.RecentTaskVisits[i].ProjectID != projectID {
			cfg.RecentTaskVisits[i].ProjectID = projectID
			updated++
		}
	}
	if updated == 0 {
		return nil
	}
	return saveConfig(mgr, cfg)
}
