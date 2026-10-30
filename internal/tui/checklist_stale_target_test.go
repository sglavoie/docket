package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/storage"
)

func TestChecklistEditRefusesAShiftedItemAndRetainsTheDraft(t *testing.T) {
	for _, reload := range []bool{false, true} {
		name := "without reload"
		if reload {
			name = "with reload"
		}
		t.Run(name, func(t *testing.T) {
			driver, workspace, view := checklistFixtureApp(t)
			expandChecklistOf(t, view, checklistFixtureFoldTitle)
			seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "e"},
				{Type: UITestActionText, Text: " my edit"},
			})
			prompt := driver.App().operations.TextPrompt
			if prompt == nil {
				t.Fatal("edit did not open a prompt")
			}
			draft := prompt.Value()
			owner, projectID := workspaceTaskTitled(t, driver.App(), checklistFixtureFoldTitle)
			writer := externalProjectCommentWriter(t, workspace)
			if _, err := writer.RemoveChecklistItem(app.TaskLocator{ProjectID: projectID, TaskID: owner.ID}, 0, owner.Checklist); err != nil {
				t.Fatal(err)
			}
			if reload {
				refreshUnderThePanel(t, driver)
			}
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "enter"},
				{Type: UITestActionWait},
			})
			// Read independently: a refused edit need not reload the covered Outliner.
			data, err := os.ReadFile(filepath.Join(checklistFixtureProjectDirectory(workspace), "tasks.md"))
			if err != nil {
				t.Fatal(err)
			}
			document, err := storage.ParseProjectDocument(string(data))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, task := range document.Tasks {
				if task.ID != owner.ID {
					continue
				}
				for _, item := range task.Checklist {
					got = append(got, item.Text)
				}
			}
			if want := []string{checklistFixtureSecondItem, checklistFixtureThirdItem}; !slices.Equal(got, want) {
				t.Fatalf("a stale edit overwrote another item: got %q, want %q", got, want)
			}
			prompt = driver.App().operations.TextPrompt
			if prompt == nil || prompt.Value() != draft || prompt.Notice() == "" {
				t.Fatal("the refused edit did not retain its draft and refusal reason")
			}
		})
	}
}

func TestChecklistEditRetriesAfterAWriteFailure(t *testing.T) {
	driver, workspace, view := checklistFixtureApp(t)
	expandChecklistOf(t, view, checklistFixtureFoldTitle)
	seatCursorOnChecklistItem(t, view, checklistFixtureSecondItem)
	unseal := sealProjectDirectory(t, checklistFixtureProjectDirectory(workspace))
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: " my edit"},
	})
	draft := driver.App().operations.TextPrompt.Value()
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	if prompt := driver.App().operations.TextPrompt; prompt == nil || prompt.Value() != draft {
		t.Fatal("a failed edit discarded its draft")
	}
	unseal()
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionWait},
	})
	if driver.App().operations.TextPrompt != nil {
		t.Fatal("a successful retry left the prompt open")
	}
	view = driver.App().crossOutliner
	want := []string{checklistFixtureFirstItem, draft, checklistFixtureThirdItem}
	if got := checklistItemTexts(t, view, checklistFixtureFoldTitle); !slices.Equal(got, want) {
		t.Fatalf("retry wrote %q, want %q", got, want)
	}
	if got := cursorTaskTitle(t, view); got != "checklist item "+draft {
		t.Fatalf("successful edit moved the cursor to %q", got)
	}
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "u"},
		{Type: UITestActionWait},
	})
	want[1] = checklistFixtureSecondItem
	if got := checklistItemTexts(t, driver.App().crossOutliner, checklistFixtureFoldTitle); !slices.Equal(got, want) {
		t.Fatalf("one undo wrote %q, want %q", got, want)
	}
}
