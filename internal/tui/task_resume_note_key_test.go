package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/operationhost"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

const resumeNoteSettleTimeout = 2 * time.Second

// resumeNoteEnv is the fixture the resume keyflows share. The tasks are spread
// across the lifecycle on purpose: a resume note is content rather than a step,
// so the key has to reach a done task exactly as it reaches a doing one.
type resumeNoteEnv struct {
	driver    *UITestDriver
	projectID string
	doingID   string
	doneID    string
	inboxID   string
}

func newResumeNoteEnv(t *testing.T) *resumeNoteEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var doingID, doneID, inboxID string
	project, err := workspace.addProject("Resume Note", func(project *models.Project) {
		doing := newFixtureTask(project, "Wire the importer")
		doing.ApplyWorkflowState(models.StateDoing)
		doingID = doing.ID

		done := newFixtureTask(project, "Ship the parser")
		done.ApplyWorkflowState(models.StateDone)
		doneID = done.ID

		inbox := newFixtureTask(project, "Triage the crash report")
		inbox.ApplyWorkflowState(models.StateInbox)
		inboxID = inbox.ID
	})
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	workspace.setCurrentProject(project.ID)
	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	driver.SetDrainTimeout(750 * time.Millisecond)
	return &resumeNoteEnv{
		driver:    driver,
		projectID: project.ID,
		doingID:   doingID,
		doneID:    doneID,
		inboxID:   inboxID,
	}
}

func (env *resumeNoteEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

func (env *resumeNoteEnv) typeText(t *testing.T, text string) {
	t.Helper()
	for _, r := range text {
		if r == ' ' {
			env.press(t, "space")
			continue
		}
		env.press(t, string(r))
	}
}

// clearPrompt empties the buffer the way a user does, which is also the only
// spelling of "clear this note".
func (env *resumeNoteEnv) clearPrompt(t *testing.T) {
	t.Helper()
	prompt := env.prompt(t)
	for range len(prompt.Value()) {
		env.press(t, "backspace")
	}
}

func (env *resumeNoteEnv) prompt(t *testing.T) *operationhost.OperationTextPromptModel {
	t.Helper()
	prompt := env.driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatal("no text prompt is mounted")
	}
	return prompt
}

func (env *resumeNoteEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	env.press(t, "home")
	for range len(env.driver.App().crossOutliner.Rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		env.press(t, "down")
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

// inToday reads the planning membership the note must not disturb.
func (env *resumeNoteEnv) inToday(taskID string) bool {
	for _, item := range env.driver.App().domain.runtime.Planning().GetToday() {
		if item.ProjectID == env.projectID && item.TaskID == taskID {
			return true
		}
	}
	return false
}

func (env *resumeNoteEnv) task(t *testing.T, taskID string) *models.Task {
	t.Helper()
	project := env.driver.App().projectByID(env.projectID)
	if project == nil {
		t.Fatalf("project %s is no longer loaded", env.projectID)
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %s is no longer loaded", taskID)
	}
	return task
}

// setResumeNote is the whole keyflow in one call: open the prompt on the cursor
// row, replace whatever it prefilled, and commit.
func (env *resumeNoteEnv) setResumeNote(t *testing.T, taskID, note string) {
	t.Helper()
	env.focusTask(t, taskID)
	env.press(t, "r")
	env.clearPrompt(t)
	env.typeText(t, note)
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)
}

// TestResumeNoteKeyWritesEditsAndClearsTheCursorTask walks the whole life of one
// note through the single key, because create, replace, and clear are one
// operation rather than three.
func TestResumeNoteKeyWritesEditsAndClearsTheCursorTask(t *testing.T) {
	env := newResumeNoteEnv(t)

	env.focusTask(t, env.doingID)
	env.press(t, "r")
	if got := env.prompt(t).Value(); got != "" {
		t.Fatalf("a task with no note prefilled %q", got)
	}
	env.typeText(t, "stopped at the CSV mapping")
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)

	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("r failed: %v", err)
	}
	if got := env.task(t, env.doingID).ResumeNote; got != "stopped at the CSV mapping" {
		t.Fatalf("note = %q", got)
	}
	// Only the target moves: the note says nothing about the tasks around it.
	if got := env.task(t, env.inboxID).ResumeNote; got != "" {
		t.Fatalf("r reached a task the cursor did not name: %q", got)
	}

	// A second press edits rather than starts over, so the buffer opens on what
	// is stored.
	env.focusTask(t, env.doingID)
	env.press(t, "r")
	if got := env.prompt(t).Value(); got != "stopped at the CSV mapping" {
		t.Fatalf("the prompt prefilled %q, want the stored note", got)
	}
	env.clearPrompt(t)
	env.typeText(t, "waiting on the vendor schema")
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "waiting on the vendor schema" {
		t.Fatalf("edited note = %q", got)
	}

	// An emptied buffer is a real answer here, and the only way to say the note
	// is gone.
	env.focusTask(t, env.doingID)
	env.press(t, "r")
	env.clearPrompt(t)
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "" {
		t.Fatalf("a blank buffer left %q behind", got)
	}
	if env.driver.App().operations.TextPrompt != nil {
		t.Error("the prompt stayed mounted after a committed clear")
	}
}

