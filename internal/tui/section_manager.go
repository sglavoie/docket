package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// SectionManagerModel is the model for the section management view.
type SectionManagerModel struct {
	projectTree   *ProjectTreeModel
	config        *config.Config
	configManager *config.Manager
	sections      []SectionInfo
	cursor        int
	width         int
	height        int
	renameMode    bool
	renameInput   textinput.Model
	addMode       bool
	addInput      textinput.Model
	errorMsg      string
	colorPickMode bool
	colorCursor   int // Index in SectionColorPalette
}

// ShowSectionManagerMsg is sent to open the section manager view.
type ShowSectionManagerMsg struct {
	ProjectTree  *ProjectTreeModel
	StartAddMode bool // If true, open directly in add mode
}

// CloseSectionManagerMsg is sent when the section manager should close.
type CloseSectionManagerMsg struct{}

// SectionRenamedMsg is sent after a section has been renamed.
type SectionRenamedMsg struct {
	OldName string
	NewName string
}

// SectionAddedMsg is sent after a section has been added.
type SectionAddedMsg struct {
	Name string
}

// SectionRemovedMsg is sent after a section has been removed.
type SectionRemovedMsg struct {
	Name string
}

// SectionColorChangedMsg is sent after a section's color has been changed.
type SectionColorChangedMsg struct {
	SectionName string
	ColorName   string
}

// NewSectionManagerModel creates a new section manager model.
func NewSectionManagerModel(pt *ProjectTreeModel, cfg *config.Config, mgr *config.Manager, startAddMode bool) *SectionManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new section name"
	renameInput.CharLimit = 50
	renameInput.Width = 30

	addInput := textinput.New()
	addInput.Placeholder = "section name"
	addInput.CharLimit = 50
	addInput.Width = 30

	m := &SectionManagerModel{
		projectTree:   pt,
		config:        cfg,
		configManager: mgr,
		cursor:        0,
		renameInput:   renameInput,
		addInput:      addInput,
		addMode:       startAddMode,
	}
	if startAddMode {
		m.addInput.Focus()
	}
	m.buildSectionList()
	return m
}

