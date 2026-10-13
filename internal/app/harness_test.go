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
	snapshots int
	snapErr   error
	commitErr error
	// preCommit runs at the start of Commit so a test can simulate a concurrent
	// writer advancing a project's on-disk revision between snapshot and commit.
	preCommit func()
	// files is an in-memory backing store for committed artifact writes (notes and
	// blockers), keyed by cleaned target path, so an in-memory ProjectArtifactReader
	// can read back exactly what a ChangeSet committed.
	files map[string][]byte
	// artifactCommits records artifact-only note transactions separately from
	// project ChangeSets; these commits must never rewrite tasks.md.
	artifactCommits         []storage.ArtifactCommitRequest
	validateChangeArtifacts bool
	// unloaded holds cataloged projects Snapshot deliberately omits — archived
	// ones in production. They are invisible to every mutation read model and
	// visible only through ReferenceDocuments, which is exactly the split the real
	// TUI repository draws between AvailableProjects and AllProjects.
	unloaded []*models.Project
	// referenceErr fails the reference-document enumeration, standing in for a
	// catalog project whose tasks.md cannot be read.
	referenceErr error
	// referenceDocumentCalls counts enumerations, so a mutation with nothing to
	// maintain can be shown never to open the wider catalog at all.
	referenceDocumentCalls int
	// censusErr fails the strict workspace census, standing in for a project the
	// production repository could read for no snapshot but must never skip here.
	censusErr error
	// censusCalls counts censuses, so an operation that must freeze exactly one
	// read set can be shown not to take a second one.
	censusCalls int
}

// fakeCatalogPath is the in-memory stand-in for workspace.json. The census
// reports it and its raw revision the way the production repository reports the
// real catalog, so a test can advance it in preCommit and watch the guard fire.
var fakeCatalogPath = filepath.Clean("mem-workspace/.tt/workspace.json")

// WorkspaceCensus implements app.WorkspaceCensusSource over the seeded snapshot
// population, which is the fake's analog of "every unarchived project": the
// separately tracked unloaded set stands in for archived projects and is
// omitted here exactly as it is from Snapshot. Each project is deep-copied
// through the durable document format, mirroring the production census's fresh
// load — a refused commit must leave the repository's own state untouched.
func (r *fakeRepo) WorkspaceCensus() (app.WorkspaceCensus, error) {
	r.censusCalls++
	if r.censusErr != nil {
		return app.WorkspaceCensus{}, r.censusErr
	}
	census := app.WorkspaceCensus{CatalogPath: fakeCatalogPath}
	if data, ok := r.files[fakeCatalogPath]; ok {
		census.CatalogRevision = storage.RawArtifactRevision(data)
	}
	for _, project := range r.projects {
		if project == nil {
			continue
		}
		census.Projects = append(census.Projects, app.CensusProject{
			Project:  cloneProjectForTest(project),
			Revision: r.revisions[project.ID],
			Document: r.documents[project.ID],
		})
	}
	return census, nil
}

// cloneProjectForTest deep-copies a project by round-tripping it through the
// durable format, so a mutation applied to the copy cannot reach the original
// through a shared task pointer.
func cloneProjectForTest(project *models.Project) *models.Project {
	content, err := documentOf(project, nil).WriteCurrent()
	if err != nil {
		panic(err)
	}
	parsed, err := storage.ParseProjectDocument(content)
	if err != nil {
		panic(err)
	}
	clone := *project
	clone.Tasks = parsed.Tasks
	clone.BuildTaskTree()
	return &clone
}

// withUnloaded seeds cataloged projects outside the snapshot population. They
// still get a document and a revision, so a commit that reaches one is held to
// the same optimistic-concurrency contract as any other project.
func (r *fakeRepo) withUnloaded(projects ...*models.Project) *fakeRepo {
	for _, p := range projects {
		doc := documentOf(p, nil)
		r.documents[p.ID] = doc
		r.revisions[p.ID] = doc.Revision
		r.unloaded = append(r.unloaded, p)
	}
	return r
}

// ReferenceDocuments implements app.ReferenceDocumentSource over the whole
// seeded catalog, snapshot population included, mirroring the production adapter
// that enumerates AllProjects rather than the mutation read model.
func (r *fakeRepo) ReferenceDocuments() ([]app.ReferenceDocument, error) {
	r.referenceDocumentCalls++
	if r.referenceErr != nil {
		return nil, r.referenceErr
	}
	all := append(append([]*models.Project(nil), r.projects...), r.unloaded...)
	documents := make([]app.ReferenceDocument, 0, len(all))
	for _, p := range all {
		if p == nil {
			continue
		}
		doc := r.documents[p.ID]
		if doc == nil {
			doc = documentOf(p, nil)
		}
		content, err := doc.WriteCurrent()
		if err != nil {
			return nil, err
		}
		documents = append(documents, app.ReferenceDocument{
			Project:     p,
			Base:        doc,
			Revision:    r.revisions[p.ID],
			RawRevision: storage.RawArtifactRevision([]byte(content)),
			TasksPath:   storage.TasksFilePath(p.Path),
		})
	}
	return documents, nil
}