// TestResumeNoteKeyEscCancelsAndUnchangedTextCommitsNothing holds the two ways a
// prompt closes without a write. Esc abandons the buffer; Enter on text nobody
// changed is not an edit, so it must not advance the touch time or mint a ledger
// entry a later u would spend on nothing.
func TestResumeNoteKeyEscCancelsAndUnchangedTextCommitsNothing(t *testing.T) {
	env := newResumeNoteEnv(t)
	env.setResumeNote(t, env.doingID, "stopped at the CSV mapping")
	touched := env.task(t, env.doingID).UpdatedAt

	env.focusTask(t, env.doingID)
	env.press(t, "r")
	env.typeText(t, " and the header row")
	env.press(t, "esc")
	if env.driver.App().operations.TextPrompt != nil {
		t.Fatal("Esc left the prompt mounted")
	}
	if got := env.task(t, env.doingID).ResumeNote; got != "stopped at the CSV mapping" {
		t.Fatalf("Esc wrote %q", got)
	}

	env.focusTask(t, env.doingID)
	env.press(t, "r")
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("an unchanged note reported a failure: %v", err)
	}
	if !env.task(t, env.doingID).UpdatedAt.Equal(touched) {
		t.Fatalf("an unchanged note advanced UpdatedAt from %s to %s",
			touched, env.task(t, env.doingID).UpdatedAt)
	}
	if got := env.driver.App().feedback.message; !strings.Contains(got, "unchanged") {
		t.Fatalf("status = %q, want it to say the note was unchanged", got)
	}
	// A no-op wrote nothing, so undo still has to reach the edit before it.
	env.press(t, "u")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "" {
		t.Fatalf("undo after a no-op left %q, want the note the last real write added", got)
	}
}

// TestResumeNotePromptCarriesItsOwnOptions holds the two rules that differ
// between the values this one prompt collects. Reading them off the built prompt
// rather than the constructor is what proves the factory wired the right pair.
func TestResumeNotePromptCarriesItsOwnOptions(t *testing.T) {
	if got := operationhost.ResumeNoteTextPromptOptions(); !got.AcceptsEmpty || got.MaxLength != models.MaxResumeNoteLength {
		t.Fatalf("resume options = %#v, want an empty-accepting prompt bounded by the model", got)
	}
	if got := operationhost.NameTextPromptOptions(); got.AcceptsEmpty || got.MaxLength != 200 {
		t.Fatalf("name options = %#v, want the unchanged 200-character non-empty rule", got)
	}

	env := newResumeNoteEnv(t)
	env.focusTask(t, env.doingID)
	prompt, err := env.driver.App().newOperationTextPrompt(operation.OperationExecutionRequest{
		ID:      operation.OperationTaskSetResumeNote,
		Context: env.driver.App().operationCommandContext(),
	}, operation.ValueProviderResumeNote)
	if err != nil {
		t.Fatalf("build the resume prompt: %v", err)
	}
	// Typing past the bound is refused by the buffer rather than by the commit,
	// so the prompt never promises a note the canonical writer would reject.
	for range models.MaxResumeNoteLength + 40 {
		prompt.HandleKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	}
	if got := len([]rune(prompt.Value())); got != models.MaxResumeNoteLength {
		t.Fatalf("the buffer accepted %d runes, want the %d bound", got, models.MaxResumeNoteLength)
	}
	if err := models.ValidateResumeNote(prompt.Value()); err != nil {
		t.Fatalf("a note the prompt accepted is unstorable: %v", err)
	}

	// An emptied buffer is an answer here and a refusal on a name prompt, which
	// is the whole difference between the two option sets.
	for range models.MaxResumeNoteLength {
		prompt.HandleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	}
	_, dismissed, msg := prompt.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, isSelected := msg.(operation.OperationValueSelectedMsg)
	if !dismissed || !isSelected || selected.Value != "" {
		t.Fatalf("an emptied resume prompt emitted %#v (dismissed %v), want the empty value", msg, dismissed)
	}
}

