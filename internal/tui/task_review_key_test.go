package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

const reviewKeySettleTimeout = 2 * time.Second

// reviewKeyEnv is the fixture the review keyflows share: two unflagged tasks in
// a parent/child pair and one flat task that starts flagged. The pair is what
// makes a member hideable — folding the parent takes the child off the screen —
// and it has to be the unflagged one, because the flag promotes a task into the
// All-mode Review lane where it is drawn flat and no fold can reach it.
type reviewKeyEnv struct {
	driver    *UITestDriver
	projectID string
	parentID  string
	childID   string
	flaggedID string
}

func newReviewKeyEnv(t *testing.T) *reviewKeyEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var parentID, childID, flaggedID string
	project, err := workspace.addProject("Review Key", func(project *models.Project) {
		parent := newFixtureTask(project, "Parent of the hideable child")
		parent.ApplyWorkflowState(models.StateBacklog)
		parentID = parent.ID

		child := newFixtureTask(project, "Hideable child")
		child.ApplyWorkflowState(models.StateBacklog)
		child.ParentID = &parent.ID
		childID = child.ID

		flagged := newFixtureTask(project, "Already flagged for review")
		flagged.ApplyWorkflowState(models.StateBacklog)
		flagged.Review = true
		flaggedID = flagged.ID
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
	return &reviewKeyEnv{
		driver:    driver,
		projectID: project.ID,
		parentID:  parentID,
		childID:   childID,
		flaggedID: flaggedID,
	}
}

func (env *reviewKeyEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

// toggleReview presses = and waits for the transaction it starts, so an
// assertion reads committed state rather than a command still in flight.
func (env *reviewKeyEnv) toggleReview(t *testing.T) {
	t.Helper()
	env.press(t, "=")
	env.driver.Wait(reviewKeySettleTimeout)
}

func (env *reviewKeyEnv) focusTask(t *testing.T, taskID string) {
	t.Helper()
	env.press(t, "home")
	out := env.driver.App().crossOutliner
	for step := 0; step <= len(out.Rows); step++ {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == taskID {
			return
		}
		// A launch folds the Backlog lane, so the walk opens it on the way.
		if row := out.Rows[out.Cursor]; row.IsCollapsibleHeader() && out.IsSectionRowCollapsed(row) {
			env.press(t, "l")
		}
		env.press(t, "down")
	}
	t.Fatalf("the cursor never reached task %s", taskID)
}

func (env *reviewKeyEnv) review(t *testing.T, taskID string) bool {
	t.Helper()
	project := env.driver.App().projectByID(env.projectID)
	if project == nil {
		t.Fatalf("project %s is no longer loaded", env.projectID)
	}
	task := project.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("task %s is no longer loaded", taskID)
	}
	return task.Review
}

func (env *reviewKeyEnv) identity(taskID string) taskref.TaskIdentity {
	return taskref.TaskIdentity{ProjectID: env.projectID, TaskID: taskID}
}

// TestReviewKeyTogglesTheCursorTask is the single-target keyflow: one press
// writes the flag, the strip offers the undo the ledger recorded, and a second
// press clears it again.
func TestReviewKeyTogglesTheCursorTask(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.parentID)

	env.toggleReview(t)
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("= failed: %v", err)
	}
	if !env.review(t, env.parentID) {
		t.Fatal("= did not flag the cursor task for review")
	}
	// The line is read back off the undo ledger entry the transaction wrote,
	// which is what proves the press produced one undoable action rather than a
	// message the presentation layer composed for itself.
	const want = "Toggle Review: review · u undo"
	if got := env.driver.App().feedback.message; !strings.HasPrefix(got, want) {
		t.Fatalf("status = %q, want it to lead with %q", got, want)
	}
	// Only the target moves: review is not a lifecycle state, and the flag says
	// nothing about the tasks around it.
	if env.review(t, env.childID) {
		t.Error("= flagged a task the cursor did not name")
	}

	env.focusTask(t, env.parentID)
	env.toggleReview(t)
	if env.review(t, env.parentID) {
		t.Fatal("a second = did not clear the flag")
	}
}

