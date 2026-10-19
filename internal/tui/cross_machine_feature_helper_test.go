package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// The cross-machine helper roles for the two mutations this release added: one
// deadline step and one checklist promotion. They live beside the roles in
// cross_machine_helper_test.go and speak the same barrier protocol, the same
// outcome vocabulary, and the same environment selectors; what they add is the
// evidence those two mutations owe that no earlier role could collect.
//
// Both are guarded by a strict read-set revision precondition rather than by the
// merge-capable ExpectedRevision every earlier role commits under. That is the
// whole reason they need roles of their own: a precondition is validated in one
// phase ahead of any three-way merge, conflict bundle, ledger image, or staged
// byte, so a loser here is refused whole and leaves nothing at all — not a
// partial document, not a bundle, not a ledger entry. "Left nothing" is a claim
// about durable state the losing process is the only one positioned to make
// before anybody else writes again, so these roles make it rather than reporting
// an error and trusting the gate to infer it.

const (
	// helperDueStepEnv names the direction one step travels and helperDueRetryEnv
	// asks a refused step to try again from whatever the winner left. They are two
	// variables because the retry is a second contract, not a second direction: a
	// role that always retried could never report a bare refusal, and the refusal
	// is what the multi-project case is about.
	helperDueStepEnv  = "TT_HELPER_DUE_STEP"
	helperDueRetryEnv = "TT_HELPER_DUE_RETRY"
	// helperDueUntouchedEnv asks a refused step to prove it left the workspace
	// exactly as it found it. It is opt-in because the claim is only true when no
	// other machine in the race writes this selection: two machines stepping one
	// task legitimately leave the winner's deadline, the winner's Today membership
	// and the winner's ledger entry behind, and a role that asserted otherwise
	// would be calling a correct race a lost update. The pairing it is set for is
	// the one where the interfering write names a different task and a different
	// action, which is exactly where "nothing moved" is a claim about this process.
	helperDueUntouchedEnv = "TT_HELPER_DUE_UNTOUCHED"

	helperDueStepForward  = "forward"
	helperDueStepBackward = "backward"

	// helperChecklistEnv selects which checklist call the editing role makes, and
	// the two index variables address the item it makes it about. A checklist item
	// carries no stored identity, so a position is the only thing the gate script
	// and the in-suite race can both spell.
	helperChecklistEnv      = "TT_HELPER_CHECKLIST"
	helperChecklistIndexEnv = "TT_HELPER_CHECKLIST_INDEX"
	helperChecklistToEnv    = "TT_HELPER_CHECKLIST_TO"

	helperChecklistEdit    = "edit"
	helperChecklistReorder = "reorder"

	// The three checklist items the seed writes onto the first race task. They are
	// named here for the reason the seeded project comments are: a helper that
	// appended its own item first would be racing over that write instead of over
	// the promotion under test, and the frozen-checklist identity a promotion is
	// refused by would then be measuring the helper's own append.
	helperSeedItemA = "Collect the migration inventory"
	helperSeedItemB = "Name the rollback owner"
	helperSeedItemC = "Book the cutover window"

	// helperTitleSeparator lets one TT_HELPER_TASK_TITLE name a whole selection.
	// A deadline step is a batch operation over targets that may live in different
	// projects, and one variable holding an ordered list is what keeps the gate
	// script from having to invent a second, differently-shaped one.
	helperTitleSeparator = "|"

	// helperDueStepUndoAction and helperPromotionUndoAction are the ledger actions
	// the two mutations record, and therefore the names an installed barrier holds
	// them by. internal/app owns the spellings; they are repeated rather than
	// exported because a test in another package reaching for them is the coupling
	// an exported constant would make permanent.
	helperDueStepUndoAction   = "Step Due Date"
	helperPromotionUndoAction = "Promote Checklist Item"

	// helperDeadlineLayout is how a deadline is compared here. Both sides of every
	// comparison are instants, so formatting both through one layout is what makes
	// "unchanged" a claim about the value rather than about the pointer.
	helperDeadlineLayout = "2006-01-02 15:04"
)

