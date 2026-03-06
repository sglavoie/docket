package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// CustomContextEditorModel handles creating/editing custom contexts.
type CustomContextEditorModel struct {
	config *config.Config

	// Editing state
	editMode     bool   // true if editing existing, false if creating new
	originalName string // Original name when editing

	// Form fields
	nameInput textinput.Model

	// Project selection
	projects        []*models.Project
	selectedIDs     map[string]bool // Project IDs that are selected
	cursor          int             // Cursor position in project list
	scrollOffset    int             // Scroll offset for project list
	projectListMode bool            // true when focus is on project list

	// View state
	width  int
	height int
	err    error

	// Discard confirmation
	confirmDiscard     bool
	confirmOverlay     ConfirmDialog
	initialSelectedIDs map[string]bool
}

// CustomContextSavedMsg is sent when a custom context is saved.
type CustomContextSavedMsg struct {
	Name         string
	IsNew        bool
	ProjectCount int
}

// NewCustomContextEditorModel creates a new custom context editor.
// If editName is empty, creates a new context. Otherwise edits existing.
func NewCustomContextEditorModel(cfg *config.Config, editName string) *CustomContextEditorModel {
	factory := DefaultFormFieldFactory()

	nameInput := factory.NewTextInput("Custom context name")
	nameInput.Focus()
	nameInput.CharLimit = 50

	projects := cfg.GetActiveProjects()
	selectedIDs := make(map[string]bool)

	editMode := editName != ""
	if editMode {
		nameInput.SetValue(editName)
		// Pre-select projects that are in the custom context
		existingIDs := cfg.GetCustomContext(editName)
		for _, id := range existingIDs {
			selectedIDs[id] = true
		}
	}

	initialSelectedIDs := make(map[string]bool, len(selectedIDs))
	for k, v := range selectedIDs {
		initialSelectedIDs[k] = v
	}

	return &CustomContextEditorModel{
		config:             cfg,
		editMode:           editMode,
		originalName:       editName,
		nameInput:          nameInput,
		projects:           projects,
		selectedIDs:        selectedIDs,
		cursor:             0,
		width:              80,
		height:             24,
		initialSelectedIDs: initialSelectedIDs,
	}
}

// SetSize sets the view dimensions.
func (m *CustomContextEditorModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	formFactory.ResizeTextInput(&m.nameInput, width)
}

func (m *CustomContextEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *CustomContextEditorModel) hasChanges() bool {
	if strings.TrimSpace(m.nameInput.Value()) != m.originalName {
		return true
	}
	if len(m.selectedIDs) != len(m.initialSelectedIDs) {
		return true
	}
	for k := range m.selectedIDs {
		if !m.initialSelectedIDs[k] {
			return true
		}
	}
	return false
}

func (m *CustomContextEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.projectListMode {
				// Exit project list mode, go back to name input
				m.projectListMode = false
				m.nameInput.Focus()
				return m, nil
			}
			if m.hasChanges() {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab":
			// Toggle between name input and project list
			if m.projectListMode {
				m.projectListMode = false
				m.nameInput.Focus()
			} else {
				m.projectListMode = true
				m.nameInput.Blur()
			}
			return m, nil

		case "enter":
			if m.projectListMode {
				// Toggle selection of current project
				if m.cursor < len(m.projects) {
					id := m.projects[m.cursor].ID
					m.selectedIDs[id] = !m.selectedIDs[id]
					if !m.selectedIDs[id] {
						delete(m.selectedIDs, id)
					}
				}
				return m, nil
			}
			// From name input, move to project list
			m.projectListMode = true
			m.nameInput.Blur()
			return m, nil

		case " ":
			// Space toggles selection in project list mode
			if m.projectListMode && m.cursor < len(m.projects) {
				id := m.projects[m.cursor].ID
				m.selectedIDs[id] = !m.selectedIDs[id]
				if !m.selectedIDs[id] {
					delete(m.selectedIDs, id)
				}
				return m, nil
			}

		case "up", "k":
			if m.projectListMode {
				if m.cursor > 0 {
					m.cursor--
					// Adjust scroll offset
					if m.cursor < m.scrollOffset {
						m.scrollOffset = m.cursor
					}
				}
				return m, nil
			}

		case "down", "j":
			if m.projectListMode {
				if m.cursor < len(m.projects)-1 {
					m.cursor++
					// Adjust scroll offset
					maxVisible := m.maxVisibleProjects()
					if m.cursor >= m.scrollOffset+maxVisible {
						m.scrollOffset = m.cursor - maxVisible + 1
					}
				}
				return m, nil
			}

		case "ctrl+s":
			return m, m.save()

		case "a":
			// Select all projects
			if m.projectListMode {
				for _, p := range m.projects {
					m.selectedIDs[p.ID] = true
				}
				return m, nil
			}

		case "A":
			// Deselect all projects
			if m.projectListMode {
				m.selectedIDs = make(map[string]bool)
				return m, nil
			}
		}
	}

	// Update name input if not in project list mode
	if !m.projectListMode {
		m.nameInput, cmd = m.nameInput.Update(msg)
	}

	return m, cmd
}

func (m *CustomContextEditorModel) maxVisibleProjects() int {
	// Reserve space for header, name input, help, etc.
	available := m.height - 15
	if available < 5 {
		available = 5
	}
	return available
}

