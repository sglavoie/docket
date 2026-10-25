package storage

import "github.com/sglavoie/docket/internal/models"

func mustTask(id string, shortID int, title string) *models.Task {
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		panic(err)
	}
	return task
}

func newProject(name string, tasks ...*models.Task) *models.Project {
	p, err := models.NewProject("proj-id", name, "")
	if err != nil {
		panic(err)
	}
	p.Slug = "test-project"
	p.Tasks = tasks
	return p
}
