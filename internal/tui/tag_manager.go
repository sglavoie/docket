package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/storage"
)

// TagInfo holds a tag name and its usage count.
type TagInfo struct {
	Name      string
	TaskCount int
}

// TagManagerModel is the model for the tag management view.
type TagManagerModel struct {
	projectTree  *ProjectTreeModel
	tags         []TagInfo
	cursor       int
	width        int
	height       int
	renameMode   bool
	renameInput  textinput.Model
	renameTarget string
	errorMsg     string
}

// ShowTagManagerMsg is sent to open the tag manager view.
type ShowTagManagerMsg struct {
	ProjectTree *ProjectTreeModel
}

// CloseTagManagerMsg is sent when the tag manager should close.
type CloseTagManagerMsg struct{}

// TagRenamedMsg is sent after a tag has been renamed.
type TagRenamedMsg struct {
	OldName string
	NewName string
}

// NewTagManagerModel creates a new tag manager model.
func NewTagManagerModel(pt *ProjectTreeModel) *TagManagerModel {
	renameInput := textinput.New()
	renameInput.Placeholder = "new tag name"
	renameInput.CharLimit = 50
	renameInput.Width = 30

	m := &TagManagerModel{
		projectTree: pt,
		cursor:      0,
		renameInput: renameInput,
	}
	m.buildTagList()
	return m
}

// SetSize sets the view dimensions.
func (m *TagManagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// buildTagList scans all tasks and builds a sorted list of tags with counts.
func (m *TagManagerModel) buildTagList() {
	tagCounts := make(map[string]int)
	for _, task := range m.projectTree.project.Tasks {
		for _, tag := range task.Tags {
			tagCounts[tag]++
		}
	}

	m.tags = make([]TagInfo, 0, len(tagCounts))
	for name, count := range tagCounts {
		m.tags = append(m.tags, TagInfo{Name: name, TaskCount: count})
	}

	sort.Slice(m.tags, func(i, j int) bool {
		return m.tags[i].Name < m.tags[j].Name
	})

	// Clamp cursor
	if m.cursor >= len(m.tags) {
		m.cursor = len(m.tags) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// renameTag renames a tag across all tasks.
func (m *TagManagerModel) renameTag(oldName, newName string) tea.Cmd {
	return func() tea.Msg {
		newName = strings.TrimSpace(newName)
		if newName == "" || newName == oldName {
			return nil
		}

		for _, task := range m.projectTree.project.Tasks {
			for i, tag := range task.Tags {
				if tag == oldName {
					task.Tags[i] = newName
					break
				}
			}
		}

		if err := storage.BackupProject(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		if err := storage.SaveProject(m.projectTree.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return TagRenamedMsg{OldName: oldName, NewName: newName}
	}
}

// filterByTag sets the project tree filter and closes the tag manager.
func (m *TagManagerModel) filterByTag(tagName string) tea.Cmd {
	return func() tea.Msg {
		m.projectTree.filterTags = map[string]bool{tagName: true}
		m.projectTree.updateVisibility()
		m.projectTree.clampCursor()
		return CloseTagManagerMsg{}
	}
}

// Init initializes the model.
func (m *TagManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TagManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case TagRenamedMsg:
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		// Close the tag manager after successful rename
		return m, func() tea.Msg {
			return CloseTagManagerMsg{}
		}

	case tea.KeyMsg:
		// Clear error on any key press
		m.errorMsg = ""

		if m.renameMode {
			return m.handleRenameMode(msg)
		}

		switch msg.String() {
		case "q", "esc", "ctrl+t":
			return m, func() tea.Msg {
				return CloseTagManagerMsg{}
			}

		case "j", "down":
			if len(m.tags) > 0 {
				m.cursor++
				if m.cursor >= len(m.tags) {
					m.cursor = 0
				}
			}
			return m, nil

		case "k", "up":
			if len(m.tags) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.tags) - 1
				}
			}
			return m, nil

		case "enter":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				return m, m.filterByTag(m.tags[m.cursor].Name)
			}
			return m, nil

		case "r":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				m.renameMode = true
				m.renameTarget = m.tags[m.cursor].Name
				m.renameInput.SetValue(m.renameTarget)
				m.renameInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case "d":
			if len(m.tags) > 0 && m.cursor < len(m.tags) {
				tag := m.tags[m.cursor]
				if tag.TaskCount > 0 {
					m.errorMsg = fmt.Sprintf("Cannot delete: tag has %d task(s)", tag.TaskCount)
				} else {
					// Tag has 0 tasks, just rebuild to remove it
					m.buildTagList()
				}
			}
			return m, nil
		}
	}

	return m, nil
}

// handleRenameMode handles keyboard input while in rename mode.
func (m *TagManagerModel) handleRenameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.renameInput.Value())
		if newName != "" && newName != m.renameTarget {
			return m, m.renameTag(m.renameTarget, newName)
		}
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// View renders the tag manager view.
func (m *TagManagerModel) View() string {
	var b strings.Builder

	// Calculate box width
	boxWidth := 46
	if m.width > 0 && m.width < boxWidth+10 {
		boxWidth = m.width - 10
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render("Tag Manager"))
	content.WriteString("\n\n")

	if len(m.tags) == 0 {
		content.WriteString(MutedStyle.Render("No tags in project"))
		content.WriteString("\n")
	} else {
		for i, tag := range m.tags {
			cursor := "  "
			style := TagManagerItemStyle
			if i == m.cursor {
				cursor = "> "
				style = TagManagerItemSelectedStyle
			}

			// Check if this is the tag being renamed
			if m.renameMode && tag.Name == m.renameTarget {
				content.WriteString(cursor)
				content.WriteString("Rename to: ")
				content.WriteString(m.renameInput.View())
				content.WriteString("\n")
			} else {
				line := fmt.Sprintf("%s%s ", cursor, tag.Name)
				content.WriteString(style.Render(line))
				countStr := fmt.Sprintf("(%d)", tag.TaskCount)
				content.WriteString(TagManagerCountStyle.Render(countStr))
				content.WriteString("\n")
			}
		}
	}

	// Error message
	if m.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(TagManagerErrorStyle.Render(m.errorMsg))
		content.WriteString("\n")
	}

	// Footer hint
	content.WriteString("\n")
	if m.renameMode {
		content.WriteString(MutedStyle.Render("enter confirm | esc cancel"))
	} else if len(m.tags) > 0 {
		content.WriteString(MutedStyle.Render("j/k nav | enter filter | r rename | d del | q close"))
	} else {
		content.WriteString(MutedStyle.Render("q close"))
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
