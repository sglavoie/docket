package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type globalTagRenameTarget struct {
	project   *models.Project
	taskCount int
}

// renameGlobalTag renames a tag across all projects that use it.
func (m *GlobalTagManagerModel) renameGlobalTag(oldName, newName string) tea.Cmd {
	// Capture tag info synchronously to avoid data races with View()
	oldName = models.NormalizeTag(oldName)
	newName = models.NormalizeTag(newName)
	if newName == "" || newName == oldName {
		return nil
	}

	// Find the tag info and snapshot the project list
	var projects []*models.Project
	for i := range m.tags {
		if m.tags[i].Name == oldName {
			for _, usage := range m.tags[i].Projects {
				projects = append(projects, usage.Project)
			}
			break
		}
	}
	if len(projects) == 0 {
		return nil
	}

	return func() tea.Msg {
		targets := make([]globalTagRenameTarget, 0, len(projects))
		for _, project := range projects {
			// Reload project to ensure fresh data
			if err := storage.LoadProject(project); err != nil {
				continue
			}
			if !project.HasTag(oldName) && !project.IsTagPinned(oldName) {
				continue
			}
			targets = append(targets, globalTagRenameTarget{
				project:   project,
				taskCount: countProjectTaskTagUsage(project, oldName),
			})
		}

		if err := m.recordGlobalTagRenameUndo(targets, oldName, newName); err != nil {
			return ErrorMsg{Err: err}
		}

		service := NewProjectService()
		projectCount := 0
		taskCount := 0

		// Rename in each project that uses this tag
		for _, target := range targets {
			project := target.project
			modified, err := project.RenameTag(oldName, newName)
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to rename tag in %s: %w", project.Name, err)}
			}

			if modified {
				if err := service.SaveWithBackupSyncOptions(project, ProjectSaveOptions{}); err != nil {
					return ErrorMsg{Err: fmt.Errorf("failed to save %s: %w", project.Name, err)}
				}
				projectCount++
				taskCount += target.taskCount
			}
		}

		return GlobalTagRenamedMsg{
			OldName:      oldName,
			NewName:      newName,
			ProjectCount: projectCount,
			TaskCount:    taskCount,
		}
	}
}

func (m *GlobalTagManagerModel) recordGlobalTagRenameUndo(targets []globalTagRenameTarget, oldName, newName string) error {
	if m.config == nil || m.config.UndoHistoryDepth == nil || *m.config.UndoHistoryDepth <= 0 || len(targets) == 0 {
		return nil
	}

	projects := make([]*models.Project, 0, len(targets))
	for _, target := range targets {
		projects = append(projects, target.project)
	}

	groupID, groupProjectIDs := undoGroupMetadataForProjects(projects)
	context := fmt.Sprintf("%s -> %s", oldName, newName)
	service := NewProjectService()
	for _, target := range targets {
		if err := recordTUIUndoSnapshotWithGroup(service, target.project, "Rename Global Tag", context, *m.config.UndoHistoryDepth, nil, groupID, groupProjectIDs); err != nil {
			return err
		}
	}
	return nil
}

func countProjectTaskTagUsage(project *models.Project, tag string) int {
	count := 0
	for _, task := range project.Tasks {
		if task.HasTag(tag) {
			count++
		}
	}
	return count
}
