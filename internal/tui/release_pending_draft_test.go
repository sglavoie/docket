package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

// A draft is the one place in the product where unsaved work exists. Everything
// else the release added commits the moment the key lands, so these two tests
// are the whole of what "pending" can mean: a save that was refused, and time
// passing underneath one.

// sealProjectDirectory makes a project's directory unwritable for the rest of the
// test, which is how a refused save is produced without stubbing the repository:
// the transaction fails where a real one would, in the filesystem. It returns the
// unseal so a test can prove the retry succeeds afterwards.
func sealProjectDirectory(t *testing.T, path string) func() {
	t.Helper()
	if err := os.Chmod(path, 0o555); err != nil {
		t.Fatalf("seal %s: %v", path, err)
	}
	unsealed := false
	unseal := func() {
		if unsealed {
			return
		}
		unsealed = true
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatalf("unseal %s: %v", path, err)
		}
	}
	t.Cleanup(unseal)
	return unseal
}

// requireSeededRepeatDraft asserts the repeat draft is still the one D opened.
// It reads the screen rather than a field because the title is seeded as a typed
// value on the expanded form, not into the capture Input: what a reader of this
// test needs to know is that the user can still see their work, and the screen is
// where that is true. The sigils are checked with it, since a draft that was
// rebuilt from the source title rather than restored would parse them away.
func requireSeededRepeatDraft(t *testing.T, env *repeatKeyEnv, when string) {
	t.Helper()
	env.composer(t)
	screen := env.driver.Render()
	for _, want := range []string{"Repeat Task", "#123", "$today", "naïve"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("the draft lost %q %s:\n%s", want, when, screen)
		}
	}
}

// TestRepeatDraftSurvivesARefusedSaveAndRetries is the repeat half of the
// contract the quick comment popup already holds: a refused write leaves the
// draft where it was, with what was typed in it, because the alternative is that
// a full disk silently throws away a task the user just described. The retry
// then has to commit exactly one task — a refusal that half-committed would show
// up here as two.
func TestRepeatDraftSurvivesARefusedSaveAndRetries(t *testing.T) {
	env := newRepeatKeyEnv(t)
	before := env.tasksFileBytes(t)

	unseal := sealProjectDirectory(t, env.project.Path)

	env.act(t, "D")
	env.composer(t)
	env.act(t, "ctrl+s")

	requireSeededRepeatDraft(t, env, "after the refused save")
	if after := env.tasksFileBytes(t); after != before {
		t.Fatal("the refused save still rewrote tasks.md")
	}

	unseal()
	env.act(t, "ctrl+s")

	if view := env.driver.App().currentView(); view != presentation.ViewCrossProjectOutliner {
		t.Fatalf("the retry left the draft open on %s", view)
	}
	fresh := env.reload(t)
	if len(fresh.Tasks) != 3 {
		t.Fatalf("task count = %d, want the source, its child, and one repeat", len(fresh.Tasks))
	}
	if repeated := env.repeatedTask(t, fresh); repeated.Title != repeatKeySourceTitle {
		t.Fatalf("the retried repeat is titled %q, want the seeded title", repeated.Title)
	}
}

// TestPendingDraftsSurviveThePeriodicRefresh advances past the one recurring
// timer while a draft is open. That tick is the app's only unprompted work — it
// re-reads the outstanding-conflict count, recounts the pulse, and wakes expired
// deferrals — and all three touch the very rows a draft was opened over. A tick
// that rebuilt the view out from under the overlay would take the typed text with
// it, and nothing else in the suite would notice, because no key was pressed.
func TestPendingDraftsSurviveThePeriodicRefresh(t *testing.T) {
	t.Run("repeat draft", func(t *testing.T) {
		env := newRepeatKeyEnv(t)
		before := env.tasksFileBytes(t)

		env.act(t, "D")
		requireSeededRepeatDraft(t, env, "when the draft opened")

		env.driver.advanceClock(2 * periodicSaveInterval)
		env.driver.Wait(repeatKeySettleTimeout)

		requireSeededRepeatDraft(t, env, "after the tick")
		if after := env.tasksFileBytes(t); after != before {
			t.Fatal("the tick committed the pending draft")
		}

		env.act(t, "ctrl+s")
		if fresh := env.reload(t); len(fresh.Tasks) != 3 {
			t.Fatalf("task count = %d, want the source, its child, and one repeat", len(fresh.Tasks))
		}
	})

	t.Run("comment draft", func(t *testing.T) {
		driver, _, view := addCommentFixtureApp(t)
		seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)

		const draft = "Typed before the tick, saved after it"
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "C"},
			{Type: UITestActionText, Text: draft},
		})

		driver.advanceClock(2 * periodicSaveInterval)
		driver.Wait(repeatKeySettleTimeout)

		prompt := driver.App().operations.TextPrompt
		if prompt == nil {
			t.Fatalf("the tick closed the comment popup:\n%s", driver.Render())
		}
		if prompt.Value() != draft {
			t.Fatalf("the tick changed the draft to %q, want %q", prompt.Value(), draft)
		}
		if task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle); len(task.Comments) != 0 {
			t.Fatalf("the tick committed the pending comment: %+v", task.Comments)
		}

		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "ctrl+s"},
			{Type: UITestActionWait},
		})
		task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
		if len(task.Comments) != 1 || task.Comments[0].Text != draft {
			t.Fatalf("after the tick comments = %+v, want the one draft that was typed", task.Comments)
		}
	})
}
