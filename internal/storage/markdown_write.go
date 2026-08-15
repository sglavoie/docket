package storage

import "github.com/sglavoie/tt/internal/models"

// WriteTasksMarkdown renders the strict live project document.
func WriteTasksMarkdown(projectName string, tasks []*models.Task) (string, error) {
	v11, err := v11ProjectDocumentFromModel(&models.Project{Name: projectName, Tasks: tasks})
	if err != nil {
		return "", err
	}
	return WriteV11ProjectDocument(v11)
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
