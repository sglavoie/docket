package tui

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
)

// Regular contexts are project labels stored on the catalog, so the hub edits
// them as membership over the active projects plus a name. Everything commits
// through ContextMutator; this file only collects intent, resolves the
// destination a project needs when it leaves the default context, and reconciles
// the hub's own list afterwards.

// newRegularContextEditor builds both new and existing regular contexts. An
// existing label pre-checks its active projects and records hidden members so a
// save cannot relabel a project the user never saw. The personal editor locks
// only its name; membership remains intentionally editable.
func newRegularContextEditor(runtime *config.RuntimeState, label string) *contextEditor {
	editor := newContextEditor(runtime, label)
	editor.nameLocked = strings.EqualFold(label, runtime.DefaultContext())

	listed := make(map[string]bool, len(editor.projects))
	for _, project := range editor.projects {
		if project == nil {
			continue
		}
		listed[project.ID] = true
		if label != "" && project.Context == label {
			editor.selected[project.ID] = true
		}
	}
	for _, entry := range runtime.Catalog().Projects {
		if label != "" && entry.Context == label && !listed[entry.ID] {
			editor.hiddenMembers = append(editor.hiddenMembers, entry.ID)
		}
	}
	maps.Copy(editor.initial, editor.selected)
	return editor
}

// openRegularEditor starts on the project list: the label already has a name, so
// membership is what an edit is usually after.
func (m *ContextHubModel) openRegularEditor(label string) {
	editor := newRegularContextEditor(m.runtime, label)
	editor.setFocus(label != "")
	m.armEditor(editor)
}

// memberIDs is the full membership a save asserts: the checked active projects
// plus the members the list never showed.
func (e *contextEditor) memberIDs() []string {
	return append(e.selectedIDs(), e.hiddenMembers...)
}

// saveRegularEditor hands the rename and the membership to one checked commit.
// The editor presents them as a single save, so a failure must leave the label,
// its members, and the editor's own identity untouched rather than stranding a
// durable rename the user was told did not happen.
func (m *ContextHubModel) saveRegularEditor() tea.Cmd {
	editor := m.editor
	label := editor.originalName
	newName := editor.savedName()
	if newName == "" {
		editor.err = "context name cannot be empty"
		return nil
	}

	wasActive := label != "" && m.active == RegularContextScope(label)
	result, err := m.mutator.SaveRegularContext(RegularContextSaveRequest{
		OriginalName: label,
		Name:         newName,
		ProjectIDs:   editor.memberIDs(),
		Destination:  editor.destination,
	})
	if err != nil {
		if errors.Is(err, ErrDestinationContextRequired) {
			m.openDestinationPrompt(newName)
			return nil
		}
		editor.err = err.Error()
		return nil
	}

	// A save can leave any label with no active members, not only the one it
	// edited: a project it takes in vacates the label it came from. Such a label
	// is no longer derived, so neither the active marker nor the cursor may keep
	// pointing at it.
	saved := selectableContextScope(m.runtime, RegularContextScope(newName))
	if wasActive {
		m.active = saved
	} else {
		m.active = selectableContextScope(m.runtime, m.active)
	}
	m.closeEditor()
	m.refreshScopes(saved)
	return func() tea.Msg {
		return RegularContextSavedMsg{
			Name:           newName,
			PreviousName:   label,
			IsNew:          result.IsNew,
			Renamed:        result.Renamed,
			RelabeledCount: result.RelabeledCount,
			Assigned:       result.Assigned,
			Removed:        result.Removed,
			Destination:    result.Destination,
			WasActive:      wasActive,
		}
	}
}

// deleteRegularContext reassigns the label's projects to the default context.
// The default itself is refused by the mutator; the hub never offers it.
func (m *ContextHubModel) deleteRegularContext(name string) tea.Cmd {
	destination := m.runtime.DefaultContext()
	count, err := m.mutator.DeleteRegularContext(name)
	if err != nil {
		m.notice = err.Error()
		return nil
	}

	wasActive := m.active == RegularContextScope(name)
	if wasActive {
		m.active = AllContextScope()
	}
	m.refreshScopes(m.active)
	return func() tea.Msg {
		return RegularContextDeletedMsg{
			Name:         name,
			Destination:  destination,
			ProjectCount: count,
			WasActive:    wasActive,
		}
	}
}

// openDestinationPrompt asks where projects leaving the default context should
// go. Only the default reaches here: every other label falls back to it.
func (m *ContextHubModel) openDestinationPrompt(label string) {
	editor := m.editor
	editor.destinations = m.destinationCandidates(label, editor.originalName)
	editor.destCursor = 0
	editor.destPrompt = true
	editor.err = ""
}

// destinationCandidates lists the labels a project can move to, matched
// case-insensitively so one label never appears twice under different casing.
// Neither the label being saved nor the one an unsaved rename is vacating can
// receive anything, and the rename is still uncommitted here, so renamedFrom is
// excluded explicitly rather than by reading the catalog.
func (m *ContextHubModel) destinationCandidates(label, renamedFrom string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, entry := range m.runtime.Catalog().Projects {
		if entry.Context == "" ||
			strings.EqualFold(entry.Context, label) ||
			strings.EqualFold(entry.Context, renamedFrom) {
			continue
		}
		key := strings.ToLower(entry.Context)
		if seen[key] {
			continue
		}
		seen[key] = true
		names = append(names, entry.Context)
	}
	sort.Strings(names)
	return names
}

func (m *ContextHubModel) updateDestinationPrompt(msg tea.KeyPressMsg) tea.Cmd {
	editor := m.editor
	switch msg.String() {
	case "esc":
		editor.destPrompt = false
	case "j", "down":
		editor.destCursor = min(editor.destCursor+1, max(len(editor.destinations)-1, 0))
	case "k", "up":
		editor.destCursor = max(editor.destCursor-1, 0)
	case "enter":
		if editor.destCursor < 0 || editor.destCursor >= len(editor.destinations) {
			editor.destPrompt = false
			return nil
		}
		editor.destination = editor.destinations[editor.destCursor]
		editor.destPrompt = false
		return m.saveEditor()
	}
	return nil
}

func (m *ContextHubModel) destinationPromptView(width int) string {
	editor := m.editor
	// The prompt names the protected personal label the removals are leaving.
	label := editor.savedName()

	var b strings.Builder
	b.WriteString(SectionHeaderStyle.Render("Move projects out of " + label))
	b.WriteString("\n\n")
	b.WriteString(MutedStyle.Render(truncateToVisualWidth(
		fmt.Sprintf("%q is the default context, so projects leaving it need a destination.", label),
		width,
	)))
	b.WriteString("\n\n")
	for i, name := range editor.destinations {
		if i == editor.destCursor {
			line := IconCursorRight + " " + SelectedItemTextStyle.Render(name)
			b.WriteString(SelectionRowStyle.Width(width).Render(truncateToVisualWidth(line, width)))
		} else {
			b.WriteString(truncateToVisualWidth("  "+name, width))
		}
		b.WriteString("\n")
	}
	return b.String()
}
