package tui

import (
	"cmp"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

func compareProjectsByDisplayOrder(a, b *models.Project) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return 1
	case b == nil:
		return -1
	}

	if c := cmp.Compare(projectDisplayNameKey(a), projectDisplayNameKey(b)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Slug, b.Slug); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

func projectDisplayNameKey(project *models.Project) string {
	if project == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(project.Name))
}

func sortProjectsByDisplayOrder(projects []*models.Project) {
	sort.SliceStable(projects, func(i, j int) bool {
		return compareProjectsByDisplayOrder(projects[i], projects[j]) < 0
	})
}

func sortCrossProjectTasksByProjectDisplayOrder(tasks []CrossProjectTask) {
	sort.SliceStable(tasks, func(i, j int) bool {
		return compareProjectsByDisplayOrder(tasks[i].Project, tasks[j].Project) < 0
	})
}