// TestResumeNoteKeyRefusesEveryTargetItIsNotFor holds the refusals as refusals:
// none of them may quietly fall back to the cursor row, which would write a note
// onto a task the user was not pointing at.
func TestResumeNoteKeyRefusesEveryTargetItIsNotFor(t *testing.T) {
	env := newResumeNoteEnv(t)
	env.focusTask(t, env.doingID)

	registry := env.driver.App().GetOperationRegistry()
	descriptor, exists := registry.Descriptor(operation.OperationTaskSetResumeNote)
	if !exists {
		t.Fatal("the resume-note setter is not registered")
	}
	resolver := operation.OperationTargetResolver{}

	base := env.driver.App().operationCommandContext()
	if _, err := resolver.Resolve(descriptor, base); err != nil {
		t.Fatalf("the plain cursor row was refused: %v", err)
	}

	// A selection mode with nothing in it is a refusal rather than a fall-back.
	// Single-target resolution would happily find the cursor row, so the refusal
	// has to come from applicability, and the executor is where that is proved:
	// the whole invocation is unavailable rather than quietly retargeted.
	empty := base
	empty.Selection.Visual = true
	empty.Selection.Tasks = nil
	empty.Selection.Entities = nil
	empty.Selection.EntityIdentities = nil
	if env.driver.App().taskActionOperationApplicable(operation.OperationTaskSetResumeNote, empty) {
		t.Error("the setter stayed applicable under an empty visual selection")
	}
	result := env.driver.App().operations.Executor.Execute(operation.OperationExecutionRequest{
		ID: operation.OperationTaskSetResumeNote, Context: empty,
	})
	if !errors.Is(result.Err, operation.ErrOperationUnavailable) {
		t.Fatalf("an empty visual selection executed with %v, want an unavailable refusal", result.Err)
	}
	if result.ValueRequired != nil {
		t.Error("a refused invocation still raised the prompt")
	}

	// A populated one is refused for the same reason, and by the manifest rather
	// than by a guard: the entry declares no Bulk, so there is nothing to widen.
	populated := base
	populated.Selection.Visual = true
	populated.Selection.Tasks = []taskref.TaskIdentity{
		{ProjectID: env.projectID, TaskID: env.doingID},
		{ProjectID: env.projectID, TaskID: env.inboxID},
	}
	populated.Selection.Entities = []operation.EntityContext{
		{ProjectID: env.projectID, TaskID: env.doingID},
		{ProjectID: env.projectID, TaskID: env.inboxID},
	}
	if env.driver.App().taskActionOperationApplicable(operation.OperationTaskSetResumeNote, populated) {
		t.Error("the setter stayed applicable under a populated visual selection")
	}

	// A cursor that positively names something other than a task belongs to that
	// entity's grammar; r may not bind a task key there.
	for name, entity := range map[string]operation.EntityContext{
		"project header": {ProjectID: env.projectID, HasProjectHeaderCursor: true},
		"section header": {ProjectID: env.projectID, HasSectionHeaderCursor: true},
	} {
		context := base
		context.Entity = entity
		context.CurrentEntity = operation.CommandContextCurrentEntity(entity)
		if env.driver.App().taskActionOperationApplicable(operation.OperationTaskSetResumeNote, context) {
			t.Errorf("the setter stayed applicable on a %s", name)
		}
	}

	// The checklist row and the archived project are refused before resolution,
	// which is where every mutation is held to those two rules and why neither
	// needs a guard of its own here.
	checklist := base
	checklist.Entity.HasChecklistCursor = true
	if err := env.driver.App().operations.Executor.Execute(operation.OperationExecutionRequest{
		ID: operation.OperationTaskSetResumeNote, Context: checklist,
	}).Err; err == nil {
		t.Error("a checklist row accepted a resume-note edit")
	}
	archived := base
	archived.Entity.Archived = true
	if err := env.driver.App().operations.Executor.Execute(operation.OperationExecutionRequest{
		ID: operation.OperationTaskSetResumeNote, Context: archived,
	}).Err; err == nil {
		t.Error("an archived project accepted a resume-note edit")
	}
	// A note row of the search projection names no entity at all, so it is target
	// resolution that refuses rather than applicability — and refuse it must,
	// because the row above it is not what the cursor is on.
	note := base
	note.Entity = operation.EntityContext{HasNoteCursor: true}
	note.CurrentEntity = operation.EntityIdentity{}
	if err := env.driver.App().operations.Executor.Execute(operation.OperationExecutionRequest{
		ID: operation.OperationTaskSetResumeNote, Context: note,
	}).Err; err == nil {
		t.Error("a note row accepted a resume-note edit")
	}

	if got := env.task(t, env.doingID).ResumeNote; got != "" {
		t.Fatalf("a refused target still wrote %q to the cursor task", got)
	}
}

