package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
		projectCount := 0
		taskCount := 0

		// Rename in each project that uses this tag
		for _, project := range projects {
			// Reload project to ensure fresh data
			if err := storage.LoadProject(project); err != nil {
				continue
			}

			renamedTaskCount := countProjectTaskTagUsage(project, oldName)
			modified, err := project.RenameTag(oldName, newName)
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to rename tag in %s: %w", project.Name, err)}
			}

			if modified {
				if err := storage.BackupProject(project); err != nil {
					return ErrorMsg{Err: fmt.Errorf("failed to backup %s: %w", project.Name, err)}
				}
				if err := storage.SaveProjectLocked(project); err != nil {
					return ErrorMsg{Err: fmt.Errorf("failed to save %s: %w", project.Name, err)}
				}
				projectCount++
				taskCount += renamedTaskCount
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

func countProjectTaskTagUsage(project *models.Project, tag string) int {
	count := 0
	for _, task := range project.Tasks {
		if task.HasTag(tag) {
			count++
		}
	}
	return count
}
