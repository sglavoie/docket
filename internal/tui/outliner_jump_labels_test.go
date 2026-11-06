package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/route"
)

// jumpLabelDriver is one project of n Doing tasks titled "Task 01".., so the
// window holds n+2 rows: the lane header, the project header, and the tasks.
func jumpLabelDriver(t *testing.T, n, width, height int) *UITestDriver {
	t.Helper()
	app := newUITestApp()
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	for i := 1; i <= n; i++ {
		project.Tasks = append(project.Tasks, &models.Task{
			ID: fmt.Sprintf("t%02d", i), ShortID: i, Title: fmt.Sprintf("Task %02d", i), WorkflowState: models.StateDoing,
		})
	}
	project.BuildTaskTree()
	setTestAppProjects(app, project)
	app.crossOutliner.Population.Projects = []*models.Project{project}
	app.crossOutliner.Population.Invalidate()
	app.crossOutliner.RebuildRows()
	return NewUITestDriver(app, width, height)
}

// jumpLabelOn reads the label painted at the start of the screen line holding
// text: the run of lowercase letters it begins with.
func jumpLabelOn(t *testing.T, d *UITestDriver, text string) string {
	t.Helper()
	for _, line := range strings.Split(ansi.Strip(d.Render()), "\n") {
		// The Now card names a Doing task above the list and carries no label.
		if !strings.Contains(line, text) || strings.HasPrefix(line, chrome.NowCardRail) {
			continue
		}
		end := 0
		for end < len(line) && end < 2 && line[end] >= 'a' && line[end] <= 'z' {
			end++
		}
		if end == 0 {
			t.Fatalf("line %q carries no jump label", line)
		}
		return line[:end]
	}
	t.Fatalf("no screen line holds %q", text)
	return ""
}

func cursorTitle(d *UITestDriver) string {
	out := d.App().crossOutliner
	if out.Cursor < 0 || out.Cursor >= len(out.Rows) || out.Rows[out.Cursor].Task == nil {
		return ""
	}
	return out.Rows[out.Cursor].Task.Title
}

// TestJumpLabelsMoveTheCursorAloneAndCtrlOReturns holds the jump to its one
// write: the cursor moves to the labeled row, the window, the rows, and the
// selection stay as they were, and Ctrl+O comes back to the origin.
func TestJumpLabelsMoveTheCursorAloneAndCtrlOReturns(t *testing.T) {
	d := jumpLabelDriver(t, 6, 80, 30)
	out := d.App().crossOutliner
	sendKeys(t, d, "j", "j")
	origin := out.Cursor
	originTitle := cursorTitle(d)
	rows, offset := len(out.Rows), out.ViewScrollOffset

	sendKeys(t, d, "'")
	if !out.Jump.Active {
		t.Fatal("' did not open jump mode")
	}
	label := jumpLabelOn(t, d, "Task 05")
	if len(label) != 1 {
		t.Fatalf("label %q: eight rows fit one letter each", label)
	}
	if guidance, ok := out.ContextualGuidance(); !ok || !strings.Contains(guidanceHintLabels(guidance), "Esc cancel") {
		t.Fatalf("jump mode footer = %+v, want its own guidance", guidance)
	}

	sendKeys(t, d, label)
	if out.Jump.Active {
		t.Fatal("a whole label left jump mode open")
	}
	if got := cursorTitle(d); got != "Task 05" {
		t.Fatalf("cursor on %q, want Task 05", got)
	}
	if len(out.Rows) != rows || out.ViewScrollOffset != offset || out.SelectionCounts().Total != 0 {
		t.Fatal("the jump wrote something besides the cursor")
	}

	sendKeys(t, d, "ctrl+o")
	if out.Cursor != origin || cursorTitle(d) != originTitle {
		t.Fatalf("Ctrl+O returned to row %d (%q), want the origin %d (%q)", out.Cursor, cursorTitle(d), origin, originTitle)
	}
}

