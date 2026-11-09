package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
)

// f on the Now screen starts a session the screen and the Now card both read,
// and its timer ends it once; f again stops it and voids that timer.
func TestFocusSessionStartsReadsAndEnds(t *testing.T) {
	lead := doingTask("lead", "Fix customer migration", time.Now().Add(-time.Hour))
	m := nowCardOutliner(t, 100, 30, lead)
	if _, refusal := m.OpenNowView(); refusal != "" {
		t.Fatal(refusal)
	}
	_, cmd := m.handleNowViewKey("f")
	if cmd == nil || !m.FocusSessionActive() {
		t.Fatal("f armed no session")
	}
	screen := ansi.Strip(m.RenderOutlinerTreeContent(100))
	if !strings.Contains(screen, "◷ focus 25m left") {
		t.Fatalf("the Now screen does not read the session:\n%s", screen)
	}
	if !strings.Contains(ansi.Strip(strings.Join(m.nowCardSpecLines(100), "\n")), "◷ focus 25m left") {
		t.Fatal("the Now card does not read the session")
	}
	seq := m.focusSession.seq
	if _, ok := m.EndFocusSession(FocusSessionEndedMsg{Seq: seq + 1}); ok {
		t.Fatal("a stale timer ended the session")
	}
	result, ok := m.EndFocusSession(FocusSessionEndedMsg{Seq: seq})
	if !ok || result.Title != "Fix customer migration" || result.Length != FocusSessionLength {
		t.Fatalf("end = %+v, %v", result, ok)
	}
	if m.FocusSessionActive() {
		t.Fatal("the session outlived its timer")
	}

	m.handleNowViewKey("f")
	running := m.focusSession.seq
	m.handleNowViewKey("f")
	if m.FocusSessionActive() {
		t.Fatal("a second f did not stop the session")
	}
	if _, ok := m.EndFocusSession(FocusSessionEndedMsg{Seq: running}); ok {
		t.Fatal("a stopped session's timer still ended something")
	}
}

// A task that leaves Doing takes its session with it, so finishing early is
// never followed by a ping.
func TestFocusSessionEndsQuietlyWhenItsTaskLeavesDoing(t *testing.T) {
	lead := doingTask("lead", "Lead", time.Now().Add(-time.Hour))
	m := nowCardOutliner(t, 100, 30, lead)
	if _, refusal := m.OpenNowView(); refusal != "" {
		t.Fatal(refusal)
	}
	m.handleNowViewKey("f")
	seq := m.focusSession.seq
	lead.WorkflowState = models.StateDone
	m.RebuildRows()
	if m.FocusSessionActive() {
		t.Fatal("the session outlived its task leaving Doing")
	}
	if _, ok := m.EndFocusSession(FocusSessionEndedMsg{Seq: seq}); ok {
		t.Fatal("the voided timer still ended something")
	}
}

func (m *OutlinerModel) nowCardSpecLines(width int) []string {
	var b strings.Builder
	m.writeNowCard(&b, width)
	return strings.Split(b.String(), "\n")
}
