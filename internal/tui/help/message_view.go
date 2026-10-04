package help

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/textfit"
)

// RecalledMessage is one session feedback event. At is the original event time,
// retained when later details amend Text.
type RecalledMessage struct {
	Text string
	At   time.Time
	Kind string
}

// MessageView reads a snapshot of feedback. Copying it never replaces the
// recalled message, and its displayed recovery hints are text, not actions.
type MessageView struct {
	pulse.WorkspacePulseState
	text                  string
	messages              []RecalledMessage
	offsets               []int
	index                 int
	copyNotice            string
	width, height, offset int
}

type messageCopiedMsg struct {
	err   error
	index int
	all   bool
}

// NewMessageView snapshots feedback in newest-first order.
func NewMessageView(messages ...RecalledMessage) *MessageView {
	m := &MessageView{width: 80, height: 24}
	for _, message := range messages {
		if message.Text != "" {
			m.messages = append(m.messages, message)
		}
	}
	if len(m.messages) > 0 {
		m.text = m.messages[0].Text
	}
	m.offsets = make([]int, len(m.messages))
	return m
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
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "y copy", "Y copy all")...)
	}
	if len(m.messages) > 1 {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "← older", "→ newer")...)
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
		if msg.index != m.index {
			return m, nil
		}
		m.copyNotice = "Copied message"
		if msg.all {
			m.copyNotice = "Copied message history"
		}
		if msg.err != nil {
			m.copyNotice = fmt.Sprintf("Could not copy: %s", msg.err)
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			if m.text != "" {
				text := m.text
				all := msg.String() == "Y"
				if all {
					text = m.historyText()
				}
				index := m.index
				return m, func() tea.Msg { return messageCopiedMsg{err: external.CopyText(text), index: index, all: all} }
			}
		case "left":
			m.selectMessage(m.index + 1)
		case "right":
			m.selectMessage(m.index - 1)
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
	subtitle := "Read-only"
	if len(m.messages) > 0 {
		subtitle = fmt.Sprintf("Message %d of %d", m.index+1, len(m.messages))
		entry := m.messages[m.index]
		if !entry.At.IsZero() {
			subtitle += " · " + messageTimeLabel(entry.At, time.Now())
		}
		if entry.Kind != "" {
			subtitle += " · " + entry.Kind
		}
		subtitle += " · Read-only"
	}
	title := chrome.RenderFullscreenTitleWithPulse("Last message", subtitle, m.CurrentWorkspacePulse(), m.width)
	return tea.NewView(chrome.FitBlockHeight(title+"\n\n"+body, max(m.height-lipgloss.Height(footer), 1)) + "\n" + footer)
}

func messageTimeLabel(at, now time.Time) string {
	at, now = at.Local(), now.Local()
	layout := "15:04"
	if at.Format(time.DateOnly) != now.Format(time.DateOnly) {
		layout = "2006-01-02 15:04"
	}
	return at.Format(layout)
}

// historyText exports the same newest-first snapshot the reader browses.
// Full timestamps retain dates and UTC offsets when shared outside this session.
func (m *MessageView) historyText() string {
	entries := make([]string, 0, len(m.messages))
	for _, entry := range m.messages {
		var metadata []string
		if !entry.At.IsZero() {
			metadata = append(metadata, entry.At.Local().Format(time.RFC3339))
		}
		if entry.Kind != "" {
			metadata = append(metadata, entry.Kind)
		}
		text := entry.Text
		if len(metadata) > 0 {
			text = strings.Join(metadata, " · ") + "\n" + text
		}
		entries = append(entries, text)
	}
	return strings.Join(entries, "\n\n")
}

func (m *MessageView) selectMessage(index int) {
	if index < 0 || index >= len(m.messages) || index == m.index {
		return
	}
	m.offsets[m.index] = m.offset
	m.index = index
	m.text = m.messages[index].Text
	m.offset = m.offsets[index]
	m.copyNotice = ""
}
