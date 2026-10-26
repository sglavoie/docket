package storage

import "github.com/sglavoie/docket/internal/models"

// WriteProjectMarkdown renders the strict live project document from the whole
// project. It takes the project rather than a name and a task list because
// tasks.md now holds project-level state too: a renderer that named only those
// two would silently drop the project's comments at every full-file rewrite,
// and a rename is exactly such a rewrite.
func WriteProjectMarkdown(project *models.Project) (string, error) {
	document, err := currentTaskProjectDocumentFromModel(project)
	if err != nil {
		return "", err
	}
	return writeValidatedCurrentTaskProjectDocument(document)
}

type taskHierarchyIndex struct {
	roots    []*models.Task
	children map[string][]*models.Task
}

func indexTaskHierarchy(tasks []*models.Task) taskHierarchyIndex {
	index := taskHierarchyIndex{children: make(map[string][]*models.Task)}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if task.ParentID == nil {
			index.roots = append(index.roots, task)
			continue
		}
		index.children[*task.ParentID] = append(index.children[*task.ParentID], task)
	}
	return index
}
