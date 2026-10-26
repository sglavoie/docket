package app

import (
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// rollbackSnapshot is the backup a mutation restores from on failure: each
// project's structured task state is the immutable repository document rather
// than the in-memory project, since project-scoped callers may already have
// edited shared in-memory objects before entering an application transaction.
//
// A mutation that succeeds reads a project or two of it at most, so the copy is
// taken lazily. Each project's own fields are captured now, because the mutation
// may edit the live project before it fails; its tasks are copied from the
// document only when the project is first asked for. Deferring that copy changes
// nothing about what it holds, since application mutations treat a snapshot's
// documents as immutable merge bases. A project with no document is copied whole
// now, as cloneSnapshot copies it, because its live tasks are all there is.
func rollbackSnapshot(source Snapshot) Snapshot {
	view := &rollbackView{
		source:    source,
		byID:      make(map[string]*models.Project),
		pending:   make(map[string]*storage.ProjectDocument),
		revisions: make(map[string]string),
		documents: make(map[string]*storage.ProjectDocument),
	}
	for _, project := range source.Projects() {
		if project == nil {
			continue
		}
		shell := detachedProjectFields(project)
		if document := source.Document(project.ID); document != nil {
			view.pending[project.ID] = document
		} else {
			shell.Comments = models.CloneProjectComments(project.Comments)
			shell.Tasks = cloneTasks(project.Tasks)
			shell.BuildTaskTree()
		}
		view.order = append(view.order, shell)
		view.byID[project.ID] = shell
		view.revisions[project.ID] = source.Revision(project.ID)
	}
	return view
}

// rollbackView is rollbackSnapshot's lazily detached Snapshot. It is read by the
// one mutation that took it, on that mutation's goroutine.
type rollbackView struct {
	source    Snapshot
	order     []*models.Project
	byID      map[string]*models.Project
	pending   map[string]*storage.ProjectDocument
	revisions map[string]string
	documents map[string]*storage.ProjectDocument
	indexed   Snapshot
}

// detach copies one project's tasks out of its document, once.
func (v *rollbackView) detach(projectID string) {
	document, ok := v.pending[projectID]
	if !ok {
		return
	}
	delete(v.pending, projectID)
	detached := cloneProjectDocument(document)
	v.documents[projectID] = detached
	shell := v.byID[projectID]
	shell.Name = detached.ProjectName
	shell.Tasks = detached.Tasks
	shell.Comments = models.CloneProjectComments(detached.Comments)
	shell.BuildTaskTree()
}

func (v *rollbackView) detachAll() {
	for _, project := range v.order {
		v.detach(project.ID)
	}
}

func (v *rollbackView) ProjectByID(projectID string) *models.Project {
	v.detach(projectID)
	return v.byID[projectID]
}

func (v *rollbackView) Document(projectID string) *storage.ProjectDocument {
	v.detach(projectID)
	return v.documents[projectID]
}

func (v *rollbackView) Projects() []*models.Project {
	v.detachAll()
	return v.order
}

func (v *rollbackView) Revision(projectID string) string { return v.revisions[projectID] }

func (v *rollbackView) UnavailableProjectIDs() []string { return v.source.UnavailableProjectIDs() }

func (v *rollbackView) LookupTaskID(taskID string) (TaskRef, TaskLookupStatus) {
	return v.index().LookupTaskID(taskID)
}

func (v *rollbackView) LookupLocator(locator TaskLocator) (TaskRef, bool) {
	return v.index().LookupLocator(locator)
}

// index is the whole view, detached and indexed, for the lookups that range over
// every task.
func (v *rollbackView) index() Snapshot {
	if v.indexed == nil {
		v.detachAll()
		v.indexed = NewPartialSnapshot(v.order, v.revisions, v.documents, v.source.UnavailableProjectIDs())
	}
	return v.indexed
}

// detachedProjectFields copies a project's own fields, its timestamps detached,
// leaving the tasks and comments to the caller.
func detachedProjectFields(project *models.Project) *models.Project {
	clone := *project
	if project.ArchivedAt != nil {
		value := *project.ArchivedAt
		clone.ArchivedAt = &value
	}
	if project.PinnedAt != nil {
		value := *project.PinnedAt
		clone.PinnedAt = &value
	}
	clone.Tasks, clone.Comments = nil, nil
	return &clone
}

func cloneTasks(tasks []*models.Task) []*models.Task {
	cloned := make([]*models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task != nil {
			cloned = append(cloned, models.CloneTaskDeep(task))
		}
	}
	return cloned
}

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
		clone := detachedProjectFields(project)
		clone.Comments = models.CloneProjectComments(project.Comments)
		clone.Tasks = cloneTasks(project.Tasks)
		clone.BuildTaskTree()
		clones = append(clones, clone)
		revisions[clone.ID] = source.Revision(clone.ID)
		if document := source.Document(clone.ID); document != nil {
			documents[clone.ID] = cloneProjectDocument(document)
		}
	}
	return NewPartialSnapshot(clones, revisions, documents, source.UnavailableProjectIDs())
}

// cloneProjectDocument detaches a repository document for rollback. The copy
// must share no task with the source, because rollbackSnapshot hands its
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
