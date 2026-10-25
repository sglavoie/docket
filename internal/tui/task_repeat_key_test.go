package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskform"
)

func TestRepeatKeyOpensASeededExpandedDraft(t *testing.T) {
	env := newRepeatKeyEnv(t)

	env.act(t, "D")

	composer := env.composer(t)
	if composer.Mode != taskform.ComposerExpanded || composer.Expanded == nil {
		t.Fatalf("composer mode = %v, expanded = %v; want the expanded form", composer.Mode, composer.Expanded != nil)
	}
	screen := env.driver.Render()
	if !strings.Contains(screen, "Repeat Task") {
		t.Fatalf("the draft is not titled Repeat Task:\n%s", screen)
	}
	// The literal sigils reach the screen unchanged, which is the visible half
	// of never serializing the source title through the capture grammar.
	for _, want := range []string{"#123", "$today", "naïve"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the seeded title lost %q:\n%s", want, screen)
		}
	}
}

func TestRepeatKeyCancelCreatesNothing(t *testing.T) {
	env := newRepeatKeyEnv(t)
	before := env.tasksFileBytes(t)

	env.act(t, "D")
	env.composer(t)
	// Esc on a draft with a title asks first; y is the discard.
	env.act(t, "esc", "y")

	if view := env.driver.App().currentView(); view != presentation.ViewCrossProjectOutliner {
		t.Fatalf("canceling left the app on %s", view)
	}
	if after := env.tasksFileBytes(t); after != before {
		t.Fatal("canceling the repeat draft rewrote tasks.md")
	}
	if got := len(env.reload(t).Tasks); got != 2 {
		t.Fatalf("task count after cancel = %d, want the original 2", got)
	}
}

// saveRepeat opens the draft, saves it untouched, and returns the project as it
// was before, as it is now, and the one task the repeat created. The copy/reset
// matrix is asserted from here in three parts, each reading committed bytes.
func (env *repeatKeyEnv) saveRepeat(t *testing.T) (before, after *models.Project, repeated *models.Task) {
	t.Helper()
	before = env.reload(t)

	env.act(t, "D")
	env.composer(t)
	env.act(t, "ctrl+s")

	after = env.reload(t)
	if len(after.Tasks) != 3 {
		t.Fatalf("task count = %d, want 3 (source, child, repeat)", len(after.Tasks))
	}
	return before, after, env.repeatedTask(t, after)
}

func TestRepeatKeySavesOneTaskRetainingTheShapeOfTheWork(t *testing.T) {
	env := newRepeatKeyEnv(t)
	original, _, repeated := env.saveRepeat(t)

	if repeated.Title != repeatKeySourceTitle {
		t.Fatalf("title = %q, want the source title verbatim", repeated.Title)
	}
	if repeated.Description != original.GetTaskByID(env.sourceID).Description {
		t.Fatalf("description = %q", repeated.Description)
	}
	if repeated.Priority != 2 {
		t.Fatalf("priority = %d, want 2", repeated.Priority)
	}
	if !slices.Equal(repeated.Tags, []string{"bug", "release"}) {
		t.Fatalf("tags = %v", repeated.Tags)
	}
	if repeated.Context != "office" {
		t.Fatalf("context = %q, want office", repeated.Context)
	}
	if got := repeatChecklistTexts(repeated.Checklist); !slices.Equal(got, []string{"Reproduce", "Patch", "Verify"}) {
		t.Fatalf("checklist = %v, want the source order", got)
	}
	for i, item := range repeated.Checklist {
		if item.Done {
			t.Fatalf("checklist item %d (%q) was copied checked", i, item.Text)
		}
	}
}

func TestRepeatKeySavesOneTaskClearingEveryOccurrenceField(t *testing.T) {
	env := newRepeatKeyEnv(t)
	_, _, repeated := env.saveRepeat(t)

	if repeated.State() != models.StateBacklog {
		t.Fatalf("state = %s, want backlog", repeated.State())
	}
	if repeated.ParentID != nil {
		t.Fatalf("parent = %v, want the project root", *repeated.ParentID)
	}
	if len(repeated.Children) != 0 {
		t.Fatalf("children = %d, want none copied", len(repeated.Children))
	}
	if repeated.DueDate != nil || repeated.DeferredUntil != nil {
		t.Fatalf("dates not cleared: due=%v deferred=%v", repeated.DueDate, repeated.DeferredUntil)
	}
	if repeated.Pinned || repeated.Review {
		t.Fatalf("flags not cleared: pinned=%v review=%v", repeated.Pinned, repeated.Review)
	}
	if repeated.ResumeNote != "" {
		t.Fatalf("resume note = %q, want cleared", repeated.ResumeNote)
	}
	if len(repeated.Comments) != 0 || len(repeated.Attachments) != 0 || len(repeated.Dependencies) != 0 {
		t.Fatalf("comments=%d attachments=%d dependencies=%d, want all cleared",
			len(repeated.Comments), len(repeated.Attachments), len(repeated.Dependencies))
	}
	for _, locator := range env.driver.App().domain.runtime.Planning().GetToday() {
		if locator.TaskID == repeated.ID {
			t.Fatal("the repeat inherited Today membership")
		}
	}
}

