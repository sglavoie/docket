// Package history presents retained workspace changes and their recovery keys.
// The shell supplies snapshots and executes every restore through the registry.
package history

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

type Entry struct {
	ID, Description, Projects string
	At                        time.Time
	Undone                    bool
}

// RestoreRequest binds the action to the next transaction advertised when the
// key was pressed. Selecting an older row never changes which action it means.
type RestoreRequest struct {
	Source        *Model
	Operation     operation.OperationID
	TransactionID string
}

type Model struct {
	pulse.WorkspacePulseState
	entries                       []Entry
	undo, redo                    string
	cursor, offset                int
	width, height                 int
	frame                         chrome.ShellFrameMetrics
	details, showingHelp, pending bool
	detailText, loadError         string
	registry                      *operation.OperationRegistry
	context                       operation.CommandContext
	readDetails                   func(string) ([]app.HistoryChange, error)
}

func New(registry *operation.OperationRegistry, context operation.CommandContext, readDetails func(string) ([]app.HistoryChange, error)) *Model {
	return &Model{registry: registry, context: context, readDetails: readDetails, width: 80, height: 22}
}

func (m *Model) Init() tea.Cmd          { return nil }
func (m *Model) HasTextInput() bool     { return false }
func (m *Model) IsFullHeight() bool     { return false }
func (m *Model) ShouldCaptureEsc() bool { return m.details || m.showingHelp }
func (m *Model) StatusBarLabel() string { return "Change History" }
func (m *Model) SetSize(width, height int) {
	m.width, m.height = max(width, 1), max(height, 1)
	m.clamp()
}
func (m *Model) SetShellFrameMetrics(frame chrome.ShellFrameMetrics) { m.frame = frame; m.clamp() }

// Replace preserves the selected transaction even when undo moves it between
// stacks. A failed reload removes stale recovery offers rather than guessing.
func (m *Model) Replace(entries []Entry, undo, redo string, err error) {
	id := ""
	if len(m.entries) > 0 {
		id = m.entries[m.cursor].ID
	}
	m.entries, m.undo, m.redo, m.pending = entries, undo, redo, false
	m.loadError = ""
	if err != nil {
		m.loadError = "Could not read change history: " + err.Error()
	}
	m.cursor = min(m.cursor, max(len(entries)-1, 0))
	for i, entry := range entries {
		if entry.ID == id {
			m.cursor = i
			break
		}
	}
	if len(entries) == 0 {
		m.details, m.detailText = false, ""
	}
	if m.details {
		m.openDetails()
	}
	m.clamp()
}

func (m *Model) ContextualGuidance() (chrome.GuidanceSpec, bool) {
	escape := "Esc close"
	if m.details || m.showingHelp {
		escape = "Esc back"
	}
	hints := chrome.RequiredGuidanceHints(escape, "? help")
	if m.loadError != "" {
		hints = append(hints, chrome.RequiredGuidanceHints("r retry")...)
	}
	if !m.details && !m.showingHelp && m.loadError == "" {
		hints = append(hints, chrome.OptionalGuidanceHints(1, "Enter details", "j/k move")...)
	} else {
		hints = append(hints, chrome.OptionalGuidanceHints(1, "j/k scroll", "PgUp/PgDn page")...)
	}
	for _, id := range []operation.OperationID{operation.OperationCoreUndo, operation.OperationCoreRedo} {
		label, exists := "undo", m.undo != ""
		if id == operation.OperationCoreRedo {
			label, exists = "redo", m.redo != ""
		}
		if exists {
			hints = append(hints, chrome.OptionalGuidanceHints(1, m.recoveryKey(id)+" "+label)...)
		}
	}
	if m.loadError == "" {
		hints = append(hints, chrome.OptionalGuidanceHints(2, "r refresh")...)
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 2}, true
}

func (m *Model) recoveryKey(id operation.OperationID) string {
	strategy, _ := m.registry.InputStrategy(operation.InputStrategySimplified)
	return operation.DisplayKeysForOperation(strategy, id)
}

func (m *Model) bodyRows() int { return max(m.height-m.frame.ExcessRows()-5, 1) }
func (m *Model) document() []string {
	text := m.detailText
	if m.loadError != "" {
		text = m.loadError + "\nPress r to retry."
	}
	if m.showingHelp {
		text = "Browse changes with j/k or the arrow keys. Enter opens the complete transaction details.\n\n" +
			m.recoveryKey(operation.OperationCoreUndo) + " always reverses Next undo; " + m.recoveryKey(operation.OperationCoreRedo) + " always reapplies Next redo. The selected row is for inspection and does not select a different restore. Each press restores one whole transaction.\n\n" +
			"Details describe the original before → after change. Undo reverses that direction. The restore checks current data and may refuse if a changed field was edited elsewhere.\n\n" +
			"Home and End reach the ends. PgUp/PgDn page the list or details. r refreshes history. Esc returns from details, then closes history."
	}
	return textfit.WrapStyled(text, max(m.width-2, 1))
}

func (m *Model) clamp() {
	m.cursor = min(max(m.cursor, 0), max(len(m.entries)-1, 0))
	if m.details || m.showingHelp || m.loadError != "" {
		m.offset = min(max(m.offset, 0), max(len(m.document())-m.bodyRows(), 0))
		return
	}
	capacity := max(m.bodyRows()/2, 1)
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+capacity {
		m.offset = m.cursor - capacity + 1
	}
	m.offset = min(max(m.offset, 0), max(len(m.entries)-capacity, 0))
}

