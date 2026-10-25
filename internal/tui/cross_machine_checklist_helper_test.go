package tui

import (
	"os"
	"slices"
	"strconv"
	"testing"

	"github.com/sglavoie/docket/internal/app"
)

// TestCrossMachineHelperPromotesChecklistItem converts one seeded checklist item
// into a child task and reports what it committed.
//
// The checklist is frozen from the bytes on disk before the barrier goes in,
// which is what makes this role a fair racer: both machines freeze the same list,
// so neither is refused by the frozen-checklist identity check and the refusal
// that does happen comes from the commit boundary rather than from one process
// having read later than the other.
//
// Either outcome makes the same claim about the document, and it is deliberately
// two-sided: the item and the child it becomes are one thing in two shapes, so
// the count of items reading that text plus the count of tasks titled it must be
// exactly one — before, during, and after. A promotion that committed the child
// without consuming the item, consumed the item without committing the child, or
// let two machines convert one item twice each breaks it from a different side.
func TestCrossMachineHelperPromotesChecklistItem(t *testing.T) {
	env := openHelperWorkspace(t)
	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	index := helperChecklistPosition(t, helperChecklistIndexEnv)

	owner := helperStoredTask(t, env, locator)
	if owner == nil {
		t.Fatalf("no task %s/%s on disk to promote from", locator.ProjectID, locator.TaskID)
	}
	if index >= len(owner.Checklist) {
		t.Fatalf("%s=%d names no item in a checklist of %d", helperChecklistIndexEnv, index, len(owner.Checklist))
	}
	frozen := slices.Clone(owner.Checklist)
	item := frozen[index].Text

	release := helperHoldAtCommit(t, helperPromotionUndoAction, locator.ProjectID, locator.TaskID)
	result, err := env.service.PromoteChecklistItem(app.ChecklistPromotionRequest{
		Owner:     locator,
		Index:     index,
		Checklist: frozen,
	})
	release()
	if err != nil {
		helperRequirePromotionIsWhole(t, env, locator, item)
		helperRecordRefusal(t, "PromoteChecklistItem", err)
		return
	}
	helperRequirePromotionIsWhole(t, env, locator, item)
	helperRequireStored(t, "promotion", func() bool {
		child := helperStoredTaskTitled(t, env, locator.ProjectID, item)
		return child != nil && child.ID == result.Child.TaskID
	})
	helperRecord(t, "committed")
}

// helperRequirePromotionIsWhole is the all-or-nothing claim both outcomes make.
// It counts rather than tests presence because presence cannot tell a promotion
// that happened from a promotion that happened twice, and the duplicate is the
// failure two machines converting one item can actually produce.
func helperRequirePromotionIsWhole(
	t *testing.T, env helperEnvironment, locator app.TaskLocator, item string,
) {
	t.Helper()
	project := helperReloadProject(t, env, locator.ProjectID)
	if project == nil {
		helperRecord(t, "lost: the promoted item's project is gone")
		t.Fatalf("no project %s on disk after a promotion", locator.ProjectID)
	}
	items, children := 0, 0
	for _, task := range project.Tasks {
		if task.ID == locator.TaskID {
			for _, entry := range task.Checklist {
				if entry.Text == item {
					items++
				}
			}
		}
		if task.Title == item {
			children++
		}
	}
	if items+children != 1 {
		helperRecord(t, "lost: a promotion left the item in an inconsistent state")
		t.Fatalf("%q reads as %d checklist item(s) and %d task(s); a promotion is one or the other",
			item, items, children)
	}
}

