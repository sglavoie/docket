package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// PriorityInfo represents a selectable priority option.
type PriorityInfo struct {
	Priority    int
	Description string
}

// FilterText returns the text used for case-insensitive filtering.
func (p PriorityInfo) FilterText() string {
	return fmt.Sprintf("%d %s", p.Priority, p.Description)
}

// DisplayText returns the primary text to display for this item.
func (p PriorityInfo) DisplayText() string {
	return fmt.Sprintf("Priority %d - %s", p.Priority, p.Description)
}

// CountValue returns 0 (not used for priorities).
func (p PriorityInfo) CountValue() int {
	return 0
}

// ItemID returns the priority as a string identifier.
func (p PriorityInfo) ItemID() string {
	return fmt.Sprintf("%d", p.Priority)
}

// PriorityPickerModel allows selecting a priority to assign to a task.
type PriorityPickerModel struct {
	*BasePicker[PriorityInfo]
	task    *models.Task
	project *models.Project
}

// ShowPriorityPickerMsg is sent to open the priority picker for assignment.
type ShowPriorityPickerMsg struct {
	Task    *models.Task
	Project *models.Project
}

// PriorityPickerDoneMsg is sent when the priority picker is done.
type PriorityPickerDoneMsg struct {
	Task     *models.Task
	Project  *models.Project
	Priority int
	Canceled bool
}

// NewPriorityPickerModel creates a new priority picker for task assignment.
func NewPriorityPickerModel(task *models.Task, project *models.Project) *PriorityPickerModel {
	priorities := []PriorityInfo{
		{Priority: 1, Description: "Critical"},
		{Priority: 2, Description: "High"},
		{Priority: 3, Description: "Normal"},
		{Priority: 4, Description: "Low"},
		{Priority: 5, Description: "Someday"},
	}

	picker := NewBasePicker(priorities, PickerConfig{
		Title:          "Set Priority",
		EmptyMessage:   "No priorities available.",
		NoMatchMessage: "No matching priorities.",
		MultiSelect:    false,
		ShowCounts:     false,
	})

	// Pre-select current priority
	picker.SetCursorToID(fmt.Sprintf("%d", task.Priority))

	return &PriorityPickerModel{
		BasePicker: picker,
		task:       task,
		project:    project,
	}
}

// Init initializes the model.
func (m *PriorityPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *PriorityPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return PriorityPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
				}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				priority := m.Filtered[m.Cursor].Priority
				return m, func() tea.Msg {
					return PriorityPickerDoneMsg{
						Task:     m.task,
						Project:  m.project,
						Priority: priority,
						Canceled: false,
					}
				}
			}
			return m, func() tea.Msg {
				return PriorityPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Canceled: true,
				}
			}

		case "1", "2", "3", "4", "5":
			// Direct selection by number
			priority := int(msg.String()[0] - '0')
			return m, func() tea.Msg {
				return PriorityPickerDoneMsg{
					Task:     m.task,
					Project:  m.project,
					Priority: priority,
					Canceled: false,
				}
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

// View renders the priority picker.
func (m *PriorityPickerModel) View() tea.View {
	var b strings.Builder

	title := "Set Priority"
	subtitle := fmt.Sprintf("Task #%d: %s", m.task.ShortID, m.task.Title)
	availableSubtitleWidth := max(m.Width()-lipgloss.Width(title)-2, 1)
	subtitle = truncateToVisualWidth(subtitle, availableSubtitleWidth)
	currentPriority := m.task.Priority
	b.WriteString(renderFullscreenTitle(title, subtitle))
	b.WriteString("\n\n")

	// Current priority
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Current: Priority %d", currentPriority)))
	b.WriteString("\n\n")

	// List with priority colors
	for i, p := range m.Filtered {
		cursor := "  "
		if i == m.Cursor {
			cursor = "  "
		}

		style := GetPriorityStyle(p.Priority)

		line := fmt.Sprintf("%s%d: %s", cursor, p.Priority, p.Description)
		if i == m.Cursor {
			b.WriteString(TaskJumperSelectedStyle.Render(style.Render(line)))
		} else {
			b.WriteString(style.Render(line))
		}
		b.WriteString("\n")
	}

	// Footer
	left := HelpStyle.Render("1-5 to select · ↑/↓ navigate · Enter confirm · Esc cancel")
	b.WriteString(renderStatusBarFooter(left, "", m.Width()))

	return tea.NewView(b.String())
}

func (m *PriorityPickerModel) StatusBarLabel() string { return "Set Priority" }
