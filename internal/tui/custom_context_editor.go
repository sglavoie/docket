package tui

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// CustomContextEditorModel handles creating/editing custom contexts.
type CustomContextEditorModel struct {
	BaseSearchableView
	config *config.Config

	// Editing state
	editMode     bool   // true if editing existing, false if creating new
	originalName string // Original name when editing

	// Form fields
	nameInput textinput.Model

	// Project selection
	projects         []*models.Project
	filteredProjects []*models.Project
	selectedIDs      map[string]bool // Project IDs that are selected
	cursor           int             // Cursor position in project list
	scrollOffset     int             // Scroll offset for project list
	projectListMode  bool            // true when focus is on project list

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
	maps.Copy(initialSelectedIDs, selectedIDs)

	m := &CustomContextEditorModel{
		config:             cfg,
		editMode:           editMode,
		originalName:       editName,
		nameInput:          nameInput,
		projects:           projects,
		filteredProjects:   projects,
		selectedIDs:        selectedIDs,
		cursor:             0,
		initialSelectedIDs: initialSelectedIDs,
	}
	m.BaseSearchableView.SetSize(80, 24)
	m.InitSearchInput("Search projects...", 50, 80)
	return m
}

// SetSize sets the view dimensions.
func (m *CustomContextEditorModel) SetSize(width, height int) {
	m.BaseSearchableView.SetSize(width, height)
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

func (m *CustomContextEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	var cmd tea.Cmd

	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
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

	case tea.KeyPressMsg:
		// Handle search mode input first
		if m.IsSearchMode() {
			_, cmdHandleSearch, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterProjects()
			}
			return m, cmdHandleSearch
		}

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

		case "/":
			if m.projectListMode {
				return m, m.ActivateSearch()
			}

		case "enter":
			if m.projectListMode {
				// Toggle selection of current project
				if m.cursor < len(m.filteredProjects) {
					id := m.filteredProjects[m.cursor].ID
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

		case "space":
			// Space toggles selection in project list mode
			if m.projectListMode && m.cursor < len(m.filteredProjects) {
				id := m.filteredProjects[m.cursor].ID
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
				if m.cursor < len(m.filteredProjects)-1 {
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
			// Select all visible (filtered) projects
			if m.projectListMode {
				for _, p := range m.filteredProjects {
					m.selectedIDs[p.ID] = true
				}
				return m, nil
			}

		case "A":
			// Deselect all visible (filtered) projects
			if m.projectListMode {
				for _, p := range m.filteredProjects {
					delete(m.selectedIDs, p.ID)
				}
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

func (m *CustomContextEditorModel) filterProjects() {
	query := strings.ToLower(m.SearchQuery())
	if query == "" {
		m.filteredProjects = m.projects
	} else {
		m.filteredProjects = nil
		for _, p := range m.projects {
			if strings.Contains(strings.ToLower(p.Name), query) ||
				strings.Contains(strings.ToLower(p.Context), query) {
				m.filteredProjects = append(m.filteredProjects, p)
			}
		}
	}
	m.cursor = 0
	m.scrollOffset = 0
}

func (m *CustomContextEditorModel) maxVisibleProjects() int {
	// Reserve space for header, name input, help, etc.
	reserved := 15
	if m.IsSearchMode() || m.SearchQuery() != "" {
		reserved++ // Extra line for search input/filter indicator
	}
	return max(m.height-reserved, 5)
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

		// Update cycle projects for this context
		contextKey := "+" + name
		if m.editMode {
			// Preserve existing cycle selections, adjusting for added/removed projects
			existingCycleIDs := m.config.GetCycleProjectIDsForContext(contextKey)
			newProjectSet := make(map[string]bool, len(projectIDs))
			for _, id := range projectIDs {
				newProjectSet[id] = true
			}
			oldProjectSet := make(map[string]bool, len(m.initialSelectedIDs))
			for id, sel := range m.initialSelectedIDs {
				if sel {
					oldProjectSet[id] = true
				}
			}
			// Keep existing cycle IDs that are still in the context
			var updatedCycleIDs []string
			for _, id := range existingCycleIDs {
				if newProjectSet[id] {
					updatedCycleIDs = append(updatedCycleIDs, id)
				}
			}
			// Add newly added projects (not in old context) to cycle
			for _, id := range projectIDs {
				if !oldProjectSet[id] {
					updatedCycleIDs = append(updatedCycleIDs, id)
				}
			}
			m.config.SetCycleProjectsForContext(contextKey, updatedCycleIDs)
		} else {
			// New context: select all projects by default
			m.config.SetCycleProjectsForContext(contextKey, projectIDs)
		}

		return CustomContextSavedMsg{
			Name:         name,
			IsNew:        !m.editMode,
			ProjectCount: len(projectIDs),
		}
	}
}

// HasTextInput returns true when the name input or search input is focused.
func (m *CustomContextEditorModel) HasTextInput() bool {
	return !m.projectListMode || m.IsSearchMode()
}

// ShouldCaptureEsc returns true to let the editor handle esc internally.
func (m *CustomContextEditorModel) ShouldCaptureEsc() bool { return true }

// IsFullHeight returns false; the context editor is a panel overlay, not full-screen.
func (m *CustomContextEditorModel) IsFullHeight() bool { return false }

func (m *CustomContextEditorModel) SupportsHelpShortcut() bool { return false }

func (m *CustomContextEditorModel) View() tea.View {
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

	// Search input (shown when search is active or has a query)
	if m.IsSearchMode() {
		b.WriteString("  " + m.SearchInputView())
		b.WriteString("\n")
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  Filter: %s (%d/%d)", m.SearchQuery(), len(m.filteredProjects), len(m.projects))))
		b.WriteString("\n")
	}

	// Project list
	if len(m.filteredProjects) == 0 {
		if m.SearchQuery() != "" {
			b.WriteString(MutedStyle.Render("  No matching projects"))
		} else {
			b.WriteString(MutedStyle.Render("  No projects available"))
		}
		b.WriteString("\n")
	} else {
		maxVisible := m.maxVisibleProjects()
		end := min(m.scrollOffset+maxVisible, len(m.filteredProjects))

		var rowBuilder strings.Builder
		for i := m.scrollOffset; i < end; i++ {
			p := m.filteredProjects[i]

			// Cursor
			cursor := "  "
			if m.projectListMode && i == m.cursor {
				cursor = "→ "
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
				fmt.Fprintf(&rowBuilder, "%s%s %s", cursor, checkbox, MutedStyle.Render(info))
			}
			rowBuilder.WriteString("\n")
		}

		scrollbar := RenderScrollbar(len(m.filteredProjects), maxVisible, m.scrollOffset)
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
	if m.IsSearchMode() {
		footerLeft = HelpStyle.Render("Type to filter · Enter: confirm · Esc: clear")
		footerRight = HelpStyle.Render("")
	} else if m.projectListMode {
		footerLeft = HelpStyle.Render("j/k: move · Space: toggle · /: search · a/A: all/none · Tab: name")
		footerRight = HelpStyle.Render("Ctrl+S: save · Esc: cancel")
	} else {
		footerLeft = HelpStyle.Render("Tab/Enter: project list")
		footerRight = HelpStyle.Render("Ctrl+S: save · Esc: cancel")
	}
	b.WriteString(renderStatusBarFooter(footerLeft, footerRight, m.width))

	return tea.NewView(b.String())
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
