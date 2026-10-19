package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// TestDueStepKeysAreTheCharactersTheTerminalSends holds the spelling rule. A
// terminal without the enhanced protocols reports `+` as itself; one with them
// reports the unshifted code and the shifted text beside it. Both are the same
// keystroke and must reach the same operation, and the `=` underneath must still
// be Toggle Review rather than a deadline the user never asked to move.
func TestDueStepKeysAreTheCharactersTheTerminalSends(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.plainID)

	env.dispatchKey(t, tea.KeyPressMsg{Code: '+', Text: "+"})
	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "an ordinary + event")

	env.focusTask(t, env.plainID)
	env.dispatchKey(t, tea.KeyPressMsg{Code: '=', ShiftedCode: '+', Mod: tea.ModShift, Text: "+"})
	env.requireDue(t, env.plainID, dueStepDay(4, 0, 0).Format("2006-01-02"), "an enhanced + event")

	env.focusTask(t, env.plainID)
	env.dispatchKey(t, tea.KeyPressMsg{Code: '-', Text: "-"})
	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "a - event")

	// The same physical key with no shift is the review flag, and it must not
	// have moved the deadline it sits beside.
	env.focusTask(t, env.plainID)
	env.dispatchKey(t, tea.KeyPressMsg{Code: '=', Text: "="})
	if !env.task(t, env.plainID).Review {
		t.Fatal("an unshifted = no longer toggles the review flag")
	}
	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "an unshifted =")
}

// TestDueStepWalksThroughUnsetAndRestoresTheClock is the line itself: no
// deadline sits between yesterday and today, arriving there is one-sided, and
// the wall clock a step cleared comes back when the user steps toward it again.
func TestDueStepWalksThroughUnsetAndRestoresTheClock(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.timedID)
	env.requireDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"), "the fixture")

	// Backward off today is the one side unset is reached from.
	env.step(t, "-")
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("- on a task due today failed: %v", err)
	}
	env.requireDue(t, env.timedID, "", "stepping backward off today")
	if got := env.driver.App().feedback.message; !strings.Contains(got, "Due date cleared") {
		t.Fatalf("status = %q, want it to name the cleared deadline", got)
	}

	// Forward off unset lands on today, and the clock the clear took away comes
	// back with it rather than a midnight the task never had.
	env.focusTask(t, env.timedID)
	env.step(t, "+")
	env.requireDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"), "stepping back onto today")

	// A task that never had a deadline starts the line from either end.
	env.focusTask(t, env.noneID)
	env.step(t, "-")
	env.requireDue(t, env.noneID, dueStepDay(-1, 0, 0).Format("2006-01-02"), "- on a task with no deadline")
	env.focusTask(t, env.noneID)
	env.step(t, "+")
	env.requireDue(t, env.noneID, "", "+ off yesterday")
}

// TestDueStepRepeatsAsSeparateTransactions holds the no-coalescing rule: a burst
// is one transaction and one ledger entry per press, so undo takes the presses
// back one at a time in the order they were typed.
func TestDueStepRepeatsAsSeparateTransactions(t *testing.T) {
	env := newDueStepKeyEnv(t)

	for range 3 {
		env.focusTask(t, env.plainID)
		env.step(t, "+")
	}
	env.requireDue(t, env.plainID, dueStepDay(5, 0, 0).Format("2006-01-02"), "three forward presses")

	env.press(t, "u")
	env.driver.Wait(dueStepSettleTimeout)
	env.requireDue(t, env.plainID, dueStepDay(4, 0, 0).Format("2006-01-02"), "one undo after three presses")

	env.press(t, "u")
	env.driver.Wait(dueStepSettleTimeout)
	env.press(t, "u")
	env.driver.Wait(dueStepSettleTimeout)
	env.requireDue(t, env.plainID, dueStepDay(2, 0, 0).Format("2006-01-02"), "undoing every press")
}