// SetSize sets the view dimensions.
func (m *SectionManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildSectionList builds a sorted list of custom sections with counts.
// GTD sections (Next, Inbox, Waiting, Someday) are excluded as they are
// managed via view modes, not the section manager.
func (m *SectionManagerModel) buildSectionList() {
	sectionCounts := m.projectTree.project.CountTasksBySection()

	// Get sections in project order, excluding GTD sections
	projectSections := m.projectTree.project.GetSections()

	m.sections = make([]SectionInfo, 0, len(projectSections))
	for _, name := range projectSections {
		if models.IsGTDSection(name) {
			continue // Skip GTD sections
		}
		count := sectionCounts[name]
		m.sections = append(m.sections, SectionInfo{Name: name, TaskCount: count})
	}

	// Clamp cursor
	if m.cursor >= len(m.sections) {
		m.cursor = len(m.sections) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// addSection adds a new section to the project.
func (m *SectionManagerModel) addSection(name string) tea.Cmd {
	return func() tea.Msg {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}

		// Block GTD section names
		if models.IsGTDSection(name) {
			return ErrorMsg{Err: fmt.Errorf("'%s' is a built-in GTD section", name)}
		}

		// Check if section already exists
		if m.projectTree.project.HasSection(name) {
			return ErrorMsg{Err: fmt.Errorf("section '%s' already exists", name)}
		}

		m.projectTree.project.EnsureSection(name)

		// Apply global color if one exists for this section name
		if globalColor := m.config.GetGlobalSectionColor(name); globalColor != "gray" {
			m.projectTree.project.SetSectionColor(name, globalColor)
		}

		if err := saveProjectSync(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionAddedMsg{Name: name}
	}
}

// renameSection renames a section.
func (m *SectionManagerModel) renameSection(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		newName = strings.TrimSpace(newName)
		if newName == "" || newName == oldName {
			return nil
		}

		// Block renaming GTD sections (defensive check)
		if models.IsGTDSection(oldName) {
			return ErrorMsg{Err: fmt.Errorf("cannot rename GTD section '%s'", oldName)}
		}

		// Block renaming to a GTD section name
		if models.IsGTDSection(newName) {
			return ErrorMsg{Err: fmt.Errorf("'%s' is a built-in GTD section", newName)}
		}

		// Check if new name already exists
		if m.projectTree.project.HasSection(newName) {
			return ErrorMsg{Err: fmt.Errorf("section '%s' already exists", newName)}
		}

		if !m.projectTree.project.RenameSection(oldName, newName) {
			return ErrorMsg{Err: fmt.Errorf("failed to rename section")}
		}

		// Apply global color if one exists for the new section name
		if globalColor := m.config.GetGlobalSectionColor(newName); globalColor != "gray" {
			m.projectTree.project.SetSectionColor(newName, globalColor)
		}

		if err := saveProjectSync(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionRenamedMsg{OldName: oldName, NewName: newName}
	}
}

// removeSection removes a section from the project.
func (m *SectionManagerModel) removeSection(name string) tea.Cmd {
	return func() tea.Msg {
		// Block removing GTD sections (defensive check)
		if models.IsGTDSection(name) {
			return ErrorMsg{Err: fmt.Errorf("cannot remove GTD section '%s'", name)}
		}

		if !m.projectTree.project.RemoveSection(name) {
			return ErrorMsg{Err: fmt.Errorf("failed to remove section")}
		}

		if err := saveProjectSync(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionRemovedMsg{Name: name}
	}
}

// filterBySection sets the project tree filter and closes the section manager.
func (m *SectionManagerModel) filterBySection(sectionName string) tea.Cmd {
	return func() tea.Msg {
		m.projectTree.filter.Sections = map[string]bool{sectionName: true}
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
		return CloseSectionManagerMsg{}
	}
}

// setSectionColor sets the color for the current section globally and propagates to all projects.
func (m *SectionManagerModel) setSectionColor(sectionName, colorName string) tea.Cmd {
	return func() tea.Msg {
		// Update global registry
		m.config.SetGlobalSectionColor(sectionName, colorName)

		// Save config first
		if err := m.configManager.Save(m.config); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save config: %w", err)}
		}

		// Propagate to all projects that have this section
		for _, proj := range m.config.Projects {
			// Load project data
			if err := storage.LoadProject(proj); err != nil {
				continue // Skip projects that can't be loaded
			}

			// Check if project has this section
			if proj.HasSection(sectionName) {
				proj.SetSectionColor(sectionName, colorName)
				// Save project to persist the color in tasks.md
				if err := storage.SaveProject(proj); err != nil {
					continue // Log error but continue with other projects
				}
			}
		}

		// Also update the current project in memory
		m.projectTree.project.SetSectionColor(sectionName, colorName)

		if err := saveProjectSync(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return SectionColorChangedMsg{SectionName: sectionName, ColorName: colorName}
	}
}

// Init initializes the model.
func (m *SectionManagerModel) Init() tea.Cmd {
	if m.addMode {
		return textinput.Blink
	}
	return nil
}

// Update handles messages.
func (m *SectionManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		// Close the section manager after successful operation
		return m, func() tea.Msg {
			return CloseSectionManagerMsg{}
		}

	case SectionColorChangedMsg:
		// Rebuild section list to show updated colors, stay in manager
		m.buildSectionList()
		m.colorPickMode = false
		return m, nil

	case ErrorMsg:
		m.errorMsg = msg.Err.Error()
		m.renameMode = false
		m.addMode = false
		m.colorPickMode = false
		m.renameInput.SetValue("")
		m.addInput.SetValue("")
		return m, nil

	case tea.KeyMsg:
		// Clear error on any key press
		m.errorMsg = ""

		if m.colorPickMode {
			return m.handleColorPickMode(msg)
		}
		if m.renameMode {
			return m.handleRenameMode(msg)
		}
		if m.addMode {
			return m.handleAddMode(msg)
		}

		switch msg.String() {
		case "q", "esc", "ctrl+s":
			return m, func() tea.Msg {
				return CloseSectionManagerMsg{}
			}

		case "j", "down":
			if len(m.sections) > 0 {
				m.cursor++
				if m.cursor >= len(m.sections) {
					m.cursor = 0
				}
			}
			return m, nil

		case "k", "up":
			if len(m.sections) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.sections) - 1
				}
			}
			return m, nil

		case "enter":
			if len(m.sections) > 0 && m.cursor < len(m.sections) {
				return m, m.filterBySection(m.sections[m.cursor].Name)
			}
			return m, nil

		case "a":
			m.addMode = true
			m.addInput.SetValue("")
			m.addInput.Focus()
			return m, textinput.Blink

		case "r":
			if len(m.sections) > 0 && m.cursor < len(m.sections) {
				section := m.sections[m.cursor]
				if models.IsGTDSection(section.Name) {
					m.errorMsg = "Cannot rename GTD sections"
					return m, nil
				}
				m.renameMode = true
				m.renameInput.SetValue(section.Name)
				m.renameInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case "d":
			if len(m.sections) > 0 && m.cursor < len(m.sections) {
				section := m.sections[m.cursor]
				if models.IsGTDSection(section.Name) {
					m.errorMsg = "Cannot remove GTD sections"
					return m, nil
				}
				return m, m.removeSection(section.Name)
			}
			return m, nil

		case "c":
			if len(m.sections) > 0 && m.cursor < len(m.sections) {
				section := m.sections[m.cursor]
				m.colorPickMode = true
				// Initialize cursor to current color
				currentColor := m.projectTree.project.GetSectionColor(section.Name)
				m.colorCursor = GetSectionColorIndex(currentColor)
				return m, nil
			}
			return m, nil
		}
	}

	return m, nil
}

// handleRenameMode handles keyboard input while in rename mode.
func (m *SectionManagerModel) handleRenameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renameMode = false
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			oldName := m.sections[m.cursor].Name
			newName := strings.TrimSpace(m.renameInput.Value())
			if newName != "" && newName != oldName {
				return m, m.renameSection(oldName, newName)
			}
		}
		m.renameMode = false
		m.renameInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// handleAddMode handles keyboard input while in add mode.
