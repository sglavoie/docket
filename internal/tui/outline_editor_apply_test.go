package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// TestOutlineEditedCarriesTheDumpRevisionAndKeepsTheBufferWhenItIsStale covers
// the other end of the round trip: the revision travels into the apply, and a
// writer that landed while the editor was open is reported as a kept buffer
// rather than a line-numbered parse failure.
func TestOutlineEditedCarriesTheDumpRevisionAndKeepsTheBufferWhenItIsStale(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	model := driver.App()

	project := outlineTestProject(t)
	buffer, scope, err := dumpOutlineBuffer(project, nil, "Backlog", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	applied := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(applied)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope, Buffer: buffer,
	})
	if applied.outline.ExpectedRevision != "rev-1" {
		t.Fatalf("ExpectedRevision = %q, want the dump revision", applied.outline.ExpectedRevision)
	}

	conflicted := &stubMutator{err: &storage.RevisionConflictError{
		ProjectID: project.ID, ProjectName: project.Name, Expected: "rev-1", Actual: "rev-2",
	}}
	model.features = coordinator.NewFeatureCoordinators(conflicted)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Path: "/tmp/tt-outline-stale.md", Buffer: buffer,
	})
	if !strings.Contains(model.statusMessage, "Workspace changed while the editor was open") {
		t.Errorf("status = %q, want the stale-workspace refusal", model.statusMessage)
	}
	if !strings.Contains(model.statusMessage, "/tmp/tt-outline-stale.md") {
		t.Errorf("status = %q, want the kept buffer named", model.statusMessage)
	}
	if model.currentView() == presentation.ViewOutlineErrors {
		t.Error("a stale workspace opened the line-error surface")
	}
}

// TestOutlineEditedArmsTheDeleteConfirmBeforeApplying is the guard the whole
// feature rests on: a buffer that dropped a line must reach a prompt, never the
// transaction.
func TestOutlineEditedArmsTheDeleteConfirmBeforeApplying(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Buffer: outlineBufferWithoutLine(t, buffer, "Sweep dead flags"),
	})

	if stub.calls != 0 {
		t.Fatalf("the apply ran before the prompt (%d calls)", stub.calls)
	}
	if !model.outlineDeleteConfirm.IsActive() {
		t.Fatal("a removed line did not arm the confirmation")
	}
	if got, want := model.outlineDeleteConfirm.Prompt(), "Delete task #4 from Sonar?"; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}
	detail := model.outlineDeleteConfirm.Detail
	if detail == nil || detail.TotalCount != 1 || !detail.UndoHint {
		t.Fatalf("detail = %+v, want one task with the undo hint", detail)
	}
	if len(detail.Items) != 1 || !strings.Contains(detail.Items[0], "Sweep dead flags") {
		t.Errorf("detail items = %v, want the removed task named", detail.Items)
	}
	if !strings.Contains(detail.CascadeNote, "1 task deleted") {
		t.Errorf("cascade note = %q, want the deletion count", detail.CascadeNote)
	}
	if !strings.Contains(detail.CascadeNote, "attachments:") {
		t.Errorf("cascade note = %q, want the attachment consequence", detail.CascadeNote)
	}
}

// TestOutlineDeleteConfirmDeclineKeepsTheBufferAndChangesNothing is the reason
// declining cancels the whole apply: a buffer that half landed would leave the
// user guessing which half, so the file stays and nothing commits.
func TestOutlineDeleteConfirmDeclineKeepsTheBufferAndChangesNothing(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(stub)

	file, err := os.CreateTemp(t.TempDir(), "tt-outline-*.md")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	path := file.Name()
	_ = file.Close()

	edited := outlineBufferWithoutLine(t, buffer, "Sweep dead flags")
	edited = strings.Replace(edited, "Ship auth rewrite", "Ship auth rewrite v2", 1)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope, Path: path, Buffer: edited,
	})

	handled, _ := model.handleOutlineDeleteConfirmKey("esc")
	if !handled {
		t.Fatal("esc was not consumed by the confirmation")
	}
	if stub.calls != 0 {
		t.Fatalf("a declined confirmation still applied the buffer (%d calls)", stub.calls)
	}
	if model.outlineDeleteConfirm.IsActive() {
		t.Error("the confirmation survived the decline")
	}
	if model.pendingOutlineApply.project != nil {
		t.Error("the pending apply survived the decline")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("the declined buffer was removed: %v", err)
	}
	if !strings.Contains(model.statusMessage, path) {
		t.Errorf("status = %q, want the kept buffer named", model.statusMessage)
	}
}

// TestOutlineDeleteConfirmCommitsEditsAndDeletesTogether is what keeps the
// removal in the same transaction and the same undo entry as the edits beside
// it: one request carries both.
func TestOutlineDeleteConfirmCommitsEditsAndDeletesTogether(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(stub)

	edited := outlineBufferWithoutLine(t, buffer, "Sweep dead flags")
	edited = strings.Replace(edited, "Ship auth rewrite", "Ship auth rewrite v2", 1)
	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope, Buffer: edited,
	})
	if _, _ = model.handleOutlineDeleteConfirmKey("y"); stub.calls != 1 {
		t.Fatalf("the confirmed apply ran %d times, want 1", stub.calls)
	}

	req := stub.outline
	if req.ExpectedRevision != "rev-1" {
		t.Errorf("ExpectedRevision = %q, want the dump revision", req.ExpectedRevision)
	}
	if strings.Join(req.ScopeTaskIDs, ",") != strings.Join(scope, ",") {
		t.Errorf("ScopeTaskIDs = %v, want the dumped ids %v", req.ScopeTaskIDs, scope)
	}
	if len(req.DeleteTaskIDs) != 1 || req.DeleteTaskIDs[0] != "71f0b3d5dddd" {
		t.Errorf("DeleteTaskIDs = %v, want the removed task", req.DeleteTaskIDs)
	}
	if !req.ConfirmDeletions {
		t.Error("the confirmed request did not record destructive intent")
	}
	if len(req.Lines) != 3 {
		t.Fatalf("the request carries %d lines, want the 3 that survived", len(req.Lines))
	}
	if req.Lines[0].Title != "Ship auth rewrite v2" {
		t.Errorf("the edit beside the deletion was lost: %q", req.Lines[0].Title)
	}
	if model.outlineDeleteConfirm.IsActive() || model.pendingOutlineApply.project != nil {
		t.Error("the pending apply outlived the commit")
	}
}

