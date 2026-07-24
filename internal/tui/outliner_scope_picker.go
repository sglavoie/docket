package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// ApplyProjectScopeMsg asks the global (cross-project) Outliner to enter a
// single-project scope, or, when Clear is set, to drop the whole scope stack and
// return to the authoritative all-projects view. It is emitted by the scope
// picker and applied in place without leaving the Outliner.
type ApplyProjectScopeMsg struct {
	ProjectID string
	Clear     bool
}

// ShowScopePickerMsg opens the unified Ctrl+X scope/administration picker: a
// single searchable overlay listing All Projects, Active projects, Archived
// projects, and the Trash, each with lifecycle-appropriate actions.
type ShowScopePickerMsg struct{}

// scopeEntryKind classifies a row in the unified scope/administration picker.
// The kind — not a fabricated *models.Project — determines which lifecycle
// actions a row supports, so a trashed generation is never mistaken for an
// active, scopeable project.
type scopeEntryKind int

const (
	// scopeEntryAllProjects is the synthetic scope-clear affordance.
	scopeEntryAllProjects scopeEntryKind = iota
	scopeEntryActive
	scopeEntryArchived
	scopeEntryTrash
)

// scopePickerEntry is a lifecycle-safe row for the scope/administration picker.
// Active and archived rows carry a live *models.Project; trash rows carry only
// the storage trash-entry identity (never a *models.Project), matching the
// distinct restore/purge lifecycle a deleted generation actually supports.
type scopePickerEntry struct {
	kind    scopeEntryKind
	project *models.Project // set for active/archived rows only

	// Trash-only identity (never promoted to a *models.Project).
	trashEntryID string
	trashSlug    string
	trashName    string
	trashDeleted string
}

// filterText returns the case-folding search text for an entry.
func (e scopePickerEntry) filterText() string {
	switch e.kind {
	case scopeEntryAllProjects:
		return "all projects"
	case scopeEntryTrash:
		return e.trashSlug + " " + e.trashName
	default:
		if e.project != nil {
			return e.project.Name + " " + e.project.Context
		}
	}
	return ""
}

// ScopePickerModel is the overlay behind Ctrl+X. It is nav-first: j/k move the
// cursor, "/" activates a fuzzy search, Enter performs each row's primary
// lifecycle action, and per-row administration keys ("X" unarchive, "x" purge)
// apply only to rows whose lifecycle allows them.
type ScopePickerModel struct {
	entries  []scopePickerEntry
	filtered []scopePickerEntry
	cursor   int

	input     textinput.Model
	searching bool

	// Inline permanent-delete confirmation for trash rows.
	confirmPurge bool
	purgeTarget  *scopePickerEntry
}

const scopePickerMaxVisible = 12

// newScopePicker builds the picker from the active/archived project lists and the
// raw trash entries. The caller supplies each list so the overlay stays testable
// and never reaches into storage itself.
func newScopePicker(active, archived []*models.Project, trash []storage.TrashEntry) *ScopePickerModel {
	ti := textinput.New()
	ti.Placeholder = "Search projects & trash..."
	ti.CharLimit = 100
	applyThemedTextInput(&ti)

	entries := make([]scopePickerEntry, 0, len(active)+len(archived)+len(trash)+1)
	entries = append(entries, scopePickerEntry{kind: scopeEntryAllProjects})
	for _, p := range active {
		entries = append(entries, scopePickerEntry{kind: scopeEntryActive, project: p})
	}
	for _, p := range archived {
		entries = append(entries, scopePickerEntry{kind: scopeEntryArchived, project: p})
	}
	for _, t := range trash {
		entries = append(entries, scopePickerEntry{
			kind:         scopeEntryTrash,
			trashEntryID: t.EntryID,
			trashSlug:    t.Slug,
			trashName:    t.Name,
			trashDeleted: storage.FormatRelativeTime(t.DeletedAt),
		})
	}

	return &ScopePickerModel{
		entries:  entries,
		filtered: entries,
		input:    ti,
	}
}

func (m *ScopePickerModel) initCmd() tea.Cmd { return nil }

