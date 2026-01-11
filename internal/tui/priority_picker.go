package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
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
	Task      *models.Task
	Project   *models.Project
	Priority  int
	Cancelled bool
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
		EmptyMessage:   "No priorities available",
		NoMatchMessage: "No matching priorities",
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

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return PriorityPickerDoneMsg{
					Task:      m.task,
					Project:   m.project,
					Cancelled: true,
				}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				priority := m.Filtered[m.Cursor].Priority
				return m, func() tea.Msg {
					return PriorityPickerDoneMsg{
						Task:      m.task,
						Project:   m.project,
						Priority:  priority,
						Cancelled: false,
					}
				}
			}
			return m, func() tea.Msg {
				return PriorityPickerDoneMsg{
					Task:      m.task,
					Project:   m.project,
					Cancelled: true,
				}
			}

		case "1", "2", "3", "4", "5":
			// Direct selection by number
			priority := int(msg.String()[0] - '0')
			return m, func() tea.Msg {
				return PriorityPickerDoneMsg{
					Task:      m.task,
					Project:   m.project,
					Priority:  priority,
					Cancelled: false,
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
func (m *PriorityPickerModel) View() string {
	boxWidth := 40
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	content.WriteString(titleStyle.Render(m.Config.Title))
	content.WriteString("\n")
	content.WriteString(MutedStyle.Render(fmt.Sprintf("Task #%d: %s", m.task.ShortID, util.Truncate(m.task.Title, 25))))
	content.WriteString("\n\n")

	// Current priority
	content.WriteString(MutedStyle.Render(fmt.Sprintf("Current: Priority %d", m.task.Priority)))
	content.WriteString("\n\n")

	// List with priority colors
	for i, p := range m.Filtered {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}

		// Color based on priority
		var style lipgloss.Style
		switch p.Priority {
		case 1:
			style = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		case 2:
			style = lipgloss.NewStyle().Foreground(ColorWarning)
		case 3:
			style = lipgloss.NewStyle().Foreground(ColorPrimary)
		case 4:
			style = lipgloss.NewStyle().Foreground(ColorMuted)
		case 5:
			style = lipgloss.NewStyle().Foreground(ColorMuted).Faint(true)
		default:
			style = NormalStyle
		}

		line := fmt.Sprintf("%s%d: %s", cursor, p.Priority, p.Description)
		if i == m.Cursor {
			content.WriteString(TaskJumperSelectedStyle.Render(style.Render(line)))
		} else {
			content.WriteString(style.Render(line))
		}
		content.WriteString("\n")
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("1-5 to select • ↑/↓ navigate • Enter confirm • Esc cancel"))

	return RenderCenteredBox(content.String(), boxWidth, m.Width(), m.Height(), TagManagerBoxStyle)
}
