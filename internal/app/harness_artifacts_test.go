package app_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// artifactPort returns in-memory workspace-note and project-blocker ports over
// the repo's committed artifact bytes.
func (r *fakeRepo) artifactPort() *memArtifactPort { return &memArtifactPort{repo: r} }

type memArtifactPort struct{ repo *fakeRepo }

func (m *memArtifactPort) noteDir(location app.NoteLocation, project *models.Project) (string, error) {
	if location.Scope == app.NoteScopeGlobal {
		return filepath.Join("mem-workspace", ".docket", "global-notes"), nil
	}
	if project == nil {
		return "", fmt.Errorf("project is required")
	}
	return storage.NotesDir(project), nil
}

func (m *memArtifactPort) Notes(location app.NoteLocation, project *models.Project) ([]*models.Note, error) {
	dir, err := m.noteDir(location, project)
	if err != nil {
		return nil, err
	}
	prefix := filepath.Clean(dir) + string(filepath.Separator)
	var notes []*models.Note
	for path, data := range m.repo.files {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		note, err := storage.ParseNoteContent(filepath.Base(path), string(data))
		if err != nil {
			return nil, err
		}
		note.RawRevision = storage.RawArtifactRevision(data)
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Filename < notes[j].Filename })
	return notes, nil
}

func (m *memArtifactPort) NotesForMutation(location app.NoteLocation, project *models.Project) ([]*models.Note, error) {
	return m.Notes(location, project)
}

func (m *memArtifactPort) EncodeNoteSave(location app.NoteLocation, project *models.Project, note *models.Note) (storage.ArtifactChange, error) {
	dir, err := m.noteDir(location, project)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	path, err := storage.NoteFilePathIn(dir, note.Filename)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	content, err := storage.RenderNoteContent(note)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	return storage.ArtifactChange{Path: path, Data: []byte(content)}, nil
}

func (m *memArtifactPort) EncodeNoteDelete(location app.NoteLocation, project *models.Project, filename string) (storage.ArtifactChange, error) {
	dir, err := m.noteDir(location, project)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	path, err := storage.NoteFilePathIn(dir, filename)
	if err != nil {
		return storage.ArtifactChange{}, err
	}
	return storage.ArtifactChange{Path: path, Delete: true}, nil
}

func (m *memArtifactPort) ProjectLocationPrecondition([]*models.Project) (*storage.ArtifactPrecondition, error) {
	return nil, nil
}

// lastCommit returns the single changeset from the most recent Commit call,
// failing the expectation implicitly if the shape is not exactly one call of one
// changeset.
func (r *fakeRepo) lastCommit() (storage.ChangeSet, bool) {
	if len(r.commits) == 0 {
		return storage.ChangeSet{}, false
	}
	last := r.commits[len(r.commits)-1]
	if len(last) != 1 {
		return storage.ChangeSet{}, false
	}
	return last[0], true
}

// fixedClock is a deterministic Clock.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// advancingClock returns a later instant on every read, so a mutation expected to
// stamp one shared timestamp fails the assertion if it reads the clock per item.
type advancingClock struct {
	t    time.Time
	step time.Duration
}

func (c *advancingClock) Now() time.Time {
	c.t = c.t.Add(c.step)
	return c.t
}

// seqIDs generates deterministic sequential task IDs.
type seqIDs struct{ n int }

func (s *seqIDs) NewID() string {
	s.n++
	return fmt.Sprintf("gen-task-%d", s.n)
}

// loc returns the project-qualified locator for taskID by finding the seeded
// project that owns it, so a test names a mutation target the way the Outliner
// does rather than relying on a workspace-unique task ID. An unknown ID panics:
// it is a broken fixture, not a runtime condition under test.
func (r *fakeRepo) loc(taskID string) app.TaskLocator {
	for _, p := range r.projects {
		for _, t := range p.Tasks {
			if t.ID == taskID {
				return app.TaskLocator{ProjectID: p.ID, TaskID: taskID}
			}
		}
	}
	panic(fmt.Sprintf("harness: no seeded project owns task %q", taskID))
}

// repoLocators maps a slice of task IDs onto project-qualified locators through
// the seeded fixture, so a table-driven bulk test keeps naming targets by ID.
func repoLocators(r *fakeRepo, taskIDs []string) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(taskIDs))
	for _, id := range taskIDs {
		locators = append(locators, r.loc(id))
	}
	return locators
}

// newTestService wires a Service over the fake repo with a fixed clock and
// deterministic ID generator.
func newTestService(repo *fakeRepo) *app.Service {
	artifacts := repo.artifactPort()
	return app.NewServiceWithArtifacts(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{}, artifacts)
}

// newTestServiceWithClock wires a Service over the fake repo with an explicit
// clock and the deterministic ID generator.
func newTestServiceWithClock(repo *fakeRepo, clock app.Clock) *app.Service {
	return app.NewServiceWithArtifacts(repo, clock, &seqIDs{}, repo.artifactPort())
}

// newProject builds an in-memory project with the given tasks appended and the
// task tree built, ready to seed the fake repo.
func newProject(id, name string, tasks ...*models.Task) *models.Project {
	p, err := models.NewProject(id, name, "")
	if err != nil {
		panic(err)
	}
	next := 1
	for _, t := range tasks {
		if t.ShortID >= next {
			next = t.ShortID + 1
		}
		p.Tasks = append(p.Tasks, t)
	}
	p.NextShortID = next
	p.BuildTaskTree()
	return p
}

// newTask builds a leaf task with a stable UUID and short ID.
func newTask(id string, shortID int, title string) *models.Task {
	t, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		panic(err)
	}
	return t
}
