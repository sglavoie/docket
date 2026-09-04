package taskform

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// A test helper cannot cross a package boundary, so the shell's doubles are
// declared here too rather than one side reaching for the other. The shell keeps
// its own copies for the surfaces it still drives; these are the ones the form
// and composer need.

func syntheticKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func syntheticSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func syntheticCtrlKey(r rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: r, Mod: tea.ModCtrl}
}

func syntheticShiftKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: tea.ModShift}
}

func guidanceEntryText(spec chrome.GuidanceSpec) string {
	parts := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		parts = append(parts, chrome.GuidanceHintText(hint))
	}
	return strings.Join(parts, " · ")
}

func assertGuidanceContains(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	if !ok {
		t.Fatalf("%s guidance is unavailable", state)
	}
	got := guidanceEntryText(spec)
	for _, entry := range want {
		if !strings.Contains(got, entry) {
			t.Errorf("%s guidance = %q, want %q", state, got, entry)
		}
	}
}

func assertRequiredGuidance(t *testing.T, state string, spec chrome.GuidanceSpec, ok bool, want ...string) {
	t.Helper()
	assertGuidanceContains(t, state, spec, ok, want...)
	for _, expected := range want {
		found := false
		for _, hint := range spec.Hints {
			if chrome.GuidanceHintText(hint) == expected && hint.Required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s guidance does not require %q: %+v", state, expected, spec.Hints)
		}
	}
}

func assertGuidanceFits(t *testing.T, spec chrome.GuidanceSpec, width int) {
	t.Helper()
	lines := chrome.GuidanceLines(spec, width)
	if len(lines) > max(spec.Rows, 1) {
		t.Fatalf("guidance uses %d lines with row budget %d: %v", len(lines), spec.Rows, lines)
	}
	for _, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("guidance line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

func newTestConfig() *config.RuntimeState {
	return config.NewRuntimeState("", config.NewDomains())
}

func testRuntimeAt(dataDir string, runtime *config.RuntimeState) *config.RuntimeState {
	if runtime == nil {
		return config.EmptyRuntimeState(dataDir)
	}
	domains := &config.Domains{
		Catalog:  runtime.Catalog(),
		Planning: runtime.Planning(),
		Machine:  runtime.Machine(),
	}
	if dataDir != "" {
		// Checked non-catalog commits require the authoritative catalog
		// identity, so persist that one foundation artifact rather than letting
		// the test mint an unrelated domain ID.
		manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
		if err := manager.CommitCatalog(domains.Catalog); err != nil {
			panic(err)
		}
	}
	return config.NewRuntimeState(dataDir, domains)
}

// newTestRuntimeWithProject mints a seeded project and a runtime whose catalog
// knows it, which is the half of the shell's newTestAppModelWithProject the
// composer and form actually read.
func newTestRuntimeWithProject() (*config.RuntimeState, *models.Project) {
	runtime := newTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	catalog := config.NewWorkspaceCatalog()
	if err := catalog.UpsertProject(project); err != nil {
		panic(err)
	}
	runtime.SetCatalogInMemory(catalog)
	return testRuntimeAt(runtime.DataDir(), runtime), project
}

// scenarioRepo is a storage-backed app.Repository that drives coordinator
// mutations through the transactional commit service with the workspace undo
// ledger attached, mirroring the CLI/TUI production repository. It tracks each
// project's revision from the commit results so optimistic concurrency holds.
type scenarioRepo struct {
	svc       *storage.CommitService
	projects  []*models.Project
	revisions map[string]string
}

func newScenarioRepo(svc *storage.CommitService, projects ...*models.Project) *scenarioRepo {
	return &scenarioRepo{svc: svc, projects: projects, revisions: make(map[string]string)}
}

func (r *scenarioRepo) seed(t *testing.T) {
	t.Helper()
	changesets := make([]storage.ChangeSet, len(r.projects))
	for i, p := range r.projects {
		changesets[i] = storage.ChangeSet{Project: p, UndoAction: "seed"}
	}
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	for i, p := range r.projects {
		r.revisions[p.ID] = results[i].Revision
	}
}

// Snapshot serves the in-memory project pointers with each project's revision
// read from DISK, exactly like the production cli/tui repositories. Reusing the
// cached commit-result revision is not safe across chained mutations: the
// result revision is computed from the in-memory model, whose canonical form
// can differ from a reparse of its own written file.
func (r *scenarioRepo) Snapshot() (app.Snapshot, error) {
	revs := make(map[string]string, len(r.projects))
	docs := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, p := range r.projects {
		doc, err := storage.LoadProjectDocument(p)
		if err != nil {
			return nil, err
		}
		revs[p.ID] = doc.Revision
		docs[p.ID] = doc
	}
	return app.NewSnapshot(r.projects, revs, docs), nil
}

func (r *scenarioRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results, err := r.svc.CommitMulti(changesets)
	if err != nil {
		return nil, err
	}
	for i := range changesets {
		r.revisions[changesets[i].Project.ID] = results[i].Revision
	}
	return results, nil
}

// scenarioEnv bundles the wired application service, its coordinators, the undo
// ledger, and the seeded project so each scenario reads as mutation → assert.
type scenarioEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	features coordinator.FeatureCoordinators
	project  *models.Project
}

// newScenarioEnv seeds an empty project on disk and wires the coordinators over
// the real transactional stack. The shell's copy takes seed tasks; every
// composer and form scenario here creates its own, so this one does not.
func newScenarioEnv(t *testing.T, slug string) *scenarioEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = slug
	project.Path = filepath.Join(dataDir, "projects", slug)
	project.NextShortID = 1
	project.BuildTaskTree()

	repo := newScenarioRepo(commitSvc, project)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
	return &scenarioEnv{
		dataDir:  dataDir,
		ledger:   ledger,
		svc:      svc,
		features: coordinator.NewFeatureCoordinators(svc),
		project:  project,
	}
}

