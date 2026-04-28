package tui

import (
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// loadRelations collects all linked tasks and their relationship types.
func (m *LinkedTaskSelectorModel) loadRelations() {
	m.relations = nil

	// Tasks that this task blocks (i.e., tasks that depend on this task).
	blockedTasks := m.project.GetBlockedByTasks(m.sourceTask)
	for _, task := range blockedTasks {
		m.relations = append(m.relations, LinkedTaskRelation{
			Task:         task,
			RelationType: "blocks",
			Direction:    "→",
			TaskRef:      task.ID,
		})
	}

	for _, depRef := range m.sourceTask.DependsOn {
		m.relations = append(m.relations, m.resolveTaskRef(depRef, "blocked by", "←"))
	}

	for _, relRef := range m.sourceTask.RelatedTo {
		m.relations = append(m.relations, m.resolveTaskRef(relRef, "related", "↔"))
	}
}

// resolveTaskRef resolves a task reference (local or cross-project) to a LinkedTaskRelation.
func (m *LinkedTaskSelectorModel) resolveTaskRef(ref, relationType, direction string) LinkedTaskRelation {
	missing := func(project *models.Project) LinkedTaskRelation {
		return LinkedTaskRelation{
			Project:      project,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
			IsMissing:    true,
		}
	}

	taskRef := models.ParseTaskRef(ref)
	if !taskRef.IsCrossProject() {
		task := m.project.GetTaskByID(taskRef.TaskID)
		if task == nil {
			return missing(nil)
		}

		return LinkedTaskRelation{
			Task:         task,
			RelationType: relationType,
			Direction:    direction,
			TaskRef:      ref,
		}
	}

	if m.config == nil {
		return missing(nil)
	}

	targetProject := m.config.GetProject(taskRef.ProjectID)
	if targetProject == nil {
		return missing(nil)
	}

	if len(targetProject.Tasks) == 0 {
		if err := storage.LoadProject(targetProject); err != nil {
			logging.Warn("failed to load project %s: %v", targetProject.Name, err)
		}
	}

	task := targetProject.GetTaskByID(taskRef.TaskID)
	if task == nil {
		return missing(targetProject)
	}

	return LinkedTaskRelation{
		Task:         task,
		Project:      targetProject,
		RelationType: relationType,
		Direction:    direction,
		TaskRef:      ref,
	}
}