// TestJumpLabelsReachHeadersAndChecklistRows covers the two row kinds that are
// not tasks: a project header and an expanded checklist item.
func TestJumpLabelsReachHeadersAndChecklistRows(t *testing.T) {
	d, ws, err := NewFixtureDriver(FixtureChecklist, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	out := d.App().crossOutliner
	// Open the Backlog lane and the runbook's checklist under it.
	sendKeys(t, d, "l", "j", "j", "l")

	sendKeys(t, d, "'")
	sendKeys(t, d, jumpLabelOn(t, d, "Name the on-call owner"))
	if row := out.Rows[out.Cursor]; !row.IsChecklistItem || row.Task.Checklist[row.ChecklistIndex].Text != "Name the on-call owner" {
		t.Fatalf("cursor row = %+v, want the checklist item", row)
	}

	sendKeys(t, d, "'")
	sendKeys(t, d, jumpLabelOn(t, d, "[checklist-delivery]"))
	if !out.Rows[out.Cursor].IsProjectHeader {
		t.Fatalf("cursor row = %+v, want the project header", out.Rows[out.Cursor])
	}
}

// TestJumpLabelsTwoLettersNarrowAndBackspaceUndoes runs a window too tall for
// one letter each: every label is two letters, Backspace takes the first back,
// and the pair still lands.
func TestJumpLabelsTwoLettersNarrowAndBackspaceUndoes(t *testing.T) {
	d := jumpLabelDriver(t, 40, 80, 40)
	out := d.App().crossOutliner
	sendKeys(t, d, "'")
	for _, title := range []string{"Task 01", "Task 20", "Task 30"} {
		if label := jumpLabelOn(t, d, title); len(label) != 2 {
			t.Fatalf("label for %s = %q, want two letters for every row", title, label)
		}
	}
	label := jumpLabelOn(t, d, "Task 30")

	sendKeys(t, d, label[:1])
	if !out.Jump.Active || out.Jump.Prefix != label[:1] {
		t.Fatalf("first letter: state %+v", out.Jump)
	}
	if guidance, _ := out.ContextualGuidance(); !strings.Contains(guidanceHintLabels(guidance), "Backspace undo") {
		t.Fatal("the footer does not offer Backspace once a letter is typed")
	}
	sendKeys(t, d, "backspace")
	if !out.Jump.Active || out.Jump.Prefix != "" {
		t.Fatalf("Backspace: state %+v, want the mode open with nothing typed", out.Jump)
	}
	sendKeys(t, d, label[:1], label[1:])
	if got := cursorTitle(d); got != "Task 30" {
		t.Fatalf("cursor on %q, want Task 30", got)
	}
}

// TestJumpLabelsCancelWithoutSideEffects holds every way out to moving nothing
// and running nothing: Esc, a key the root grammar binds, a label letter that
// names no row, and a click.
func TestJumpLabelsCancelWithoutSideEffects(t *testing.T) {
	d := jumpLabelDriver(t, 4, 80, 30)
	out := d.App().crossOutliner
	sendKeys(t, d, "j")
	origin := out.Cursor

	for _, key := range []string{"esc", "/", "ctrl+a", "Q", "z"} {
		sendKeys(t, d, "'")
		sendKeys(t, d, key)
		if out.Jump.Active || out.Cursor != origin {
			t.Fatalf("key %q: jump active %v, cursor %d, want a cancel at %d", key, out.Jump.Active, out.Cursor, origin)
		}
		if out.IsSearchMode() || out.SelectionCounts().Total != 0 || out.QueryReadout.Expanded {
			t.Fatalf("key %q fell through to the grammar underneath", key)
		}
	}

	sendKeys(t, d, "'")
	d.App().crossOutliner.HandlePointer(route.PointerEvent{X: 5, Y: 5, Button: route.PointerButtonLeft, Clicks: 1})
	if out.Jump.Active || out.Cursor != origin {
		t.Fatal("a click did not cancel jump mode in place")
	}
}

// TestJumpLabelsExtendARangeSelection decides the visual case: the jump is a
// cursor move, so an anchored V range follows it exactly as j would.
func TestJumpLabelsExtendARangeSelection(t *testing.T) {
	d := jumpLabelDriver(t, 6, 80, 30)
	out := d.App().crossOutliner
	sendKeys(t, d, "j", "j", "V")
	sendKeys(t, d, "'")
	sendKeys(t, d, jumpLabelOn(t, d, "Task 04"))
	if got := cursorTitle(d); got != "Task 04" {
		t.Fatalf("cursor on %q, want Task 04", got)
	}
	if got := out.SelectionCounts().Total; got != 4 {
		t.Fatalf("selection = %d, want the range Task 01..Task 04", got)
	}
}

// TestJumpLabelsKeepEveryRowExactlyAsWide paints the labels at the floor width
// with the Peek dock open: every screen line is still the terminal's width.
func TestJumpLabelsKeepEveryRowExactlyAsWide(t *testing.T) {
	d := jumpLabelDriver(t, 12, chrome.MinTerminalWidth, chrome.MinTerminalHeight)
	sendKeys(t, d, "j", "j", "b", "'")
	if !d.App().crossOutliner.Jump.Active {
		t.Fatal("jump mode did not open over the Peek dock")
	}
	for i, line := range strings.Split(d.Render(), "\n") {
		if got := lipgloss.Width(line); got > chrome.MinTerminalWidth {
			t.Errorf("line %d is %d cells wide, over %d: %q", i, got, chrome.MinTerminalWidth, ansi.Strip(line))
		}
	}
	obs := d.Observe(ObserveOptions{})
	if len(obs.LayoutIssues) > 0 {
		t.Fatalf("layout issues in jump mode: %+v", obs.LayoutIssues)
	}
}

// TestApostropheStaysTextInsideInputs keeps ' a character wherever a text input
// owns the key: the search query and the capture draft.
func TestApostropheStaysTextInsideInputs(t *testing.T) {
	d := jumpLabelDriver(t, 3, 80, 30)
	out := d.App().crossOutliner
	sendKeys(t, d, "/", "'")
	if out.Jump.Active || out.SearchInputValue() != "'" {
		t.Fatalf("search: jump %v, query %q, want the apostrophe typed", out.Jump.Active, out.SearchInputValue())
	}

	// Capture needs a real workspace behind it, so it runs over a fixture.
	fixture, ws, err := NewFixtureDriver(FixtureRich, 80, 30)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })
	sendKeys(t, fixture, "a", "'")
	if fixture.App().crossOutliner.Jump.Active {
		t.Fatal("capture: ' opened jump mode under the composer")
	}
	if !strings.Contains(ansi.Strip(fixture.Render()), "'") {
		t.Fatal("capture: the apostrophe never reached the draft")
	}
}