// current returns the entry under the cursor, or nil when the list is empty.
func (m *ScopePickerModel) current() *scopePickerEntry {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

func (m *ScopePickerModel) moveCursor(delta int) {
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if maxCursor := len(m.filtered) - 1; m.cursor > maxCursor {
		m.cursor = maxCursor
	}
}

func (m *ScopePickerModel) applyFilter() {
	query := strings.TrimSpace(m.input.Value())
	if query == "" {
		m.filtered = m.entries
	} else {
		out := make([]scopePickerEntry, 0, len(m.entries))
		for _, e := range m.entries {
			if fuzzy.MatchTerms(e.filterText(), query) {
				out = append(out, e)
			}
		}
		m.filtered = out
	}
	m.cursor = 0
}

// handleKey processes a key press, returning (inputCmd, dismiss, resultMsg) per
// the shared overlay contract used by dispatchOverlayKey.
func (m *ScopePickerModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	key := msg.String()

	// Permanent-delete confirmation captures every key until resolved.
	if m.confirmPurge {
		switch key {
		case "y", "Y":
			target := m.purgeTarget
			m.confirmPurge = false
			m.purgeTarget = nil
			if target != nil {
				return nil, true, PurgeFromTrashMsg{EntryID: target.trashEntryID, Slug: target.trashSlug}
			}
			return nil, false, nil
		case "n", "N", "esc":
			m.confirmPurge = false
			m.purgeTarget = nil
			return nil, false, nil
		default:
			return nil, false, nil
		}
	}

	if m.searching {
		switch key {
		case "esc":
			m.searching = false
			m.input.Blur()
			m.input.SetValue("")
			m.applyFilter()
			return nil, false, nil
		case "enter":
			m.searching = false
			m.input.Blur()
			return nil, false, nil
		case "up", "ctrl+k", "ctrl+p":
			m.moveCursor(-1)
			return nil, false, nil
		case "down", "ctrl+j", "ctrl+n":
			m.moveCursor(1)
			return nil, false, nil
		default:
			prev := m.input.Value()
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			if m.input.Value() != prev {
				m.applyFilter()
			}
			return cmd, false, nil
		}
	}

	switch key {
	case "esc":
		if m.input.Value() != "" {
			m.input.SetValue("")
			m.applyFilter()
			return nil, false, nil
		}
		return nil, true, nil
	case "/":
		m.searching = true
		return m.input.Focus(), false, nil
	case "enter":
		return m.activateCurrent()
	case "j", "down", "ctrl+n":
		m.moveCursor(1)
		return nil, false, nil
	case "k", "up", "ctrl+p":
		m.moveCursor(-1)
		return nil, false, nil
	case "ctrl+d":
		m.moveCursor(scopePickerMaxVisible)
		return nil, false, nil
	case "ctrl+u":
		m.moveCursor(-scopePickerMaxVisible)
		return nil, false, nil
	case "x":
		// Permanent deletion is a trash-only, confirmation-gated action.
		if e := m.current(); e != nil && e.kind == scopeEntryTrash {
			target := *e
			m.purgeTarget = &target
			m.confirmPurge = true
		}
		return nil, false, nil
	case "X":
		// Lifecycle recovery: bring an archived project back to active.
		if e := m.current(); e != nil && e.kind == scopeEntryArchived && e.project != nil {
			return nil, true, ProjectArchiveRequestMsg{Project: e.project, Archive: false}
		}
		return nil, false, nil
	}
	return nil, false, nil
}

// activateCurrent runs the primary (Enter) action for the focused row.
func (m *ScopePickerModel) activateCurrent() (tea.Cmd, bool, tea.Msg) {
	e := m.current()
	if e == nil {
		return nil, true, nil
	}
	switch e.kind {
	case scopeEntryAllProjects:
		return nil, true, ApplyProjectScopeMsg{Clear: true}
	case scopeEntryActive:
		if e.project != nil {
			return nil, true, ApplyProjectScopeMsg{ProjectID: e.project.ID}
		}
		return nil, true, nil
	case scopeEntryArchived:
		// Archived projects are excluded from the cross Outliner's active-only
		// load, so scoping to one would create an immediately-empty work scope
		// that the next reload classifies as missing and unwinds. Enter therefore
		// performs the lifecycle recovery action — restore to active — after which
		// the project loads and can be scoped normally. Archived rows deliberately
		// never emit ApplyProjectScopeMsg.
		if e.project != nil {
			return nil, true, ProjectArchiveRequestMsg{Project: e.project, Archive: false}
		}
		return nil, true, nil
	case scopeEntryTrash:
		return nil, true, RestoreFromTrashMsg{EntryID: e.trashEntryID, Slug: e.trashSlug}
	}
	return nil, true, nil
}

func scopeSectionLabel(k scopeEntryKind) string {
	switch k {
	case scopeEntryActive:
		return "Active"
	case scopeEntryArchived:
		return "Archived"
	case scopeEntryTrash:
		return "Trash"
	}
	return ""
}

func (m *ScopePickerModel) rowLabel(e scopePickerEntry) string {
	switch e.kind {
	case scopeEntryAllProjects:
		return " ⊘ All Projects (clear scope)"
	case scopeEntryActive, scopeEntryArchived:
		icon := " "
		name := ""
		if e.project != nil {
			if ic := e.project.StatusIconEmoji(); ic != "" {
				icon = ic
			}
			name = e.project.Name
		}
		suffix := ""
		if e.kind == scopeEntryArchived {
			suffix = MutedStyle.Render(" [archived]")
		}
		return fmt.Sprintf(" %s %s%s", icon, name, suffix)
	case scopeEntryTrash:
		return fmt.Sprintf(" 🗑 %s %s", e.trashSlug, MutedStyle.Render("(deleted "+e.trashDeleted+")"))
	}
	return ""
}

func (m *ScopePickerModel) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Project Scope & Administration")
	footer := "↑/↓ move · enter act · X/x admin · / find · esc close"
	switch {
	case m.confirmPurge:
		footer = "y: permanently delete · n/esc: cancel"
	case m.searching:
		footer = "type filter · ↑/↓ move · enter commit · esc clear"
	case m.input.Value() != "":
		footer = "↑/↓ move · enter act · X/x admin · / edit · esc clear"
	}
	footerText := HelpStyle.Render(footer)

	innerWidth := max(min(max(width*3/5, 44), width-4), 1)
	m.input.SetWidth(max(innerWidth-2, 1))

	var content strings.Builder
	if m.searching || m.input.Value() != "" {
		content.WriteString(m.input.View())
	} else {
		content.WriteString(MutedStyle.Render("Press / to search"))
	}
	content.WriteString("\n")

	if m.confirmPurge && m.purgeTarget != nil {
		content.WriteString(PreviewWarningStyle.Render(
			fmt.Sprintf("PERMANENTLY delete '%s'? (y/n)", m.purgeTarget.trashSlug)))
		return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
	}

	if len(m.filtered) == 0 {
		content.WriteString(MutedStyle.Render("No matching projects or trash."))
		return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
	}

	start := 0
	if m.cursor >= scopePickerMaxVisible {
		start = m.cursor - scopePickerMaxVisible + 1
	}
	end := min(start+scopePickerMaxVisible, len(m.filtered))

	lastKind := scopeEntryKind(-1)
	for i := start; i < end; i++ {
		e := m.filtered[i]
		if e.kind != lastKind && e.kind != scopeEntryAllProjects {
			content.WriteString(MutedStyle.Render(" " + scopeSectionLabel(e.kind)))
			content.WriteString("\n")
		}
		lastKind = e.kind

		line := m.rowLabel(e)
		budget := max(innerWidth-2, 1)
		line = truncateIntentional(line, budget, "")
		if w := lipgloss.Width(line); w < budget {
			line += strings.Repeat(" ", budget-w)
		}
		if i == m.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < end-1 {
			content.WriteString("\n")
		}
	}

	if len(m.filtered) > scopePickerMaxVisible {
		content.WriteString("\n")
		content.WriteString(MutedStyle.Render(fmt.Sprintf(" %d/%d", m.cursor+1, len(m.filtered))))
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}

// applyProjectScope routes a scope request to the base cross-project Outliner.
// It is a no-op (with a status hint) when the Outliner is not the active base
// project view, because scope frames are owned by the global Outliner instance.
func (m *AppModel) applyProjectScope(msg ApplyProjectScopeMsg) (bool, tea.Cmd) {
	if m.baseProjectView() != ViewCrossProjectOutliner || m.crossOutliner == nil {
		m.setInfoStatus("Project scope is available from the global Outliner")
		return true, nil
	}
	if msg.Clear {
		m.crossOutliner.clearProjectScope()
		return true, nil
	}
	// Defensive lifecycle guard: the cross Outliner can only scope projects it
	// has actually loaded (active-only). Refusing an unloaded target here means
	// an archived or externally-removed project can never create an unresolved,
	// immediately-empty work scope that a later reload would unwind — regardless
	// of which caller emitted the request.
	if !m.crossOutliner.projectIsLoaded(msg.ProjectID) {
		m.setInfoStatus("That project isn't loaded — restore it before scoping")
		return true, nil
	}
	m.crossOutliner.pushProjectScope(msg.ProjectID)
	return true, nil
}

// crossOutlinerHeaderProject returns the project whose synthetic header is under
// the cursor in the global Outliner, or nil when the cursor is not on a project
// header (or the global Outliner is not the active base view). It backs the
// Shift+Enter contextual administration menu for project headers.
func (m *AppModel) crossOutlinerHeaderProject() *models.Project {
	if m.baseProjectView() != ViewCrossProjectOutliner || m.crossOutliner == nil {
		return nil
	}
	o := m.crossOutliner
	if o.cursor < 0 || o.cursor >= len(o.rows) {
		return nil
	}
	row := o.rows[o.cursor]
	if !row.IsProjectHeader {
		return nil
	}
	for _, p := range o.loadedProjects {
		if p != nil && p.ID == row.ProjectID {
			return p
		}
	}
	return nil
}

// archivedScopeProjects returns the archived projects for the scope picker's
// Archived section, preserving loaded task trees via allProjects().
func (m *AppModel) archivedScopeProjects() []*models.Project {
	var out []*models.Project
	for _, p := range m.allProjects() {
		if p != nil && p.Archived {
			out = append(out, p)
		}
	}
	return out
}
