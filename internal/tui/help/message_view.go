package help

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// MessageView reads a snapshot of feedback. Copying it never replaces the
// recalled message, and its displayed recovery hints are text, not actions.
type MessageView struct {
	pulse.WorkspacePulseState
	text                  string
	copyNotice            string
	width, height, offset int
}

type messageCopiedMsg struct{ err error }

func NewMessageView(text string) *MessageView {
	return &MessageView{text: text, width: 80, height: 24}
}

func (m *MessageView) Init() tea.Cmd          { return nil }
func (m *MessageView) IsFullHeight() bool     { return true }
func (m *MessageView) HasTextInput() bool     { return false }
func (m *MessageView) ShouldCaptureEsc() bool { return false }
func (m *MessageView) StatusBarLabel() string { return "Last message" }

func (m *MessageView) SetSize(width, height int) {
	m.width, m.height = max(width, 1), max(height, 1)
	m.clampOffset()
}

func (m *MessageView) guidance() chrome.GuidanceSpec {
	spec := chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("Esc close"), Rows: 2}
	if m.text != "" {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "y copy")...)
	}
	if len(m.lines()) > m.capacity() {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(2, "j/k scroll", "PgUp/PgDn page")...)
	}
	return spec
}

func (m *MessageView) lines() []string {
	text := m.text
	if text == "" {
		text = "No feedback yet this session."
	}
	if m.copyNotice != "" {
		text += "\n\n" + m.copyNotice
	}
	return textfit.WrapText(text, m.width)
}
func (m *MessageView) capacity() int { return max(m.height-5, 1) }
func (m *MessageView) clampOffset() {
	m.offset = min(max(m.offset, 0), max(len(m.lines())-m.capacity(), 0))
}

func (m *MessageView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case messageCopiedMsg:
		m.copyNotice = "Copied message"
		if msg.err != nil {
			m.copyNotice = fmt.Sprintf("Could not copy: %s", msg.err)
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y":
			if m.text != "" {
				text := m.text
				return m, func() tea.Msg { return messageCopiedMsg{err: external.CopyText(text)} }
			}
		case "up", "k":
			m.offset--
		case "down", "j":
			m.offset++
		case "pgup", "ctrl+u":
			m.offset -= m.capacity()
		case "pgdown", "ctrl+d":
			m.offset += m.capacity()
		case "home":
			m.offset = 0
		case "end", "G":
			m.offset = len(m.lines())
		}
	}
	m.clampOffset()
	return m, nil
}

func (m *MessageView) View() tea.View {
	lines := m.lines()
	end := min(m.offset+m.capacity(), len(lines))
	body := chrome.FitBlockHeight(strings.Join(lines[m.offset:end], "\n"), m.capacity())
	footer := chrome.RenderGuidance(m.guidance(), m.width)
	title := chrome.RenderFullscreenTitleWithPulse("Last message", "Read-only", m.CurrentWorkspacePulse(), m.width)
	return tea.NewView(chrome.FitBlockHeight(title+"\n\n"+body, max(m.height-lipgloss.Height(footer), 1)) + "\n" + footer)
}