// TestCrossMachineHelperEditsChecklist rewrites or reorders the same checklist a
// promotion converts from, which is the collision the frozen-checklist identity
// exists for: an index alone would have moved onto a different item.
//
// It is the promotion's counterparty rather than a role of its own interest. The
// two calls it makes commit under the ordinary merge-capable revision, not under
// a strict precondition, so pairing it with a promotion is what puts one of each
// kind of guard into one race.
func TestCrossMachineHelperEditsChecklist(t *testing.T) {
	env := openHelperWorkspace(t)
	kind := helperEnv(t, helperChecklistEnv)
	locator := helperTaskLocator(t, env, helperEnv(t, helperTitleEnv))
	from := helperChecklistPosition(t, helperChecklistIndexEnv)
	before := helperStoredChecklist(t, env, locator)

	switch kind {
	case helperChecklistEdit:
		text := helperEnv(t, helperTextEnv)
		defer helperHoldAtCommit(t, "Edit Checklist Item", locator.ProjectID, locator.TaskID)()
		if _, err := env.service.EditChecklistItem(locator, from, text); err != nil {
			helperRequireChecklistOrderSurvived(t, env, locator, before)
			helperRecordRefusal(t, "EditChecklistItem", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			return slices.Contains(helperStoredChecklist(t, env, locator), text)
		})
	case helperChecklistReorder:
		to := helperChecklistPosition(t, helperChecklistToEnv)
		if from >= len(before) || to >= len(before) {
			t.Fatalf("reorder %d->%d names no pair in a checklist of %d", from, to, len(before))
		}
		moved := before[from]
		defer helperHoldAtCommit(t, "Move Checklist Item", locator.ProjectID, locator.TaskID)()
		if _, err := env.service.MoveChecklistItem(locator, from, to); err != nil {
			helperRequireChecklistOrderSurvived(t, env, locator, before)
			helperRecordRefusal(t, "MoveChecklistItem", err)
			return
		}
		helperRequireStored(t, kind, func() bool {
			stored := helperStoredChecklist(t, env, locator)
			return to < len(stored) && stored[to] == moved
		})
	default:
		t.Fatalf("%s=%q names no checklist mutation", helperChecklistEnv, kind)
	}
	helperRecord(t, "committed")
}

// helperChecklistPosition reads one required checklist position. A position is
// required rather than defaulted for the reason the step direction is: a role
// invoked without one would address item zero and every claim about which item
// moved would be about the wrong one.
func helperChecklistPosition(t *testing.T, name string) int {
	t.Helper()
	index, err := strconv.Atoi(helperEnv(t, name))
	if err != nil || index < 0 {
		t.Fatalf("%s=%q is not a checklist position", name, os.Getenv(name))
	}
	return index
}

// helperStoredChecklist reads one task's checklist texts back off disk, in
// order, which is the only shape both an edit and a reorder can be judged by.
func helperStoredChecklist(t *testing.T, env helperEnvironment, locator app.TaskLocator) []string {
	t.Helper()
	task := helperStoredTask(t, env, locator)
	if task == nil {
		return nil
	}
	texts := make([]string, 0, len(task.Checklist))
	for _, item := range task.Checklist {
		texts = append(texts, item.Text)
	}
	return texts
}

// helperRequireChecklistOrderSurvived is a refused checklist mutation's own
// claim, and it is deliberately weaker than "the list is unchanged". The machine
// this one raced may have won legitimately, and the promotion it races removes
// an item, so a byte-identical list is not something a loser is entitled to.
//
// What a loser can still prove is that it wrote nothing: the stored list is the
// list it read with zero or more items deleted, in the same relative order. An
// applied edit would introduce text that was never in `before`, and an applied
// reorder would transpose two survivors, so either one breaks the subsequence
// while any number of concurrent removals leaves it intact.
func helperRequireChecklistOrderSurvived(
	t *testing.T, env helperEnvironment, locator app.TaskLocator, before []string,
) {
	t.Helper()
	after := helperStoredChecklist(t, env, locator)
	next := 0
	for _, text := range after {
		for next < len(before) && before[next] != text {
			next++
		}
		if next == len(before) {
			helperRecord(t, "lost: a refused checklist mutation rewrote the list")
			t.Fatalf("checklist = %v after a refusal, want a subsequence of the %v it read", after, before)
		}
		next++
	}
}