// TestResumeNoteKeyReachesEveryStateAndPreservesTheWorkflow is the coverage
// claim and the boundary claim at once: every lifecycle state takes a note, and
// taking one changes nothing about where the task sits.
func TestResumeNoteKeyReachesEveryStateAndPreservesTheWorkflow(t *testing.T) {
	env := newResumeNoteEnv(t)

	// Today is planning rather than lifecycle, so it is the membership a note
	// most plausibly disturbs.
	env.focusTask(t, env.doingID)
	env.press(t, "w", "t")
	env.driver.Wait(resumeNoteSettleTimeout)
	if !env.inToday(env.doingID) {
		t.Fatal("the fixture task never reached Today")
	}

	for _, target := range []struct {
		taskID string
		// mode is the key pair that makes the row reachable. A terminal task has
		// no row in All, which says nothing about whether it may take a note: the
		// note is content, so the only question is whether the cursor can get
		// there, and Recent is where a just-finished task is.
		mode []string
	}{
		{env.doingID, []string{"g", "a"}},
		{env.doneID, []string{"g", "r"}},
		{env.inboxID, []string{"g", "a"}},
	} {
		env.press(t, target.mode...)
		env.driver.Wait(resumeNoteSettleTimeout)
		taskID := target.taskID
		before := env.task(t, taskID)
		state, stateClock := before.State(), before.StateChangedAt

		env.setResumeNote(t, taskID, "picked up here")
		if err := env.driver.App().feedback.err; err != nil {
			t.Fatalf("%s refused a resume note: %v", state, err)
		}
		after := env.task(t, taskID)
		if after.ResumeNote != "picked up here" {
			t.Fatalf("%s note = %q", state, after.ResumeNote)
		}
		if after.State() != state {
			t.Fatalf("%s became %s", state, after.State())
		}
		if !after.StateChangedAt.Equal(stateClock) {
			t.Fatalf("%s state clock moved from %s to %s", state, stateClock, after.StateChangedAt)
		}
	}
	if !env.inToday(env.doingID) {
		t.Fatal("writing a note dropped the task out of Today")
	}

	// Moving through the workflow neither asks for a note nor throws one away:
	// the note is the user's to clear, so pausing does not collect one and
	// starting or finishing does not discard one.
	env.press(t, "g", "a")
	for _, transition := range [][]string{{"w", "p"}, {"w", "g"}, {"w", "d"}} {
		env.focusTask(t, env.doingID)
		env.press(t, transition...)
		env.driver.Wait(resumeNoteSettleTimeout)
		if env.driver.App().operations.TextPrompt != nil {
			t.Fatalf("%v raised a resume prompt", transition)
		}
		if got := env.task(t, env.doingID).ResumeNote; got != "picked up here" {
			t.Fatalf("%v changed the note to %q", transition, got)
		}
	}
	// Navigating is the same: the note follows the task rather than the view.
	env.press(t, "g", "a")
	env.press(t, "down", "up", "home", "end")
	if got := env.task(t, env.doingID).ResumeNote; got != "picked up here" {
		t.Fatalf("navigating changed the note to %q", got)
	}
}

// TestResumeNoteKeyUndoRedoCarriesTheNote holds the one-transaction claim: the
// edit is a single ledger entry, so u takes it back whole and Ctrl+R puts it
// forward again — clears included, which are edits like any other.
func TestResumeNoteKeyUndoRedoCarriesTheNote(t *testing.T) {
	env := newResumeNoteEnv(t)
	env.setResumeNote(t, env.doingID, "stopped at the CSV mapping")

	env.press(t, "u")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "" {
		t.Fatalf("undo left %q behind", got)
	}
	env.press(t, "ctrl+r")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "stopped at the CSV mapping" {
		t.Fatalf("redo left %q", got)
	}

	// A clear is undoable in exactly the same way, which is what makes emptying
	// the buffer safe to try.
	env.focusTask(t, env.doingID)
	env.press(t, "r")
	env.clearPrompt(t)
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "" {
		t.Fatalf("the clear did not land: %q", got)
	}
	env.press(t, "u")
	env.driver.Wait(resumeNoteSettleTimeout)
	if got := env.task(t, env.doingID).ResumeNote; got != "stopped at the CSV mapping" {
		t.Fatalf("undoing a clear left %q, want the note back", got)
	}
}

