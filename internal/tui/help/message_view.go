package help

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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
	query                 string
	searching             bool
	searchInput           listsurface.PickerSearchInput
	matches               []int // Indices into the unchanged newest-first snapshot.
	searchOrigin          int
	searchOriginOffset    int
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
	m.filterMessages()
	return m
}

func (m *MessageView) Init() tea.Cmd          { return nil }
func (m *MessageView) IsFullHeight() bool     { return true }
func (m *MessageView) HasTextInput() bool     { return m.searching }
func (m *MessageView) ShouldCaptureEsc() bool { return m.searching || m.query != "" }
func (m *MessageView) StatusBarLabel() string { return "Recent messages" }

func (m *MessageView) SetSize(width, height int) {
	m.width, m.height = max(width, 1), max(height, 1)
	m.clampOffset()
}

func (m *MessageView) guidance() chrome.GuidanceSpec {
	if m.searching {
		escape := "Esc cancel search"
		if m.query != "" {
			escape = "Esc clear search"
		}
		return chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("Enter apply search", escape), Rows: 2}
	}
	escape := "Esc close"
	if m.query != "" {
		escape = "Esc clear search"
	}
	spec := chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints(escape), Rows: 2}
	spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "/ search")...)
	if m.text != "" {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "y copy")...)
	}
	if len(m.messages) > 0 {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "Y copy all")...)
	}
	if len(m.matches) > 1 {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(1, "← older", "→ newer")...)
	}
	if len(m.lines()) > m.capacity() {
		spec.Hints = append(spec.Hints, chrome.OptionalGuidanceHints(2, "j/k scroll", "PgUp/PgDn page")...)
	}
	return spec
}

func (m *MessageView) lines() []string {
	text, _ := m.highlightedMessage()
	if text == "" {
		text = "No feedback yet this session."
		if m.query != "" {
			text = "No matching messages. Esc clears the search."
		}
	}
	if m.copyNotice != "" {
		text += "\n\n" + m.copyNotice
	}
	return textfit.WrapStyled(text, m.width)
}

// Match the same combined fields as filtering, then map rune positions back to
// each field. A query can match both the message text and its kind.
func (m *MessageView) highlightedMessage() (string, string) {
	if m.index < 0 || m.index >= len(m.messages) {
		return m.text, ""
	}
	kind := m.messages[m.index].Kind
	if strings.TrimSpace(m.query) == "" {
		return m.text, kind
	}
	_, positions := fuzzy.NewMatcher(false).ScoreTermsWithPositions(m.text+" "+kind, m.query)
	textLength := len([]rune(m.text))
	var textPositions, kindPositions []int
	for _, position := range positions {
		if position < textLength {
			textPositions = append(textPositions, position)
		} else if position > textLength {
			kindPositions = append(kindPositions, position-textLength-1)
		}
	}
	return textfit.HighlightText(m.text, textPositions, lipgloss.NewStyle()),
		textfit.HighlightText(kind, kindPositions, theme.MutedStyle)
}
func (m *MessageView) capacity() int {
	rows := m.height - 5
	if m.searching || m.query != "" {
		rows--
	}
	return max(rows, 1)
}
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
		if m.searching {
			switch msg.String() {
			case "esc":
				if m.query != "" {
					m.clearSearch()
				} else {
					m.searching = false
				}
			case "enter":
				m.searching = false
			default:
				m.query = m.searchInput.HandleKey(m.query, msg)
				m.filterMessages()
			}
			m.clampOffset()
			return m, nil
		}
		switch msg.String() {
		case "/":
			if m.query == "" {
				m.searchOrigin = m.index
				m.searchOriginOffset = m.offset
			}
			m.searching = true
		case "esc":
			m.clearSearch()
		case "y", "Y":
			if m.text != "" || (msg.String() == "Y" && len(m.messages) > 0) {
				text := m.text
				all := msg.String() == "Y"
				if all {
					text = m.historyText()
				}
				index := m.index
				return m, func() tea.Msg { return messageCopiedMsg{err: external.CopyText(text), index: index, all: all} }
			}
		case "left":
			m.moveMatch(1)
		case "right":
			m.moveMatch(-1)
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
	if m.index >= 0 && len(m.messages) > 0 {
		subtitle = fmt.Sprintf("Message %d of %d", m.index+1, len(m.messages))
		if m.query != "" {
			subtitle = fmt.Sprintf("Match %d of %d", slices.Index(m.matches, m.index)+1, len(m.matches))
		}
		entry := m.messages[m.index]
		if !entry.At.IsZero() {
			subtitle += " · " + messageTimeLabel(entry.At, time.Now())
		}
		if entry.Kind != "" {
			_, kind := m.highlightedMessage()
			subtitle += " · " + kind
		}
		subtitle += " · Read-only"
	}
	title := chrome.RenderFullscreenTitleWithPulse("Recent messages", subtitle, m.CurrentWorkspacePulse(), m.width)
	if m.searching || m.query != "" {
		title += "\n" + m.searchInput.View(m.query, m.searching, m.width)
	}
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
	if m.index >= 0 && m.index < len(m.offsets) {
		m.offsets[m.index] = m.offset
	}
	m.index = index
	m.text = m.messages[index].Text
	m.offset = m.offsets[index]
	m.copyNotice = ""
}

func (m *MessageView) moveMatch(delta int) {
	position := slices.Index(m.matches, m.index) + delta
	if position >= 0 && position < len(m.matches) {
		m.selectMessage(m.matches[position])
	}
}

func (m *MessageView) filterMessages() {
	m.matches = nil
	matcher := fuzzy.NewMatcher(false)
	query := strings.TrimSpace(m.query)
	for index, entry := range m.messages {
		if query == "" || matcher.ScoreTerms(entry.Text+" "+entry.Kind, query) > 0 {
			m.matches = append(m.matches, index)
		}
	}
	if len(m.matches) == 0 {
		if m.index >= 0 && m.index < len(m.offsets) {
			m.offsets[m.index] = m.offset
		}
		m.index, m.offset, m.text, m.copyNotice = -1, 0, "", ""
	} else if m.query == "" {
		m.selectMessage(m.searchOrigin)
		m.offset = m.searchOriginOffset
	} else if !slices.Contains(m.matches, m.index) {
		m.selectMessage(m.matches[0])
	}
}

func (m *MessageView) clearSearch() {
	if m.query == "" {
		return
	}
	m.query = ""
	m.filterMessages()
}
