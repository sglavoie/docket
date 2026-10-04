package help

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestMessageViewWrapsScrollsAndResizesWithoutLosingText(t *testing.T) {
	var lines []string
	for i := range 60 {
		lines = append(lines, fmt.Sprintf("Message line %d: %s", i, strings.Repeat("detail ", 15)))
	}
	original := strings.Join(lines, "\n")
	m := NewMessageView(original)
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
	m := NewMessageView("")
	if !strings.Contains(m.View().Content, "No feedback yet this session.") {
		t.Fatal("missing empty state")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
		t.Fatal("empty message offered copy")
	}
}
