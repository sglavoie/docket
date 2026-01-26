package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/workflow"
)

// TriggerFieldPickerModel allows selecting a trigger field from the full list.
type TriggerFieldPickerModel struct {
	*BasePicker[TriggerFieldInfo]
	triggerIndex   int
	currentFieldID string
}

// ShowTriggerFieldPickerMsg is sent to open the trigger field picker.
type ShowTriggerFieldPickerMsg struct {
	TriggerIndex int
	CurrentField workflow.TriggerField
}

// TriggerFieldPickerDoneMsg is sent when the trigger field picker is done.
type TriggerFieldPickerDoneMsg struct {
	TriggerIndex int
	Field        workflow.TriggerField
	Canceled     bool
}

// NewTriggerFieldPickerModel creates a new trigger field picker.
func NewTriggerFieldPickerModel(triggerIndex int, currentField workflow.TriggerField) *TriggerFieldPickerModel {
	items := BuildTriggerFieldItems()

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Select Trigger Field",
		EmptyMessage:   "No trigger fields available",
		NoMatchMessage: "No matching fields",
		MultiSelect:    false,
		ShowCounts:     false,
		BoxWidth:       60,
	})

	currentID := string(currentField)
	picker.SetCursorToID(currentID)

	return &TriggerFieldPickerModel{
		BasePicker:     picker,
		triggerIndex:   triggerIndex,
		currentFieldID: currentID,
	}
}

// Init initializes the model.
func (m *TriggerFieldPickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *TriggerFieldPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Handle search mode first
		if m.IsSearchMode() {
			switch msg.String() {
			case "enter":
				m.ConfirmSearch()
				m.Filter(m.SearchQuery())
				return m, nil
			case "esc":
				m.DeactivateSearch()
				m.Filter("")
				return m, nil
			default:
				handled, cmd := m.HandleKeyMsg(msg)
				if handled {
					return m, cmd
				}
			}
			return m, nil
		}

		// Not in search mode - normal handling
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg {
				return TriggerFieldPickerDoneMsg{
					TriggerIndex: m.triggerIndex,
					Canceled:     true,
				}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				field := m.Filtered[m.Cursor].Field
				return m, func() tea.Msg {
					return TriggerFieldPickerDoneMsg{
						TriggerIndex: m.triggerIndex,
						Field:        field,
						Canceled:     false,
					}
				}
			}
			return m, func() tea.Msg {
				return TriggerFieldPickerDoneMsg{
					TriggerIndex: m.triggerIndex,
					Canceled:     true,
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

// View renders the trigger field picker.
func (m *TriggerFieldPickerModel) View() string {
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	content.WriteString(m.RenderSearchBar())

	if m.currentFieldID != "" {
		content.WriteString(MutedStyle.Render("Current: " + m.currentFieldID))
		content.WriteString("\n\n")
	}

	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 14
	}
	content.WriteString(m.RenderList(maxVisible))

	content.WriteString("\n")
	content.WriteString(m.RenderFooter())

	return renderOverlayCard(content.String(), m.Width(), m.Height(), OverlayCardConfig{
		Title:    "Select Trigger Field",
		Subtitle: "Type to search",
		Width:    boxWidth,
		Centered: true,
	})
}
