package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

const (
	overlayKeptComment      = "First remark"
	overlayDoomedComment    = "Second remark"
	overlayAddedComment     = "Added after the overlay"
	overlayReplacementDraft = "Typed into the replacement panel"
)

// commentOverlayOwner is one of the two entities the shared Comments panel
// commits for. They are covered together because the defect is the shell's
// dispatch rather than either adapter's: the outcome is correlated by owner key
// and token no matter which owner minted it, and both correlations were being
// delivered to Help.
type commentOverlayOwner struct {
	// name is the subtest prefix and the owner kind in every failure line.
	name string
	// open seats the Outliner cursor on the entity and mounts the panel over it.
	open func(t *testing.T, driver *UITestDriver, press func(string))
	// stored reads the comments the canonical writer persisted for that entity,
	// in canonical storage order. What never reached tasks.md is not a comment.
	stored func(t *testing.T, workspace *uitestWorkspace) []string
	// breakTheTarget makes the frozen delete uncommittable, from outside this
	// shell. A refusal the panel can see coming is refused before submission and
	// never opens the window this file is about, so the perturbation has to be one
	// only the application layer's fresh snapshot can notice.
	breakTheTarget func(t *testing.T, driver *UITestDriver, workspace *uitestWorkspace)
	// refusal is a fragment of the application layer's own sentence.
	refusal string
}

// projectCommentOverlayOwner is the identity-addressed owner. Its mutations
// carry the comment's id and the text the panel last read, so rewriting that
// text from another surface is exactly what the precondition exists to catch.
func projectCommentOverlayOwner() commentOverlayOwner {
	return commentOverlayOwner{
		name: "project",
		open: func(t *testing.T, driver *UITestDriver, press func(string)) {
			t.Helper()
			seatCursorOnProjectHeader(t, driver.App().crossOutliner)
			press(";")
		},
		stored: func(t *testing.T, workspace *uitestWorkspace) []string {
			t.Helper()
			return projectCommentTextsOnDisk(t, workspace, projectCommentFixtureSlug)
		},
		breakTheTarget: func(t *testing.T, driver *UITestDriver, workspace *uitestWorkspace) {
			t.Helper()
			owner := driver.App().projectByID(projectCommentFixtureSlug)
			if owner == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			stored := projectCommentsOnDisk(t, workspace)
			if len(stored) == 0 {
				t.Fatal("the fixture project carries no comment to rewrite")
			}
			doomed := stored[len(stored)-1]
			target := app.ProjectCommentTarget{
				ProjectID: owner.ID,
				CommentID: doomed.ID,
				Original:  doomed.Text,
			}
			if _, err := externalProjectCommentWriter(t, workspace).
				EditProjectComment(target, "Rewritten by another surface"); err != nil {
				t.Fatalf("the external writer could not rewrite the target: %v", err)
			}
		},
		refusal: "comment changed since it was read",
	}
}

// taskCommentOverlayOwner is the positional one. A task comment carries no
// identity, so the only precondition the application layer can enforce is the
// index, and removing the comment above the frozen one is what puts it out of
// range.
func taskCommentOverlayOwner() commentOverlayOwner {
	return commentOverlayOwner{
		name: "task",
		open: func(t *testing.T, driver *UITestDriver, press func(string)) {
			t.Helper()
			seatCursorOnTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
			press(";")
		},
		stored: func(t *testing.T, workspace *uitestWorkspace) []string {
			t.Helper()
			return taskCommentTextsOnDisk(t, workspace, checklistFixturePlainTitle)
		},
		breakTheTarget: func(t *testing.T, driver *UITestDriver, workspace *uitestWorkspace) {
			t.Helper()
			project := driver.App().projectByID(projectCommentFixtureSlug)
			if project == nil {
				t.Fatal("the fixture project is not in the shell cache")
			}
			task := projectedTaskTitled(t, driver.App().crossOutliner, checklistFixturePlainTitle)
			locator := app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}
			if _, err := externalProjectCommentWriter(t, workspace).RemoveComment(locator, 0); err != nil {
				t.Fatalf("the external writer could not remove the neighbor: %v", err)
			}
		},
		refusal: "comment index 1 out of range",
	}
}