// TestReviewKeyFlagsTheWholeFrozenSelectionIncludingHiddenMembers is the bulk
// keyflow. The selection is mixed — one member already flagged — and one member
// has no row to draw, so it proves both halves of the direction rule and that
// the operation acts on the frozen identities rather than on what is visible.
func TestReviewKeyFlagsTheWholeFrozenSelectionIncludingHiddenMembers(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.childID)
	env.press(t, "v")
	env.focusTask(t, env.flaggedID)
	env.press(t, "v")

	// t folds every task subtree, which takes the selected child off the screen
	// without touching the selection that names it.
	env.press(t, "t")
	outlinerView := env.driver.App().crossOutliner
	if !outlinerView.IsMultiSelectActive() || outlinerView.SelectedCount() != 2 {
		t.Fatalf("selection = active %v count %d, want two frozen tasks",
			outlinerView.IsMultiSelectActive(), outlinerView.SelectedCount())
	}
	counts := outlinerView.SelectionCounts()
	if counts.Visible != 1 || counts.Hidden != 1 {
		t.Fatalf("selection classification = %d visible %d hidden, want one of each", counts.Visible, counts.Hidden)
	}

	env.toggleReview(t)
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("= on a mixed selection failed: %v", err)
	}
	// Not every member was flagged, so every member ends flagged: the one that
	// already carried the flag keeps it rather than being toggled against the
	// rest, and the hidden one is written even though nothing drew it.
	if !env.review(t, env.childID) || !env.review(t, env.flaggedID) {
		t.Fatalf("selection review = child %v flagged %v, want both flagged",
			env.review(t, env.childID), env.review(t, env.flaggedID))
	}
	if env.review(t, env.parentID) {
		t.Error("= reached a task the selection did not name")
	}

	// The selection survives its own mutation, so the next act can use it.
	outlinerView = env.driver.App().crossOutliner
	if !outlinerView.IsMultiSelectActive() || outlinerView.SelectedCount() != 2 {
		t.Fatalf("committed selection = active %v count %d, want the frozen two",
			outlinerView.IsMultiSelectActive(), outlinerView.SelectedCount())
	}

	// Now every member is flagged, so the same press clears every member.
	env.toggleReview(t)
	if env.review(t, env.childID) || env.review(t, env.flaggedID) {
		t.Fatalf("all-flagged selection = child %v flagged %v, want both cleared",
			env.review(t, env.childID), env.review(t, env.flaggedID))
	}
}

// TestReviewKeyRefusesAnEmptyVisualSelection holds the rule that a selection
// mode with nothing in it is a refusal rather than a silent fall-back to the
// cursor, which would mutate a task the user never selected.
func TestReviewKeyRefusesAnEmptyVisualSelection(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.parentID)

	context := env.driver.App().operationCommandContext()
	context.Selection.Visual = true
	context.Selection.Tasks = nil
	context.Selection.Entities = nil
	context.Selection.EntityIdentities = nil

	registry := env.driver.App().GetOperationRegistry()
	descriptor, exists := registry.Descriptor(operation.OperationTaskToggleReview)
	if !exists {
		t.Fatal("the review setter is not registered")
	}
	targets, err := (operation.OperationTargetResolver{}).Resolve(descriptor, context)
	if err == nil {
		t.Fatalf("an empty visual selection resolved %d target(s)", len(targets.Targets))
	}
	if env.review(t, env.parentID) {
		t.Error("the cursor task was flagged by a selection that named nothing")
	}
}