func (m *SectionManagerModel) handleAddMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addMode = false
		m.addInput.SetValue("")
		return m, nil

	case "enter":
		name := strings.TrimSpace(m.addInput.Value())
		if name != "" {
			return m, m.addSection(name)
		}
		m.addMode = false
		m.addInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	}
}

// handleColorPickMode handles keyboard input while in color pick mode.
func (m *SectionManagerModel) handleColorPickMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	paletteLen := len(SectionColorPalette)

	switch msg.String() {
	case "esc", "q":
		m.colorPickMode = false
		return m, nil

	case "enter":
		// Confirm color selection
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			sectionName := m.sections[m.cursor].Name
			colorName := GetSectionColorByIndex(m.colorCursor)
			m.colorPickMode = false
			return m, m.setSectionColor(sectionName, colorName)
		}
		m.colorPickMode = false
		return m, nil

	case "c":
		// Quick cycle to next color
		m.colorCursor = (m.colorCursor + 1) % paletteLen
		return m, nil

	case "h", "left":
		m.colorCursor = (m.colorCursor - 1 + paletteLen) % paletteLen
		return m, nil

	case "l", "right":
		m.colorCursor = (m.colorCursor + 1) % paletteLen
		return m, nil

	case "j", "down":
		// Jump forward 5 colors (half the palette)
		m.colorCursor = (m.colorCursor + 5) % paletteLen
		return m, nil

	case "k", "up":
		// Jump backward 5 colors
		m.colorCursor = (m.colorCursor - 5 + paletteLen) % paletteLen
		return m, nil
	}

	return m, nil
}