func (m *CustomContextEditorModel) save() tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return ErrorMsg{Err: fmt.Errorf("custom context name cannot be empty")}
		}

		// Check for duplicate name (case-insensitive) when creating new
		if !m.editMode && m.config.CustomContextExists(name) {
			return ErrorMsg{Err: fmt.Errorf("custom context '%s' already exists", name)}
		}

		// When editing, check if name changed and new name already exists
		if m.editMode && !strings.EqualFold(name, m.originalName) && m.config.CustomContextExists(name) {
			return ErrorMsg{Err: fmt.Errorf("custom context '%s' already exists", name)}
		}

		// Collect selected project IDs
		var projectIDs []string
		for _, p := range m.projects {
			if m.selectedIDs[p.ID] {
				projectIDs = append(projectIDs, p.ID)
			}
		}

		if len(projectIDs) == 0 {
			return ErrorMsg{Err: fmt.Errorf("select at least one project")}
		}

		// Delete old name if we renamed
		if m.editMode && !strings.EqualFold(name, m.originalName) {
			m.config.DeleteCustomContext(m.originalName)
			// Also migrate cycle projects to new key
			oldKey := "+" + m.originalName
			newKey := "+" + name
			if oldCycleIDs := m.config.GetCycleProjectIDsForContext(oldKey); len(oldCycleIDs) > 0 {
				m.config.SetCycleProjectsForContext(newKey, oldCycleIDs)
				m.config.SetCycleProjectsForContext(oldKey, nil)
			}
		}

		// Save custom context
		m.config.SetCustomContext(name, projectIDs)

		// Auto-set cycle projects to match the custom context
		// This ensures cycling works immediately after creating/editing
		contextKey := "+" + name
		m.config.SetCycleProjectsForContext(contextKey, projectIDs)

		return CustomContextSavedMsg{
			Name:         name,
			IsNew:        !m.editMode,
			ProjectCount: len(projectIDs),
		}
	}
}

// HasTextInput returns true when the name input is focused.
func (m *CustomContextEditorModel) HasTextInput() bool {
	return !m.projectListMode
}

// ShouldCaptureEsc returns true to let the editor handle esc internally.
func (m *CustomContextEditorModel) ShouldCaptureEsc() bool {
	return true
}

func (m *CustomContextEditorModel) SupportsHelpShortcut() bool { return false }

func (m *CustomContextEditorModel) View() string {
	var b strings.Builder

	// Title
	if m.editMode {
		b.WriteString(renderFullscreenTitle("Edit Custom Context", m.originalName))
	} else {
		b.WriteString(renderFullscreenTitle("New Custom Context", ""))
	}
	b.WriteString("\n\n")

	// Name input
	var nameLabel string
	if !m.projectListMode {
		nameLabel = FieldLabelFocusedStyle.Render("Name: ")
	} else {
		nameLabel = FieldLabelStyle.Render("Name: ")
	}
	b.WriteString(nameLabel)
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Project selection label
	selectedCount := len(m.selectedIDs)
	var projectsLabel string
	if m.projectListMode {
		projectsLabel = FieldLabelFocusedStyle.Render(fmt.Sprintf("Projects (%d selected):", selectedCount))
	} else {
		projectsLabel = FieldLabelStyle.Render(fmt.Sprintf("Projects (%d selected):", selectedCount))
	}
	b.WriteString(projectsLabel)
	b.WriteString("\n")

	// Project list
	if len(m.projects) == 0 {
		b.WriteString(MutedStyle.Render("  No projects available"))
		b.WriteString("\n")
	} else {
		maxVisible := m.maxVisibleProjects()
		end := m.scrollOffset + maxVisible
		if end > len(m.projects) {
			end = len(m.projects)
		}

		var rowBuilder strings.Builder
		for i := m.scrollOffset; i < end; i++ {
			p := m.projects[i]

			// Cursor
			cursor := "  "
			if m.projectListMode && i == m.cursor {
				cursor = "> "
			}

			// Checkbox
			checkbox := "[ ]"
			if m.selectedIDs[p.ID] {
				checkbox = "[x]"
			}

			// Project info
			info := fmt.Sprintf("%s (%s) - %d tasks", p.Name, p.Context, p.CountTasks())

			// Style based on selection
			if m.projectListMode && i == m.cursor {
				rowBuilder.WriteString(TagManagerItemSelectedStyle.Render(fmt.Sprintf("%s%s %s", cursor, checkbox, info)))
			} else if m.selectedIDs[p.ID] {
				rowBuilder.WriteString(TagManagerItemSelectedStyle.Render(fmt.Sprintf("%s%s %s", cursor, checkbox, info)))
			} else {
				rowBuilder.WriteString(fmt.Sprintf("%s%s %s", cursor, checkbox, MutedStyle.Render(info)))
			}
			rowBuilder.WriteString("\n")
		}

		scrollbar := RenderScrollbar(len(m.projects), maxVisible, m.scrollOffset)
		b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))
	}
	b.WriteString("\n")

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	// Footer
	var footerLeft, footerRight string
	if m.projectListMode {
		footerLeft = HelpStyle.Render("j/k: move · Space: toggle · a/A: all/none · Tab: name")
		footerRight = HelpStyle.Render("Ctrl+S: save · Esc: cancel")
	} else {
		footerLeft = HelpStyle.Render("Tab/Enter: project list")
		footerRight = HelpStyle.Render("Ctrl+S: save · Esc: cancel")
	}
	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, m.width))

	return b.String()
}

func (m *CustomContextEditorModel) GetConfirmDialog() *ConfirmDialog {
	if m.confirmDiscard {
		m.confirmOverlay.Show("discard", "Discard unsaved changes?")
	} else {
		m.confirmOverlay.Cancel()
	}
	return &m.confirmOverlay
}

func (m *CustomContextEditorModel) StatusBarLabel() string { return "Custom Context Editor" }
