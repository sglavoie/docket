package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	apppkg "github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/testutil"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/taskform"
)

func TestHandleViewMessagesShowTaskComposer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	msg := app.captureComposerMessage(project, app.taskWorkspaceProjects())
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowTaskComposerMsg should be handled")
	}
	if app.currentView() != presentation.ViewTaskComposer {
		t.Errorf("currentView should be TaskComposer after ShowTaskComposerMsg, got %v", app.currentView())
	}
	capture, ok := app.viewNav.views.Get(presentation.ViewTaskComposer).(*taskform.TaskComposerModel)
	if !ok {
		t.Fatalf("composer view = %T, want *TaskComposerModel", app.viewNav.views.Get(presentation.ViewTaskComposer))
	}
	if capture.Preset.Project != project {
		t.Fatalf("quick capture project = %v, want contextual project %v (system Inbox was %v)", capture.Preset.Project, project, inbox)
	}
	if !capture.Preset.AllowProjectSwitch {
		t.Fatal("contextual quick capture unexpectedly disabled project switching")
	}
	if len(capture.Preset.AllProjects) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.Preset.AllProjects))
	}
}

func TestTaskComposerCaptureDefaultsToSystemUnfiled(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	handled, _ := app.handleViewMessages(app.captureComposerMessage(nil, nil))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	capture, ok := app.viewNav.views.Get(presentation.ViewTaskComposer).(*taskform.TaskComposerModel)
	if !ok {
		t.Fatalf("composer view = %T, want *TaskComposerModel", app.viewNav.views.Get(presentation.ViewTaskComposer))
	}
	if capture.Preset.Project != inbox {
		t.Fatalf("quick capture project = %v, want system Inbox %v", capture.Preset.Project, inbox)
	}
	if !capture.Preset.AllowProjectSwitch {
		t.Fatal("context-free quick capture unexpectedly disabled project switching")
	}
	if len(capture.Preset.AllProjects) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.Preset.AllProjects))
	}
}

func installTestSystemInbox(app *AppModel, projects ...*models.Project) *models.Project {
	inbox := testutil.NewTestProjectWithTasks("Unfiled")
	setTestAppProjects(app, append(projects, inbox)...)
	catalog := app.domain.runtime.Catalog()
	catalog.SystemUnfiledProjectID = inbox.ID
	catalog.SystemUnfiledProjectID = inbox.ID
	app.domain.runtime.SetCatalogInMemory(catalog)
	return inbox
}

func TestHandleViewMessagesTextEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(presentation.ViewTextEditor)

	msg := chrome.TextEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TextEditorDoneMsg should be handled")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after text editor done, got %v", app.currentView())
	}
}

// E opens the description editor named for its task: the header carries the
// task title, and the header and the breadcrumb label are the same word.
func TestHandleShowTaskTextEditorNamesTask(t *testing.T) {
	app := newTestAppModel()
	project := testutil.NewTestProjectWithTasks("Work")
	task := project.Tasks[0]

	handled, _ := app.handleEditorMessages(chrome.ShowTaskTextEditorMsg{
		Task:      task,
		Project:   project,
		Operation: apppkg.TaskOperationEditDescription,
	})
	if !handled || app.currentView() != presentation.ViewTextEditor {
		t.Fatalf("handled=%v view=%v, want the text editor", handled, app.currentView())
	}
	editor, ok := app.activeView().(*chrome.TextEditorModel)
	if !ok {
		t.Fatalf("active view = %T, want *chrome.TextEditorModel", app.activeView())
	}
	editor.SetSize(110, 30)
	header := strings.SplitN(ansi.Strip(editor.View().Content), "\n", 2)[0]
	if !strings.Contains(header, "Edit Description") || !strings.Contains(header, task.Title) {
		t.Fatalf("header = %q, want Edit Description naming %q", header, task.Title)
	}
	if got := editor.StatusBarLabel(); got != "Edit Description" {
		t.Fatalf("StatusBarLabel() = %q, want Edit Description", got)
	}
}

// The three contracts below are the panel's side of the host budget: what the
// outline underneath capture keeps while the panel is open, what it gets back
// when the panel closes, and what a host that never asked for any of it still
// gets. The window itself stays ComputeOutlinerViewport's — nothing here asserts
// an offset this file computed, only one the render settled on.

