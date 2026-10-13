package app

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// cloneSnapshot returns a detached copy suitable for rollback after an
// in-memory mutation or a failed commit.
func cloneSnapshot(source Snapshot) Snapshot {
	projects := source.Projects()
	clones := make([]*models.Project, 0, len(projects))
	revisions := make(map[string]string, len(projects))
	documents := make(map[string]*storage.ProjectDocument, len(projects))
	for _, project := range projects {
		if project == nil {
			continue
		}
		clone := *project
		if project.ArchivedAt != nil {
			value := *project.ArchivedAt
			clone.ArchivedAt = &value
		}
		if project.PinnedAt != nil {
			value := *project.PinnedAt
			clone.PinnedAt = &value
		}
		clone.Comments = models.CloneProjectComments(project.Comments)
		clone.Tasks = make([]*models.Task, 0, len(project.Tasks))
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			clone.Tasks = append(clone.Tasks, models.CloneTaskDeep(task))
		}
		clone.BuildTaskTree()
		clones = append(clones, &clone)
		revisions[clone.ID] = source.Revision(clone.ID)
		if document := source.Document(clone.ID); document != nil {
			documents[clone.ID] = cloneProjectDocument(document)
		}
	}
	return NewPartialSnapshot(clones, revisions, documents, source.UnavailableProjectIDs())
}

// cloneProjectDocument detaches a repository document for rollback. The copy
// must share no task with the source, because useRollbackDocuments hands its
// tasks to a live project that later mutations edit in place. Copying the
// fields directly keeps the document's own revision and costs a fraction of
// rendering it and parsing the result, which this clone once did for every
// project on every commit.
func cloneProjectDocument(document *storage.ProjectDocument) *storage.ProjectDocument {
	copied := *document
	copied.Tasks = make([]*models.Task, 0, len(document.Tasks))
	for _, task := range document.Tasks {
		if task != nil {
			copied.Tasks = append(copied.Tasks, models.CloneTaskDeep(task))
		}
	}
	copied.Comments = models.CloneProjectComments(document.Comments)
	return &copied
}

// useRollbackDocuments replaces each backup project's structured task state
// with the immutable repository document. Project-scoped callers may have
// already edited shared in-memory objects before entering an application
// transaction.
func useRollbackDocuments(before Snapshot) {
	if before == nil {
		return
	}
	for _, project := range before.Projects() {
		if project == nil {
			continue
		}
		document := before.Document(project.ID)
		if document == nil {
			continue
		}
		project.Name = document.ProjectName
		project.Tasks = document.Tasks
		project.Comments = models.CloneProjectComments(document.Comments)
		project.BuildTaskTree()
	}
}

// restoreSnapshotProjects puts repository-owned in-memory objects back to
// their pre-mutation values when validation, artifact preparation, or commit
// fails.
func restoreSnapshotProjects(target, before Snapshot) {
	if target == nil || before == nil {
		return
	}
	for _, project := range target.Projects() {
		if project == nil {
			continue
		}
		saved := before.ProjectByID(project.ID)
		if saved == nil {
			continue
		}
		originalTasks := make(map[string]*models.Task, len(project.Tasks))
		for _, task := range project.Tasks {
			if task != nil {
				originalTasks[task.ID] = task
			}
		}
		restoredTasks := make([]*models.Task, 0, len(saved.Tasks))
		for _, savedTask := range saved.Tasks {
			if savedTask == nil {
				continue
			}
			task := originalTasks[savedTask.ID]
			if task == nil {
				copyTask := *savedTask
				task = &copyTask
			} else {
				*task = *savedTask
			}
			restoredTasks = append(restoredTasks, task)
		}
		restoredProject := *saved
		restoredProject.Tasks = restoredTasks
		// Detach the collection the backup snapshot still owns; restoring an alias
		// would let the next mutation edit the value a second rollback reads.
		restoredProject.Comments = models.CloneProjectComments(saved.Comments)
		*project = restoredProject
		project.BuildTaskTree()
	}
}
