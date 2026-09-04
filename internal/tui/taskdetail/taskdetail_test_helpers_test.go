package taskdetail

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

// The helpers below mirror the shell's own copies in
// internal/tui/config_test_helpers_test.go,
// internal/tui/secondary_coordinator_helpers_test.go,
// internal/tui/task_attachments_test.go,
// internal/tui/outliner_guidance_test.go and
// internal/tui/projects_notes_guidance_test.go. The shell keeps its versions
// for the driver-level tests that mount these panels through a live app, so
// duplicating them here is what lets the panel's own tests live beside the
// panel instead of forcing a fixture package into production reach.

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
		// Checked non-catalog commits now require the authoritative catalog
		// identity. Persist that one foundation artifact for real-directory test
		// runtimes instead of letting each test mint an unrelated domain ID.
		manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
		if err := manager.CommitCatalog(domains.Catalog); err != nil {
			panic(fmt.Sprintf("seed test workspace catalog: %v", err))
		}
	}
	return config.NewRuntimeState(dataDir, domains)
}

// testAppRepo is a minimal app.Repository for isolated panel tests, backed by
// storage.DefaultRepository. Snapshot mirrors the production repository's
// reload semantics: it loads each registered project's last-saved state back
// into the SHARED project pointer, so a panel's optimistic in-memory mutation
// is discarded before the use case re-applies it against the loaded state.
type testAppRepo struct {
	projects []*models.Project
}

func newTestAppRepo(projects ...*models.Project) *testAppRepo {
	for _, p := range projects {
		// Seed the backing store so the first Snapshot load succeeds; projects
		// without a usable path simply skip persistence (their tests never
		// dispatch a coordinator mutation).
		_ = storage.SaveProject(p)
	}
	return &testAppRepo{projects: projects}
}

func (r *testAppRepo) Snapshot() (app.Snapshot, error) {
	revisions := make(map[string]string, len(r.projects))
	documents := make(map[string]*storage.ProjectDocument, len(r.projects))
	for _, p := range r.projects {
		if err := storage.LoadProject(p); err != nil {
			return nil, err
		}
		doc, err := storage.LoadProjectDocument(p)
		if err != nil {
			return nil, err
		}
		revisions[p.ID] = doc.Revision
		documents[p.ID] = doc
	}
	return app.NewSnapshot(r.projects, revisions, documents), nil
}

func (r *testAppRepo) Commit(changesets ...storage.ChangeSet) ([]*storage.CommitResult, error) {
	results := make([]*storage.CommitResult, len(changesets))
	for i, cs := range changesets {
		if cs.Project != nil {
			if err := storage.SaveProject(cs.Project); err != nil {
				return nil, err
			}
		}
		results[i] = &storage.CommitResult{Revision: "test-revision", TransactionID: "test-transaction"}
	}
	return results, nil
}

// newTestSecondaryCoordinators builds production-shaped secondary coordinators
// over an application service seeded with the given projects, for isolated
// panel tests that exercise coordinator-backed mutations.
func newTestSecondaryCoordinators(projects ...*models.Project) coordinator.SecondaryCoordinators {
	return coordinator.NewSecondaryCoordinators(app.NewServiceWithArtifacts(newTestAppRepo(projects...), nil, nil, nil))
}

// fakeAttachmentMutator records the application use cases the attachments panel
// dispatches through its coordinator (the panel no longer mutates or saves the
// project itself).
type fakeAttachmentMutator struct {
	urls     []string
	codeRefs []string
	files    []string
	removals []fakeRemoval
	reorders []fakeReorder
}

type fakeRemoval struct {
	index      int
	deleteFile bool
}

type fakeReorder struct {
	from int
	to   int
}

func (f *fakeAttachmentMutator) AttachURL(locator app.TaskLocator, url string) (app.TaskResult, error) {
	f.urls = append(f.urls, url)
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func (f *fakeAttachmentMutator) AttachCodeReference(locator app.TaskLocator, path string, line int) (app.TaskResult, error) {
	f.codeRefs = append(f.codeRefs, path)
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func (f *fakeAttachmentMutator) AttachFile(locator app.TaskLocator, sourcePath string) (app.AttachFileResult, error) {
	f.files = append(f.files, sourcePath)
	return app.AttachFileResult{Task: app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}}, nil
}

func (f *fakeAttachmentMutator) RemoveAttachment(locator app.TaskLocator, index int, deleteFile bool) (app.RemoveAttachmentResult, error) {
	f.removals = append(f.removals, fakeRemoval{index: index, deleteFile: deleteFile})
	return app.RemoveAttachmentResult{Task: app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}}, nil
}

func (f *fakeAttachmentMutator) ReorderAttachment(locator app.TaskLocator, from, to int) (app.TaskResult, error) {
	f.reorders = append(f.reorders, fakeReorder{from: from, to: to})
	return app.TaskResult{TaskID: locator.TaskID, ProjectID: locator.ProjectID}, nil
}

func newFakeAttachmentCoordinator() (coordinator.AttachmentCoordinator, *fakeAttachmentMutator) {
	fake := &fakeAttachmentMutator{}
	return coordinator.NewAttachmentCoordinator(fake), fake
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
}

// assertFitsWithin fails if any rendered line exceeds width or the line count
// exceeds height. It mirrors internal/tui/task_form_view_test.go.
func assertFitsWithin(t *testing.T, rendered string, width, height int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	if len(lines) > height {
		t.Fatalf("render has %d lines, exceeds height %d:\n\n%s",
			len(lines), height, ansi.Strip(rendered))
	}
	for _, line := range lines {
		if w := lipgloss.Width(ansi.Strip(line)); w > width {
			t.Fatalf("line exceeds width %d: %q (width=%d)\n\n%s",
				width, ansi.Strip(line), w, ansi.Strip(rendered))
		}
	}
}
