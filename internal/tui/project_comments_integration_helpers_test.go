package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// openProjectCommentsDraft mounts the panel on the fixture project's header and
// leaves an unsaved add-mode draft in it, which is the state every refresh case
// below perturbs.
func openProjectCommentsDraft(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: draft},
	})
	assertDraftIntact(t, driver, draft)
}

// openProjectCommentEditDraft commits one comment and then leaves an unsaved edit
// of it open, which is the other draft a refresh can land under. It is the harder
// one to keep: the add composer only holds text nobody has ever stored, while this
// buffer is a replacement for something that exists, and the panel has to be able
// to say so without either writing it or throwing it away.
func openProjectCommentEditDraft(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: seededProjectComment},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
		{Type: UITestActionKey, Key: "e"},
	})
	panel := mountedProjectCommentsPanel(t, driver.App())
	if !panel.EditMode {
		t.Fatalf("e did not open the edit composer over the committed comment:\n%s", driver.Render())
	}
	panel.EditTextInput.SetValue(draft)
	assertDraftIntact(t, driver, draft)
}

// seededProjectComment is the one comment the edit case rewrites. It is committed
// through the panel's own keys, so what the edit draft replaces is a comment the
// stored document really holds.
const seededProjectComment = "Committed before the removal"

// panelDraft reads whichever composer the panel has open. One accessor serves both
// drafts because what the user can still reach is the buffer of the composer that
// is on screen, and a panel with neither open has no draft left to lose.
func panelDraft(panel *taskdetail.CommentsModel) (string, bool) {
	switch {
	case panel.AddMode:
		return panel.TextInput.Value(), true
	case panel.EditMode:
		return panel.EditTextInput.Value(), true
	default:
		return "", false
	}
}

// assertDraftIntact is the claim every refresh owes: the panel is still the one
// the user was typing into, it is still drawing their text, and the text is
// exactly what they typed. The rendered half is not redundant — a composer that
// holds the buffer and stops painting it is a draft the user cannot reach.
func assertDraftIntact(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	got, open := panelDraft(panel)
	if !open {
		t.Fatalf("the panel is not holding a draft:\n%s", driver.Render())
	}
	if got != draft {
		t.Fatalf("draft = %q, want the untouched %q", got, draft)
	}
	if screen := driver.Render(); !strings.Contains(screen, draft) {
		t.Fatalf("the panel holds the draft but does not draw it:\n%s", screen)
	}
}

// externalProjectCommentWriter opens a second application service over the same
// workspace directory, which is what another surface — or another machine — is
// from this shell's point of view: a writer whose commits this process learns
// about only by reloading.
func externalProjectCommentWriter(t *testing.T, workspace *uitestWorkspace) *app.Service {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(workspace.DataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", workspace.DataDir, err)
	}
	runtime, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", workspace.DataDir, err)
	}
	return newTUIAppService(workspace.DataDir, manager.RuntimeDependencies(), workspaceLedgerFromConfig(runtime))
}

// refreshUnderThePanel runs the one fan-out every committed mutation ends in,
// which is also the only path that can dismiss an open panel, and then delivers
// everything it queued.
//
// The delivery is the point. A refresh that wants the panel gone says so with a
// command carrying BackMsg, and the pop happens when the shell handles that
// message: a case that stopped at the returned slice would be reading a panel that
// is on its way out as though it had stayed. Running the commands through the shell
// is what makes the route afterwards the answer.
func refreshUnderThePanel(t *testing.T, driver *UITestDriver) {
	t.Helper()
	for _, cmd := range driver.App().refreshTaskViews(true, false) {
		runSession2AppCommand(t, driver.App(), cmd)
	}
}

// assertPanelIsStillMounted reads that answer. Only the route can give it: a
// popped panel leaves its model behind intact, so its own fields say nothing about
// whether the user can still see it.
func assertPanelIsStillMounted(t *testing.T, driver *UITestDriver) {
	t.Helper()
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("the refresh left the route on %s, want the Comments panel:\n%s", got, driver.Render())
	}
}

