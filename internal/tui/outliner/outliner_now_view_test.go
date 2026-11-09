package outliner

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
)

// gd opens the Now screen on the work in Doing; it replaces the list with that
// one task at the full frame and leaves the rows untouched underneath.
func TestNowViewOpensOnTheDoingLeadAndFillsTheFrame(t *testing.T) {
	now := time.Now()
	lead := doingTask("lead", "Fix customer migration", now.Add(-12*time.Minute))
	lead.ResumeNote = "rerun with the expired token"
	lead.Checklist = []models.ChecklistItem{{Text: "write migration", Done: true}, {Text: "run against staging"}, {Text: "ship"}}
	idle := primaryRowsTask("idle", models.StateBacklog)
	m := nowCardOutliner(t, 100, 30, idle, lead)

	if _, refusal := m.OpenNowView(); refusal != "" {
		t.Fatalf("OpenNowView refused: %s", refusal)
	}
	view := m.RenderOutlinerTreeContent(100)
	plain := ansi.Strip(view)
	for _, want := range []string{"NOW", "12m in Doing", "Fix customer migration", "Left off: rerun with the expired token", "1/3", "✓ write migration", "→ run against staging", "○ ship"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Now screen lacks %q:\n%s", want, plain)
		}
	}
	lines := strings.Split(view, "\n")
	if strings.TrimSpace(ansi.Strip(lines[0])) != "" {
		t.Fatalf("the column is not set down from the top:\n%s", plain)
	}
	if got, want := len(lines), m.outlinerHostFrameHeight(100); got != want {
		t.Fatalf("screen is %d rows, want the %d-row frame", got, want)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 100 {
			t.Fatalf("line %d is %d wide, want 100", i, w)
		}
	}
	if strings.Contains(plain, "Backlog") {
		t.Fatalf("the list still draws under the Now screen:\n%s", plain)
	}
}

// The screen's keys run the row operations against its task, seating the
// cursor there first; a key it does not name is swallowed, and Esc closes it.
func TestNowViewKeysRouteToTheTaskAndEscCloses(t *testing.T) {
	now := time.Now()
	lead := doingTask("lead", "Lead", now.Add(-time.Hour))
	idle := primaryRowsTask("idle", models.StateBacklog)
	m := nowCardOutliner(t, 100, 30, idle, lead)
	if _, refusal := m.OpenNowView(); refusal != "" {
		t.Fatal(refusal)
	}
	m.Cursor = 0
	_, cmd := m.HandleKeyMsg(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if cmd == nil {
		t.Fatal("d ran nothing")
	}
	route, ok := cmd().(TriageRouteMsg)
	if !ok || route.OperationID != operation.OperationTaskSetState || route.Value != models.StateDone {
		t.Fatalf("d routed %#v, want Done", route)
	}
	if row := m.Rows[m.Cursor]; row.Task == nil || row.Task.ID != lead.ID {
		t.Fatalf("cursor on %v, want the Now task", row.Task)
	}
	if _, cmd := m.HandleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"}); cmd != nil || !m.NowView.Active {
		t.Fatal("an unnamed key escaped the screen")
	}
	m.HandleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.NowView.Active {
		t.Fatal("Esc left the screen open")
	}
}

// Once the task leaves Doing, the next build closes the screen, and with
// nothing in Doing it refuses to open.
func TestNowViewClosesWhenItsTaskLeavesDoing(t *testing.T) {
	lead := doingTask("lead", "Lead", time.Now().Add(-time.Hour))
	m := nowCardOutliner(t, 100, 30, lead)
	if _, refusal := m.OpenNowView(); refusal != "" {
		t.Fatal(refusal)
	}
	lead.WorkflowState = models.StateDone
	m.RebuildRows()
	if m.NowView.Active {
		t.Fatal("the screen outlived its task leaving Doing")
	}
	if _, refusal := m.OpenNowView(); refusal == "" || m.NowView.Active {
		t.Fatal("the screen opened with nothing in Doing")
	}
}

// A commit that rewrites the task Peek is reading lights the lines it changed,
// and only those; a document change no commit is owed for lights nothing.
func TestPeekFlashLightsTheLinesACommitChanged(t *testing.T) {
	task := primaryRowsTask("task", models.StateBacklog)
	task.Title = "Draft the agenda"
	m := nowCardOutliner(t, 100, 40, task)
	m.SetMotion(true)
	m.Cursor = taskRowIndex(t, m, task.ID)
	m.TogglePeek()
	m.SettleMotion()

	task.Priority = 1
	m.RebuildRows()
	m.SettleMotion()
	if m.peekFlash != nil {
		t.Fatal("a change with no commit owed lit the dock")
	}

	task.Title = "Draft the offsite agenda"
	m.RebuildRows()
	m.NotePeekCommit()
	if cmd := m.SettleMotion(); cmd == nil || m.peekFlash == nil {
		t.Fatal("the committed rename did not light the dock")
	}
	row := m.Rows[m.Cursor]
	_, doc, _ := m.peekCursorDocument()
	lit := 0
	for _, line := range m.paintPeekFlash(row, doc.Lines(0)) {
		if strings.HasPrefix(ansi.Strip(line), "▌") {
			lit++
			if !strings.Contains(ansi.Strip(line), "Draft the offsite agenda") {
				t.Fatalf("lit an unchanged line: %q", ansi.Strip(line))
			}
		}
	}
	if lit != 1 {
		t.Fatalf("lit %d lines, want the one title line", lit)
	}
	m.endPeekFlash(PeekFlashTickMsg{Seq: m.peekFlashSeq})
	if m.peekFlash != nil {
		t.Fatal("the expiry tick left the flash lit")
	}
}

func taskRowIndex(t *testing.T, m *OutlinerModel, id string) int {
	t.Helper()
	for i, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == id {
			return i
		}
	}
	t.Fatalf("no row for %s", id)
	return 0
}

// A mode that draws no Doing work is not a refusal: gd steps to All, opens the
// screen there, and Esc returns to the mode it left.
func TestNowViewStepsToAllFromAModeThatHidesDoing(t *testing.T) {
	lead := doingTask("lead", "Fix customer migration", time.Now().Add(-time.Hour))
	idle := primaryRowsTask("idle", models.StateInbox)
	m := nowCardOutliner(t, 100, 30, idle, lead)
	m.SelectOutlinerMode(chrome.ModeInbox)
	if _, drawn := m.nowViewRowIndex(m.TaskIdentityForRow(OutlinerRow{})); drawn {
		t.Fatal("setup: the zero identity is drawn")
	}
	if _, refusal := m.OpenNowView(); refusal != "" {
		t.Fatalf("OpenNowView refused from Inbox: %s", refusal)
	}
	if !m.NowView.Active || m.ActiveOutlinerMode() != chrome.ModeAll {
		t.Fatalf("screen active=%v in mode %s, want open in All", m.NowView.Active, m.ActiveOutlinerMode())
	}
	m.handleNowViewKey("esc")
	if m.NowView.Active || m.ActiveOutlinerMode() != chrome.ModeInbox {
		t.Fatalf("Esc left active=%v in mode %s, want closed in Inbox", m.NowView.Active, m.ActiveOutlinerMode())
	}
}