// reload reads the project fresh from disk so an assertion sees exactly the
// committed bytes, not the in-memory pointer the use case mutated.
func (e *scenarioEnv) reload(t *testing.T) *models.Project {
	t.Helper()
	fresh := &models.Project{ID: e.project.ID, Name: e.project.Name, Slug: e.project.Slug, Path: e.project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	return fresh
}

// undo reverts the most recent committed transaction through the workspace
// ledger, restoring the on-disk state the mutation replaced.
func (e *scenarioEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

// stubMutator is a hand-rolled FeatureMutator recording the last call and
// returning a canned result/error, so the form's save plumbing can be tested
// without the storage stack.
type stubMutator struct {
	result app.TaskResult
	err    error
	calls  int
	// project is what ProjectAtRevision hands back, paired with result.Revision.
	project *models.Project
	// outline records the last outline plan the coordinator submitted.
	outline app.OutlinePlanRequest
}

func (s *stubMutator) Capture(app.CaptureRequest) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) CaptureMany([]app.CaptureRequest) ([]app.TaskResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return []app.TaskResult{s.result}, nil
}
func (s *stubMutator) AddTaskWithChildren(app.AddTaskRequest) (app.FormCreateResult, error) {
	s.calls++
	return app.FormCreateResult{Task: s.result}, s.err
}
func (s *stubMutator) Edit(app.EditRequest) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) Transition(app.StateRequest) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) Complete(string) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) SetPriority(string, int) (app.TaskResult, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubMutator) ApplyOutlinePlan(req app.OutlinePlanRequest) (app.OutlinePlanResult, error) {
	s.calls++
	s.outline = req
	if s.err != nil {
		return app.OutlinePlanResult{}, s.err
	}
	return app.OutlinePlanResult{Updated: []app.TaskResult{s.result}}, nil
}
func (s *stubMutator) ProjectAtRevision(string) (*models.Project, string, error) {
	s.calls++
	return s.project, s.result.Revision, s.err
}