// Identity is minted by the ordinary creation transaction, and the source is
// left exactly as the repeat found it.
func TestRepeatKeyMintsFreshIdentityAndLeavesTheSourceAlone(t *testing.T) {
	env := newRepeatKeyEnv(t)
	original, fresh, repeated := env.saveRepeat(t)
	originalNextShortID := original.NextShortID

	if repeated.ID == env.sourceID || repeated.ShortID == original.GetTaskByID(env.sourceID).ShortID {
		t.Fatalf("identity was copied: id=%s shortID=%d", repeated.ID, repeated.ShortID)
	}
	if repeated.ShortID != originalNextShortID || fresh.NextShortID != originalNextShortID+1 {
		t.Fatalf("shortID = %d with next = %d, want %d minted once", repeated.ShortID, fresh.NextShortID, originalNextShortID)
	}
	if repeated.CreatedAt.IsZero() || repeated.UpdatedAt.IsZero() || repeated.StateChangedAt.IsZero() {
		t.Fatalf("timestamps not stamped: created=%v updated=%v stateClock=%v",
			repeated.CreatedAt, repeated.UpdatedAt, repeated.StateChangedAt)
	}

	// The source is untouched by its own repetition.
	source := fresh.GetTaskByID(env.sourceID)
	if source == nil {
		t.Fatal("the source task disappeared")
	}
	if source.State() != models.StateDoing || !source.Pinned || !source.Review || source.DueDate == nil ||
		source.DeferredUntil == nil || source.ResumeNote == "" || len(source.Comments) != 1 ||
		len(source.Attachments) != 1 || len(source.Children) != 1 {
		t.Fatalf("the source changed: %+v", source)
	}
	if got := source.Checklist; !got[0].Done || got[1].Done || !got[2].Done {
		t.Fatalf("the source checklist was reset: %+v", got)
	}
}

func TestRepeatKeyUndoAndRedoMoveOnlyTheNewTask(t *testing.T) {
	env := newRepeatKeyEnv(t)

	env.act(t, "D")
	env.composer(t)
	env.act(t, "ctrl+s")
	created := env.repeatedTask(t, env.reload(t))

	env.act(t, "u")
	afterUndo := env.reload(t)
	if len(afterUndo.Tasks) != 2 || afterUndo.GetTaskByID(created.ID) != nil {
		t.Fatalf("undo left %d tasks", len(afterUndo.Tasks))
	}
	if source := afterUndo.GetTaskByID(env.sourceID); source == nil || !source.Pinned || source.State() != models.StateDoing {
		t.Fatalf("undo disturbed the source: %+v", source)
	}

	env.act(t, "ctrl+r")
	afterRedo := env.reload(t)
	if len(afterRedo.Tasks) != 3 || afterRedo.GetTaskByID(created.ID) == nil {
		t.Fatalf("redo left %d tasks without the repeat", len(afterRedo.Tasks))
	}
}

// Preparation is a copy, not a view: once the draft is open it owes the source
// nothing, so editing and then deleting the source leaves the draft — and the
// task it saves — exactly as it was seeded.
func TestRepeatDraftSurvivesEditingAndDeletingTheSource(t *testing.T) {
	env := newRepeatKeyEnv(t)

	env.act(t, "D")
	env.composer(t)

	service := env.driver.App().DomainService()
	renamed := "Rewritten after the draft opened"
	if _, err := service.Edit(app.EditRequest{
		Locator: app.TaskLocator{ProjectID: env.project.ID, TaskID: env.sourceID},
		Title:   &renamed,
	}); err != nil {
		t.Fatalf("edit source: %v", err)
	}
	if _, err := service.Delete(app.DeleteRequest{
		Locator: app.TaskLocator{ProjectID: env.project.ID, TaskID: env.sourceID},
		Confirm: true,
	}); err != nil {
		t.Fatalf("delete source: %v", err)
	}

	env.act(t, "ctrl+s")

	fresh := env.reload(t)
	if len(fresh.Tasks) != 1 {
		t.Fatalf("task count = %d, want only the repeat (the source subtree is gone)", len(fresh.Tasks))
	}
	repeated := fresh.Tasks[0]
	if repeated.Title != repeatKeySourceTitle {
		t.Fatalf("title = %q, want the title the draft was seeded with", repeated.Title)
	}
	if got := repeatChecklistTexts(repeated.Checklist); !slices.Equal(got, []string{"Reproduce", "Patch", "Verify"}) {
		t.Fatalf("checklist = %v, want the seeded items", got)
	}
}

