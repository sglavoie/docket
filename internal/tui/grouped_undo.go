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

func repairConfigTaskRefsToLoadedProjects(mgr *config.Manager, cfg *config.Config, projects []*models.Project) (int, error) {
	if cfg == nil || len(projects) == 0 {
		return 0, nil
	}

	taskProjects := make(map[string]string)
	for _, project := range projects {
		projectID := tuiUndoProjectID(project)
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
		return 0, nil
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
	if updated > 0 && mgr != nil {
		return updated, mgr.Save(cfg)
	}
	return updated, nil
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

func activeUndoProjects(cfg *config.Config) []*models.Project {
	if cfg == nil {
		return nil
	}
	return cfg.GetActiveProjects()
}

func repairConfigTaskRefsAfterRestore(mgr *config.Manager, cfg *config.Config) postRestoreHook {
	return func(projects []*models.Project) error {
		_, err := repairConfigTaskRefsToLoadedProjects(mgr, cfg, projects)
		return err
	}
}
