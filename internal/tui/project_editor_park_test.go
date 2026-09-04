package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/status"
)

func openProjectEditor(
	t *testing.T,
	fixture *session4LifecycleFixture,
	project *models.Project,
	identityProtected bool,
) *projecthub.ProjectEditorModel {
	t.Helper()
	editor := projecthub.NewProjectEditorModel(
		project,
		fixture.app.configManager,
		fixture.app.runtime,
		fixture.app.appService,
		identityProtected,
	)
	fixture.app.showView(presentation.ViewProjectEditor, editor)
	return editor
}

// projectEditorRowClearPresses is more Backspaces than any fixture row holds
// characters, so one loop empties a row whatever it arrived carrying.
const projectEditorRowClearPresses = 64

// fillProjectEditor walks the editor's three rows in order with Tab and
// replaces the text of each one it is given, so the test reaches every field
// the way a user does rather than through the editor's internals. An empty
// value leaves that row exactly as the project stored it.
func fillProjectEditor(t *testing.T, editor *projecthub.ProjectEditorModel, name, description, tags string) {
	t.Helper()
	for index, value := range []string{name, description, tags} {
		if index > 0 {
			editor.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		}
		if value == "" {
			continue
		}
		for range projectEditorRowClearPresses {
			editor.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
		}
		for _, r := range value {
			editor.Update(syntheticKey(r))
		}
	}
	rendered := ansi.Strip(editor.View().Content)
	for _, value := range []string{name, description, tags} {
		if value != "" && !strings.Contains(rendered, value) {
			t.Fatalf("editor rows do not carry %q:\n%s", value, rendered)
		}
	}
}

func pressProjectEditorSave(t *testing.T, editor *projecthub.ProjectEditorModel) tea.Msg {
	t.Helper()
	_, cmd := editor.Update(syntheticCtrlKey('s'))
	if cmd == nil {
		t.Fatal("Ctrl+S produced no command")
	}
	return cmd()
}

