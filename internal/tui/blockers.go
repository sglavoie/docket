package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var blockersTabNames = []string{"Active", "Resolved", "All"}

const (
	blockersTabActive   = 0
	blockersTabResolved = 1
	blockersTabAll      = 2
)

// BlockersModel displays and manages blockers for a project.
type BlockersModel struct {
	project *models.Project

	// Navigation state
	blockers []*models.Blocker
	cursor   int

	// Tab filtering
	activeTab int

	// Add mode
	addMode   bool
	textInput textarea.Model

	// Edit mode
	editMode      bool
	editIndex     int
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

// NewBlockersModel creates a new blockers viewer.
func NewBlockersModel(project *models.Project) *BlockersModel {
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

	blockers, _ := storage.LoadBlockers(project)

	return &BlockersModel{
		project:       project,
		blockers:      blockers,
		cursor:        0,
		activeTab:     blockersTabActive,
		addMode:       false,
		textInput:     ti,
		editTextInput: editTi,
	}
}

// NewBlockersModelWithEditor creates a blockers viewer with editor config.
func NewBlockersModelWithEditor(project *models.Project, editor string) *BlockersModel {
	m := NewBlockersModel(project)
	m.editor = editor
	return m
}

func (m *BlockersModel) Init() tea.Cmd {
	return nil
}

// SetSize updates the view dimensions.
func (m *BlockersModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textInput.SetWidth(width - 4)
	m.editTextInput.SetWidth(width - 4)
}

// getVisibleBlockers returns blockers based on current tab.
func (m *BlockersModel) getVisibleBlockers() []*models.Blocker {
	switch m.activeTab {
	case blockersTabResolved:
		return m.getResolvedBlockers()
	case blockersTabAll:
		var all []*models.Blocker
		all = append(all, m.blockers...)
		return all
	default:
		return m.getActiveBlockers()
	}
}

// getActiveBlockers returns only active blockers.
func (m *BlockersModel) getActiveBlockers() []*models.Blocker {
	var active []*models.Blocker
	for _, b := range m.blockers {
		if !b.IsResolved() {
			active = append(active, b)
		}
	}
	return active
}

// getResolvedBlockers returns only resolved blockers.
func (m *BlockersModel) getResolvedBlockers() []*models.Blocker {
	var resolved []*models.Blocker
	for _, b := range m.blockers {
		if b.IsResolved() {
			resolved = append(resolved, b)
		}
	}
	return resolved
}

func (m *BlockersModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		visible := m.getVisibleBlockers()

		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return CloseBlockersMsg{} }

		case "j", "down":
			if len(visible) > 0 && m.cursor < len(visible)-1 {
				m.cursor++
			}

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

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
				m.editIndex = m.findBlockerIndex(visible[m.cursor])
				return m, m.editInExternalEditor()
			}

		case "R":
			// Resolve/unresolve toggle
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
			visible = m.getVisibleBlockers()
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
			visible = m.getVisibleBlockers()
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

func (m *BlockersModel) findBlockerIndex(blocker *models.Blocker) int {
	for i, b := range m.blockers {
		if b.ID == blocker.ID {
			return i
		}
	}
	return -1
}