// captureHostBudgetDriver opens the release catalog scrolled off its last row,
// which is the one reviewed fixture with enough rows for a window to be anywhere
// but the top.
func captureHostBudgetDriver(t *testing.T) (*UITestDriver, *outliner.OutlinerModel) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRelease, 100, 35)
	if err != nil {
		t.Fatalf("release fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	view := driver.App().crossOutliner
	if view == nil {
		t.Fatal("the release fixture app has no Outliner")
	}
	uxClarityPress(t, driver, uxClarityKeys("G")...)
	for range uxDailyUseScrollLift {
		uxClarityPress(t, driver, uxClarityKeys("k")...)
	}
	driver.Observe(ObserveOptions{})
	if view.ViewScrollOffset == 0 {
		t.Fatalf("the window is still at the top after %d rows, so there is no position to keep", len(view.Rows))
	}
	return driver, view
}

// TestCapturePanelRoundTripKeepsItsHostsWindow walks the panel through every
// height it has — opening, the syntax guide, and closing — against the host the
// presentation path actually resolved, rather than against a view this test
// resized itself. The outline keeps the row its cursor is on throughout, and the
// close gives back the window the open borrowed from.
func TestCapturePanelRoundTripKeepsItsHostsWindow(t *testing.T) {
	driver, view := captureHostBudgetDriver(t)
	cursor, before := view.Cursor, view.ViewScrollOffset

	uxClarityPress(t, driver, uxClarityKeys("a")...)
	driver.Observe(ObserveOptions{})
	frame, hosted := driver.App().hostedPanelFrame()
	if !hosted || frame.host != presentation.ViewCrossProjectOutliner {
		t.Fatalf("capture panel host = %v (hosted=%v), want the outline it was raised from", frame.host, hosted)
	}
	start, end := view.VisibleRowWindow()
	if view.Cursor < start || view.Cursor >= end {
		t.Fatalf("the shortened window [%d,%d) does not draw the cursor on row %d", start, end, view.Cursor)
	}

	// The syntax guide hides the host rather than shrinking it, so the loan is
	// unchanged and neither opening nor closing it is a height change at all.
	held := view.ViewScrollOffset
	uxClarityPress(t, driver, uxClarityKeys("?")...)
	driver.Observe(ObserveOptions{})
	uxClarityPress(t, driver, uxClarityKeys("?")...)
	driver.Observe(ObserveOptions{})
	if view.ViewScrollOffset != held {
		t.Fatalf("the syntax guide moved the host's window from %d to %d", held, view.ViewScrollOffset)
	}

	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	driver.Observe(ObserveOptions{})
	if _, stillHosted := driver.App().hostedPanelFrame(); stillHosted {
		t.Fatal("the panel is still hosted after Esc")
	}
	if view.Cursor != cursor {
		t.Fatalf("the round trip moved the cursor from row %d to row %d", cursor, view.Cursor)
	}
	if view.ViewScrollOffset != before {
		t.Fatalf("the round trip came back to offset %d, want the %d the panel opened over",
			view.ViewScrollOffset, before)
	}
}

// TestCaptureExpansionReturnsItsHostsWindow is the other height the panel has.
// Ctrl+F gives the whole content area to the expanded form, which is the panel
// declining to be a panel, so the loan is repaid there and the outline standing
// behind the form is the one the user left.
func TestCaptureExpansionReturnsItsHostsWindow(t *testing.T) {
	driver, view := captureHostBudgetDriver(t)
	before := view.ViewScrollOffset

	uxClarityPress(t, driver, uxClarityKeys("a")...)
	driver.Observe(ObserveOptions{})
	uxClarityPress(t, driver, uxClarityKeys("ctrl+f")...)
	driver.Observe(ObserveOptions{})

	if _, hosted := driver.App().hostedPanelFrame(); hosted {
		t.Fatal("the expanded form is still a hosted panel, so no budget was released")
	}
	if view.ViewScrollOffset != before {
		t.Fatalf("expanding left the host at offset %d, want the %d it lent from", view.ViewScrollOffset, before)
	}

	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	driver.Observe(ObserveOptions{})
	if view.ViewScrollOffset != before {
		t.Fatalf("closing the expanded form left the host at offset %d, want %d", view.ViewScrollOffset, before)
	}
}

// hostBudgetRecorder is a host that never says it keeps a scroll window, which
// is every surface but the outline.
type hostBudgetRecorder struct {
	sizes [][2]int
}

func (h *hostBudgetRecorder) Init() tea.Cmd                       { return nil }
func (h *hostBudgetRecorder) Update(tea.Msg) (tea.Model, tea.Cmd) { return h, nil }
func (h *hostBudgetRecorder) View() tea.View                      { return tea.NewView("") }
func (h *hostBudgetRecorder) SetSize(width, height int) {
	h.sizes = append(h.sizes, [2]int{width, height})
}

var _ route.View = (*hostBudgetRecorder)(nil)

// TestHostedBudgetFallsBackToSetSize states what a panel raised over any other
// surface costs it: nothing new. Both halves of the loan reach a host that
// implements neither as the resize it has always been handed, so adding the seam
// changed no surface that did not ask for it.
func TestHostedBudgetFallsBackToSetSize(t *testing.T) {
	host := &hostBudgetRecorder{}
	applyHostedBudget(host, 80, 12)
	releaseHostedBudget(host, 80, 24)

	want := [][2]int{{80, 12}, {80, 24}}
	if len(host.sizes) != len(want) {
		t.Fatalf("host was sized %v, want %v", host.sizes, want)
	}
	for index, size := range want {
		if host.sizes[index] != size {
			t.Fatalf("host was sized %v, want %v", host.sizes, want)
		}
	}
}

// TestCapturePanelSurvivesATerminalResize is the line between the two acts, seen
// from the shell. A terminal that changed size while the panel was open is
// showing a different amount of the document, so the window it dropped stays
// dropped and the close repays the loan the new size took out. What the outline
// must not do is come back holding a window from a terminal that no longer
// exists, or one its cursor is not in.
func TestCapturePanelSurvivesATerminalResize(t *testing.T) {
	driver, view := captureHostBudgetDriver(t)
	before := view.ViewScrollOffset

	uxClarityPress(t, driver, uxClarityKeys("a")...)
	driver.Observe(ObserveOptions{})
	for _, size := range [][2]int{{100, 24}, {90, 40}, {100, 35}} {
		driver.Resize(size[0], size[1])
		driver.Observe(ObserveOptions{})
	}
	uxClarityPress(t, driver, uxClarityKeys("esc")...)
	driver.Observe(ObserveOptions{})

	if view.ViewScrollOffset == before {
		t.Fatalf("the close put back offset %d, which three terminal resizes had already dropped", before)
	}
	start, end := view.VisibleRowWindow()
	if view.Cursor < start || view.Cursor >= end {
		t.Fatalf("the window [%d,%d) does not draw the cursor on row %d", start, end, view.Cursor)
	}
}
