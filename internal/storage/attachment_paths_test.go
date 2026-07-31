package storage

import (
	"github.com/sglavoie/tt/internal/models"
)

func GetTaskAttachmentsDir(project *models.Project, task *models.Task) string {
	path, err := ResolveTaskAttachmentsDir(project, task)
	if err != nil {
		panic(err)
	}
	return path
}

func GetAttachmentPath(project *models.Project, task *models.Task, filename string) string {
	path, err := ResolveAttachmentPath(project, task, filename)
	if err != nil {
		panic(err)
	}
	return path
}
