package history

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func historyReader(t *testing.T, read func(string) ([]app.HistoryChange, error)) *Model {
	t.Helper()
	registry := operation.NewOperationRegistry()
	for _, descriptor := range operation.CoreInputOperationDescriptors() {
		if descriptor.ID == operation.OperationCoreUndo || descriptor.ID == operation.OperationCoreRedo {
			if err := registry.Register(descriptor); err != nil {
				t.Fatal(err)
			}
		}
	}
	strategy := operation.SimplifiedInputStrategy()
	var recovery []operation.OperationKeyBinding
	for _, binding := range strategy.Bindings {
		if binding.OperationID == operation.OperationCoreUndo || binding.OperationID == operation.OperationCoreRedo {
			recovery = append(recovery, binding)
		}
	}
	strategy.Bindings, strategy.ChordGroups = recovery, nil
	if err := registry.RegisterInputStrategy(strategy); err != nil {
		t.Fatal(err)
	}
	context := operation.CommandContext{View: presentation.ViewCrossProjectOutliner, Capabilities: operation.NewCapabilitySet(operation.CapabilitySimplifiedInput)}
	return New(registry, context, read)
}

func TestHistoryReaderKeepsWholeValuesReachableAcrossResize(t *testing.T) {
	body := strings.Repeat("A complete paragraph with Unicode 請求書 and context.\n", 80) + "LAST DETAIL LINE"
	m := historyReader(t, func(string) ([]app.HistoryChange, error) {
		return []app.HistoryChange{{Entity: "[project] Task", Field: "Description", Before: "Old body", After: body}}, nil
	})
	m.Replace([]Entry{{ID: "one", Description: strings.Repeat("Long action ", 15), Projects: "project", At: time.Now()}}, "one", "", nil)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, size := range [][2]int{{60, 18}, {100, 33}, {60, 18}} {
		m.SetSize(size[0], size[1])
		m.SetShellFrameMetrics(chrome.ShellFrameMetrics{ControlRows: 2, FeedbackRows: 2})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		frame := ansi.Strip(m.View().Content)
		if !strings.Contains(frame, "LAST DETAIL LINE") {
			t.Fatalf("last detail not reachable:\n%s", frame)
		}
		if lipgloss.Height(frame) > size[1]-m.frame.ExcessRows() {
			t.Fatalf("content covers shell footer: %d", lipgloss.Height(frame))
		}
		for _, line := range strings.Split(frame, "\n") {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("wide line: %q", line)
			}
		}
	}
	m.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !m.details || m.showingHelp {
		t.Fatal("help did not return to inspected details")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	if !strings.Contains(ansi.Strip(m.View().Content), "Long action") {
		t.Fatal("full action name unavailable")
	}
}

func TestHistoryReaderRecoveryIgnoresInspectedRowAndSuppressesPendingRepeat(t *testing.T) {
	m := historyReader(t, func(string) ([]app.HistoryChange, error) { return nil, nil })
	entries := []Entry{{ID: "new", Description: "Newest"}, {ID: "old", Description: "Older"}}
	m.Replace(entries, "new", "", nil)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"})
	request, ok := cmd().(RestoreRequest)
	if !ok || request.TransactionID != "new" || request.Source != m {
		t.Fatalf("wrong recovery request: %+v", request)
	}
	if _, repeated := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"}); repeated != nil {
		t.Fatal("pending restore repeated")
	}
	entries[0].Undone = true
	m.Replace(entries, "old", "new", nil)
	if m.entries[m.cursor].ID != "old" {
		t.Fatal("restore moved the inspected row")
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if request := cmd().(RestoreRequest); request.TransactionID != "new" || request.Operation != operation.OperationCoreRedo {
		t.Fatalf("wrong redo request: %+v", request)
	}
	if spec, _ := m.ContextualGuidance(); len(spec.Hints) == 0 {
		t.Fatal("recovery has no guidance")
	}
}

func TestHistoryReaderErrorsRefreshAndLongTimeline(t *testing.T) {
	m := historyReader(t, func(string) ([]app.HistoryChange, error) { return nil, errors.New("retained image missing") })
	var entries []Entry
	for i := range 50 {
		entries = append(entries, Entry{ID: fmt.Sprint(i), Description: fmt.Sprintf("Change number %d", i), Undone: i < 2})
	}
	m.Replace(entries, "2", "1", nil)
	m.SetSize(60, 18)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if !strings.Contains(m.View().Content, "Change number 49") {
		t.Fatal("oldest transaction unreachable")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(m.View().Content, "retained image missing") {
		t.Fatal("detail read failure hidden")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Replace(nil, "", "", errors.New(strings.Repeat("unreadable ledger ", 25)))
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if !strings.Contains(m.View().Content, "Press r to retry") {
		t.Fatal("load error recovery unreachable")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if request := cmd().(RefreshRequest); request.Source != m {
		t.Fatal("refresh lost its owner")
	}
	m.Replace(nil, "", "", nil)
	if !strings.Contains(m.View().Content, "No retained changes yet") {
		t.Fatal("failed refresh never recovered")
	}
}