// TestDueStepQueuesPressesTypedWhileOneIsRunning is the serialization rule from
// the keyboard. A second press lands before the first commit comes back, waits
// for it, and is then computed from what that commit actually wrote — so two
// presses move two days rather than landing on the same one twice.
func TestDueStepQueuesPressesTypedWhileOneIsRunning(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.timedID)

	env.driver.dispatch(tea.KeyPressMsg{Code: '+', Text: "+"})
	pending := env.driver.pendingCmd
	env.driver.pendingCmd = nil
	if pending == nil {
		t.Fatal("the first + raised no transaction command")
	}

	// The second press cannot dispatch: it joins the queue and raises nothing.
	env.driver.dispatch(tea.KeyPressMsg{Code: '+', Text: "+"})
	if env.driver.pendingCmd != nil {
		t.Fatal("a second + dispatched while the first was still running")
	}
	env.requireDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"), "a queued press")

	env.driver.pendingCmd = pending
	env.driver.drain()
	env.driver.Wait(dueStepSettleTimeout)

	env.requireDue(t, env.timedID, dueStepDay(2, 14, 30).Format("2006-01-02 15:04"), "two queued presses")

	// Two presses are still two transactions: the first undo gives back one day.
	env.press(t, "u")
	env.driver.Wait(dueStepSettleTimeout)
	env.requireDue(t, env.timedID, dueStepDay(1, 14, 30).Format("2006-01-02 15:04"), "one undo after two presses")
}

// TestDueStepDropsQueuedWorkAnExplicitEditSuperseded is the coordination rule. A
// deliberate due edit lands between a dispatched step and the one queued behind
// it, which moves the deadline that queued step was to be computed from, so the
// queue is dropped rather than applied to a premise that no longer holds.
func TestDueStepDropsQueuedWorkAnExplicitEditSuperseded(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.plainID)

	env.driver.dispatch(tea.KeyPressMsg{Code: '+', Text: "+"})
	pending := env.driver.pendingCmd
	env.driver.pendingCmd = nil
	if pending == nil {
		t.Fatal("+ raised no transaction command")
	}
	env.driver.dispatch(tea.KeyPressMsg{Code: '+', Text: "+"})

	// One explicit deadline, typed while both steps are outstanding.
	env.press(t, "@")
	env.press(t, "t")
	env.driver.Wait(dueStepSettleTimeout)
	env.requireDue(t, env.plainID, dueStepDay(0, 0, 0).Format("2006-01-02"), "an explicit due edit")

	// The dispatched step still commits — its transaction ran, and it steps the
	// deadline the edit left rather than the one the key press saw — and the step
	// queued behind it is dropped, so the deadline lands one day past the edit
	// rather than two.
	env.driver.pendingCmd = pending
	env.driver.drain()
	env.driver.Wait(dueStepSettleTimeout)
	env.requireDue(t, env.plainID, dueStepDay(1, 0, 0).Format("2006-01-02"), "the superseded queue")
	if pending := env.driver.App().dueSteps.Pending(); pending != 0 {
		t.Fatalf("%d deadline step(s) survived the edit that superseded them", pending)
	}
}

// TestDueStepKeepsTheClockAnUnrelatedEditNeverNamed is the other half of that
// rule, and the half the queue alone cannot answer. The same explicit edit drops
// the pending tail whatever it named — a queued step is computed from a
// committed state the edit has moved — but a deadline typed on one task says
// nothing about the wall clock another task's step just cleared. The step that
// did commit keeps its own evidence, so stepping back out of unset hands the
// clock back rather than inventing a date-only deadline.
func TestDueStepKeepsTheClockAnUnrelatedEditNeverNamed(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.timedID)

	env.driver.dispatch(tea.KeyPressMsg{Code: '-', Text: "-"})
	pending := env.driver.pendingCmd
	env.driver.pendingCmd = nil
	if pending == nil {
		t.Fatal("- raised no transaction command")
	}
	env.driver.dispatch(tea.KeyPressMsg{Code: '-', Text: "-"})

	// One explicit deadline on a task the outstanding steps never named.
	env.focusTask(t, env.plainID)
	env.press(t, "@", "t")
	env.driver.Wait(dueStepSettleTimeout)
	env.requireDue(t, env.plainID, dueStepDay(0, 0, 0).Format("2006-01-02"), "an explicit due edit")

	env.driver.pendingCmd = pending
	env.driver.drain()
	env.driver.Wait(dueStepSettleTimeout)

	env.requireDue(t, env.timedID, "", "the dispatched step")
	if queued := env.driver.App().dueSteps.Pending(); queued != 0 {
		t.Fatalf("%d deadline step(s) survived the edit that superseded the queue", queued)
	}

	env.focusTask(t, env.timedID)
	env.step(t, "+")
	env.requireDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"), "stepping back out of unset")
}

