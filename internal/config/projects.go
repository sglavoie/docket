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

func deleteProjectKey[T any](values map[string]T, id, slug string) {
	delete(values, id)
	if slug != "" {
		delete(values, slug)
	}
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

func filterProjectIDs(ids []string, keep func(string) bool) ([]string, bool) {
	return filterProjectRefSlice(ids, func(id string) string { return id }, keep)
}

func filterProjectIDMap(projectIDs map[string][]string, keep func(string) bool) bool {
	changed := false
	for key, ids := range projectIDs {
		var filtered bool
		projectIDs[key], filtered = filterProjectIDs(ids, keep)
		changed = filtered || changed
	}
	return changed
}
