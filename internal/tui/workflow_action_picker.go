package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/workflow"
)

// ActionTypePickerModel allows selecting an action type from the full list.
type ActionTypePickerModel struct {
	*ListSurfaceAdapter[ActionTypeInfo]
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

	picker := NewListSurfaceAdapter(items, PickerConfig{
		Title:          "Select Action Type",
		EmptyMessage:   "No action types available.",
		NoMatchMessage: "No matching actions.",
		MultiSelect:    false,
		ShowCounts:     false,
		BoxWidth:       60,
	})

	currentID := string(currentType)
	picker.SetCursorToID(currentID)

	return &ActionTypePickerModel{
		ListSurfaceAdapter: picker,
		actionIndex:        actionIndex,
		currentActionID:    currentID,
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

	case tea.KeyPressMsg:
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
		case "esc":
			return m, func() tea.Msg {
				return ActionTypePickerDoneMsg{
					ActionIndex: m.actionIndex,
					Canceled:    true,
				}
			}

		case "enter":
			if len(m.FilteredItems()) > 0 && m.CursorIndex() < len(m.FilteredItems()) {
				actionType := m.FilteredItems()[m.CursorIndex()].ActionType
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
func (m *ActionTypePickerModel) View() tea.View {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Select Action Type", "Type to search"))
	b.WriteString("\n\n")

	b.WriteString(m.RenderSearchBar())

	if m.currentActionID != "" {
		b.WriteString(MutedStyle.Render("Current: " + m.currentActionID))
		b.WriteString("\n\n")
	}

	maxVisible := 15
	if m.Height() > 0 {
		maxVisible = m.Height() - 8
	}
	b.WriteString(m.RenderList(maxVisible))

	left := HelpStyle.Render(m.RenderFooter())
	right := MutedStyle.Render(fmt.Sprintf("%d items", len(m.FilteredItems())))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return tea.NewView(b.String())
}
