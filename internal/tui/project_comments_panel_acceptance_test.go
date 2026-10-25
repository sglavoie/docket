package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/entityops"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

// outlinerCommentsPanelOperations is `;`'s whole family in Outliner declaration
// order, which is also the order the strategy resolves them in.
var outlinerCommentsPanelOperations = []operation.OperationID{
	operation.OperationTaskComments,
	operation.OperationProjectComments,
}

func applicableCommentsPanelOperations(
	t *testing.T, app *AppModel, context operation.CommandContext,
) []operation.OperationID {
	t.Helper()
	var applicable []operation.OperationID
	for _, id := range outlinerCommentsPanelOperations {
		descriptor, ok := app.GetOperationRegistry().Descriptor(id)
		if !ok {
			t.Fatalf("%s is not registered", id)
		}
		if descriptor.Applicable(context) {
			applicable = append(applicable, id)
		}
	}
	return applicable
}

func assertCommentsPanelKeyResolvesTo(t *testing.T, app *AppModel, want operation.OperationID) {
	t.Helper()
	applicable := applicableCommentsPanelOperations(t, app, app.operationCommandContext())
	if want == "" {
		if len(applicable) != 0 {
			t.Fatalf("; applied as %v, want no operation", applicable)
		}
		return
	}
	if len(applicable) != 1 {
		t.Fatalf("; applied as %v, want exactly one operation", applicable)
	}
	if applicable[0] != want {
		t.Fatalf("; resolved to %s, want %s", applicable[0], want)
	}
}

// TestProjectCommentsAppliesOnProjectHeadersAndNowhereElse is the disjointness
// half of `;`. The key is now one key over two descriptors, and nothing but
// their applicability keeps them apart, so every row shape has to admit exactly
// one of them — and the project half must never reach past the cursor for the
// sole scoped project, which in this single-project fixture is the failure a
// leak would take.
func TestProjectCommentsAppliesOnProjectHeadersAndNowhereElse(t *testing.T) {
	driver, _, view := addCommentFixtureApp(t)
	app := driver.App()
	if _, ok := app.GetOperationRegistry().Descriptor(operation.OperationProjectComments); !ok {
		t.Fatal("Project comments is not registered")
	}

	cases := []struct {
		name string
		seat func(t *testing.T)
		want operation.OperationID
	}{
		{
			name: "a project header opens the project's comments",
			seat: func(t *testing.T) { seatCursorOnProjectHeader(t, view) },
			want: operation.OperationProjectComments,
		},
		{
			name: "a task row opens the task's comments",
			seat: func(t *testing.T) { seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle) },
			want: operation.OperationTaskComments,
		},
		{
			name: "a checklist item row opens nothing",
			seat: func(t *testing.T) {
				seatCursorOnTaskTitled(t, view, checklistFixtureFoldTitle)
				view.Folds.SetChecklist(view.TaskIdentityForRow(view.Rows[view.Cursor]), true)
				view.RebuildRows()
				seatCursorOnRow(t, view, "checklist item", func(row outliner.OutlinerRow) bool {
					return row.IsChecklistItem
				})
			},
			want: "",
		},
		{
			name: "a section header falls to the task half",
			seat: func(t *testing.T) {
				seatCursorOnRow(t, view, "section header", func(row outliner.OutlinerRow) bool {
					return row.IsSectionHeader && !row.IsProjectHeader
				})
			},
			want: operation.OperationTaskComments,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.seat(t)
			assertCommentsPanelKeyResolvesTo(t, app, testCase.want)
		})
	}

	seatCursorOnProjectHeader(t, view)
	selected := app.operationCommandContext()
	selected.Selection.Visual = true
	if applicableCommentsPanelOperations(t, app, selected) != nil {
		t.Error("; applied under a visual selection")
	}
}

func panelCommentTexts(panel *taskdetail.CommentsModel) []string {
	entries := panel.Entries()
	texts := make([]string, 0, len(entries))
	for _, entry := range entries {
		texts = append(texts, entry.Text)
	}
	return texts
}