func (m *Model) openDetails() {
	if len(m.entries) == 0 {
		return
	}
	entry := m.entries[m.cursor]
	text := entry.Description + "\n" + entry.At.Local().Format("2006-01-02 15:04:05 MST") + "\nProjects: " + entry.Projects + "\n\nOriginal change (before → after)\n"
	changes, err := m.readDetails(entry.ID)
	if err != nil {
		text += "\nCould not read details: " + err.Error()
	} else {
		entity := ""
		for _, change := range changes {
			if change.Entity != entity {
				text += "\n" + change.Entity + "\n"
				entity = change.Entity
			}
			a, b := change.Before, change.After
			if a == "" {
				a = "(empty)"
			}
			if b == "" {
				b = "(empty)"
			}
			if strings.ContainsAny(a+b, "\n") || len([]rune(a+b))+len(change.Field) > m.width-8 {
				text += change.Field + "\nBefore:\n" + a + "\nAfter:\n" + b + "\n"
			} else {
				text += change.Field + ": " + a + " → " + b + "\n"
			}
		}
		if len(changes) == 0 {
			text += "\nNo additional field details retained."
		}
	}
	m.detailText, m.details = text, true
}

type RefreshRequest struct{ Source *Model }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	// Resolve the recovery keys through the existing strategy rather than
	// defining a second spelling of undo and redo on this surface.
	binding, _, bound := m.registry.InputBindingForKey(operation.InputStrategySimplified, key.String(), m.context)
	if bound && (binding.OperationID == operation.OperationCoreUndo || binding.OperationID == operation.OperationCoreRedo) {
		id := m.undo
		if binding.OperationID == operation.OperationCoreRedo {
			id = m.redo
		}
		if id != "" && !m.pending {
			m.pending = true
			return m, func() tea.Msg { return RestoreRequest{m, binding.OperationID, id} }
		}
		return m, nil
	}
	switch key.String() {
	case "esc":
		if m.showingHelp {
			m.showingHelp = false
		} else {
			m.details = false
		}
		m.offset = 0
	case "?":
		m.showingHelp = !m.showingHelp
		m.offset = 0
	case "enter":
		if !m.showingHelp && !m.details {
			m.offset = 0
			m.openDetails()
		}
	case "r":
		return m, func() tea.Msg { return RefreshRequest{m} }
	default:
		delta := 0
		switch key.String() {
		case "j", "down":
			delta = 1
		case "k", "up":
			delta = -1
		case "pgdown", "ctrl+d":
			delta = max(m.bodyRows()/2, 1)
		case "pgup", "ctrl+u":
			delta = -max(m.bodyRows()/2, 1)
		case "home":
			delta = -1 << 30
		case "end", "G":
			delta = 1 << 30
		}
		if m.details || m.showingHelp || m.loadError != "" {
			m.offset += delta
		} else {
			m.cursor += delta
		}
	}
	m.clamp()
	return m, nil
}

func (m *Model) nextLabel(id, empty string) string {
	if m.loadError != "" {
		return "Unavailable until history reloads"
	}
	for _, entry := range m.entries {
		if entry.ID == id {
			return entry.Description
		}
	}
	return empty
}

func (m *Model) View() tea.View {
	subtitle := fmt.Sprintf("%d retained changes", len(m.entries))
	if m.details {
		subtitle = "Original transaction details"
	}
	if m.showingHelp {
		subtitle = "Help"
	}
	header := chrome.RenderFullscreenTitleWithPulse("Change History", subtitle, m.CurrentWorkspacePulse(), m.width)
	header += "\n" + textfit.TruncateIntentional("Next undo: "+m.nextLabel(m.undo, "Nothing to undo"), m.width, "…")
	header += "\n" + textfit.TruncateIntentional("Next redo: "+m.nextLabel(m.redo, "Nothing to redo"), m.width, "…") + "\n"
	var body []string
	position := ""
	switch {
	case m.details || m.showingHelp || m.loadError != "":
		lines := m.document()
		body = lines[m.offset:min(m.offset+m.bodyRows(), len(lines))]
		position = fmt.Sprintf("Lines %d–%d of %d", m.offset+1, min(m.offset+m.bodyRows(), len(lines)), len(lines))
	case len(m.entries) == 0:
		body = textfit.WrapStyled("No retained changes yet.\nSaved task, project, note and attachment changes appear here.", m.width)
	default:
		capacity := max(m.bodyRows()/2, 1)
		for i := m.offset; i < min(m.offset+capacity, len(m.entries)); i++ {
			entry := m.entries[i]
			marker, state := "  ", "Applied"
			if i == m.cursor {
				marker = "› "
			}
			if entry.Undone {
				state = "Undone"
			}
			if entry.ID == m.undo {
				state = "Next undo"
			}
			if entry.ID == m.redo {
				state = "Next redo"
			}
			line := marker + entry.Description
			line = textfit.TruncateIntentional(line, m.width, "…")
			if i == m.cursor {
				line = chrome.SelectedLine(line, m.width)
			}
			metadata := textfit.TruncateIntentional("  "+entry.At.Local().Format("Jan 02 15:04")+" · "+state+" · "+entry.Projects, m.width, "…")
			body = append(body, line, theme.MutedStyle.Render(metadata))
		}
		position = fmt.Sprintf("Change %d of %d · Enter reads full details", m.cursor+1, len(m.entries))
	}
	content := header + "\n" + chrome.FitBlockHeight(strings.Join(body, "\n"), m.bodyRows()) + "\n" + textfit.TruncateIntentional(position, m.width, "…")
	return tea.NewView(content)
}