// View renders the section manager view.
func (m *SectionManagerModel) View() string {
	var b strings.Builder

	// Calculate box width
	boxWidth := 50
	if m.width > 0 && m.width < boxWidth+10 {
		boxWidth = m.width - 10
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Section Manager"))
	content.WriteString("\n\n")

	// Add mode input
	if m.addMode {
		content.WriteString("New section: ")
		content.WriteString(m.addInput.View())
		content.WriteString("\n\n")
	}

	if len(m.sections) == 0 {
		content.WriteString(MutedStyle.Render("No sections in project"))
		content.WriteString("\n")
	} else {
		for i, section := range m.sections {
			cursor := "  "
			style := TagManagerItemStyle
			if i == m.cursor {
				cursor = "> "
				style = TagManagerItemSelectedStyle
			}

			// Check if this is the section being renamed
			if m.renameMode && i == m.cursor {
				content.WriteString(cursor)
				content.WriteString("Rename to: ")
				content.WriteString(m.renameInput.View())
				content.WriteString("\n")
			} else {
				// Get section color and render swatch
				sectionColor := m.projectTree.project.GetSectionColor(section.Name)
				colorStyle := GetSectionColorStyle(sectionColor)
				colorSwatch := colorStyle.Render("█") + " "

				line := fmt.Sprintf("%s%s%s ", cursor, colorSwatch, section.Name)
				content.WriteString(style.Render(line))
				countStr := fmt.Sprintf("(%d)", section.TaskCount)
				content.WriteString(TagManagerCountStyle.Render(countStr))
				content.WriteString("\n")
			}
		}
	}

	// Show color picker when in color pick mode
	if m.colorPickMode {
		content.WriteString("\n")
		content.WriteString(MutedStyle.Render("Select color:"))
		content.WriteString("\n")

		for i, pc := range SectionColorPalette {
			style := lipgloss.NewStyle().Foreground(pc.Color)
			swatch := " █ "
			if i == m.colorCursor {
				// Highlight selected color with brackets
				swatch = "[█]"
				style = style.Bold(true)
			}
			content.WriteString(style.Render(swatch))
		}
		content.WriteString("\n")
		// Show selected color name
		selectedColor := SectionColorPalette[m.colorCursor]
		content.WriteString(MutedStyle.Render(selectedColor.Name))
		content.WriteString("\n")
	}

	// Error message
	if m.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		content.WriteString("\n")
	}

	// Footer hint
	content.WriteString("\n")
	if m.colorPickMode {
		content.WriteString(MutedStyle.Render("h/l nav | c cycle | enter confirm | esc cancel"))
	} else if m.renameMode {
		content.WriteString(MutedStyle.Render("enter confirm | esc cancel"))
	} else if m.addMode {
		content.WriteString(MutedStyle.Render("enter add | esc cancel"))
	} else if len(m.sections) > 0 {
		content.WriteString(MutedStyle.Render("j/k nav | enter filter | a add | r rename | c color | d del | q close"))
	} else {
		content.WriteString(MutedStyle.Render("a add | q close"))
	}

	// Apply box styling
	box := TagManagerBoxStyle.Width(boxWidth).Render(content.String())

	// Center the box
	boxHeight := lipgloss.Height(box)
	topPadding := (m.height - boxHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}
	leftPadding := (m.width - lipgloss.Width(box)) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	// Build final output with padding
	for i := 0; i < topPadding; i++ {
		b.WriteString("\n")
	}

	lines := strings.Split(box, "\n")
	padding := strings.Repeat(" ", leftPadding)
	for _, line := range lines {
		b.WriteString(padding)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