func (r *fakeRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	if r.preCommit != nil {
		r.preCommit()
	}
	if r.commitErr != nil {
		return "", r.commitErr
	}
	// Validate the complete optimistic-concurrency set before mutating the map so
	// a stale source or occupied destination leaves the batch all-or-none.
	for _, artifact := range request.Artifacts {
		if artifact.ExpectedRevision == nil {
			continue
		}
		if err := r.artifactConflict(artifact.Path, *artifact.ExpectedRevision); err != nil {
			return "", err
		}
	}
	for _, artifact := range request.Artifacts {
		path := filepath.Clean(artifact.Path)
		if artifact.Delete {
			delete(r.files, path)
			continue
		}
		r.files[path] = append([]byte(nil), artifact.Data...)
	}
	r.artifactCommits = append(r.artifactCommits, request)
	return fmt.Sprintf("txn-note-%d", len(r.artifactCommits)), nil
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
	r.snapshots++
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
	return app.NewPartialSnapshot(r.projects, revs, docs, nil), nil
}

// artifactConflict applies one raw-revision expectation to the in-memory file
// store and returns the conflict a real commit boundary would raise: an empty
// expectation requires absence, a non-empty one requires an exact match.
func (r *fakeRepo) artifactConflict(path, expected string) error {
	clean := filepath.Clean(path)
	data, exists := r.files[clean]
	actual := ""
	if exists {
		actual = storage.RawArtifactRevision(data)
	}
	if (!exists && expected != "") || (exists && (expected == "" || expected != actual)) {
		return &storage.ArtifactConflictError{Path: clean, Expected: expected, Actual: actual}
	}
	return nil
}

// validateReadOnlyGuards enforces every changeset's read-only guards before the
// in-memory repository changes, in the same order and with the same meaning
// CommitService gives them — artifacts first, then projects — so a dropped guard
// is never mistaken for a satisfied one.
func (r *fakeRepo) validateReadOnlyGuards(changesets []storage.ChangeSet) error {
	for i := range changesets {
		for _, precondition := range changesets[i].ArtifactPreconditions {
			if err := r.artifactConflict(precondition.Path, precondition.ExpectedRevision); err != nil {
				return err
			}
		}
	}
	for i := range changesets {
		for _, precondition := range changesets[i].ProjectPreconditions {
			stored := r.revisions[precondition.ProjectID]
			if precondition.ExpectedRevision != stored {
				return &storage.RevisionConflictError{
					ProjectID:   precondition.ProjectID,
					ProjectName: precondition.ProjectName,
					Expected:    precondition.ExpectedRevision,
					Actual:      stored,
				}
			}
		}
	}
	return nil
}

func (r *fakeRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	if r.preCommit != nil {
		r.preCommit()
	}
	if r.commitErr != nil {
		return nil, r.commitErr
	}
	if err := r.validateReadOnlyGuards(changesets); err != nil {
		return nil, err
	}
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
		for _, artifact := range cs.Artifacts {
			if !r.validateChangeArtifacts || artifact.ExpectedRevision == nil {
				continue
			}
			if err := r.artifactConflict(artifact.Path, *artifact.ExpectedRevision); err != nil {
				return nil, err
			}
		}
	}
	results := make([]*storage.CommitResult, len(changesets))
	transactionID := fmt.Sprintf("txn-project-%d", len(r.commits)+1)
	for i := range changesets {
		cs := changesets[i]
		newDoc := documentOf(cs.Project, cs.Base)
		r.documents[cs.Project.ID] = newDoc
		r.revisions[cs.Project.ID] = newDoc.Revision
		for j, project := range r.projects {
			if project != nil && project.ID == cs.Project.ID {
				r.projects[j] = cs.Project
				break
			}
		}
		results[i] = &storage.CommitResult{Revision: newDoc.Revision, TransactionID: transactionID}
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

// artifactPort returns in-memory workspace-note and project-blocker ports over
// the repo's committed artifact bytes.
func (r *fakeRepo) artifactPort() *memArtifactPort { return &memArtifactPort{repo: r} }

type memArtifactPort struct{ repo *fakeRepo }

func (m *memArtifactPort) noteDir(location app.NoteLocation, project *models.Project) (string, error) {
	if location.Scope == app.NoteScopeGlobal {
		return filepath.Join("mem-workspace", ".tt", "global-notes"), nil
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
