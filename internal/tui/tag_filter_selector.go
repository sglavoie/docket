package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// TagInfo holds a task tag name and its usage count. ContextCount is the share
// of that count contributed by the task context axis, and stays zero for every
// builder that reads tags alone.
type TagInfo struct {
	Name         string
	TaskCount    int
	ContextCount int
}

// BuildProjectTagList builds a sorted task-used tag list for a project.
func BuildProjectTagList(project *models.Project, excludeCompleted bool) []TagInfo {
	if project == nil {
		return []TagInfo{}
	}
	return BuildCatalogTagList([]*models.Project{project}, project.Tasks, excludeCompleted)
}

// BuildProjectsTagList builds a sorted task-used tag list across projects.
func BuildProjectsTagList(projects []*models.Project, excludeCompleted bool) []TagInfo {
	var tasks []*models.Task
	for _, project := range uniqueProjects(projects) {
		tasks = append(tasks, project.Tasks...)
	}
	return BuildCatalogTagList(projects, tasks, excludeCompleted)
}

// BuildTagFacetList builds the filter-only union of task tags and contexts.
// Counts are additive when one task carries the same value on both axes.
func BuildTagFacetList(projects []*models.Project, excludeCompleted bool) []TagInfo {
	var tasks []*models.Task
	for _, project := range uniqueProjects(projects) {
		tasks = append(tasks, project.Tasks...)
	}
	tagCounts := make(map[string]int)
	contextCounts := make(map[string]int)
	addTaskTagCounts(tagCounts, tasks, excludeCompleted)
	addTaskContextCounts(contextCounts, tasks, excludeCompleted)
	for name, count := range contextCounts {
		tagCounts[name] += count
	}
	facets := tagInfosFromCounts(tagCounts)
	for index := range facets {
		facets[index].ContextCount = contextCounts[facets[index].Name]
	}
	return facets
}

// BuildCatalogTagList is a compatibility wrapper over task-derived tag counts.
func BuildCatalogTagList(projects []*models.Project, tasks []*models.Task, excludeCompleted bool) []TagInfo {
	tagCounts := make(map[string]int)
	addTaskTagCounts(tagCounts, tasks, excludeCompleted)

	return tagInfosFromCounts(tagCounts)
}

func addTaskTagCounts(tagCounts map[string]int, tasks []*models.Task, excludeCompleted bool) {
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if excludeCompleted && task.IsComplete() {
			continue
		}
		for _, tag := range models.DedupeTags(task.Tags) {
			tagCounts[tag]++
		}
	}
}

func addTaskContextCounts(tagCounts map[string]int, tasks []*models.Task, excludeCompleted bool) {
	for _, task := range tasks {
		if task == nil || task.Context == "" {
			continue
		}
		if excludeCompleted && task.IsComplete() {
			continue
		}
		context, err := models.NormalizeTaskContext(task.Context)
		if err == nil {
			tagCounts[context]++
		}
	}
}

func tagInfosFromCounts(tagCounts map[string]int) []TagInfo {
	tags := make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		tags = append(tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].Name < tags[j].Name
	})

	return tags
}

func uniqueProjects(projects []*models.Project) []*models.Project {
	seenIDs := make(map[string]bool, len(projects))
	seenPtrs := make(map[*models.Project]bool, len(projects))
	result := make([]*models.Project, 0, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		if project.ID != "" {
			if seenIDs[project.ID] {
				continue
			}
			seenIDs[project.ID] = true
		} else {
			if seenPtrs[project] {
				continue
			}
			seenPtrs[project] = true
		}
		result = append(result, project)
	}
	return result
}
