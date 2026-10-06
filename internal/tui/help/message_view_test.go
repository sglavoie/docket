package help

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestMessageViewWrapsScrollsAndResizesWithoutLosingText(t *testing.T) {
	var lines []string
	for i := range 60 {
		lines = append(lines, fmt.Sprintf("Message line %d: %s", i, strings.Repeat("detail ", 15)))
	}
	original := strings.Join(lines, "\n")
	m := NewMessageView(RecalledMessage{Text: original})
	for _, size := range [][2]int{{60, 18}, {100, 33}, {60, 18}} {
		m.SetSize(size[0], size[1])
		view := ansi.Strip(m.View().Content)
		if got := lipgloss.Height(view); got != size[1] {
			t.Fatalf("height %d want %d", got, size[1])
		}
		for _, line := range strings.Split(view, "\n") {
			if lipgloss.Width(line) > size[0] {
				t.Fatalf("overflow: %q", line)
			}
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
		if !strings.Contains(ansi.Strip(m.View().Content), "Message line 59") {
			t.Fatal("last line unavailable")
		}
	}
	m.Update(messageCopiedMsg{})
	if m.text != original || m.copyNotice != "Copied message" {
		t.Fatal("copy changed the recalled message")
	}
	m.Update(messageCopiedMsg{err: errors.New("clipboard unavailable")})
	if m.text != original || !strings.Contains(m.copyNotice, "clipboard unavailable") {
		t.Fatal("copy refusal lost text")
	}
}

func TestMessageViewEmptySession(t *testing.T) {
	m := NewMessageView(RecalledMessage{})
	if !strings.Contains(m.View().Content, "No feedback yet this session.") {
		t.Fatal("missing empty state")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
		t.Fatal("empty message offered copy")
	}
}

func TestMessageSearchHighlightsTextAndKindWithoutChangingContent(t *testing.T) {
	previous := theme.HighlightStyle
	theme.HighlightStyle = lipgloss.NewStyle().Underline(true)
	t.Cleanup(func() { theme.HighlightStyle = previous })
	original := "Read 界界 café details\n" + strings.Repeat("more detail ", 12) + "zebra"
	m := NewMessageView(RecalledMessage{Text: original, Kind: "Error"})
	m.query = "café zebra error"
	m.filterMessages()
	text, kind := m.highlightedMessage()
	if ansi.Strip(text) != original || ansi.Strip(kind) != "Error" {
		t.Fatal("highlighting changed message text or kind")
	}
	for _, char := range []string{"é", "z"} {
		if !strings.Contains(text, theme.HighlightStyle.Render(char)) {
			t.Fatalf("missing body highlight for %q", char)
		}
	}
	if !strings.Contains(kind, theme.HighlightStyle.Render("E")) {
		t.Fatal("missing kind highlight")
	}
	for _, size := range [][2]int{{60, 18}, {100, 33}, {60, 18}} {
		m.SetSize(size[0], size[1])
		view := m.View().Content
		if !utf8.ValidString(view) || lipgloss.Width(view) > size[0] || lipgloss.Height(view) != size[1] {
			t.Fatalf("invalid highlighted layout at %v: %q", size, view)
		}
		if !strings.Contains(view, theme.HighlightStyle.Render("z")) || !strings.Contains(view, theme.HighlightStyle.Render("E")) {
			t.Fatal("rendering lost body or kind highlighting")
		}
	}
	if m.text != original || m.messages[0].Text != original || strings.Contains(m.historyText(), "\x1b") {
		t.Fatal("highlighting changed the plain snapshot used for copying")
	}
	m.clearSearch()
	text, kind = m.highlightedMessage()
	if text != original || kind != "Error" {
		t.Fatal("clearing search retained highlight styling")
	}
}

func TestMessageViewBrowsesSnapshotWithoutReactivatingRecovery(t *testing.T) {
	messages := []RecalledMessage{{Text: "Latest success"}, {Text: strings.Repeat("Older detail\n", 40) + "u undo"}}
	m := NewMessageView(messages...)
	messages[0].Text = "Changed externally"
	m.SetSize(60, 18)
	if m.text != "Latest success" || !strings.Contains(m.View().Content, "Message 1 of 2") {
		t.Fatal("viewer did not take a newest-first snapshot")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !strings.Contains(m.View().Content, "Message 2 of 2") || !strings.Contains(m.text, "Older detail") {
		t.Fatal("older message unavailable")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if m.offset == 0 {
		t.Fatal("older message did not scroll")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'u', Text: "u"}); cmd != nil {
		t.Fatal("historical undo hint became an action")
	}
	m.Update(messageCopiedMsg{index: 1})
	if m.copyNotice != "Copied message" {
		t.Fatal("copy feedback missing")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.text != "Latest success" || m.offset != 0 || m.copyNotice != "" {
		t.Fatal("navigation retained the previous message's scroll or copy notice")
	}
	m.Update(messageCopiedMsg{index: 1, err: errors.New("late clipboard failure")})
	if m.copyNotice != "" {
		t.Fatal("late clipboard result decorated another message")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.index != 0 {
		t.Fatal("navigation wrapped beyond newest message")
	}
}

func TestMessageHistoryRetainsIndependentScrollAndMetadata(t *testing.T) {
	now := time.Now()
	at := time.Date(now.Year(), now.Month(), now.Day(), 14, 32, 0, 0, time.Local)
	m := NewMessageView(RecalledMessage{Text: strings.Repeat("new detail\n", 80), At: at, Kind: "Error"}, RecalledMessage{Text: strings.Repeat("old detail\n", 40), At: at.Add(-time.Minute), Kind: "Feedback"})
	m.SetSize(60, 18)
	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "14:32 · Error") {
		t.Fatalf("metadata missing: %s", plain)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	first := m.offset
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	second := m.offset
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.offset != first {
		t.Fatalf("newest scroll = %d, want %d", m.offset, first)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.offset != second {
		t.Fatalf("older scroll = %d, want %d", m.offset, second)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	m.SetSize(100, 100)
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.offset != 0 {
		t.Fatalf("restored scroll not clamped after resize: %d", m.offset)
	}
}

func TestMessageHistoryCopyAllUsesSnapshotAndRetainsFeedbackOnFailure(t *testing.T) {
	at := time.Date(2026, 10, 4, 14, 32, 17, 0, time.Local)
	m := NewMessageView(RecalledMessage{Text: "Latest\nDetails", At: at, Kind: "Error"}, RecalledMessage{Text: "Older", At: at.Add(-24 * time.Hour), Kind: "Feedback"})
	original := external.CopyText
	t.Cleanup(func() { external.CopyText = original })
	var copied string
	external.CopyText = func(text string) error { copied = text; return nil }
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	if cmd == nil {
		t.Fatal("copy-all unavailable")
	}
	m.Update(cmd())
	want := at.Format(time.RFC3339) + " · Error\nLatest\nDetails\n\n" + at.Add(-24*time.Hour).Format(time.RFC3339) + " · Feedback\nOlder"
	if copied != want || m.copyNotice != "Copied message history" || m.index != 1 || m.text != "Older" {
		t.Fatalf("copy changed history or omitted content: %q / %+v", copied, m)
	}
	external.CopyText = func(string) error { return errors.New("clipboard unavailable") }
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	m.Update(cmd())
	if !strings.Contains(m.copyNotice, "clipboard unavailable") || m.historyText() != want {
		t.Fatal("failed copy changed retained feedback")
	}
	if _, cmd := NewMessageView().Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"}); cmd != nil {
		t.Fatal("empty history offered copy-all")
	}
}

func TestMessageTimeLabelUsesLocalCalendarDate(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 5, 0, 0, time.Local)
	for _, tc := range []struct {
		at   time.Time
		want string
	}{
		{now, "00:05"},
		{now.Add(-10 * time.Minute), "2025-12-31 23:55"},
		{now.UTC(), "00:05"},
	} {
		if got := messageTimeLabel(tc.at, now); got != tc.want {
			t.Fatalf("label = %q, want %q", got, tc.want)
		}
	}
}

func TestMessageSearchFiltersKindAndTextAndRestoresOriginalMessage(t *testing.T) {
	at := time.Date(2026, 10, 4, 14, 32, 0, 0, time.Local)
	m := NewMessageView(
		RecalledMessage{Text: "Saved task", At: at, Kind: "Feedback"},
		RecalledMessage{Text: strings.Repeat("Original detail\n", 40), At: at, Kind: "Feedback"},
		RecalledMessage{Text: "Disk unavailable", At: at, Kind: "Error"},
		RecalledMessage{Text: "Save refused", At: at, Kind: "Error"},
	)
	m.SetSize(60, 18)
	m.Update(helpKeyPress("left"))
	m.Update(helpKeyPress("end"))
	originalOffset := m.offset
	m.Update(helpKeyPress("/"))
	for _, key := range []string{"e", "r", "r", "o", "r"} {
		m.Update(helpKeyPress(key))
	}
	if !m.HasTextInput() || !m.ShouldCaptureEsc() || m.index != 2 || len(m.matches) != 2 {
		t.Fatalf("search did not match message kinds: %+v", m)
	}
	m.Update(helpKeyPress("enter"))
	m.Update(helpKeyPress("left"))
	if m.index != 3 || !strings.Contains(ansi.Strip(m.View().Content), "14:32 · Error") {
		t.Fatal("filtered navigation lost identity or metadata")
	}
	m.Update(helpKeyPress("esc"))
	if m.index != 1 || m.offset != originalOffset || m.ShouldCaptureEsc() {
		t.Fatalf("clear did not restore original message and scroll: index %d, offset %d", m.index, m.offset)
	}
	m.Update(helpKeyPress("/"))
	for _, key := range []string{"d", "i", "s", "k"} {
		m.Update(helpKeyPress(key))
	}
	if m.index != 2 || len(m.matches) != 1 {
		t.Fatal("message text is not searchable")
	}
	for _, size := range [][2]int{{60, 18}, {100, 33}, {60, 18}} {
		m.SetSize(size[0], size[1])
		view := ansi.Strip(m.View().Content)
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) != size[1] {
			t.Fatalf("search frame overflows %v:\n%s", size, view)
		}
	}
	m.Update(helpKeyPress("z"))
	if !strings.Contains(m.View().Content, "No matching messages") {
		t.Fatal("empty result has no recovery guidance")
	}
	if _, cmd := m.Update(messageCopiedMsg{index: 2}); cmd != nil || m.copyNotice != "" {
		t.Fatal("late copy decorated an empty search")
	}
	m.Update(helpKeyPress("esc"))
	if m.index != 1 || !m.searching || m.query != "" {
		t.Fatal("first Esc failed to clear and restore")
	}
	m.Update(helpKeyPress("esc"))
	if m.ShouldCaptureEsc() {
		t.Fatal("second Esc did not leave input")
	}
}

func TestMessageSearchCopyAndScrollKeepTheOriginalSnapshot(t *testing.T) {
	m := NewMessageView(
		RecalledMessage{Text: strings.Repeat("detail\n", 40) + "LAST LINE", Kind: "Error"},
		RecalledMessage{Text: "Saved task", Kind: "Feedback"},
	)
	m.SetSize(60, 18)
	original := external.CopyText
	t.Cleanup(func() { external.CopyText = original })
	var copied string
	external.CopyText = func(text string) error { copied = text; return nil }
	m.Update(helpKeyPress("/"))
	for _, key := range []string{"e", "r", "r", "o", "r", "enter", "end"} {
		m.Update(helpKeyPress(key))
	}
	if !strings.Contains(m.View().Content, "LAST LINE") {
		t.Fatal("search chrome hid the last line of the message")
	}
	_, cmd := m.Update(helpKeyPress("y"))
	if cmd == nil {
		t.Fatal("filtered message cannot be copied")
	}
	m.Update(cmd())
	if copied != m.messages[0].Text {
		t.Fatal("copy included the query or omitted message text")
	}
	m.Update(helpKeyPress("esc"))
	if m.index != 0 || m.offset != 0 {
		t.Fatal("clear did not restore the original scroll when its message matched")
	}
	for _, key := range []string{"/", "z", "z", "z", "enter"} {
		m.Update(helpKeyPress(key))
	}
	if _, emptyCopy := m.Update(helpKeyPress("y")); emptyCopy != nil {
		t.Fatal("empty results offered to copy one message")
	}
	_, cmd = m.Update(helpKeyPress("Y"))
	if cmd == nil {
		t.Fatal("empty results prevented copying retained history")
	}
	m.Update(cmd())
	if copied != m.historyText() || !strings.Contains(copied, "Saved task") {
		t.Fatal("copy-all filtered or mutated the original snapshot")
	}
}

func TestMessageCopyMatchesSnapshotsFilterAndPreservesMetadata(t *testing.T) {
	at := time.Date(2026, 10, 5, 10, 30, 0, 0, time.Local)
	m := NewMessageView(
		RecalledMessage{Text: "First failure 界", Kind: "Error", At: at},
		RecalledMessage{Text: "Hidden success", Kind: "Feedback", At: at.Add(-time.Minute)},
		RecalledMessage{Text: "Second failure", Kind: "Error", At: at.Add(-2 * time.Minute)},
	)
	original := external.CopyText
	t.Cleanup(func() { external.CopyText = original })
	var copied string
	external.CopyText = func(text string) error { copied = text; return nil }
	for _, key := range []string{"/", "E", "r", "r", "o", "r", "enter"} {
		m.Update(helpKeyPress(key))
	}
	for _, size := range [][2]int{{60, 18}, {100, 33}, {60, 18}} {
		m.SetSize(size[0], size[1])
		view := ansi.Strip(m.View().Content)
		if !strings.Contains(view, "Ctrl+Y copy matches") || lipgloss.Width(view) > size[0] || lipgloss.Height(view) != size[1] {
			t.Fatalf("matching-copy guidance/layout at %v: %s", size, view)
		}
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("matching copy unavailable")
	}
	// The clipboard command owns the accepted filter, even after it is cleared.
	m.Update(helpKeyPress("esc"))
	m.Update(cmd())
	want := at.Format(time.RFC3339) + " · Error\nFirst failure 界\n\n" +
		at.Add(-2*time.Minute).Format(time.RFC3339) + " · Error\nSecond failure"
	if copied != want || m.copyNotice != "Copied 2 matching messages" {
		t.Fatalf("copy = %q, notice = %q", copied, m.copyNotice)
	}
	if len(m.messages) != 3 || m.messages[1].Text != "Hidden success" {
		t.Fatal("copy modified message history")
	}
}

func TestMessageCopyMatchesGuardsAndClipboardFailure(t *testing.T) {
	m := NewMessageView(RecalledMessage{Text: "Failure", Kind: "Error"})
	key := tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}
	for _, query := range []string{"", " ", "absent"} {
		m.query = query
		m.filterMessages()
		if _, cmd := m.Update(key); cmd != nil || strings.Contains(ansi.Strip(m.View().Content), "Ctrl+Y copy matches") {
			t.Fatalf("matching copy offered for query %q", query)
		}
	}
	m.query = "Error"
	m.filterMessages()
	m.searching = true
	if _, cmd := m.Update(key); cmd != nil {
		t.Fatal("matching copy ran while typing")
	}
	m.searching = false
	original := external.CopyText
	t.Cleanup(func() { external.CopyText = original })
	external.CopyText = func(string) error { return errors.New("clipboard unavailable") }
	_, cmd := m.Update(key)
	if cmd == nil {
		t.Fatal("matching copy unavailable")
	}
	m.Update(cmd())
	if m.copyNotice != "Could not copy: clipboard unavailable" || m.text != "Failure" || m.query != "Error" {
		t.Fatal("clipboard failure lost message or search")
	}
	external.CopyText = func(string) error { return nil }
	_, cmd = m.Update(key)
	m.Update(cmd())
	if m.copyNotice != "Copied 1 matching message" {
		t.Fatal("single matching message reported incorrectly")
	}
}

func TestFilteredMessageCopyExplainsFullHistory(t *testing.T) {
	m := NewMessageView(RecalledMessage{Text: "Visible", Kind: "Error"}, RecalledMessage{Text: "Hidden", Kind: "Feedback"})
	m.SetSize(60, 20)
	m.query = "Error"
	m.filterMessages()
	if !strings.Contains(ansi.Strip(m.View().Content), "Y copy all 2") {
		t.Fatal("filtered copy hint does not name the full count")
	}
	original := external.CopyText
	t.Cleanup(func() { external.CopyText = original })
	var copied string
	external.CopyText = func(text string) error { copied = text; return nil }
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'Y', Text: "Y"})
	// Feedback describes the filter at the time of the copy, even if it changes.
	m.query = ""
	m.filterMessages()
	m.Update(cmd())
	if !strings.Contains(copied, "Visible") || !strings.Contains(copied, "Hidden") ||
		m.copyNotice != "Copied all 2 messages, including 1 hidden by the filter" {
		t.Fatalf("copy scope mismatch: %q / %q", copied, m.copyNotice)
	}
}