func (m *BlockersModel) saveBlocker() tea.Cmd {
	return func() tea.Msg {
		title := strings.TrimSpace(m.textInput.Value())
		if title == "" {
			m.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		blocker := models.NewBlocker(title)
		m.blockers = append(m.blockers, blocker)

		if err := storage.SaveBlockers(m.project, m.blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		m.addMode = false
		m.textInput.Reset()
		m.err = nil
		m.cursor = len(m.getActiveBlockers()) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}

		return BlockerAddedMsg{Project: m.project}
	}
}

func (m *BlockersModel) executeDelete() tea.Cmd {
	return func() tea.Msg {
		visible := m.getVisibleBlockers()
		if m.cursor >= len(visible) {
			return nil
		}

		blocker := visible[m.cursor]
		idx := m.findBlockerIndex(blocker)
		if idx < 0 {
			return nil
		}

		m.blockers = append(m.blockers[:idx], m.blockers[idx+1:]...)

		if err := storage.SaveBlockers(m.project, m.blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		visible = m.getVisibleBlockers()
		if m.cursor >= len(visible) && m.cursor > 0 {
			m.cursor--
		}

		return BlockerDeletedMsg{Project: m.project}
	}
}

func (m *BlockersModel) startEditBlocker(blocker *models.Blocker) tea.Cmd {
	m.editMode = true
	m.editIndex = m.findBlockerIndex(blocker)

	// Set content: title on first line, then blank line, then description
	content := blocker.Title
	if blocker.Description != "" {
		content += "\n\n" + blocker.Description
	}
	m.editTextInput.SetValue(content)
	m.editTextInput.Focus()
	return textarea.Blink
}

func (m *BlockersModel) saveEditedBlocker() tea.Cmd {
	return func() tea.Msg {
		content := strings.TrimSpace(m.editTextInput.Value())
		if content == "" {
			m.err = fmt.Errorf("blocker title cannot be empty")
			return nil
		}

		// Parse title and description (first line is title, rest is description)
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

		m.blockers[m.editIndex].Title = title
		m.blockers[m.editIndex].Description = description

		if err := storage.SaveBlockers(m.project, m.blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		m.editMode = false
		m.editTextInput.Reset()
		m.err = nil

		return BlockerEditedMsg{Project: m.project}
	}
}

func (m *BlockersModel) toggleResolve(blocker *models.Blocker) tea.Cmd {
	return func() tea.Msg {
		idx := m.findBlockerIndex(blocker)
		if idx < 0 {
			return nil
		}

		if blocker.IsResolved() {
			m.blockers[idx].Unresolve()
		} else {
			m.blockers[idx].Resolve()
		}

		if err := storage.SaveBlockers(m.project, m.blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		// Adjust cursor if item disappears from current tab
		if m.activeTab == blockersTabActive || m.activeTab == blockersTabResolved {
			visible := m.getVisibleBlockers()
			if m.cursor >= len(visible) && m.cursor > 0 {
				m.cursor--
			}
		}

		return BlockerEditedMsg{Project: m.project}
	}
}

func (m *BlockersModel) editInExternalEditor() tea.Cmd {
	blocker := m.blockers[m.editIndex]

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

	blockers := m.blockers
	project := m.project
	editIndex := m.editIndex

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

		blockers[editIndex].Title = title
		blockers[editIndex].Description = description

		if err := storage.SaveBlockers(project, blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		return BlockerEditedMsg{Project: project}
	})
}

func (m *BlockersModel) addBlockerInExternalEditor() tea.Cmd {
	currentText := strings.TrimSpace(m.textInput.Value())

	content := fmt.Sprintf("# New Blocker for %s\n\n", m.project.Name)
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

	blockers := m.blockers
	project := m.project

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
		blockers = append(blockers, blocker)

		if err := storage.SaveBlockers(project, blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		return BlockerAddedMsg{Project: project}
	})
}

func (m *BlockersModel) editBlockerInExternalEditor() tea.Cmd {
	blocker := m.blockers[m.editIndex]
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

	blockers := m.blockers
	project := m.project
	editIndex := m.editIndex
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

		blockers[editIndex].Title = title
		blockers[editIndex].Description = description

		if err := storage.SaveBlockers(project, blockers); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save blockers: %w", err)}
		}

		return BlockerEditedMsg{Project: project}
	})
}