// TestResumeNoteFailedSaveReopensThePromptWithTheTypedText is the failure half
// of the contract. A commit can fail after the overlay closed, and retyping a
// sentence the user already wrote is the wrong thing to ask of them.
func TestResumeNoteFailedSaveReopensThePromptWithTheTypedText(t *testing.T) {
	env := newResumeNoteEnv(t)
	project := env.driver.App().projectByID(env.projectID)
	if project == nil {
		t.Fatal("the fixture project is not loaded")
	}
	if err := os.Chmod(project.Path, 0o500); err != nil {
		t.Skipf("cannot make the project directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(project.Path, 0o700) })

	env.focusTask(t, env.doingID)
	env.press(t, "r")
	env.typeText(t, "stopped at the CSV mapping")
	env.press(t, "enter")
	env.driver.Wait(resumeNoteSettleTimeout)

	prompt := env.driver.App().operations.TextPrompt
	if prompt == nil {
		if env.task(t, env.doingID).ResumeNote == "stopped at the CSV mapping" {
			t.Skip("the project directory stayed writable; the failure path is untested here")
		}
		t.Fatal("a failed save closed the prompt and dropped the typed text")
	}
	if got := prompt.Value(); got != "stopped at the CSV mapping" {
		t.Fatalf("the reopened prompt holds %q, want the text the user typed", got)
	}
	if got := prompt.Notice(); got == "" {
		t.Error("the reopened prompt does not say why the save did not land")
	}
	if got := env.task(t, env.doingID).ResumeNote; got != "" {
		t.Fatalf("a rejected save published %q anyway", got)
	}
}

// TestResumeNoteIsOneOperationAcrossEveryProjection holds the descriptor, the
// menu bar, and the sheet to one another: one registration, one key, and no
// second route through the Space menu.
func TestResumeNoteIsOneOperationAcrossEveryProjection(t *testing.T) {
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()

	strategy, ok := app.GetOperationRegistry().InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	if got := operation.DisplayKeysForOperation(strategy, operation.OperationTaskSetResumeNote); got != "r" {
		t.Fatalf("display key = %q, want r", got)
	}

	menus := resolveOutlinerMenus(t, app, chrome.ModeAll)
	task := menuWithID(t, menus, outliner.MenuIDTask)
	found := false
	for _, entry := range task.Entries {
		if entry.OperationID != operation.OperationTaskSetResumeNote {
			continue
		}
		found = true
		if entry.Key != "r" {
			t.Errorf("menu key = %q, want the key the strategy binds", entry.Key)
		}
	}
	if !found {
		t.Fatal("the Task menu does not project the resume-note setter")
	}

	for _, entry := range taskOperationManifest() {
		if entry.ID == operation.OperationTaskSetResumeNote {
			t.Fatal("the resume-note setter reached the task Space menu")
		}
	}
	if err := validateOutlinerOperationOnePath(strategy); err != nil {
		t.Fatalf("one-path rule: %v", err)
	}
}

// TestResumeNotePeekSurvivesNarrowAndResizedDimensions is the geometry claim at
// the shell level: the note is readable where the row is tightest, and a resize
// does not leave the viewport reserving a height the render disagrees with.
func TestResumeNotePeekSurvivesNarrowAndResizedDimensions(t *testing.T) {
	env := newResumeNoteEnv(t)
	env.setResumeNote(t, env.doingID, "stopped at the CSV mapping")
	env.focusTask(t, env.doingID)
	env.press(t, "b")
	if !env.driver.App().crossOutliner.PeekMode {
		t.Fatal("Peek did not open")
	}

	for _, size := range []struct{ width, height int }{{100, 35}, {80, 24}, {60, 20}, {120, 40}} {
		env.driver.Resize(size.width, size.height)
		env.focusTask(t, env.doingID)
		screen := env.driver.Render()
		if !strings.Contains(screen, "Resume:") {
			t.Fatalf("%dx%d Peek drew no resume line:\n%s", size.width, size.height, screen)
		}
		for number, line := range strings.Split(screen, "\n") {
			if got := len([]rune(line)); got > size.width {
				t.Fatalf("%dx%d line %d overflowed to %d", size.width, size.height, number, got)
			}
		}
		if issues := env.driver.Observe(ObserveOptions{}).LayoutIssues; len(issues) != 0 {
			t.Fatalf("%dx%d layout issues: %v", size.width, size.height, issues)
		}
	}
}
