package config

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

func sortProjectsByName(projects []*models.Project) {
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

func sortProjectsWithPinnedFirst(projects []*models.Project) {
	sort.SliceStable(projects, func(i, j int) bool {
		if projects[i].Pinned != projects[j].Pinned {
			return projects[i].Pinned
		}
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})
}

func filterProjectRefSlice[T any](values []T, projectID func(T) string, keep func(string) bool) ([]T, bool) {
	changed := false
	filtered := values[:0]
	for _, value := range values {
		if !keep(projectID(value)) {
			changed = true
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered, changed
}
