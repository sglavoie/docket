package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// MessageLogLevel indicates the severity of a log entry.
type MessageLogLevel int

const (
	MessageLogLevelSuccess MessageLogLevel = iota
	MessageLogLevelInfo
	MessageLogLevelError
)

// MessageLogEntry is a single timestamped entry in the message log.
type MessageLogEntry struct {
	Timestamp time.Time
	Message   string
	Level     MessageLogLevel
}

const messageLogCapacity = 50

// MessageLog is a fixed-capacity ring buffer of status message entries.
type MessageLog struct {
	entries [messageLogCapacity]MessageLogEntry
	head    int // next write position
	count   int // number of valid entries (capped at capacity)
}

// Append adds a new entry to the log, overwriting the oldest if full.
func (l *MessageLog) Append(msg string, level MessageLogLevel) {
	l.entries[l.head] = MessageLogEntry{
		Timestamp: time.Now(),
		Message:   msg,
		Level:     level,
	}
	l.head = (l.head + 1) % messageLogCapacity
	if l.count < messageLogCapacity {
		l.count++
	}
}

// Entries returns all entries in chronological order (oldest first).
func (l *MessageLog) Entries() []MessageLogEntry {
	if l.count == 0 {
		return nil
	}
	result := make([]MessageLogEntry, l.count)
	start := (l.head - l.count + messageLogCapacity) % messageLogCapacity
	for i := 0; i < l.count; i++ {
		result[i] = l.entries[(start+i)%messageLogCapacity]
	}
	return result
}

// AppendToMessageLogMsg asks the app to append an entry to the message log
// without also updating the status bar. Used for bulk/background errors.
type AppendToMessageLogMsg struct {
	Message string
	Level   MessageLogLevel
}

// ShowMessageLogMsg opens the message log overlay.
type ShowMessageLogMsg struct{}

// MessageLogViewModel is the overlay that shows scrollable message history.
type MessageLogViewModel struct {
	BaseView
	entries []MessageLogEntry
	scroll  int // offset from top (0 = show from beginning)
}

// NewMessageLogViewModel creates the overlay populated with current log entries.
func NewMessageLogViewModel(entries []MessageLogEntry) *MessageLogViewModel {
	m := &MessageLogViewModel{entries: entries}
	// Start scrolled to the bottom so the most recent entry is visible.
	m.scroll = max(0, len(entries)-1)
	return m
}

func (m *MessageLogViewModel) Init() tea.Cmd { return nil }

func (m *MessageLogViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "j", "down":
			m.scroll = min(m.scroll+1, max(0, len(m.entries)-1))
		case "k", "up":
			if m.scroll > 0 {
				m.scroll--
			}
		case "g":
			m.scroll = 0
		case "G":
			m.scroll = max(0, len(m.entries)-1)
		case "ctrl+d", "pgdown":
			half := max(m.visibleLines()/2, 1)
			m.scroll = min(m.scroll+half, max(0, len(m.entries)-1))
		case "ctrl+u", "pgup":
			half := max(m.visibleLines()/2, 1)
			m.scroll = max(0, m.scroll-half)
		}
	}
	return m, nil
}

func (m *MessageLogViewModel) HandledKeys() []string {
	return []string{
		"j", "down", "k", "up", "g", "G",
		"ctrl+d", "ctrl+u",
	}
}

func (m *MessageLogViewModel) View() tea.View {
	title := TitleStyle.Render("Message Log") + MutedStyle.Render(fmt.Sprintf("  (%d entries)", len(m.entries)))
	hint := MutedStyle.Render("j/k scroll · g/G top/bottom · Esc close")

	visible := m.visibleLines()
	var lines []string
	if len(m.entries) == 0 {
		lines = append(lines, MutedStyle.Render("  (no messages yet)"))
	} else {
		start := m.scroll
		if start > len(m.entries)-1 {
			start = max(0, len(m.entries)-1)
		}
		end := min(start+visible, len(m.entries))
		for _, e := range m.entries[start:end] {
			ts := MutedStyle.Render("[" + e.Timestamp.Format("15:04:05") + "]")
			msg := m.levelStyle(e.Level).Render(e.Message)
			lines = append(lines, "  "+ts+" "+msg)
		}
	}

	// Pad to fill the view area so the border looks clean.
	for len(lines) < visible {
		lines = append(lines, "")
	}

	inner := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		strings.Join(lines, "\n"),
		"",
		hint,
	)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(m.width - 4).
		Render(inner)

	return tea.NewView(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box))
}

func (m *MessageLogViewModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *MessageLogViewModel) ShouldCaptureEsc() bool { return false }
func (m *MessageLogViewModel) HasTextInput() bool     { return false }
func (m *MessageLogViewModel) IsFullHeight() bool     { return false }

func (m *MessageLogViewModel) visibleLines() int {
	// Account for border (2), padding (2 top+bottom), title line, blank, hint, blank = 6 overhead lines.
	return max(m.height-10, 1)
}

func (m *MessageLogViewModel) levelStyle(level MessageLogLevel) lipgloss.Style {
	switch level {
	case MessageLogLevelError:
		return ErrorIconStyle
	case MessageLogLevelSuccess:
		return SuccessStyle
	default:
		return MutedStyle
	}
}