// TestReviewKeyClearingUnderTheReviewFacetRemovesTheRow is the disappearance
// case: the facet narrows by the flag the key writes, so clearing it takes the
// row out of the population that key press was standing in.
func TestReviewKeyClearingUnderTheReviewFacetRemovesTheRow(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.press(t, "f", "=")
	env.driver.Wait(reviewKeySettleTimeout)

	if !reviewKeyRowsContain(env, env.flaggedID) {
		t.Fatal("the review facet did not admit the flagged task")
	}
	if reviewKeyRowsContain(env, env.parentID) {
		t.Fatal("the review facet admitted an unflagged task")
	}
	env.focusTask(t, env.flaggedID)

	env.toggleReview(t)
	if env.review(t, env.flaggedID) {
		t.Fatal("= did not clear the flag the facet was narrowing by")
	}
	if reviewKeyRowsContain(env, env.flaggedID) {
		t.Fatal("a task that no longer passes the review facet is still drawn under it")
	}
}

// TestReviewLanePrecedenceSurvivesTheKey holds the All-mode lane order across
// the new write path: the flag promotes a task into Review, and a stronger claim
// still takes the row instead.
func TestReviewLanePrecedenceSurvivesTheKey(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.parentID)
	env.toggleReview(t)
	if section := reviewKeySectionFor(env, env.parentID); section != "Review" {
		t.Fatalf("the flagged task draws under %q, want Review", section)
	}

	// Pinned outranks Review, so the same task claims one row and it is the
	// stronger one.
	env.focusTask(t, env.parentID)
	env.press(t, "*")
	env.driver.Wait(reviewKeySettleTimeout)
	if !env.review(t, env.parentID) {
		t.Fatal("pinning cleared the review flag")
	}
	if section := reviewKeySectionFor(env, env.parentID); section != "Pinned" {
		t.Fatalf("a pinned flagged task draws under %q, want Pinned", section)
	}
	if count := reviewKeyRowCount(env, env.parentID); count != 1 {
		t.Fatalf("the task is drawn %d times, want exactly one row", count)
	}
}

// TestReviewKeyUndoRedoCarriesTheWholeSelection holds the one-transaction claim:
// a bulk toggle is a single ledger entry, so one u takes all of it back and one
// Ctrl+R puts all of it forward again.
func TestReviewKeyUndoRedoCarriesTheWholeSelection(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.parentID)
	env.press(t, "v")
	env.focusTask(t, env.childID)
	env.press(t, "v")

	env.toggleReview(t)
	if !env.review(t, env.parentID) || !env.review(t, env.childID) {
		t.Fatal("= did not flag the whole selection")
	}

	env.press(t, "u")
	env.driver.Wait(reviewKeySettleTimeout)
	if env.review(t, env.parentID) || env.review(t, env.childID) {
		t.Fatalf("undo left parent %v child %v flagged; one entry covers the batch",
			env.review(t, env.parentID), env.review(t, env.childID))
	}
	// The third task was never in the selection, so undo may not reach it either.
	if !env.review(t, env.flaggedID) {
		t.Error("undo reached a task the transaction never touched")
	}

	env.press(t, "ctrl+r")
	env.driver.Wait(reviewKeySettleTimeout)
	if !env.review(t, env.parentID) || !env.review(t, env.childID) {
		t.Fatalf("redo left parent %v child %v unflagged",
			env.review(t, env.parentID), env.review(t, env.childID))
	}
}

