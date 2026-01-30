package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// GlobalBlockerEntry pairs a blocker with its source project.
type GlobalBlockerEntry struct {
	Blocker *models.Blocker
	Project *models.Project
}

// GlobalBlockersModel displays and manages blockers across all active projects.
type GlobalBlockersModel struct {
	config *config.Config

	entries []GlobalBlockerEntry
	cursor  int

	activeTab int

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int // index into entries
	editTextInput textarea.Model

	// Delete confirmation
	confirmDelete bool

	// Expanded view mode
	expandedView bool

	// Editor config
	editor string

	// Dimensions
	width  int
	height int

	// Error display
	err error
}

// NewGlobalBlockersModel loads blockers from all active projects.
func NewGlobalBlockersModel(cfg *config.Config) *GlobalBlockersModel {
	ti := textarea.New()
	ti.Placeholder = "Enter blocker title..."
	ti.CharLimit = 500
	ti.SetHeight(2)
	ti.ShowLineNumbers = false

	editTi := textarea.New()
	editTi.Placeholder = "Edit blocker..."
	editTi.CharLimit = 2000
	editTi.SetHeight(6)
	editTi.ShowLineNumbers = false

	m := &GlobalBlockersModel{
		config:        cfg,
		activeTab:     blockersTabActive,
		editor:        cfg.Editor,
		textInput:     ti,
		editTextInput: editTi,
	}
	m.loadAllBlockers()
	return m
}

func (m *GlobalBlockersModel) loadAllBlockers() {
	m.entries = nil
	for _, project := range m.config.GetActiveProjects() {
		blockers, err := storage.LoadBlockers(project)
		if err != nil {
			continue
		}
		for _, b := range blockers {
			m.entries = append(m.entries, GlobalBlockerEntry{
				Blocker: b,
				Project: project,
			})
		}
	}
}

// getProjectBlockers returns all blockers for a given project from the entries slice.
func (m *GlobalBlockersModel) getProjectBlockers(project *models.Project) []*models.Blocker {
	var blockers []*models.Blocker
	for _, e := range m.entries {
		if e.Project == project {
			blockers = append(blockers, e.Blocker)
		}
	}
	return blockers
}

func (m *GlobalBlockersModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *GlobalBlockersModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textInput.SetWidth(width - 4)
	m.editTextInput.SetWidth(width - 4)
}

func (m *GlobalBlockersModel) getVisibleEntries() []GlobalBlockerEntry {
	switch m.activeTab {
	case blockersTabResolved:
		return m.getResolved()
	case blockersTabAll:
		result := make([]GlobalBlockerEntry, len(m.entries))
		copy(result, m.entries)
		return result
	default:
		return m.getActive()
	}
}

func (m *GlobalBlockersModel) getActive() []GlobalBlockerEntry {
	var result []GlobalBlockerEntry
	for _, e := range m.entries {
		if !e.Blocker.IsResolved() {
			result = append(result, e)
		}
	}
	return result
}

func (m *GlobalBlockersModel) getResolved() []GlobalBlockerEntry {
	var result []GlobalBlockerEntry
	for _, e := range m.entries {
		if e.Blocker.IsResolved() {
			result = append(result, e)
		}
	}
	return result
}

func (m *GlobalBlockersModel) activeCount() int {
	count := 0
	for _, e := range m.entries {
		if !e.Blocker.IsResolved() {
			count++
		}
	}
	return count
}

func (m *GlobalBlockersModel) resolvedCount() int {
	count := 0
	for _, e := range m.entries {
		if e.Blocker.IsResolved() {
			count++
		}
	}
	return count
}

func (m *GlobalBlockersModel) findEntryIndex(blocker *models.Blocker) int {
	for i, e := range m.entries {
		if e.Blocker.ID == blocker.ID {
			return i
		}
	}
	return -1
}

// saveProjectBlockers persists all blockers for a given project.
func (m *GlobalBlockersModel) saveProjectBlockers(project *models.Project) error {
	blockers := m.getProjectBlockers(project)
	return storage.SaveBlockers(project, blockers)
}

