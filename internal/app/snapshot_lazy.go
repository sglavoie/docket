package app

import (
	"slices"
	"sync"

	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// LazyProject is one project of a snapshot that loads it only when asked. Its
// revision and task identities are known without loading it; Load fills the
// project with the version the repository read and returns that version's
// document.
type LazyProject struct {
	Project  *models.Project
	Revision string
	TaskIDs  []string
	Load     func(*models.Project) (*storage.ProjectDocument, error)
}

// NewLazySnapshot is a Snapshot whose projects load on first use. A repository
// reads every project to know its revision, but a mutation reaches one or two of
// them, so copying every task of a large workspace into projects nobody touches
// is what this saves. Each project loads the version the repository read, so
// the snapshot is as revision-consistent as one loaded whole; Projects, and the
// lookups that range over every task, load what they need.
func NewLazySnapshot(projects []LazyProject, unavailable []string) Snapshot {
	view := &lazySnapshot{byID: make(map[string]*lazyProject, len(projects))}
	for _, project := range projects {
		if project.Project == nil {
			continue
		}
		entry := &lazyProject{LazyProject: project}
		view.order = append(view.order, entry)
		view.byID[project.Project.ID] = entry
	}
	for _, id := range unavailable {
		if id != "" && !slices.Contains(view.unavailable, id) {
			view.unavailable = append(view.unavailable, id)
		}
	}
	slices.Sort(view.unavailable)
	return view
}

type lazySnapshot struct {
	mu          sync.Mutex
	order       []*lazyProject
	byID        map[string]*lazyProject
	unavailable []string
	// owners indexes every task identity to the projects holding it, built from
	// the task identities alone on the first unqualified lookup.
	owners map[string][]*lazyProject
}

type lazyProject struct {
	LazyProject
	loaded   bool
	failed   bool
	document *storage.ProjectDocument
	refs     map[string]TaskRef
}

// load fills one project, once. The repository verified when it read the file
// that a load succeeds, so a failure here is a defect; it is logged and the
// project is reported unavailable rather than half-loaded. The caller holds mu.
func (v *lazySnapshot) load(entry *lazyProject) {
	if entry.loaded {
		return
	}
	entry.loaded = true
	document, err := entry.Load(entry.Project)
	if err != nil {
		logging.Warn("skipping project %s: %v", entry.Project.Name, err)
		entry.failed = true
		if !slices.Contains(v.unavailable, entry.Project.ID) {
			v.unavailable = append(v.unavailable, entry.Project.ID)
			slices.Sort(v.unavailable)
		}
		return
	}
	entry.document = document
	entry.refs = make(map[string]TaskRef, len(entry.Project.Tasks))
	for _, task := range entry.Project.Tasks {
		if task != nil && task.ID != "" {
			entry.refs[task.ID] = TaskRef{Task: task, Project: entry.Project, Base: document}
		}
	}
}

func (v *lazySnapshot) loadedEntry(projectID string) *lazyProject {
	entry := v.byID[projectID]
	if entry == nil {
		return nil
	}
	v.load(entry)
	if entry.failed {
		return nil
	}
	return entry
}

func (v *lazySnapshot) ProjectByID(projectID string) *models.Project {
	v.mu.Lock()
	defer v.mu.Unlock()
	if entry := v.loadedEntry(projectID); entry != nil {
		return entry.Project
	}
	return nil
}

func (v *lazySnapshot) LookupTaskID(taskID string) (TaskRef, TaskLookupStatus) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.owners == nil {
		v.owners = make(map[string][]*lazyProject)
		for _, entry := range v.order {
			for _, id := range entry.TaskIDs {
				v.owners[id] = append(v.owners[id], entry)
			}
		}
	}
	owners := v.owners[taskID]
	switch len(owners) {
	case 0:
		return TaskRef{}, TaskLookupMissing
	case 1:
		if entry := v.loadedEntry(owners[0].Project.ID); entry != nil {
			if ref, ok := entry.refs[taskID]; ok {
				return ref, TaskLookupUnique
			}
		}
		return TaskRef{}, TaskLookupMissing
	default:
		return TaskRef{}, TaskLookupAmbiguous
	}
}

func (v *lazySnapshot) LookupLocator(locator TaskLocator) (TaskRef, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry := v.loadedEntry(locator.ProjectID)
	if entry == nil {
		return TaskRef{}, false
	}
	ref, ok := entry.refs[locator.TaskID]
	return ref, ok
}

func (v *lazySnapshot) Revision(projectID string) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if entry := v.byID[projectID]; entry != nil && !entry.failed {
		return entry.Revision
	}
	return ""
}

func (v *lazySnapshot) Document(projectID string) *storage.ProjectDocument {
	v.mu.Lock()
	defer v.mu.Unlock()
	if entry := v.loadedEntry(projectID); entry != nil {
		return entry.document
	}
	return nil
}

func (v *lazySnapshot) Projects() []*models.Project {
	v.mu.Lock()
	defer v.mu.Unlock()
	projects := make([]*models.Project, 0, len(v.order))
	for _, entry := range v.order {
		v.load(entry)
		if !entry.failed {
			projects = append(projects, entry.Project)
		}
	}
	return projects
}

func (v *lazySnapshot) UnavailableProjectIDs() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Clone(v.unavailable)
}

// projectShells names every project in order without loading any; a project
// not yet loaded holds its catalog fields and no tasks.
func (v *lazySnapshot) projectShells() []*models.Project {
	v.mu.Lock()
	defer v.mu.Unlock()
	shells := make([]*models.Project, 0, len(v.order))
	for _, entry := range v.order {
		if !entry.failed {
			shells = append(shells, entry.Project)
		}
	}
	return shells
}

// isLoaded reports whether a project has been loaded, which is whether anything
// could have edited it since the repository read it.
func (v *lazySnapshot) isLoaded(projectID string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry := v.byID[projectID]
	return entry != nil && entry.loaded && !entry.failed
}

// loadedProjects is every project loaded so far: the only ones a mutation can
// have edited, and so the only ones a failed one has to put back.
func (v *lazySnapshot) loadedProjects() []*models.Project {
	v.mu.Lock()
	defer v.mu.Unlock()
	var projects []*models.Project
	for _, entry := range v.order {
		if entry.loaded && !entry.failed {
			projects = append(projects, entry.Project)
		}
	}
	return projects
}

// lazyProjectSource is a snapshot that can name its projects without loading
// them, which the rollback copies use so taking one loads nothing.
type lazyProjectSource interface {
	projectShells() []*models.Project
	isLoaded(projectID string) bool
	loadedProjects() []*models.Project
}
