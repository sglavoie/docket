package grammar

import (
	"maps"
	"slices"

	"github.com/sglavoie/tt/internal/models"
)

// CollectTagNamesForProject returns the sorted unique tag names a project's
// tasks carry. It is the completion vocabulary behind `#tag`, so it reads task
// usage alone and counts nothing: the counted census the tag facet pickers read
// is the filter layer's, and it is built over the same task tags.
func CollectTagNamesForProject(project *models.Project) []string {
	if project == nil {
		return []string{}
	}
	tags := make(map[string]bool)
	for _, task := range project.Tasks {
		if task == nil {
			continue
		}
		for _, tag := range models.DedupeTags(task.Tags) {
			tags[tag] = true
		}
	}
	names := make([]string, 0, len(tags))
	for name := range tags {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// CollectContextNamesForProject returns the sorted unique task contexts used in
// a project. Context has no project catalog, so task usage is its full vocabulary.
func CollectContextNamesForProject(project *models.Project) []string {
	if project == nil {
		return nil
	}
	contexts := make(map[string]bool)
	for _, task := range project.Tasks {
		if task == nil || task.Context == "" {
			continue
		}
		normalized, err := models.NormalizeTaskContext(task.Context)
		if err == nil {
			contexts[normalized] = true
		}
	}
	result := maps.Keys(contexts)
	return slices.Sorted(result)
}
