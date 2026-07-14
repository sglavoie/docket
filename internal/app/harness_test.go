package app_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// fakeRepo is an in-memory Repository that mirrors the transactional storage
// boundary closely enough to exercise the application use cases: it hands out
// the live project pointers through a snapshot, tracks each project's revision
// independently, and enforces the same optimistic-concurrency check the real
// commit service does (a stale ExpectedRevision aborts with a
// RevisionConflictError that unwraps to storage.ErrStaleRevision).
type fakeRepo struct {
	projects  []*models.Project
	revisions map[string]string
	documents map[string]*storage.ProjectDocument
	commits   [][]storage.ChangeSet
	snapErr   error
	commitErr error
	// preCommit runs at the start of Commit so a test can simulate a concurrent
	// writer advancing a project's on-disk revision between snapshot and commit.
	preCommit func()
	// files is an in-memory backing store for committed artifact writes (notes and
	// blockers), keyed by cleaned target path, so an in-memory ProjectArtifactReader
	// can read back exactly what a ChangeSet committed.
	files map[string][]byte
}

func newFakeRepo(projects ...*models.Project) *fakeRepo {
	r := &fakeRepo{revisions: make(map[string]string), documents: make(map[string]*storage.ProjectDocument), files: make(map[string][]byte)}
	for _, p := range projects {
		r.projects = append(r.projects, p)
		doc := documentOf(p, nil)
		r.documents[p.ID] = doc
		r.revisions[p.ID] = doc.Revision
	}
	return r
}

func documentOf(p *models.Project, base *storage.ProjectDocument) *storage.ProjectDocument {
	doc, err := storage.CanonicalProjectDocument(p, base)
	if err != nil {
		panic(err)
	}
	return doc
}

func (r *fakeRepo) Snapshot() (app.Snapshot, error) {
	if r.snapErr != nil {
		return nil, r.snapErr
	}
	revs := make(map[string]string, len(r.revisions))
	for k, v := range r.revisions {
		revs[k] = v
	}
	docs := make(map[string]*storage.ProjectDocument, len(r.documents))
	for k, v := range r.documents {
		docs[k] = v
	}
	return app.NewSnapshot(r.projects, revs, docs), nil
}

func (r *fakeRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.preCommit != nil {
		r.preCommit()
	}
	if r.commitErr != nil {
		return nil, r.commitErr
	}
	results := make([]*storage.CommitResult, len(changesets))
	for i := range changesets {
		cs := changesets[i]
		stored := r.revisions[cs.Project.ID]
		if cs.ExpectedRevision != "" && cs.ExpectedRevision != stored {
			return nil, &storage.RevisionConflictError{
				ProjectID:   cs.Project.ID,
				ProjectName: cs.Project.Name,
				Expected:    cs.ExpectedRevision,
				Actual:      stored,
			}
		}
		newDoc := documentOf(cs.Project, cs.Base)
		r.documents[cs.Project.ID] = newDoc
		r.revisions[cs.Project.ID] = newDoc.Revision
		results[i] = &storage.CommitResult{Revision: newDoc.Revision, TransactionID: "txn-test"}
		// Apply artifact writes/deletes to the in-memory file store so an in-memory
		// reader observes exactly what the real commit service would persist.
		for _, art := range cs.Artifacts {
			path := filepath.Clean(art.Path)
			if art.Delete {
				delete(r.files, path)
				continue
			}
			r.files[path] = append([]byte(nil), art.Data...)
		}
	}
	r.commits = append(r.commits, changesets)
	return results, nil
}

// artifactReader returns an in-memory ProjectArtifactReader over the repo's
// committed artifact bytes, parsing them with the same storage codecs the
// filesystem reader uses.
func (r *fakeRepo) artifactReader() app.ProjectArtifactReader { return &memArtifactReader{repo: r} }

type memArtifactReader struct{ repo *fakeRepo }

func (m *memArtifactReader) Notes(project *models.Project) ([]*models.Note, error) {
	prefix := filepath.Clean(storage.NotesDir(project)) + string(filepath.Separator)
	var notes []*models.Note
	for path, data := range m.repo.files {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		note, err := storage.ParseNoteContent(filepath.Base(path), string(data))
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].Filename < notes[j].Filename })
	return notes, nil
}

func (m *memArtifactReader) Blockers(project *models.Project) ([]*models.Blocker, error) {
	data, ok := m.repo.files[filepath.Clean(storage.BlockersFilePath(project))]
	if !ok {
		return []*models.Blocker{}, nil
	}
	return storage.ParseBlockersContent(string(data))
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

// seqIDs generates deterministic sequential task IDs.
type seqIDs struct{ n int }

func (s *seqIDs) NewID() string {
	s.n++
	return fmt.Sprintf("gen-task-%d", s.n)
}

// newTestService wires a Service over the fake repo with a fixed clock and
// deterministic ID generator.
func newTestService(repo *fakeRepo) *app.Service {
	return app.NewService(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{})
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
	t, err := models.NewTask(id, shortID, models.TaskTypeTask, title)
	if err != nil {
		panic(err)
	}
	return t
}
