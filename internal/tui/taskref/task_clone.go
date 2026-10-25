package taskref

import "github.com/sglavoie/docket/internal/models"

// CloneTaskDetachingDependencies shallow-copies a task and detaches its
// dependency list. A bare `*task` keeps the reference slice's backing array,
// so a projection or snapshot clone would alias repository-owned or view-owned
// state and a later element write or in-capacity append would reach the source
// task. Every clone that outlives the read it was made for goes through here.
func CloneTaskDetachingDependencies(task *models.Task) models.Task {
	clone := *task
	clone.Dependencies = models.CloneTaskReferences(task.Dependencies)
	return clone
}
