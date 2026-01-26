package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/workflow"
)

// ActionTypePickerModel allows selecting an action type from the full list.
type ActionTypePickerModel struct {
	*BasePicker[ActionTypeInfo]
	actionIndex     int
	currentActionID string
}

// ShowActionTypePickerMsg is sent to open the action type picker.
type ShowActionTypePickerMsg struct {
	ActionIndex int
	CurrentType workflow.ActionType
}

// ActionTypePickerDoneMsg is sent when the action type picker is done.
type ActionTypePickerDoneMsg struct {
	ActionIndex int
	ActionType  workflow.ActionType
	Canceled    bool
}

// NewActionTypePickerModel creates a new action type picker.
func NewActionTypePickerModel(actionIndex int, currentType workflow.ActionType) *ActionTypePickerModel {
	items := BuildActionTypeItems()

	picker := NewBasePicker(items, PickerConfig{
		Title:          "Select Action Type",
		EmptyMessage:   "No action types available",
		NoMatchMessage: "No matching actions",
		MultiSelect:    false,
		ShowCounts:     false,
		BoxWidth:       60,
	})

	currentID := string(currentType)
	picker.SetCursorToID(currentID)

	return &ActionTypePickerModel{
		BasePicker:      picker,
		actionIndex:     actionIndex,
		currentActionID: currentID,
	}
}

// Init initializes the model.
func (m *ActionTypePickerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *ActionTypePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				return ActionTypePickerDoneMsg{
					ActionIndex: m.actionIndex,
					Canceled:    true,
				}
			}

		case "enter":
			if len(m.Filtered) > 0 && m.Cursor < len(m.Filtered) {
				actionType := m.Filtered[m.Cursor].ActionType
				return m, func() tea.Msg {
					return ActionTypePickerDoneMsg{
						ActionIndex: m.actionIndex,
						ActionType:  actionType,
						Canceled:    false,
					}
				}
			}
			return m, func() tea.Msg {
				return ActionTypePickerDoneMsg{
					ActionIndex: m.actionIndex,
					Canceled:    true,
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

// View renders the action type picker.
func (m *ActionTypePickerModel) View() string {
	boxWidth := 60
	if m.Width() > 0 && m.Width() < boxWidth+10 {
		boxWidth = m.Width() - 10
	}

	var content strings.Builder

	content.WriteString(m.RenderSearchBar())

	if m.currentActionID != "" {
		content.WriteString(MutedStyle.Render("Current: " + m.currentActionID))
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
		Title:    "Select Action Type",
		Subtitle: "Type to search",
		Width:    boxWidth,
		Centered: true,
	})
}