// removeProjectFromShell drops one project out of the live catalog the panel's
// adapter resolves through, which is what a committed delete — or an archive whose
// reload no longer lists it — leaves behind for an open panel.
func removeProjectFromShell(shell *AppModel, projectID string) {
	for id, project := range shell.domain.projects {
		if project != nil && project.ID == projectID {
			delete(shell.domain.projects, id)
		}
	}
}

// assertMissingOwnerReported holds the half a dismissal used to swallow: the panel
// carries the missing-owner refusal and the user can read it beside their text.
// The wording is the panel's own, so the assertion asks the model for it rather
// than restating a sentence that lives in another package.
func assertMissingOwnerReported(t *testing.T, driver *UITestDriver) {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	if panel.Err == nil {
		t.Fatalf("the panel reports no refusal for the removed owner:\n%s", driver.Render())
	}
	if !strings.Contains(panel.Err.Error(), "no longer available") {
		t.Fatalf("the refusal %q does not name the missing owner", panel.Err)
	}
	if screen := driver.Render(); !strings.Contains(screen, panel.Err.Error()) {
		t.Fatalf("the refusal %q is not on screen:\n%s", panel.Err, screen)
	}
}

// assertStoredCommentTexts names the stored collection by text in stored order.
// Not a count: a draft that landed and a comment that was rewritten both keep the
// count the document already had.
func assertStoredCommentTexts(t *testing.T, workspace *uitestWorkspace, after string, want []string) {
	t.Helper()
	stored := projectCommentsOnDisk(t, workspace)
	got := make([]string, 0, len(stored))
	for _, comment := range stored {
		if comment != nil {
			got = append(got, comment.Text)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("after %s the stored comments are %q, want %q", after, got, want)
	}
}

// projectCommentTextsOnDisk reads one named project's stored comments, so a run
// that renames a project or moves a task out of one can read either document.
func projectCommentTextsOnDisk(t *testing.T, workspace *uitestWorkspace, slug string) []string {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", slug, "tasks.md")
	content, err := os.ReadFile(path) //nolint:gosec // The path is this test's own workspace.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	texts := make([]string, 0, len(document.Comments))
	for _, comment := range document.Comments {
		if comment != nil {
			texts = append(texts, comment.Text)
		}
	}
	return texts
}

// selectProjectPickerRow walks the open value picker onto the row the caller
// names, which is how the `%` move chooses a destination without depending on
// the fixture's project order.
func selectProjectPickerRow(t *testing.T, driver *UITestDriver, label string) {
	t.Helper()
	for range 32 {
		for _, line := range strings.Split(driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
			if strings.Contains(line, "\u203a") && strings.Contains(line, label) {
				return
			}
		}
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "down"}})
	}
	t.Fatalf("no picker row named %q:\n%s", label, driver.Render())
}

// seatCursorOnProjectHeaderFor parks the cursor on one project's header in the
// cross-project view, which is where the project half of `e` answers.
func seatCursorOnProjectHeaderFor(t *testing.T, driver *UITestDriver, projectID string) {
	t.Helper()
	view := driver.App().crossOutliner
	// A grouping projection draws the header only once the group is folded, and a
	// scoped one leads with a focused header no fold produces, so the fold is
	// what a miss spends rather than what the seat opens with. The launch's
	// Backlog fold can hide the header too, so the first miss opens every fold.
	for attempt := range 3 {
		switch attempt {
		case 1:
			view.ClearAllFolds()
		case 2:
			foldEveryProjectGroup(t, view)
		}
		for index, row := range view.Rows {
			if row.IsProjectHeader && row.ProjectID == projectID {
				view.Cursor = index
				return
			}
		}
		if attempt > 0 && len(view.VisibleProjectGroups()) == 0 {
			break
		}
	}
	t.Fatalf("no project header for %q:\n%s", projectID, driver.Render())
}