// TestDueStepRefusesARowThatIsNotATask keeps the keys off the rows that answer
// no deadline: a project header refuses by name instead of stepping whatever the
// cursor last stood on.
func TestDueStepRefusesARowThatIsNotATask(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.press(t, "home")
	if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil {
		t.Fatalf("the first row is task %q, want a header", task.Title)
	}

	env.step(t, "+")
	err := env.driver.App().feedback.err
	if err == nil {
		t.Fatal("+ on a header row was accepted")
	}
	if !strings.Contains(err.Error(), "cursor") {
		t.Fatalf("refusal = %q, want it to name the cursor", err)
	}
	env.requireDue(t, env.plainID, dueStepDay(2, 0, 0).Format("2006-01-02"), "a refused header press")
	env.requireDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"), "a refused header press")
}

// TestDueStepFreezesTargetsWhileTheSaveIsRunning is the retargeting rule. The
// transaction command is held back while the cursor walks away, which is exactly
// what a slow save looks like from the keyboard, and the deadline that moves is
// still the one the key press named.
func TestDueStepFreezesTargetsWhileTheSaveIsRunning(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.plainID)

	env.driver.dispatch(tea.KeyPressMsg{Code: '+', Text: "+"})
	pending := env.driver.pendingCmd
	env.driver.pendingCmd = nil
	if pending == nil {
		t.Fatal("+ raised no transaction command")
	}

	// The cursor moves onto a different task before the commit comes back.
	env.focusTask(t, env.noneID)

	env.driver.pendingCmd = pending
	env.driver.drain()
	env.driver.Wait(dueStepSettleTimeout)

	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "the task the key named")
	env.requireDue(t, env.noneID, "", "the task the cursor moved onto")
}

// TestDueStepAppliesToHiddenSelectionMembers is the bulk keyflow: one member has
// no row to draw, and the operation acts on the frozen identities rather than on
// what is visible, reporting the part the user could not see.
func TestDueStepAppliesToHiddenSelectionMembers(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.childID)
	env.press(t, "v")
	env.focusTask(t, env.plainID)
	env.press(t, "v")

	// t folds every task subtree, which takes the selected child off the screen
	// without touching the selection that names it.
	env.press(t, "t")
	outlinerView := env.driver.App().crossOutliner
	counts := outlinerView.SelectionCounts()
	if counts.Visible != 1 || counts.Hidden != 1 {
		t.Fatalf("selection classification = %d visible %d hidden, want one of each", counts.Visible, counts.Hidden)
	}

	env.step(t, "+")
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("+ on a mixed selection failed: %v", err)
	}
	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "the visible member")
	env.requireDue(t, env.childID, dueStepDay(5, 0, 0).Format("2006-01-02"), "the hidden member")
	env.requireDue(t, env.timedID, dueStepDay(0, 14, 30).Format("2006-01-02 15:04"), "a task the selection never named")

	status := env.driver.App().feedback.message
	for _, want := range []string{"2 stepped", "2 set", "1 hidden"} {
		if !strings.Contains(status, want) {
			t.Fatalf("status = %q, want it to contain %q", status, want)
		}
	}
}

// TestDueStepKeysStayLiteralInATextInput holds the one rule the dispatch order
// gives for free and a second key router would break: while an overlay owns the
// keyboard, `+` and `-` are characters.
func TestDueStepKeysStayLiteralInATextInput(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.plainID)

	env.press(t, "a")
	env.press(t, "+", "-")

	screen := env.driver.Render()
	if !strings.Contains(screen, "+-") {
		t.Fatalf("the composer did not take + and - as text:\n%s", screen)
	}
	env.requireDue(t, env.plainID, dueStepDay(2, 0, 0).Format("2006-01-02"), "a deadline no key named")
}

// TestDueStepResultLandsWhileHelpIsOpen is the route-change rule from the other
// side: the commit is delivered and reloaded wherever the user has gone, because
// a transaction result is the shell's and not one surface's.
func TestDueStepResultLandsWhileHelpIsOpen(t *testing.T) {
	env := newDueStepKeyEnv(t)
	env.focusTask(t, env.plainID)

	env.driver.dispatch(tea.KeyPressMsg{Code: '+', Text: "+"})
	pending := env.driver.pendingCmd
	env.driver.pendingCmd = nil
	if pending == nil {
		t.Fatal("+ raised no transaction command")
	}

	env.press(t, "?")
	env.driver.pendingCmd = pending
	env.driver.drain()
	env.driver.Wait(dueStepSettleTimeout)

	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "a commit delivered under the help overlay")
	if !env.driver.App().Overlays().Active() {
		t.Fatal("delivering a deadline commit closed the help overlay")
	}
}