// TestOutlineEditedAppliesWithoutAConfirmWhenNothingIsAbsent keeps the prompt
// proportional: the confirmation exists for deletions, so an ordinary edit must
// not grow one.
func TestOutlineEditedAppliesWithoutAConfirmWhenNothingIsAbsent(t *testing.T) {
	model, project, buffer, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Buffer: strings.Replace(buffer, "Retry budget", "Retry budget v2", 1),
	})

	if model.outlineDeleteConfirm.IsActive() {
		t.Fatal("a buffer that deleted nothing armed the confirmation")
	}
	if stub.calls != 1 {
		t.Fatalf("the apply ran %d times, want 1", stub.calls)
	}
	if len(stub.outline.DeleteTaskIDs) != 0 || stub.outline.ConfirmDeletions {
		t.Errorf("an ordinary edit carried destructive intent: %+v", stub.outline)
	}
}

// TestOutlineEditedEmptiedBufferDeletesEveryTask is the case emptiness used to
// swallow: once absence deletes, a buffer with every line removed asks for the
// whole project, which is the one request the old syntax guard could not express.
func TestOutlineEditedEmptiedBufferDeletesEveryTask(t *testing.T) {
	model, project, _, scope := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project: project, Revision: "rev-1", ScopeTaskIDs: scope,
		Buffer: "# every line was removed\n\n",
	})

	if model.currentView() == presentation.ViewOutlineErrors {
		t.Fatal("an emptied buffer was still rejected as a syntax error")
	}
	if !model.outlineDeleteConfirm.IsActive() {
		t.Fatal("an emptied buffer did not arm the confirmation")
	}
	if got, want := model.outlineDeleteConfirm.Prompt(), "Delete 4 tasks from Sonar?"; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}

	if _, _ = model.handleOutlineDeleteConfirmKey("y"); stub.calls != 1 {
		t.Fatalf("the confirmed apply ran %d times, want 1", stub.calls)
	}
	req := stub.outline
	if len(req.Lines) != 0 {
		t.Errorf("the request carries %d lines, want none", len(req.Lines))
	}
	if strings.Join(req.DeleteTaskIDs, ",") != strings.Join(scope, ",") {
		t.Errorf("DeleteTaskIDs = %v, want every dumped id %v", req.DeleteTaskIDs, scope)
	}
	if !req.ConfirmDeletions {
		t.Error("the confirmed request did not record destructive intent")
	}
}

// TestOutlineEditedRefusesAnEmptyBufferThatDeletesNothing keeps the truncation
// accident refused: a buffer holding no lines over a scope that held no tasks
// asks for no change at all, so it is far likelier to be a lost file.
func TestOutlineEditedRefusesAnEmptyBufferThatDeletesNothing(t *testing.T) {
	model, _, _, _ := outlineConfirmFixture(t)
	stub := &stubMutator{result: app.TaskResult{Revision: "rev-2"}}
	model.features = coordinator.NewFeatureCoordinators(stub)

	model.handleOutlineEdited(OutlineEditedMsg{
		Project:  &models.Project{ID: "project-2", Name: "Empty", Slug: "empty"},
		Revision: "rev-1",
		Buffer:   "# only a comment\n\n",
	})

	if model.currentView() != presentation.ViewOutlineErrors {
		t.Fatalf("view = %v, want the errors surface", model.currentView())
	}
	if model.outlineDeleteConfirm.IsActive() {
		t.Error("a buffer that deletes nothing armed the confirmation")
	}
	if stub.calls != 0 {
		t.Errorf("the empty buffer was applied (%d calls)", stub.calls)
	}
}

func outlineConfirmFixture(t *testing.T) (*AppModel, *models.Project, string, []string) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 120, 40)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	project := outlineTestProject(t)
	buffer, scope, err := dumpOutlineBuffer(project, nil, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	return driver.App(), project, buffer, scope
}

func outlineBufferWithoutLine(t *testing.T, buffer, needle string) string {
	t.Helper()
	kept := make([]string, 0, strings.Count(buffer, "\n")+1)
	dropped := 0
	for _, line := range strings.Split(buffer, "\n") {
		if strings.Contains(line, needle) {
			dropped++
			continue
		}
		kept = append(kept, line)
	}
	if dropped != 1 {
		t.Fatalf("dropped %d lines matching %q, want 1", dropped, needle)
	}
	return strings.Join(kept, "\n")
}

func outlineDeepBuffer(depth int) string {
	lines := make([]string, 0, depth+1)
	for level := 0; level <= depth; level++ {
		lines = append(lines, strings.Repeat(" ", level*grammar.OutlineIndentWidth)+fmt.Sprintf("Level %d", level))
	}
	return strings.Join(lines, "\n")
}

func outlineProblemsContain(problems []grammar.LineError, line int, message string) bool {
	for _, problem := range problems {
		if problem.Line == line && problem.Message == message {
			return true
		}
	}
	return false
}