// confirmDeleteAtCursor walks Delete's only route on this surface — the Space
// sheet, which confirms first — and leaves the confirmation open over the row
// under the cursor.
func confirmDeleteAtCursor(t *testing.T, driver *UITestDriver, press func(string), panel *taskdetail.CommentsModel) {
	t.Helper()
	press("space")
	press("enter")
	if !panel.Confirm.IsActive() || panel.Confirm.Action() != "delete" {
		t.Fatalf("the sheet's Delete row raised confirm %q, want the delete prompt:\n%s",
			panel.Confirm.Action(), driver.Render())
	}
	if err := driver.SendKey("y"); err != nil {
		t.Fatalf("SendKey(y): %v", err)
	}
}

// holdCommentOutcome runs everything the confirmed delete queued except the one
// message that settles the panel, and returns that message instead.
//
// The transaction itself runs here — that is the point. What is held back is the
// delivery, which is the interval the user's `?` lands in, and the only way to
// observe that interval from a driver whose drain is otherwise instantaneous.
func holdCommentOutcome(t *testing.T, driver *UITestDriver) tea.Msg {
	t.Helper()
	queue := []tea.Cmd{driver.pendingCmd}
	driver.pendingCmd = nil

	var held tea.Msg
	steps := 0
	for len(queue) > 0 && steps < maxDrainSteps {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil || isAnimationCmd(cmd) {
			continue
		}
		steps++

		var msg tea.Msg
		select {
		case msg = <-runCmd(cmd):
		case <-time.After(defaultDrainTimeout):
			t.Fatal("a command the confirmed delete queued never finished")
		}
		if isCommentOutcome(msg) {
			if held != nil {
				t.Fatalf("the delete produced a second comment outcome: %T", msg)
			}
			held = msg
			continue
		}
		queue = append(queue, driver.handleMsg(msg)...)
	}
	if held == nil {
		t.Fatal("the confirmed delete produced no comment outcome to hold")
	}
	return held
}

// isCommentOutcome names the four correlated messages a comment commit resolves
// to. They are the vocabulary this file holds back; everything else the delete
// queued is ordinary shell work and runs when it is produced.
func isCommentOutcome(msg tea.Msg) bool {
	switch msg.(type) {
	case taskdetail.CommentAddedMsg, taskdetail.CommentDeletedMsg,
		taskdetail.CommentEditedMsg, taskdetail.CommentMutationFailedMsg:
		return true
	}
	return false
}

// deliverHeldOutcome hands the held message to the shell the way the runtime
// would have, and settles whatever it queues.
func deliverHeldOutcome(driver *UITestDriver, held tea.Msg) {
	driver.dispatch(held)
	driver.drain()
	driver.Wait(defaultWaitTimeout)
}

// commentsPanelUnderTheRoute is the Comments model itself, covered or not.
// ActiveCommentsPanel deliberately answers only when Comments is the current
// view, and the whole subject here is the window where Help is.
func commentsPanelUnderTheRoute(t *testing.T, driver *UITestDriver) *taskdetail.CommentsModel {
	t.Helper()
	panel, _ := driver.App().viewNav.views.Get(presentation.ViewTaskComments).(*taskdetail.CommentsModel)
	if panel == nil {
		t.Fatalf("no Comments model is registered; view = %s", driver.App().currentView())
	}
	return panel
}

// commentOverlayKeys presses and types through the production driver, draining
// each message the way the live program does.
func commentOverlayKeys(t *testing.T, driver *UITestDriver) (press func(string), typeText func(string)) {
	t.Helper()
	press = func(key string) {
		t.Helper()
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("SendKey(%q): %v", key, err)
		}
		driver.drain()
	}
	typeText = func(text string) {
		t.Helper()
		for _, r := range text {
			driver.dispatch(tea.KeyPressMsg{Code: r, Text: string(r)})
			driver.drain()
		}
	}
	return press, typeText
}

// collapseWhitespace folds the wrapping the panel applies to a long refusal, so
// a fragment assertion reads the sentence rather than the column it was fitted
// to.
func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// taskCommentTextsOnDisk reads one task's stored comments out of the canonical
// document, which is the only place a comment counts as written.
func taskCommentTextsOnDisk(t *testing.T, workspace *uitestWorkspace, title string) []string {
	t.Helper()
	path := filepath.Join(workspace.DataDir, "projects", projectCommentFixtureSlug, "tasks.md")
	content, err := os.ReadFile(path) //nolint:gosec // The path is this test's own workspace.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	for _, task := range document.Tasks {
		if task == nil || task.Title != title {
			continue
		}
		texts := make([]string, 0, len(task.Comments))
		for _, comment := range task.Comments {
			if comment != nil {
				texts = append(texts, comment.Text)
			}
		}
		return texts
	}
	t.Fatalf("no task titled %q in %s", title, path)
	return nil
}