// TestDueStepRefusesTheWholeSelectionAndRetriesClean is the failure path. A
// terminal task cannot enter Today, so stepping it onto today refuses, and the
// refusal takes the whole selection with it rather than committing the half it
// could. The retry afterwards proves nothing was left armed.
func TestDueStepRefusesTheWholeSelectionAndRetriesClean(t *testing.T) {
	env := newDueStepKeyEnv(t)

	if state := env.task(t, env.doneID).State(); state != models.StateDone {
		t.Fatalf("fixture state = %s, want the task completed", state)
	}

	// A done task has no row in the open projection, so the search for its state
	// is what puts the cursor on it at all.
	env.press(t, "/")
	env.typeText("$done")
	env.press(t, "enter")

	env.focusTask(t, env.doneID)
	env.step(t, "-")
	if env.driver.App().feedback.err == nil {
		t.Fatal("stepping a completed task onto today was accepted")
	}
	env.requireDue(t, env.doneID, dueStepDay(1, 0, 0).Format("2006-01-02"), "a refused step")

	// The queue kept nothing from the refusal: once the refusal toast lapses, an
	// ordinary step right after it commits normally.
	env.press(t, "esc")
	env.driver.advanceClock(10 * time.Second)
	env.driver.drain()
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("the refusal toast outlived its timeout: %v", err)
	}

	env.focusTask(t, env.plainID)
	env.step(t, "+")
	if err := env.driver.App().feedback.err; err != nil {
		t.Fatalf("the step after a refusal failed: %v", err)
	}
	env.requireDue(t, env.plainID, dueStepDay(3, 0, 0).Format("2006-01-02"), "the step after a refusal")
}

// TestDueStepOperationsAreRegisteredOnOnePath holds the registry projections the
// two keys feed: each is one direct-only descriptor with a label the generated
// help and menu bar can name, and neither declares a value provider, because a
// step opens nothing.
func TestDueStepOperationsAreRegisteredOnOnePath(t *testing.T) {
	env := newDueStepKeyEnv(t)
	registry := env.driver.App().GetOperationRegistry()

	for _, id := range []operation.OperationID{
		operation.OperationTaskStepDueEarlier,
		operation.OperationTaskStepDueLater,
	} {
		descriptor, exists := registry.Descriptor(id)
		if !exists {
			t.Fatalf("%s is not registered", id)
		}
		if descriptor.Label == "" {
			t.Fatalf("%s carries no label for the generated help to name", id)
		}
		if descriptor.ValueProviderID != "" {
			t.Errorf("%s declares the value provider %q; a step opens nothing", id, descriptor.ValueProviderID)
		}
		if descriptor.TargetSource != operation.OperationTargetSelectedTasks {
			t.Errorf("%s targets %v, want the visual selection", id, descriptor.TargetSource)
		}
	}

	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	for _, want := range []struct {
		id  operation.OperationID
		key string
	}{
		{operation.OperationTaskStepDueEarlier, "-"},
		{operation.OperationTaskStepDueLater, "+"},
	} {
		if got := operation.DisplayKeysForOperation(strategy, want.id); got != want.key {
			t.Errorf("%s displays as %q, want %q", want.id, got, want.key)
		}
	}

	// The generated help is the projection the menu bar and `?` both read, so a
	// row per operation there is the parity that matters.
	items := registry.HelpItemsForInputStrategy(
		operation.InputStrategySimplified,
		env.driver.App().operationCommandContext(),
	)
	labels := map[string]string{}
	for _, item := range items {
		labels[item.Key] = item.Description
	}
	if got := labels["-"]; got != "Step Due Earlier" {
		t.Errorf("generated help lists %q under -, want %q", got, "Step Due Earlier")
	}
	if got := labels["+"]; got != "Step Due Later" {
		t.Errorf("generated help lists %q under +, want %q", got, "Step Due Later")
	}
}
