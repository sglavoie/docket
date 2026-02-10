package tui

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/config"
)

// KanbanDisplaySettingsDoneMsg is sent when the popup finishes (save or cancel).
type KanbanDisplaySettingsDoneMsg struct{}

// ShowKanbanDisplaySettingsMsg opens the kanban display settings popup.
type ShowKanbanDisplaySettingsMsg struct{}

// KanbanDisplaySettingsModel is a popup overlay for adjusting kanban display settings.
type KanbanDisplaySettingsModel struct {
	config        *config.Config
	configManager *config.Manager

	maxColumns      int // Current value being edited
	maxSwimlaneRows int // Current value being edited
	cursor          int // 0=columns, 1=swimlane rows

	width  int
	height int
}

const (
	kanbanDisplayMin = 1
	kanbanDisplayMax = 20
)

// NewKanbanDisplaySettingsModel creates a new display settings editor.
func NewKanbanDisplaySettingsModel(cfg *config.Config, cfgManager *config.Manager) *KanbanDisplaySettingsModel {
	return &KanbanDisplaySettingsModel{
		config:          cfg,
		configManager:   cfgManager,
		maxColumns:      cfg.GetKanbanMaxColumns(),
		maxSwimlaneRows: cfg.GetKanbanMaxSwimlaneRows(),
	}
}

// SetSize implements View.
func (m *KanbanDisplaySettingsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Init implements tea.Model.
func (m *KanbanDisplaySettingsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *KanbanDisplaySettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg {
				return KanbanDisplaySettingsDoneMsg{}
			}

		case "enter":
			return m, m.save()

		case "j", "down":
			if m.cursor < 1 {
				m.cursor++
			}
			return m, nil

		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "l", "right", "+":
			m.adjustValue(1)
			return m, nil

		case "h", "left", "-":
			m.adjustValue(-1)
			return m, nil

		default:
			// Direct number input
			if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
				digit, _ := strconv.Atoi(msg.String())
				m.setDirectValue(digit)
			}
			return m, nil
		}
	}
	return m, nil
}

// adjustValue adjusts the current field by delta, clamping to valid range.
func (m *KanbanDisplaySettingsModel) adjustValue(delta int) {
	switch m.cursor {
	case 0:
		m.maxColumns = clampInt(m.maxColumns+delta, kanbanDisplayMin, kanbanDisplayMax)
	case 1:
		m.maxSwimlaneRows = clampInt(m.maxSwimlaneRows+delta, kanbanDisplayMin, kanbanDisplayMax)
	}
}

// setDirectValue sets value from a single digit. For two-digit values,
// appends if current value is a single digit recently entered.
func (m *KanbanDisplaySettingsModel) setDirectValue(digit int) {
	// Build two-digit number from current value if current is single digit
	current := m.currentValue()
	newVal := digit
	if current >= 1 && current <= 9 {
		twoDigit := current*10 + digit
		if twoDigit >= kanbanDisplayMin && twoDigit <= kanbanDisplayMax {
			newVal = twoDigit
		}
	}
	if newVal < kanbanDisplayMin {
		newVal = kanbanDisplayMin
	}
	if newVal > kanbanDisplayMax {
		newVal = kanbanDisplayMax
	}

	switch m.cursor {
	case 0:
		m.maxColumns = newVal
	case 1:
		m.maxSwimlaneRows = newVal
	}
}

// currentValue returns the value of the currently focused field.
func (m *KanbanDisplaySettingsModel) currentValue() int {
	switch m.cursor {
	case 0:
		return m.maxColumns
	case 1:
		return m.maxSwimlaneRows
	}
	return 0
}

// save writes the settings to config and signals done.
func (m *KanbanDisplaySettingsModel) save() tea.Cmd {
	cols := m.maxColumns
	rows := m.maxSwimlaneRows
	m.config.KanbanMaxColumns = &cols
	m.config.KanbanMaxSwimlaneRows = &rows
	_ = m.configManager.Save(m.config)

	return func() tea.Msg {
		return KanbanDisplaySettingsDoneMsg{}
	}
}

// View implements tea.Model.
func (m *KanbanDisplaySettingsModel) View() string {
	contentWidth := m.width - 10
	if contentWidth < 30 {
		contentWidth = 30
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	selectedStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle := lipgloss.NewStyle()
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	var lines []string
	lines = append(lines, titleStyle.Width(contentWidth).Render("Kanban Display Settings"))
	lines = append(lines, "")

	type field struct {
		name  string
		value int
		desc  string
	}

	fields := []field{
		{"Max Columns", m.maxColumns, "Maximum visible columns (auto-reduces for narrow terminals)"},
		{"Max Swimlane Rows", m.maxSwimlaneRows, "Maximum visible swimlane rows"},
	}

	for i, f := range fields {
		cursor := "  "
		style := dimStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedStyle
		}

		// Value bar
		bar := ""
		for v := kanbanDisplayMin; v <= kanbanDisplayMax; v++ {
			if v <= f.value {
				bar += "█"
			} else {
				bar += "░"
			}
		}

		line := fmt.Sprintf("%s%-20s %s %d/%d", cursor, f.name, bar, f.value, kanbanDisplayMax)
		lines = append(lines, style.Render(line))
		lines = append(lines, "  "+mutedStyle.Render(f.desc))
		lines = append(lines, "")
	}

	helpText := "j/k: navigate · h/l: adjust · 0-9: set · Enter: save · Esc: cancel"
	lines = append(lines, mutedStyle.Render(renderWrappingHelpLine(helpText, contentWidth)))

	content := ""
	for i, l := range lines {
		if i > 0 {
			content += "\n"
		}
		content += l
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("99")).
		Padding(1, 2)

	box := boxStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box)
}

// ShouldCaptureEsc returns true since the editor handles Esc internally.
func (m *KanbanDisplaySettingsModel) ShouldCaptureEsc() bool {
	return true
}

// HasTextInput returns false since there's no free-form text input.
func (m *KanbanDisplaySettingsModel) HasTextInput() bool {
	return false
}

// clampInt clamps v to [min, max].
func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