// TestProjectCommentsPanelAddsEditsAndDeletes is the session's acceptance walk:
// `;` on a project header opens the shared panel over that project, and the add,
// edit, and delete controls all commit against the project's own comments —
// newest first, so every one of them addresses a comment the stored collection
// holds last.
func TestProjectCommentsPanelAddsEditsAndDeletes(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)

	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: ";"}})
	if got := driver.App().currentView(); got != presentation.ViewTaskComments {
		t.Fatalf("; on a project header left the route on %s", got)
	}
	panel := mountedProjectCommentsPanel(t, driver.App())
	if !strings.Contains(driver.Render(), checklistFixtureProjectName) {
		t.Fatalf("the mounted panel does not name its project:\n%s", driver.Render())
	}
	if len(panel.Entries()) != 0 {
		t.Fatalf("the fixture project already carries comments: %v", panelCommentTexts(panel))
	}

	for _, text := range []string{"First remark", "Second remark"} {
		runChecklistActions(t, driver, []UITestAction{
			{Type: UITestActionKey, Key: "a"},
			{Type: UITestActionText, Text: text},
			{Type: UITestActionKey, Key: "ctrl+s"},
			{Type: UITestActionWait},
		})
	}

	panel = mountedProjectCommentsPanel(t, driver.App())
	if got := panelCommentTexts(panel); !reflect.DeepEqual(got, []string{"Second remark", "First remark"}) {
		t.Fatalf("panel lists %v, want the newest comment first", got)
	}
	if panel.Cursor != 0 {
		t.Fatalf("cursor = %d, want it on the comment just added", panel.Cursor)
	}
	if stored := projectCommentsOnDisk(t, workspace); len(stored) != 2 || stored[0].Text != "First remark" {
		t.Fatalf("stored comments = %+v, want both in canonical oldest-first order", stored)
	}

	// Editing the top row edits the comment stored last, which is the whole
	// reason the mutation carries an identity rather than a position.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "e"},
		{Type: UITestActionText, Text: ", revised"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 2 || stored[0].Text != "First remark" || stored[1].Text != "Second remark, revised" {
		t.Fatalf("after the edit stored comments = %+v", stored)
	}
	if edited := commentByID(t, stored, stored[1].ID); edited.CreatedAt.IsZero() {
		t.Fatalf("the edited comment lost its instant: %+v", edited)
	}

	// Delete has no direct key here either: the Space sheet is its only route,
	// and it confirms first.
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "space"}})
	sheet := driver.App().operations.ActionSheet
	if sheet == nil {
		t.Fatal("Space did not open the comment action sheet over a project comment")
	}
	if got := sheet.OperationIDs(); !reflect.DeepEqual(got, []operation.OperationID{entityops.OperationCommentDelete}) {
		t.Fatalf("the project comment sheet lists %v, want Delete alone", got)
	}
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "enter"},
		{Type: UITestActionKey, Key: "y"},
		{Type: UITestActionWait},
	})

	remaining := projectCommentsOnDisk(t, workspace)
	if len(remaining) != 1 || remaining[0].Text != "First remark" {
		t.Fatalf("after the delete stored comments = %+v, want the older one alone", remaining)
	}
	if got := panelCommentTexts(mountedProjectCommentsPanel(t, driver.App())); !reflect.DeepEqual(got, []string{"First remark"}) {
		t.Fatalf("the panel still lists %v after the delete", got)
	}

	// Undo takes the removal back through the one workspace ledger every other
	// mutation records to, and the open panel shows it again.
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "u"},
		{Type: UITestActionWait},
	})
	if got := len(projectCommentsOnDisk(t, workspace)); got != 2 {
		t.Fatalf("after undo stored comments = %d, want both back", got)
	}
	if got := panelCommentTexts(mountedProjectCommentsPanel(t, driver.App())); len(got) != 2 {
		t.Fatalf("after undo the panel lists %v, want both back", got)
	}
}

func commentByID(t *testing.T, comments []*models.ProjectComment, id string) *models.ProjectComment {
	t.Helper()
	for _, comment := range comments {
		if comment != nil && comment.ID == id {
			return comment
		}
	}
	t.Fatalf("no stored comment carries id %q", id)
	return nil
}

// TestProjectCommentsPanelSurvivesARenameAndAReplacement holds the reload
// contract the adapter exists for: the panel resolves its project by identity on
// every reload, so a rename that replaces the shell's project object leaves the
// panel reading the live one rather than a discarded copy.
func TestProjectCommentsPanelSurvivesARenameAndAReplacement(t *testing.T) {
	driver, workspace, view := addCommentFixtureApp(t)
	seatCursorOnProjectHeader(t, view)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: "Before the rename"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})

	owner := driver.App().projectByID(projectCommentFixtureSlug)
	if owner == nil {
		t.Fatal("the fixture project is not in the shell cache")
	}
	owner.Name = "Renamed Under The Panel"
	if !mountedProjectCommentsPanel(t, driver.App()).RefreshAfterReload() {
		t.Fatal("the panel lost its owner across a reload")
	}
	if !strings.Contains(driver.Render(), "Renamed Under The Panel") {
		t.Fatalf("the panel still names the old project:\n%s", driver.Render())
	}

	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: "a"},
		{Type: UITestActionText, Text: "After the rename"},
		{Type: UITestActionKey, Key: "ctrl+s"},
		{Type: UITestActionWait},
	})
	stored := projectCommentsOnDisk(t, workspace)
	if len(stored) != 2 || stored[1].Text != "After the rename" {
		t.Fatalf("stored comments = %+v, want both written to the same project", stored)
	}
}
