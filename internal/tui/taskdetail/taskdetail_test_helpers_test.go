package taskdetail

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
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

// newTestCommentsPanel mounts the shared panel over a task owner whose commits
// reach a real application service seeded with the given project, which is what
// the shell does.
func newTestCommentsPanel(task *models.Task, project *models.Project) *CommentsModel {
	owner := NewTaskCommentOwner(task, project, newTestSecondaryCoordinators(project).Subview)
	return NewCommentsPanel(owner, "")
}

// newRefusingCommentsPanel mounts the same panel over coordinators that know
// nothing about the project, so every commit is refused at the application
// layer. It is how the draft-preserving failure paths are forced without a
// hand-written double that could disagree with a real refusal.
func newRefusingCommentsPanel(task *models.Task, project *models.Project) *CommentsModel {
	owner := NewTaskCommentOwner(task, project, newTestSecondaryCoordinators().Subview)
	return NewCommentsPanel(owner, "")
}

// The project-comment scaffolding below lives here rather than beside any one of
// its cases because too many files read it: a shared helper declared in a
// test-bearing file is what tui_shared_test_helper_home_test.go refuses, and this
// is the home it names.

const (
	olderCommentID = "11111111-2222-3333-4444-555555555555"
	newerCommentID = "66666666-7777-8888-9999-aaaaaaaaaaaa"
)

// projectCommentsFixture builds a project inside a real directory carrying two
// comments in canonical storage order, oldest first. The panel is expected to
// draw them the other way round, which is exactly what makes an index an unsafe
// address here.
func projectCommentsFixture(t *testing.T) *models.Project {
	t.Helper()
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	created := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	project.Comments = []*models.ProjectComment{
		{ID: olderCommentID, Text: "Older", CreatedAt: created},
		{ID: newerCommentID, Text: "Newer", CreatedAt: created.Add(time.Hour)},
	}
	return project
}

// refusalText runs a refusal command once and names the error it reported,
// failing when the command committed something instead. Running it a second time
// would submit whatever it carries twice, so the outcome is read from a single
// invocation.
func refusalText(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("the refused act produced no command at all")
	}
	outcome := cmd()
	msg, ok := outcome.(status.ErrorMsg)
	if !ok {
		t.Fatalf("the refused act resolved to %T, want a refusal", outcome)
	}
	if msg.Err == nil {
		t.Fatal("the refusal named no error")
	}
	return msg.Err.Error()
}

// saveExternalChange writes a change made by another surface so the application
// layer's fresh snapshot can see it. Mutating the in-memory project alone is
// invisible to a mutation, which re-resolves its target from storage.
func saveExternalChange(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
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

// panelText renders the panel and strips styling. The buffer alone is not the
// contract for any of its readers: the defect they lock is a composer that held
// the draft and stopped drawing it, so every assertion reads what a user would
// see.
func panelText(t *testing.T, model *CommentsModel) string {
	t.Helper()
	return ansi.Strip(model.View().Content)
}

// exportedDraftPathFromView reassembles the file the panel is naming on screen.
//
// The status is laid out at the panel's content width like every other sentence
// it writes, so a generated temporary path sits on one row in a wide panel and on
// several in a narrow one. What the user reads is the whole path either way, and
// a reader that stopped at the first newline would take the first row for the
// whole of it and open a file that does not exist. Rows are joined without a
// separator because a path carries no spaces: the only break in it is the one the
// wrap put there.
func exportedDraftPathFromView(view string) string {
	marker := strings.TrimSpace(draftExportStatusPrefix)
	start := strings.Index(view, marker)
	if start < 0 {
		return ""
	}
	var path strings.Builder
	for _, line := range strings.Split(view[start+len(marker):], "\n") {
		piece := strings.TrimSpace(line)
		if piece == "" && path.Len() == 0 {
			continue
		}
		if piece == "" || strings.ContainsAny(piece, "─━") {
			break
		}
		path.WriteString(piece)
	}
	return path.String()
}

// assertVisible fails when the rendered panel is missing something the user needs
// on screen — their own text, or the reason the panel refused to write it.
func assertVisible(t *testing.T, model *CommentsModel, want ...string) {
	t.Helper()
	view := panelText(t, model)
	for _, fragment := range want {
		if !strings.Contains(view, fragment) {
			t.Fatalf("the panel does not show %q:\n%s", fragment, view)
		}
	}
}

// assertStoredComments names the stored collection by text in stored order, which
// is what says a refusal wrote nothing: not a smaller count, not a different
// count, but the same comments saying the same things.
func assertStoredComments(t *testing.T, project *models.Project, want ...string) {
	t.Helper()
	got := make([]string, 0, len(project.Comments))
	for _, comment := range project.Comments {
		got = append(got, comment.Text)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stored comments = %q, want %q", got, want)
	}
}

// newTestProjectCommentsPanel mounts the shared panel over the project adapter,
// with commits reaching a real application service seeded with that project and
// a resolver that answers by identity, which is what the shell supplies.
func newTestProjectCommentsPanel(t *testing.T, project *models.Project) (*CommentsModel, *models.Project) {
	t.Helper()
	coordinators := newTestSecondaryCoordinators(project)
	return newProjectCommentsPanel(project, coordinators.ProjectComment, func(id string) *models.Project {
		if project.ID == id {
			return project
		}
		return nil
	}), project
}

func newProjectCommentsPanel(
	project *models.Project,
	comments coordinator.ProjectCommentCoordinator,
	resolve func(string) *models.Project,
) *CommentsModel {
	return NewCommentsPanel(NewProjectCommentOwner(project, resolve, comments), "")
}

// assertRecoverable names the exact text the panel is still holding for one
// composer. This is the contract itself: not "something survived" and not "the
// buffer is non-empty", but the editor's bytes, unchanged.
func assertRecoverable(t *testing.T, model *CommentsModel, kind commentMutationKind, want string) {
	t.Helper()
	got, ok := model.retainedDraftText(kind)
	if !ok {
		t.Fatalf("the panel is holding no recoverable draft at all")
	}
	if got != want {
		t.Fatalf("recoverable draft lost %d rune(s) and differs from the editor's result:\ngot  %q\nwant %q",
			len([]rune(want))-len([]rune(got)), tailOf(got), tailOf(want))
	}
}

// tailOf keeps a failure message readable when the texts under comparison are
// thousands of runes long; the end is where they differ.
func tailOf(text string) string {
	runes := []rune(text)
	if len(runes) <= 60 {
		return text
	}
	return "..." + string(runes[len(runes)-60:])
}

// deliverToPanel hands the panel one message the way the shell does and returns
// whatever it asked for in reply, so every step a test takes — an external
// editor's result and a key press alike — runs through Update's own routing
// rather than calling the handler underneath it.
func deliverToPanel(t *testing.T, model *CommentsModel, msg tea.Msg) tea.Cmd {
	t.Helper()
	_, cmd := model.Update(msg)
	return cmd
}

// editorResult is the message the external editor's command goroutine returns,
// addressed to the mounted owner.
func editorResult(model *CommentsModel, kind commentMutationKind, ref CommentRef, text string) commentEditorFinishedMsg {
	return commentEditorFinishedMsg{owner: model.ownerKey(), kind: kind, ref: ref, text: text}
}

// settle runs one panel command and feeds whatever it resolved to back through
// Update, the way the shell's delegation does. Every commit-aware transition
// happens there, so a test that only inspected the command would be asserting
// half the behavior.
func settle(t *testing.T, model *CommentsModel, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a commit command")
	}
	msg := cmd()
	model.Update(msg)
	return msg
}
