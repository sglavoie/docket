package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// StatusIconInfo represents a selectable status icon option.
type StatusIconInfo struct {
	Key   string
	Emoji string
	Label string
}

// FilterText returns the text used for case-insensitive filtering.
func (s StatusIconInfo) FilterText() string {
	return fmt.Sprintf("%s %s", s.Key, s.Label)
}

// DisplayText returns the primary text to display for this item.
func (s StatusIconInfo) DisplayText() string {
	if s.Key == "" {
		return s.Label
	}
	return fmt.Sprintf("%s  %s", s.Emoji, s.Label)
}

// CountValue returns 0 (not used for status icons).
func (s StatusIconInfo) CountValue() int {
	return 0
}

// ItemID returns the key as a string identifier.
func (s StatusIconInfo) ItemID() string {
	return s.Key
}

// StatusIconPickerModel allows selecting a status icon for a project.
type StatusIconPickerModel struct {
	*BasePicker[StatusIconInfo]
	project *models.Project
}

// ShowStatusIconPickerMsg is sent to open the status icon picker.
type ShowStatusIconPickerMsg struct {
	Project *models.Project
}

// StatusIconSelectedMsg is sent when a status icon is selected.
type StatusIconSelectedMsg struct {
	Project  *models.Project
	Key      string
	Canceled bool
}

// NewStatusIconPickerModel creates a new status icon picker.
func NewStatusIconPickerModel(project *models.Project) *StatusIconPickerModel {
	var items []StatusIconInfo

	// "None" option to clear the icon
	items = append(items, StatusIconInfo{Key: "", Emoji: "  ", Label: "None (clear icon)"})

	for _, icon := range models.AllProjectStatusIcons() {
		items = append(items, StatusIconInfo{
			Key:   icon.Key,
			Emoji: icon.Emoji,
			Label: icon.Label,
		})
	}

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Set Status Icon",
		EmptyMessage:   "No status icons available.",
		NoMatchMessage: "No matching icons.",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	// Pre-select current icon
	if project.StatusIcon != "" {
		picker.SetCursorToID(project.StatusIcon)
	}

	return &StatusIconPickerModel{
		BasePicker: picker,
		project:    project,
	}
}

func (m *StatusIconPickerModel) Init() tea.Cmd {
	return nil
}

func (m *StatusIconPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			project := m.project
			return m, func() tea.Msg {
				return StatusIconSelectedMsg{Project: project, Canceled: true}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				key := m.Filtered[m.Cursor].Key
				project := m.project
				return m, func() tea.Msg {
					return StatusIconSelectedMsg{Project: project, Key: key}
				}
			}
			project := m.project
			return m, func() tea.Msg {
				return StatusIconSelectedMsg{Project: project, Canceled: true}
			}

		default:
			handled, cmd := m.HandleKeyMsg(msg)
			if handled {
				return m, cmd
			}
		}
	}

	return m, nil
}

func (m *StatusIconPickerModel) View() string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Set Status Icon", m.project.Name))
	b.WriteString("\n\n")

	// Show current icon
	if m.project.StatusIcon != "" {
		if icon := models.ProjectStatusIconByKey(m.project.StatusIcon); icon != nil {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s %s", icon.Emoji, icon.Label)))
		} else {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", m.project.StatusIcon)))
		}
	} else {
		b.WriteString(MutedStyle.Render("Current: None"))
	}
	b.WriteString("\n\n")

	// List items
	for i, item := range m.Filtered {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}

		line := cursor + item.DisplayText()
		if i == m.Cursor {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Footer
	left := HelpStyle.Render("↑/↓ navigate · Enter confirm · Esc skip")
	b.WriteString(renderStatusBarFooter(left, "", m.Width()))

	return b.String()
}

func (m *StatusIconPickerModel) StatusBarLabel() string { return "Set Status Icon" }