// D is not a bulk operation, so a visual selection has to refuse it rather than
// silently repeat whatever the cursor happens to name.
func TestRepeatKeyRefusesUnderAVisualSelection(t *testing.T) {
	env := newRepeatKeyEnv(t)
	before := env.tasksFileBytes(t)

	env.press(t, "v")
	env.act(t, "D")

	if view := env.driver.App().currentView(); view != presentation.ViewCrossProjectOutliner {
		t.Fatalf("D under a selection opened %s", view)
	}
	if env.driver.App().taskComposer != nil {
		t.Fatal("D under a selection opened a repeat draft")
	}
	if after := env.tasksFileBytes(t); after != before {
		t.Fatal("D under a selection wrote to tasks.md")
	}
}

// The destination is validated, not assumed: an archived project cannot take a
// new task, and the refusal is visible rather than a draft that fails on save.
func TestRepeatKeyRefusesAnArchivedSource(t *testing.T) {
	env := newArchivedGuardEnv(t)
	env.focusTask(t, env.taskID(t, env.archived, archivedGuardArchivedTaskTitle))
	before := env.tasksFileBytes(t, env.archived)

	env.act(t, "D")

	env.archivedRejection(t, "repeat task")
	if env.driver.App().taskComposer != nil {
		t.Fatal("D opened a draft over an archived project")
	}
	if after := env.tasksFileBytes(t, env.archived); after != before {
		t.Fatal("the refused repeat rewrote the archived project's tasks file")
	}
}

// Parked is a project tag, not an archive: a parked project is still writable,
// so repeating inside one an explicit scope reached works exactly as it does
// anywhere else. The scope is armed through the real picker, which is the only
// route a parked project's rows have.
func TestRepeatKeyRepeatsInsideAnExplicitlyScopedParkedProject(t *testing.T) {
	const parkedProjectName = "Paused Initiative"
	const parkedTaskTitle = "Revive the pilot"

	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	if _, addErr := workspace.addProject("Active Delivery", func(p *models.Project) {
		newFixtureTask(p, "Ship the release").ApplyWorkflowState(models.StateBacklog)
	}); addErr != nil {
		t.Fatalf("add active project: %v", addErr)
	}
	parked, err := workspace.addProject(parkedProjectName, func(p *models.Project) {
		p.Tags = []string{models.ParkedProjectTag}
		task := newFixtureTask(p, parkedTaskTitle)
		task.ApplyWorkflowState(models.StateBacklog)
		task.Tags = []string{"pilot"}
	})
	if err != nil {
		t.Fatalf("add parked project: %v", err)
	}
	driver, err := workspace.launch(120, 40)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	press := func(keys ...string) {
		t.Helper()
		for _, key := range keys {
			if keyErr := driver.SendKey(key); keyErr != nil {
				t.Fatalf("send key %q: %v", key, keyErr)
			}
			driver.drain()
		}
	}

	// Every task is Backlog, so launch seats the cursor on that folded lane's
	// header and `l` opens it; the fold outlives the scope change below.
	press("l")
	press("P")
	for range 32 {
		screen := driver.Observe(ObserveOptions{}).PlainScreen
		cursorOnParked := false
		for _, line := range strings.Split(screen, "\n") {
			if strings.Contains(line, "›") && strings.Contains(line, parkedProjectName) {
				cursorOnParked = true
			}
		}
		if cursorOnParked {
			break
		}
		press("down")
	}
	press("enter")
	driver.Wait(repeatKeySettleTimeout)

	out, ok := driver.App().outlinerPrimary()
	if !ok || out == nil || out.Scope.ProjectID != parked.ID {
		t.Fatalf("the picker did not scope to the parked project (scope=%+v)", out.Scope)
	}
	press("home")
	for range len(out.Rows) + 1 {
		if task, _ := driver.App().getCurrentCursorTaskAndProject(); task != nil && task.Title == parkedTaskTitle {
			break
		}
		press("down")
	}
	if task, _ := driver.App().getCurrentCursorTaskAndProject(); task == nil || task.Title != parkedTaskTitle {
		t.Fatalf("the cursor never reached the parked task: %+v", task)
	}

	press("D")
	driver.Wait(repeatKeySettleTimeout)
	if refusal := driver.App().feedback.err; refusal != nil {
		t.Fatalf("D refused inside a parked project: %v", refusal)
	}
	if driver.App().taskComposer == nil {
		t.Fatal("D opened no draft inside a parked project")
	}
	press("ctrl+s")
	driver.Wait(repeatKeySettleTimeout)

	fresh, err := models.NewProject(parked.ID, parked.Name, "")
	if err != nil {
		t.Fatalf("rebuild parked project: %v", err)
	}
	fresh.Path = parked.Path
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("load parked project: %v", err)
	}
	if len(fresh.Tasks) != 2 {
		t.Fatalf("parked project task count = %d, want the source and its repeat", len(fresh.Tasks))
	}
	for _, task := range fresh.Tasks {
		if task.Title != parkedTaskTitle {
			t.Fatalf("repeat title = %q, want %q", task.Title, parkedTaskTitle)
		}
	}
}
