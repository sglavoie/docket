package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

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
	if s.Key == "" {
		return "status-icon:none"
	}
	return s.Key
}

// StatusIconPickerModel allows selecting a status icon for a project.
type StatusIconPickerModel struct {
	*ListSurfaceAdapter[StatusIconInfo]
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
			Emoji: icon.DisplayIcon(),
			Label: icon.Label,
		})
	}

	picker := NewListSurfaceAdapter(items, PickerConfig{
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
		ListSurfaceAdapter: picker,
		project:            project,
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

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			project := m.project
			return m, func() tea.Msg {
				return StatusIconSelectedMsg{Project: project, Canceled: true}
			}

		case "enter":
			if len(m.FilteredItems()) > 0 && m.CursorIndex() < len(m.FilteredItems()) {
				key := m.FilteredItems()[m.CursorIndex()].Key
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

func (m *StatusIconPickerModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Set Status Icon", m.project.Name))
	b.WriteString("\n\n")

	// Show current icon
	if m.project.StatusIcon != "" {
		if icon := models.ProjectStatusIconByKey(m.project.StatusIcon); icon != nil {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s %s", icon.DisplayIcon(), icon.Label)))
		} else {
			b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: %s", m.project.StatusIcon)))
		}
	} else {
		b.WriteString(MutedStyle.Render("Current: None"))
	}
	b.WriteString("\n\n")

	// List items
	for i, item := range m.FilteredItems() {
		cursor := "  "
		if i == m.CursorIndex() {
			cursor = "  "
		}

		line := cursor + item.DisplayText()
		if i == m.CursorIndex() {
			b.WriteString(TaskJumperSelectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Footer
	left := HelpStyle.Render("↑/↓ navigate · Enter confirm · Esc cancel")
	b.WriteString(renderStatusBarFooter(left, "", m.Width()))

	return tea.NewView(b.String())
}

func (m *StatusIconPickerModel) StatusBarLabel() string { return "Set Status Icon" }