// TestCrossMachineHelperStepsDueDate walks one selection's deadlines a single
// calendar place and reports what it committed, or proves that a refusal left
// the workspace exactly as it found it.
//
// It is the first role whose mutation spans projects. The barrier filters on the
// first target alone because awaitBulkCommitBarrier fires once per resolved
// target: holding on every one would publish a second revision over the first,
// and the party reading the ready file would compare a value nobody was waiting
// at. One hold is enough — the whole batch is one transaction, so a process held
// at any of its targets is a process that has not written.
//
// A refusal is the interesting half. The read-set guard is strict, so a step that
// lost is refused before a merge is attempted, and the three things it must not
// have touched are the deadlines it calculated, the planning artifact a step onto
// today would have rewritten, and the ledger. All three are captured before the
// barrier goes in and re-read from disk afterwards.
func TestCrossMachineHelperStepsDueDate(t *testing.T) {
	env := openHelperWorkspace(t)
	direction := helperDueStepDirection(t)
	targets := helperDueStepTargets(t, env)

	deadlines := helperStoredDeadlines(t, env, targets)
	planning := helperPlanningBytes(t, env)
	newestUndo := helperNewestUndoAction(t, env)

	release := helperHoldAtCommit(t, helperDueStepUndoAction, targets[0].ProjectID, targets[0].TaskID)
	result, err := env.service.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationStepDueDate,
		Targets:   targets,
		DueStep:   app.DueStepRequest{Direction: direction},
	})
	// Released before anything else runs: the claims below re-read the workspace
	// and the retry commits again, and a barrier still installed would hold the
	// second commit at a rendezvous the partner has already left.
	release()
	if err != nil {
		if strings.TrimSpace(os.Getenv(helperDueUntouchedEnv)) != "" {
			helperRequireStepLeftNothing(t, env, targets, deadlines, planning, newestUndo)
		}
		if strings.TrimSpace(os.Getenv(helperDueRetryEnv)) == "" {
			helperRecordRefusal(t, "StepDueDate", err)
			return
		}
		helperRetryDueStep(t, env, targets, direction)
		return
	}
	helperRequireDueStepsStored(t, env, result.DueSteps)
	helperRecord(t, "committed")
}

// helperDueStepDirection reads the one direction a step may travel. There is no
// default: a role invoked without a direction would silently step forward, and
// every claim about where a deadline landed would be a claim about the wrong
// sequence.
func helperDueStepDirection(t *testing.T) app.DueStepDirection {
	t.Helper()
	switch value := helperEnv(t, helperDueStepEnv); value {
	case helperDueStepForward:
		return app.DueStepForward
	case helperDueStepBackward:
		return app.DueStepBackward
	default:
		t.Fatalf("%s=%q names no deadline step direction", helperDueStepEnv, value)
		return app.DueStepForward
	}
}

// helperDueStepTargets resolves the ordered selection one step is made over.
// Titles are separated rather than passed as several variables because the order
// is part of the input: the first target is the one the barrier holds on.
func helperDueStepTargets(t *testing.T, env helperEnvironment) []app.TaskLocator {
	t.Helper()
	titles := strings.Split(helperEnv(t, helperTitleEnv), helperTitleSeparator)
	targets := make([]app.TaskLocator, 0, len(titles))
	for _, title := range titles {
		trimmed := strings.TrimSpace(title)
		if trimmed == "" {
			continue
		}
		targets = append(targets, helperTaskLocator(t, env, trimmed))
	}
	if len(targets) == 0 {
		t.Fatalf("%s named no task to step", helperTitleEnv)
	}
	return targets
}

// helperRequireStepLeftNothing is the refused step's whole claim, and the reason
// this role exists. A strict precondition is validated ahead of every write, so
// the loser owes three absences rather than one: no deadline moved, the planning
// artifact a step onto today would have rewritten is byte-identical, and the
// ledger's newest entry is still whatever it was before this process started.
func helperRequireStepLeftNothing(
	t *testing.T,
	env helperEnvironment,
	targets []app.TaskLocator,
	deadlines map[app.TaskLocator]string,
	planning []byte,
	newestUndo string,
) {
	t.Helper()
	for locator, want := range helperStoredDeadlines(t, env, targets) {
		if want != deadlines[locator] {
			helperRecord(t, "lost: a refused step moved a deadline")
			t.Fatalf("task %s/%s holds deadline %q after a refused step, want the untouched %q",
				locator.ProjectID, locator.TaskID, want, deadlines[locator])
		}
	}
	if after := helperPlanningBytes(t, env); string(after) != string(planning) {
		helperRecord(t, "lost: a refused step rewrote the planning artifact")
		t.Fatalf("planning.json changed under a refused deadline step")
	}
	if after := helperNewestUndoAction(t, env); after == helperDueStepUndoAction {
		helperRecord(t, "lost: a refused step recorded a ledger entry")
		t.Fatalf("the ledger's newest entry is %q after a refused step, want the untouched %q",
			after, newestUndo)
	}
}

