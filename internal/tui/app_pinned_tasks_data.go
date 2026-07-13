package tui

import (
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type pinnedTaskInfo struct {
	Title       string
	ShortID     int
	ProjectName string
	PinnedAt    time.Time
	Task        *models.Task
	Project     *models.Project
}

func (m *AppModel) collectPinnedTasks() []pinnedTaskInfo {
	projects := m.config.GetActiveProjects()
	var pinned []pinnedTaskInfo

	for _, project := range projects {
		if project.Tasks == nil {
			if err := storage.LoadProject(project); err != nil {
				continue
			}
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			if task.Pinned && !task.IsComplete() {
				info := pinnedTaskInfo{
					Title:       task.Title,
					ShortID:     task.ShortID,
					ProjectName: project.Name,
					Task:        task,
					Project:     project,
				}
				if task.PinnedAt != nil {
					info.PinnedAt = *task.PinnedAt
				}
				pinned = append(pinned, info)
			}
		}
	}

	sort.Slice(pinned, func(i, j int) bool {
		return pinned[i].PinnedAt.Before(pinned[j].PinnedAt)
	})

	return pinned
}