func (m *GlobalBlockersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Handle expanded view mode
	if m.expandedView {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "enter", "q":
				m.expandedView = false
				return m, nil
			}
		}
		return m, nil
	}

	// Handle delete confirmation mode
	if m.confirmDelete {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y", "Y":
				m.confirmDelete = false
				return m, m.executeDelete()
			case "n", "N", "esc":
				m.confirmDelete = false
				m.err = nil
				return m, nil
			}
		}
		return m, nil
	}

	// Handle edit mode
	if m.editMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.editMode = false
				m.editTextInput.Reset()
				return m, nil
			case "ctrl+s":
				return m, m.saveEditedBlocker()
			case "ctrl+o":
				return m, m.editBlockerInExternalEditor()
			}
		}
		m.editTextInput, cmd = m.editTextInput.Update(msg)
		return m, cmd
	}

	// Handle add mode
	if m.addMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.addMode = false
				m.textInput.Reset()
				return m, nil
			case "ctrl+s":
				return m, m.saveBlocker()
			case "ctrl+o":
				return m, m.addBlockerInExternalEditor()
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	// Normal mode
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		visible := m.getVisibleEntries()

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseGlobalBlockersMsg{} }

		case "j", "down":
			if len(visible) > 0 && m.cursor < len(visible)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			if len(visible) > 0 && m.cursor < len(visible) {
				return m, func() tea.Msg {
					return BlockerGoToProjectMsg{Project: visible[m.cursor].Project}
				}
			}

		case "G":
			if len(visible) > 0 {
				m.cursor = len(visible) - 1
			}

		case "a", "n":
			m.addMode = true
			m.textInput.Focus()
			return m, textarea.Blink

		case "e":
			if len(visible) > 0 && m.cursor < len(visible) {
				return m, m.startEditBlocker(visible[m.cursor])
			}

		case "E":
			if len(visible) > 0 && m.cursor < len(visible) {
				m.editIndex = m.findEntryIndex(visible[m.cursor].Blocker)
				return m, m.editInExternalEditor()
			}

		case "R":
			if len(visible) > 0 && m.cursor < len(visible) {
				return m, m.toggleResolve(visible[m.cursor])
			}

		case "d", "x":
			if len(visible) > 0 && m.cursor < len(visible) {
				m.confirmDelete = true
				return m, nil
			}

		case "[", "h":
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(blockersTabNames) - 1
			}
			visible = m.getVisibleEntries()
			if m.cursor >= len(visible) {
				m.cursor = len(visible) - 1
				if m.cursor < 0 {
					m.cursor = 0
				}
			}

		case "]", "l":
			if m.activeTab < len(blockersTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			visible = m.getVisibleEntries()
			if m.cursor >= len(visible) {
				m.cursor = len(visible) - 1
				if m.cursor < 0 {
					m.cursor = 0
				}
			}

		case "enter":
			if len(visible) > 0 && m.cursor < len(visible) {
				m.expandedView = true
				return m, nil
			}
		}
	}

	return m, nil
}

// selectedProject returns the project of the currently selected blocker, or the first active project.
func (m *GlobalBlockersModel) selectedProject() *models.Project {
	visible := m.getVisibleEntries()
	if len(visible) > 0 && m.cursor < len(visible) {
		return visible[m.cursor].Project
	}
	projects := m.config.GetActiveProjects()
	if len(projects) > 0 {
		return projects[0]
	}
	return nil
}