// TestReviewKeySaveFailureKeepsTheSelection holds the failure half of the
// selection contract. A rejected batch never reaches the committed-result
// handler, which is the only thing that clears a selection, so the user keeps
// the set they built and can retry it.
func TestReviewKeySaveFailureKeepsTheSelection(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.parentID)
	env.press(t, "v")
	env.focusTask(t, env.childID)
	env.press(t, "v")

	project := env.driver.App().projectByID(env.projectID)
	if project == nil {
		t.Fatal("the fixture project is not loaded")
	}
	if err := os.Chmod(project.Path, 0o500); err != nil {
		t.Skipf("cannot make the project directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(project.Path, 0o700) })

	env.toggleReview(t)
	if env.driver.App().feedback.err == nil {
		t.Skip("the project directory stayed writable; the failure path is untested here")
	}
	if env.review(t, env.parentID) || env.review(t, env.childID) {
		t.Error("a rejected toggle published a flag anyway")
	}
	outlinerView := env.driver.App().crossOutliner
	if !outlinerView.IsMultiSelectActive() || outlinerView.SelectedCount() != 2 {
		t.Fatalf("selection after refusal = active %v count %d, want the frozen two",
			outlinerView.IsMultiSelectActive(), outlinerView.SelectedCount())
	}
}

// TestReviewKeyStaysLiteralInsideTextInput holds the transient-overlay
// exemption: a composer is modal value entry, not a grammar, so = is a
// character there and toggles nothing behind it.
func TestReviewKeyStaysLiteralInsideTextInput(t *testing.T) {
	env := newReviewKeyEnv(t)
	env.focusTask(t, env.parentID)

	env.press(t, "a")
	if got := env.driver.App().currentView(); got != presentation.ViewTaskComposer {
		t.Fatalf("a opened %v, want the composer", got)
	}
	env.press(t, "=", "r", "e", "v", "i", "e", "w")
	if got := env.driver.App().taskComposer.Input.Value(); !strings.Contains(got, "=review") {
		t.Fatalf("composer input = %q, want the literal =review sigil", got)
	}
	if env.review(t, env.parentID) {
		t.Error("a keystroke inside the composer reached the row behind it")
	}
	env.press(t, "esc")
}

// TestReviewKeyIsOneOperationAcrossEveryProjection holds the descriptor, the
// menu bar, and the `?` help to one another. The bar names the operation and
// spells the key from the strategy, and the Space sheet stays the one entry it
// has always been.
func TestReviewKeyIsOneOperationAcrossEveryProjection(t *testing.T) {
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
	if got := operation.DisplayKeysForOperation(strategy, operation.OperationTaskToggleReview); got != "=" {
		t.Fatalf("display key = %q, want =", got)
	}

	menus := resolveOutlinerMenus(t, app, chrome.ModeAll)
	task := menuWithID(t, menus, outliner.MenuIDTask)
	found := false
	for _, entry := range task.Entries {
		if entry.OperationID != operation.OperationTaskToggleReview {
			continue
		}
		found = true
		if entry.Label != "Toggle Review" {
			t.Errorf("menu label = %q, want the descriptor's own label", entry.Label)
		}
		if entry.Key != "=" {
			t.Errorf("menu key = %q, want the key the strategy binds", entry.Key)
		}
	}
	if !found {
		t.Fatal("the Task menu does not project the review setter")
	}

	// The setter is DirectOnly, so the one-path rule keeps it out of the sheet
	// the Space menu draws.
	for _, entry := range taskOperationManifest() {
		if entry.ID == operation.OperationTaskToggleReview {
			t.Fatal("the review setter reached the task Space menu")
		}
	}
	if err := validateOutlinerOperationOnePath(strategy); err != nil {
		t.Fatalf("one-path rule: %v", err)
	}
}

// reviewKeyRowsContain reports whether the Outliner currently draws a row for
// the task.
func reviewKeyRowsContain(env *reviewKeyEnv, taskID string) bool {
	return reviewKeyRowCount(env, taskID) > 0
}

func reviewKeyRowCount(env *reviewKeyEnv, taskID string) int {
	count := 0
	outlinerView := env.driver.App().crossOutliner
	if outlinerView == nil {
		return 0
	}
	for _, row := range outlinerView.Rows {
		if row.IsTaskRow() && outlinerView.TaskIdentityForRow(row) == env.identity(taskID) {
			count++
		}
	}
	return count
}

// reviewKeySectionFor names the lane the task's row is drawn under, which is
// what makes lane precedence observable from a keyflow.
func reviewKeySectionFor(env *reviewKeyEnv, taskID string) string {
	outlinerView := env.driver.App().crossOutliner
	if outlinerView == nil {
		return ""
	}
	section := ""
	for _, row := range outlinerView.Rows {
		if row.IsSectionHeader {
			section = row.SectionName
			continue
		}
		if row.IsTaskRow() && outlinerView.TaskIdentityForRow(row) == env.identity(taskID) {
			return section
		}
	}
	return ""
}