// helperRetryDueStep is the second half of the two-steps-on-one-task contract:
// a refused step, retried explicitly, advances from the deadline the winner
// wrote rather than from the one this process had calculated against.
//
// The claim is made on the retry's own reported Before, compared with the bytes
// on disk. Reporting a deadline the file does not hold would mean the retry
// re-used its first attempt's stale read, which is exactly how two reported
// successes could describe only one step.
func helperRetryDueStep(
	t *testing.T, env helperEnvironment, targets []app.TaskLocator, direction app.DueStepDirection,
) {
	t.Helper()
	winner := helperStoredDeadlines(t, env, targets)
	result, err := env.service.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationStepDueDate,
		Targets:   targets,
		DueStep:   app.DueStepRequest{Direction: direction},
	})
	if err != nil {
		helperRecord(t, "failed: "+err.Error())
		t.Fatalf("retrying a refused deadline step: %v", err)
	}
	moves := make([]string, 0, len(result.DueSteps))
	for _, outcome := range result.DueSteps {
		from := helperDeadlineValue(outcome.Before)
		if from != winner[outcome.Target] {
			helperRecord(t, "lost: a retry stepped from a deadline nobody wrote")
			t.Fatalf("the retry stepped from %q, want the winner's %q", from, winner[outcome.Target])
		}
		moves = append(moves, from+">"+helperDeadlineValue(outcome.After))
	}
	helperRequireDueStepsStored(t, env, result.DueSteps)
	// Comma-separated, because a deadline carries a space of its own and a
	// space-separated list would not survive being read back as one move per
	// target.
	helperRecord(t, "retried: "+strings.Join(moves, ","))
}

// helperRequireDueStepsStored turns a reported step into a claim about the
// bytes. A machine that reports a deadline the file does not hold is the lost
// update the whole gate exists to catch.
func helperRequireDueStepsStored(t *testing.T, env helperEnvironment, outcomes []app.DueStepOutcome) {
	t.Helper()
	for _, outcome := range outcomes {
		want := helperDeadlineValue(outcome.After)
		got := helperDeadlineText(helperStoredTask(t, env, outcome.Target))
		if got != want {
			helperRecord(t, "lost: "+got)
			t.Fatalf("task %s/%s holds deadline %q on disk, want the committed %q",
				outcome.Target.ProjectID, outcome.Target.TaskID, got, want)
		}
	}
}

// helperStoredDeadlines reads every target's deadline back off disk, which is
// what every stored claim here does: the question is what the bytes say after
// the other machine finished, and an in-memory task would answer with what this
// process calculated regardless.
func helperStoredDeadlines(
	t *testing.T, env helperEnvironment, targets []app.TaskLocator,
) map[app.TaskLocator]string {
	t.Helper()
	deadlines := make(map[app.TaskLocator]string, len(targets))
	for _, locator := range targets {
		deadlines[locator] = helperDeadlineText(helperStoredTask(t, env, locator))
	}
	return deadlines
}

// helperDeadlineText and helperDeadlineValue are one spelling of a deadline for
// a stored task and for a reported instant. Both sides of every comparison go
// through one of them, so "unchanged" is a claim about the value a reader would
// see rather than about how the two were obtained.
func helperDeadlineText(task *models.Task) string {
	if task == nil {
		return ""
	}
	return helperDeadlineValue(task.DueDate)
}

func helperDeadlineValue(due *time.Time) string {
	if due == nil {
		return ""
	}
	return due.Format(helperDeadlineLayout)
}

// helperPlanningBytes reads the synced planning artifact verbatim. A deadline
// step that lands on today joins Today, so planning.json is genuinely part of
// what a refused batch must not have written; comparing the bytes rather than
// the parsed state is what makes the claim independent of what the domain
// happens to consider significant.
func helperPlanningBytes(t *testing.T, env helperEnvironment) []byte {
	t.Helper()
	raw, err := os.ReadFile(config.PlanningStatePath(env.dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read planning.json: %v", err)
	}
	return raw
}

// helperNewestUndoAction names the action the workspace ledger's newest undo
// entry records, or "" when there is none. A mutation refused by a precondition
// records nothing, so the entry that was newest before it ran must still be
// newest after.
func helperNewestUndoAction(t *testing.T, env helperEnvironment) string {
	t.Helper()
	entry, ok, err := workspaceLedgerFromConfig(env.workspace.Runtime).SelectNewestUndo(storage.GlobalScope())
	if err != nil {
		t.Fatalf("read the workspace ledger: %v", err)
	}
	if !ok {
		return ""
	}
	return entry.Action
}