func (m *GlobalBlockersModel) saveBlocker() tea.Cmd {
	return func() tea.Msg {
		title := strings.TrimSpace(m.textInput.Value())
		if title == "" {
			m.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		project := m.selectedProject()
		if project == nil {
			m.err = fmt.Errorf("no project available")
			return nil
		}

		blocker := models.NewBlocker(title)
		m.entries = append(m.entries, GlobalBlockerEntry{
			Blocker: blocker,
			Project: project,
		})

		if err := m.saveProjectBlockers(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		m.addMode = false
		m.textInput.Reset()
		m.err = nil
		m.cursor = len(m.getActive()) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}

		return BlockerAddedMsg{Project: project}
	}
}

func (m *GlobalBlockersModel) executeDelete() tea.Cmd {
	return func() tea.Msg {
		visible := m.getVisibleEntries()
		if m.cursor >= len(visible) {
			return nil
		}

		entry := visible[m.cursor]
		idx := m.findEntryIndex(entry.Blocker)
		if idx < 0 {
			return nil
		}

		project := entry.Project
		m.entries = append(m.entries[:idx], m.entries[idx+1:]...)

		if err := m.saveProjectBlockers(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		visible = m.getVisibleEntries()
		if m.cursor >= len(visible) && m.cursor > 0 {
			m.cursor--
		}

		return BlockerDeletedMsg{Project: project}
	}
}

func (m *GlobalBlockersModel) startEditBlocker(entry GlobalBlockerEntry) tea.Cmd {
	m.editMode = true
	m.editIndex = m.findEntryIndex(entry.Blocker)

	content := entry.Blocker.Title
	if entry.Blocker.Description != "" {
		content += "\n\n" + entry.Blocker.Description
	}
	m.editTextInput.SetValue(content)
	m.editTextInput.Focus()
	return textarea.Blink
}

func (m *GlobalBlockersModel) saveEditedBlocker() tea.Cmd {
	return func() tea.Msg {
		content := strings.TrimSpace(m.editTextInput.Value())
		if content == "" {
			m.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		lines := strings.SplitN(content, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			m.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		entry := m.entries[m.editIndex]
		entry.Blocker.Title = title
		entry.Blocker.Description = description

		if err := m.saveProjectBlockers(entry.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		m.editMode = false
		m.editTextInput.Reset()
		m.err = nil

		return BlockerEditedMsg{Project: entry.Project}
	}
}

func (m *GlobalBlockersModel) toggleResolve(entry GlobalBlockerEntry) tea.Cmd {
	return func() tea.Msg {
		idx := m.findEntryIndex(entry.Blocker)
		if idx < 0 {
			return nil
		}

		if entry.Blocker.IsResolved() {
			m.entries[idx].Blocker.Unresolve()
		} else {
			m.entries[idx].Blocker.Resolve()
		}

		if err := m.saveProjectBlockers(entry.Project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		if m.activeTab == blockersTabActive || m.activeTab == blockersTabResolved {
			visible := m.getVisibleEntries()
			if m.cursor >= len(visible) && m.cursor > 0 {
				m.cursor--
			}
		}

		return BlockerEditedMsg{Project: entry.Project}
	}
}

func (m *GlobalBlockersModel) editInExternalEditor() tea.Cmd {
	entry := m.entries[m.editIndex]
	blocker := entry.Blocker

	content := fmt.Sprintf("# Blocker: %s\n\n", blocker.Title)
	content += "<!-- Edit the title on the first line below, then description after a blank line. -->\n\n"
	content += blocker.Title
	if blocker.Description != "" {
		content += "\n\n" + blocker.Description
	}

	tmpFile, err := os.CreateTemp("", "tt-blocker-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	editor := m.editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	entries := m.entries
	editIndex := m.editIndex
	project := entry.Project

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		newContent := util.ParseEditedCommentContent(string(modified))
		if newContent == "" {
			return ErrorMsg{Err: fmt.Errorf("blocker title cannot be empty")}
		}

		lines := strings.SplitN(newContent, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			return ErrorMsg{Err: fmt.Errorf("blocker title cannot be empty")}
		}

		entries[editIndex].Blocker.Title = title
		entries[editIndex].Blocker.Description = description

		// Save all blockers for this project
		var blockers []*models.Blocker
		for _, e := range entries {
			if e.Project == project {
				blockers = append(blockers, e.Blocker)
			}
		}
		if err := storage.SaveBlockers(project, blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		return BlockerEditedMsg{Project: project}
	})
}

func (m *GlobalBlockersModel) addBlockerInExternalEditor() tea.Cmd {
	project := m.selectedProject()
	if project == nil {
		m.err = fmt.Errorf("no project available")
		return nil
	}

	currentText := strings.TrimSpace(m.textInput.Value())

	content := fmt.Sprintf("# New Blocker for %s\n\n", project.Name)
	content += "<!-- Enter the blocker title on the first line, then description after a blank line. -->\n\n"
	content += currentText

	tmpFile, err := os.CreateTemp("", "tt-blocker-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	editor := m.editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	entries := &m.entries
	targetProject := project

	m.addMode = false
	m.textInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		newContent := util.ParseEditedCommentContent(string(modified))
		if newContent == "" {
			return nil
		}

		lines := strings.SplitN(newContent, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			return nil
		}

		blocker := models.NewBlocker(title)
		blocker.Description = description
		*entries = append(*entries, GlobalBlockerEntry{
			Blocker: blocker,
			Project: targetProject,
		})

		var blockers []*models.Blocker
		for _, e := range *entries {
			if e.Project == targetProject {
				blockers = append(blockers, e.Blocker)
			}
		}
		if err := storage.SaveBlockers(targetProject, blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		return BlockerAddedMsg{Project: targetProject}
	})
}

func (m *GlobalBlockersModel) editBlockerInExternalEditor() tea.Cmd {
	entry := m.entries[m.editIndex]
	blocker := entry.Blocker
	currentText := strings.TrimSpace(m.editTextInput.Value())

	content := fmt.Sprintf("# Blocker: %s\n\n", blocker.Title)
	content += "<!-- Edit the title on the first line below, then description after a blank line. -->\n\n"
	content += currentText

	tmpFile, err := os.CreateTemp("", "tt-blocker-*.md")
	if err != nil {
		m.err = fmt.Errorf("failed to create temp file: %w", err)
		return nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(content)
	_ = tmpFile.Close()

	editor := m.editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "code"
	}

	var c *exec.Cmd
	if editor == "code" {
		c = exec.Command(editor, "--wait", tmpPath)
	} else {
		c = exec.Command(editor, tmpPath)
	}

	entries := m.entries
	editIndex := m.editIndex
	project := entry.Project
	originalTitle := blocker.Title
	originalDesc := blocker.Description

	m.editMode = false
	m.editTextInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		newContent := util.ParseEditedCommentContent(string(modified))
		if newContent == "" {
			return ErrorMsg{Err: fmt.Errorf("blocker title cannot be empty")}
		}

		lines := strings.SplitN(newContent, "\n", 2)
		title := strings.TrimSpace(lines[0])
		description := ""
		if len(lines) > 1 {
			description = strings.TrimSpace(lines[1])
		}

		if title == "" {
			return ErrorMsg{Err: fmt.Errorf("blocker title cannot be empty")}
		}

		if title == originalTitle && description == originalDesc {
			return nil
		}

		entries[editIndex].Blocker.Title = title
		entries[editIndex].Blocker.Description = description

		var blockers []*models.Blocker
		for _, e := range entries {
			if e.Project == project {
				blockers = append(blockers, e.Blocker)
			}
		}
		if err := storage.SaveBlockers(project, blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		return BlockerEditedMsg{Project: project}
	})
}

func (m *GlobalBlockersModel) renderTabBar() string {
	cardWidth := min(m.width-6, 54)
	availableWidth := cardWidth - 2

	plainStyle := lipgloss.NewStyle().Padding(0, 1)
	totalWidth := 0
	for _, name := range blockersTabNames {
		totalWidth += lipgloss.Width(plainStyle.Render(name))
	}
	fitsSingleLine := totalWidth <= availableWidth

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)

	if fitsSingleLine {
		activeStyle = activeStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(ColorPrimary)
		inactiveStyle = inactiveStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder)
	}

	var tabs []string
	for i, name := range blockersTabNames {
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	if fitsSingleLine {
		tabWidth := lipgloss.Width(row)
		remaining := availableWidth - tabWidth
		if remaining > 0 {
			filler := lipgloss.NewStyle().
				BorderBottom(true).
				BorderStyle(lipgloss.Border{Bottom: "─"}).
				BorderForeground(ColorBorder).
				Width(remaining).
				Render("")
			row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
		}
	}

	return row
}

func (m *GlobalBlockersModel) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("Blockers Across All Projects"))
	b.WriteString("\n")

	b.WriteString(MutedStyle.Render(fmt.Sprintf("%d active, %d resolved", m.activeCount(), m.resolvedCount())))
	b.WriteString("\n\n")

	// Expanded view
	if m.expandedView {
		visible := m.getVisibleEntries()
		if len(visible) > 0 && m.cursor < len(visible) {
			entry := visible[m.cursor]
			blocker := entry.Blocker
			relativeTime := storage.FormatRelativeTime(blocker.CreatedAt)

			b.WriteString(FieldLabelFocusedStyle.Render(blocker.Title))
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", entry.Project.Name)))
			b.WriteString("\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Created: %s (%s)", blocker.CreatedAt.Format("2006-01-02 15:04"), relativeTime)))
			if blocker.IsResolved() {
				resolvedTime := storage.FormatRelativeTime(*blocker.ResolvedAt)
				b.WriteString("\n")
				b.WriteString(DoneStyle.Render(fmt.Sprintf("Resolved: %s (%s)", blocker.ResolvedAt.Format("2006-01-02 15:04"), resolvedTime)))
			}
			b.WriteString("\n\n")

			if blocker.Description != "" {
				maxLen := m.width - 4
				if maxLen < 40 {
					maxLen = 40
				}
				lines := strings.Split(blocker.Description, "\n")
				for _, line := range lines {
					for len(line) > maxLen {
						b.WriteString(line[:maxLen])
						b.WriteString("\n")
						line = line[maxLen:]
					}
					b.WriteString(line)
					b.WriteString("\n")
				}
			} else {
				b.WriteString(MutedStyle.Render("No description."))
				b.WriteString("\n")
			}

			b.WriteString("\n")
			b.WriteString(HelpStyle.Render("Enter/Esc: close expanded view"))
		}
		return b.String()
	}

	// Edit mode
	if m.editMode {
		entry := m.entries[m.editIndex]
		b.WriteString(FieldLabelFocusedStyle.Render("Edit Blocker:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s | Created: %s", entry.Project.Name, storage.FormatRelativeTime(entry.Blocker.CreatedAt))))
		b.WriteString("\n\n")
		b.WriteString(m.editTextInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save | Ctrl+O: external editor | Esc: cancel"))
		return b.String()
	}

	// Add mode
	if m.addMode {
		project := m.selectedProject()
		projectName := ""
		if project != nil {
			projectName = project.Name
		}
		b.WriteString(FieldLabelFocusedStyle.Render("Add Blocker:"))
		b.WriteString("\n")
		if projectName != "" {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Project: %s", projectName)))
			b.WriteString("\n")
		}
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save | Ctrl+O: external editor | Esc: cancel"))
		return b.String()
	}

	// Delete confirmation
	if m.confirmDelete {
		visible := m.getVisibleEntries()
		if len(visible) > 0 && m.cursor < len(visible) {
			entry := visible[m.cursor]
			warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
			b.WriteString(warningStyle.Render("Delete blocker?"))
			b.WriteString("\n\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  \"%s\" (%s)", entry.Blocker.Title, entry.Project.Name)))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("y: confirm delete | n/Esc: cancel"))
		}
		return b.String()
	}

	// Error display
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n\n")
	}

	visible := m.getVisibleEntries()

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	if len(visible) == 0 {
		switch m.activeTab {
		case blockersTabActive:
			b.WriteString(MutedStyle.Render("No active blockers. Press 'a' to add one."))
		case blockersTabResolved:
			b.WriteString(MutedStyle.Render("No resolved blockers."))
		default:
			b.WriteString(MutedStyle.Render("No blockers. Press 'a' to add one."))
		}
		b.WriteString("\n")
	} else {
		headerLines := 8
		availableHeight := m.height - headerLines
		maxVisible := availableHeight / 3
		if maxVisible < 3 {
			maxVisible = 3
		}

		scrollOffset := 0
		if m.cursor >= maxVisible {
			scrollOffset = m.cursor - maxVisible + 1
		}

		for i := scrollOffset; i < len(visible) && i < scrollOffset+maxVisible; i++ {
			entry := visible[i]
			blocker := entry.Blocker

			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			relativeTime := storage.FormatRelativeTime(blocker.CreatedAt)

			var line strings.Builder
			line.WriteString(cursor)

			if i == m.cursor {
				if blocker.IsResolved() {
					line.WriteString(DoneStyle.Render("✓ "))
					line.WriteString(MutedStyle.Render(blocker.Title))
				} else {
					line.WriteString(DueSoonStyle.Render("🚧 "))
					line.WriteString(BoldStyle.Render(blocker.Title))
				}
			} else {
				if blocker.IsResolved() {
					line.WriteString(DoneStyle.Render("✓ "))
					line.WriteString(MutedStyle.Render(blocker.Title))
				} else {
					line.WriteString(DueSoonStyle.Render("🚧 "))
					line.WriteString(blocker.Title)
				}
			}
			line.WriteString("\n")
			line.WriteString(fmt.Sprintf("      %s", MutedStyle.Render(fmt.Sprintf("[%s] %s", entry.Project.Name, relativeTime))))

			if blocker.Description != "" {
				desc := blocker.Description
				maxLen := m.width - 10
				if maxLen < 30 {
					maxLen = 30
				}
				desc = strings.ReplaceAll(desc, "\n", " ")
				if len(desc) > maxLen {
					desc = desc[:maxLen-3] + "..."
				}
				line.WriteString(" - ")
				line.WriteString(MutedStyle.Render(desc))
			}
			line.WriteString("\n")

			b.WriteString(line.String())
		}

		if len(visible) > maxVisible {
			remaining := len(visible) - scrollOffset - maxVisible
			if remaining > 0 {
				b.WriteString(MutedStyle.Render(fmt.Sprintf("  ... and %d more (use j/k to scroll)", remaining)))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(MutedStyle.Render("a: add | e: edit | R: resolve/unresolve | d: delete | g: go to project | Enter: expand | h/l/[/] tabs | Esc: close"))

	return b.String()
}

// HasTextInput returns true when add/edit mode is active.
func (m *GlobalBlockersModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// ShouldCaptureEsc returns true when in expanded, add/edit, or delete confirmation modes.
func (m *GlobalBlockersModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.confirmDelete || m.addMode || m.editMode
}

// ShowGlobalBlockersMsg requests opening the global blockers view.
type ShowGlobalBlockersMsg struct{}

// CloseGlobalBlockersMsg requests closing the global blockers view.
type CloseGlobalBlockersMsg struct{}
