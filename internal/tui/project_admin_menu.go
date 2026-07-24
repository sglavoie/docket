package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// projectAdminAction enumerates the contextual administration commands offered
// for a focused project header inside the Outliner.
type projectAdminAction int

const (
	adminActionPreview projectAdminAction = iota
	adminActionScope
	adminActionEdit
	adminActionStatus
	adminActionPin
	adminActionArchive
	adminActionNotes
)

type projectAdminMenuItem struct {
	action projectAdminAction
	label  string
}

// ProjectAdminMenuModel is the Shift+Enter overlay for a project header. It
// offers a preview (metrics) plus the contextual edit/status/pin/archive/notes
// lifecycle commands for one project, each dispatched through the existing
// project messages.
type ProjectAdminMenuModel struct {
	project *models.Project
	items   []projectAdminMenuItem
	cursor  int
}

// newProjectAdminMenu builds the contextual menu for a single project. Pin and
// archive labels reflect the project's current lifecycle state.
func newProjectAdminMenu(project *models.Project) *ProjectAdminMenuModel {
	pinLabel := "Pin project"
	if project.Pinned {
		pinLabel = "Unpin project"
	}
	archiveLabel := "Archive project"
	if project.Archived {
		archiveLabel = "Unarchive project"
	}
	items := []projectAdminMenuItem{
		{adminActionPreview, "Preview metrics"},
		{adminActionScope, "Scope to project"},
		{adminActionEdit, "Edit project"},
		{adminActionStatus, "Set status"},
		{adminActionPin, pinLabel},
		{adminActionArchive, archiveLabel},
		{adminActionNotes, "Open notes"},
	}
	return &ProjectAdminMenuModel{project: project, items: items}
}

func (m *ProjectAdminMenuModel) initCmd() tea.Cmd { return nil }

// handleKey implements the shared overlay contract (inputCmd, dismiss, resultMsg).
func (m *ProjectAdminMenuModel) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool, tea.Msg) {
	switch msg.String() {
	case "esc":
		return nil, true, nil
	case "j", "down", "ctrl+n":
		m.cursor = (m.cursor + 1) % len(m.items)
		return nil, false, nil
	case "k", "up", "ctrl+p":
		m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
		return nil, false, nil
	case "enter":
		return nil, true, m.dispatch()
	}
	return nil, false, nil
}

// dispatch maps the focused action to its project message.
func (m *ProjectAdminMenuModel) dispatch() tea.Msg {
	p := m.project
	if p == nil {
		return nil
	}
	switch m.items[m.cursor].action {
	case adminActionPreview:
		return ShowMetricsDashboardMsg{Project: p}
	case adminActionScope:
		return ApplyProjectScopeMsg{ProjectID: p.ID}
	case adminActionEdit:
		return EditProjectMsg{Project: p}
	case adminActionStatus:
		return ShowStatusInputMsg{Project: p}
	case adminActionPin:
		return ProjectPinnedToggledMsg{Project: p}
	case adminActionArchive:
		return ProjectArchiveRequestMsg{Project: p, Archive: !p.Archived}
	case adminActionNotes:
		return ShowNotesMsg{Project: p}
	}
	return nil
}

func (m *ProjectAdminMenuModel) render(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(ColorPrimary)
	name := ""
	if m.project != nil {
		name = m.project.Name
	}
	title := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Project: " + name)
	footerText := HelpStyle.Render("enter: run · ↑↓/j/k: navigate · esc: close")

	innerWidth := max(min(max(width*2/5, 34), width-4), 1)

	var content strings.Builder
	for i, item := range m.items {
		line := fmt.Sprintf(" %s", item.label)
		budget := max(innerWidth-2, 1)
		if w := lipgloss.Width(line); w < budget {
			line += strings.Repeat(" ", budget-w)
		}
		if i == m.cursor {
			line = SelectedStyle.Render(line)
		}
		content.WriteString(line)
		if i < len(m.items)-1 {
			content.WriteString("\n")
		}
	}

	return renderTitledBox(content.String(), innerWidth, title, footerText, borderStyle)
}