// answerProjectParkConfirm drives the shell key rather than the confirmed
// message, so the test fails if the editor's prompt stops being the shared
// shell overlay.
func answerProjectParkConfirm(t *testing.T, app *AppModel, key rune) tea.Msg {
	t.Helper()
	if !app.projectParkConfirm.IsActive() {
		t.Fatal("no park confirmation is active")
	}
	_, cmd := app.Update(syntheticKey(key))
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestProjectEditorParkAsksTheSharedConfirmationBeforeWriting(t *testing.T) {
	project := newSession4Project(t, "Sonar")
	fixture := newSession4LifecycleFixture(t, project)
	live := fixture.app.projectByID(project.ID)

	// The picker path's prompt for the same project is the baseline the editor
	// must match word for word.
	if cmd := fixture.app.requestProjectTags(live, []string{models.ParkedProjectTag}); cmd != nil {
		t.Fatalf("picker park committed without confirmation: %#v", cmd())
	}
	pickerPrompt := fixture.app.projectParkConfirm.Prompt()
	fixture.app.clearPendingProjectTags()

	editor := openProjectEditor(t, fixture, live, false)
	fillProjectEditor(t, editor, "", "", models.ParkedProjectTag)
	msg := pressProjectEditorSave(t, editor)
	if _, ok := msg.(projecthub.ProjectEditorParkRequestMsg); !ok {
		t.Fatalf("parking save emitted %#v, want a park request", msg)
	}
	if _, cmd := fixture.app.Update(msg); cmd != nil {
		t.Fatalf("park request committed without confirmation: %#v", cmd())
	}

	if !fixture.app.projectParkConfirm.IsActive() {
		t.Fatal("parking editor save raised no confirmation")
	}
	if got := fixture.app.projectParkConfirm.Prompt(); got != pickerPrompt {
		t.Fatalf("editor prompt = %q, want the picker prompt %q", got, pickerPrompt)
	}
	if slices.Contains(live.Tags, models.ParkedProjectTag) {
		t.Fatal("parking editor save wrote before the answer")
	}
	if disk := session4DiskProject(t, fixture.dataDir, project.ID); len(disk.Tags) != 0 {
		t.Fatalf("durable tags = %v, want nothing written before the answer", disk.Tags)
	}
}

func TestProjectEditorParkCancelLeavesEveryFieldUnchanged(t *testing.T) {
	project := newSession4Project(t, "Sonar")
	fixture := newSession4LifecycleFixture(t, project)
	live := fixture.app.projectByID(project.ID)
	originalName, originalDescription := live.Name, live.Description

	editor := openProjectEditor(t, fixture, live, false)
	fillProjectEditor(t, editor, "Renamed Sonar", "office", models.ParkedProjectTag)
	fixture.app.Update(pressProjectEditorSave(t, editor))

	if msg := answerProjectParkConfirm(t, fixture.app, 'n'); msg != nil {
		t.Fatalf("canceling the park emitted %#v", msg)
	}
	if fixture.app.projectParkConfirm.IsActive() {
		t.Fatal("canceled park left the confirmation open")
	}
	if fixture.app.pendingProjectEditorSave != nil {
		t.Fatal("canceled park left the editor save pending")
	}
	if fixture.app.currentView() != presentation.ViewProjectEditor {
		t.Fatalf("canceled park left %s open, want the editor", fixture.app.currentView())
	}

	disk := session4DiskProject(t, fixture.dataDir, project.ID)
	if disk.Name != originalName || disk.Description != originalDescription || len(disk.Tags) != 0 {
		t.Fatalf(
			"canceled park wrote name %q description %q tags %v",
			disk.Name,
			disk.Description,
			disk.Tags,
		)
	}
}

func TestProjectEditorParkConfirmCommitsTheWholeEditAtomically(t *testing.T) {
	project := newSession4Project(t, "Sonar")
	sibling := newSession4Project(t, "Radar")
	fixture := newSession4LifecycleFixture(t, project, sibling)
	live := fixture.app.projectByID(project.ID)

	editor := openProjectEditor(t, fixture, live, false)
	fillProjectEditor(t, editor, "Renamed Sonar", "office", models.ParkedProjectTag)
	fixture.app.Update(pressProjectEditorSave(t, editor))

	confirmed := answerProjectParkConfirm(t, fixture.app, 'y')
	if _, ok := confirmed.(projecthub.ProjectParkConfirmedMsg); !ok {
		t.Fatalf("y emitted %#v, want the shared park confirmation", confirmed)
	}
	_, commitCmd := fixture.app.Update(confirmed)
	if commitCmd == nil {
		t.Fatal("confirmed park committed nothing")
	}
	updated, ok := commitCmd().(projecthub.ProjectUpdatedMsg)
	if !ok {
		t.Fatalf("confirmed park emitted %#v, want ProjectUpdatedMsg", commitCmd())
	}
	if !updated.Parked || updated.Project == nil {
		t.Fatalf("confirmed park reported %#v, want a parked project", updated)
	}

	fixture.app.Update(updated)

	disk := session4DiskProject(t, fixture.dataDir, project.ID)
	if disk.Name != "Renamed Sonar" || disk.Description != "office" {
		t.Fatalf("durable metadata = name %q description %q", disk.Name, disk.Description)
	}
	if !slices.Contains(disk.Tags, models.ParkedProjectTag) {
		t.Fatalf("durable tags = %v, want the reserved tag", disk.Tags)
	}
	if session4HasLoadedProject(fixture.cross, project.ID) &&
		slices.Contains(fixture.app.runtime.TaskWorkspaceProjects(), live) {
		t.Fatal("parked project stayed in the base task workspace")
	}
	if got := fixture.cross.Population.ReloadHeaderProjectID; got != sibling.ID {
		t.Fatalf("park anchor = %q, want the surviving sibling header %q", got, sibling.ID)
	}
	if !strings.Contains(fixture.app.statusMessage, "Fp to show · u undo") {
		t.Fatalf("park status = %q, want both recovery routes", fixture.app.statusMessage)
	}
}

func TestProjectEditorDescriptiveTagsSaveWithoutAPrompt(t *testing.T) {
	project := newSession4Project(t, "Sonar")
	fixture := newSession4LifecycleFixture(t, project)
	live := fixture.app.projectByID(project.ID)

	editor := openProjectEditor(t, fixture, live, false)
	fillProjectEditor(t, editor, "", "", "Client-Acme, billable")
	msg := pressProjectEditorSave(t, editor)
	if _, ok := msg.(projecthub.ProjectUpdatedMsg); !ok {
		t.Fatalf("descriptive tag save emitted %#v, want a committed update", msg)
	}
	if fixture.app.projectParkConfirm.IsActive() {
		t.Fatal("descriptive tag save raised a confirmation")
	}
	disk := session4DiskProject(t, fixture.dataDir, project.ID)
	if !slices.Equal(disk.Tags, []string{"billable", "client-acme"}) {
		t.Fatalf("durable tags = %v, want the normalized descriptive set", disk.Tags)
	}
}

func TestProjectEditorRefusesToParkTheSystemUnfiledProject(t *testing.T) {
	project := newSession4Project(t, "Unfiled")
	fixture := newSession4LifecycleFixture(t, project)
	live := fixture.app.projectByID(project.ID)

	editor := openProjectEditor(t, fixture, live, true)
	fillProjectEditor(t, editor, "", "", models.ParkedProjectTag)
	msg := pressProjectEditorSave(t, editor)
	errMsg, ok := msg.(status.ErrorMsg)
	if !ok || !strings.Contains(errMsg.Err.Error(), "cannot park the system Unfiled project") {
		t.Fatalf("protected park emitted %#v", msg)
	}
	if fixture.app.projectParkConfirm.IsActive() {
		t.Fatal("protected park raised a confirmation it would then refuse")
	}
	if disk := session4DiskProject(t, fixture.dataDir, project.ID); len(disk.Tags) != 0 {
		t.Fatalf("protected park wrote tags %v", disk.Tags)
	}
}
