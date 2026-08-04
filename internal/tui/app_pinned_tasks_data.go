package tui

import (
	"cmp"
	"sort"
	"strings"
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
}

func (m *AppModel) collectPinnedTasks() []pinnedTaskInfo {
	projects := m.taskWorkspaceProjects()
	var pinned []pinnedTaskInfo
	now := time.Now()

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
			if task.Pinned && !task.IsTerminalState() && !task.IsSnoozedAt(now) {
				info := pinnedTaskInfo{
					Title:       task.Title,
					ShortID:     task.ShortID,
					ProjectName: project.Name,
					Task:        task,
				}
				if task.PinnedAt != nil {
					info.PinnedAt = *task.PinnedAt
				}
				pinned = append(pinned, info)
			}
		}
	}

	sort.SliceStable(pinned, func(i, j int) bool {
		switch {
		case !pinned[i].PinnedAt.IsZero() && pinned[j].PinnedAt.IsZero():
			return true
		case pinned[i].PinnedAt.IsZero() && !pinned[j].PinnedAt.IsZero():
			return false
		case !pinned[i].PinnedAt.Equal(pinned[j].PinnedAt):
			return pinned[i].PinnedAt.After(pinned[j].PinnedAt)
		}
		if c := cmp.Compare(strings.ToLower(pinned[i].ProjectName), strings.ToLower(pinned[j].ProjectName)); c != 0 {
			return c < 0
		}
		if c := cmp.Compare(pinned[i].ShortID, pinned[j].ShortID); c != 0 {
			return c < 0
		}
		return pinned[i].Task.ID < pinned[j].Task.ID
	})

	return pinned
}