func (m *BlockersModel) renderBlockersTabBar() string {
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

func (m *BlockersModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render(fmt.Sprintf("Blockers for %s", m.project.Name)))
	b.WriteString("\n")

	// Counts
	activeCount := len(m.getActiveBlockers())
	resolvedCount := len(m.getResolvedBlockers())
	b.WriteString(MutedStyle.Render(fmt.Sprintf("%d active, %d resolved", activeCount, resolvedCount)))
	b.WriteString("\n\n")

	// Expanded view mode
	if m.expandedView {
		visible := m.getVisibleBlockers()
		if len(visible) > 0 && m.cursor < len(visible) {
			blocker := visible[m.cursor]
			relativeTime := storage.FormatRelativeTime(blocker.CreatedAt)

			b.WriteString(FieldLabelFocusedStyle.Render(blocker.Title))
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
		blocker := m.blockers[m.editIndex]
		b.WriteString(FieldLabelFocusedStyle.Render("Edit Blocker:"))
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Created: %s", storage.FormatRelativeTime(blocker.CreatedAt))))
		b.WriteString("\n\n")
		b.WriteString(m.editTextInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save | Ctrl+O: external editor | Esc: cancel"))
		return b.String()
	}

	// Add mode
	if m.addMode {
		b.WriteString(FieldLabelFocusedStyle.Render("Add Blocker:"))
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Ctrl+S: save | Ctrl+O: external editor | Esc: cancel"))
		return b.String()
	}

	// Delete confirmation
	if m.confirmDelete {
		visible := m.getVisibleBlockers()
		if len(visible) > 0 && m.cursor < len(visible) {
			blocker := visible[m.cursor]
			warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
			b.WriteString(warningStyle.Render("Delete blocker?"))
			b.WriteString("\n\n")
			b.WriteString(MutedStyle.Render(fmt.Sprintf("  \"%s\"", blocker.Title)))
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

	visible := m.getVisibleBlockers()

	// Tab bar
	b.WriteString(m.renderBlockersTabBar())
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
			blocker := visible[i]

			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}

			relativeTime := storage.FormatRelativeTime(blocker.CreatedAt)

			var entry strings.Builder
			entry.WriteString(cursor)

			if i == m.cursor {
				if blocker.IsResolved() {
					entry.WriteString(DoneStyle.Render("✓ "))
					entry.WriteString(MutedStyle.Render(blocker.Title))
				} else {
					entry.WriteString(DueSoonStyle.Render("🚧 "))
					entry.WriteString(BoldStyle.Render(blocker.Title))
				}
			} else {
				if blocker.IsResolved() {
					entry.WriteString(DoneStyle.Render("✓ "))
					entry.WriteString(MutedStyle.Render(blocker.Title))
				} else {
					entry.WriteString(DueSoonStyle.Render("🚧 "))
					entry.WriteString(blocker.Title)
				}
			}
			entry.WriteString("\n")
			entry.WriteString(fmt.Sprintf("      %s", MutedStyle.Render(relativeTime)))

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
				entry.WriteString(" - ")
				entry.WriteString(MutedStyle.Render(desc))
			}
			entry.WriteString("\n")

			b.WriteString(entry.String())
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

	// Help footer
	b.WriteString(MutedStyle.Render("a: add | e: edit | R: resolve/unresolve | d: delete | Enter: expand | h/l/[/] tabs | Esc: close"))

	return b.String()
}

// HasTextInput returns true when add/edit mode is active.
func (m *BlockersModel) HasTextInput() bool {
	return m.addMode || m.editMode
}

// ShouldCaptureEsc returns true when in expanded, add/edit, or delete confirmation modes.
func (m *BlockersModel) ShouldCaptureEsc() bool {
	return m.expandedView || m.confirmDelete || m.addMode || m.editMode
}

// Blocker-related messages

// ShowBlockersMsg requests opening the blockers view for a project.
type ShowBlockersMsg struct {
	Project *models.Project
}

// CloseBlockersMsg requests closing the blockers view.
type CloseBlockersMsg struct{}

// BlockerAddedMsg signals a blocker was added.
type BlockerAddedMsg struct {
	Project *models.Project
}

// BlockerEditedMsg signals a blocker was edited.
type BlockerEditedMsg struct {
	Project *models.Project
}

// BlockerDeletedMsg signals a blocker was deleted.
type BlockerDeletedMsg struct {
	Project *models.Project
}
