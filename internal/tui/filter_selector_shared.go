package tui

import (
	"fmt"
	"sort"

	tea "charm.land/bubbletea/v2"
)

// filterSelectorUpdate handles the common Update logic for filter selectors.
// cancelMsg is called to produce the cancel message, doneMsg to produce the done message with selected IDs.
func filterSelectorUpdate[T PickerItem](picker *BasePicker[T], msg tea.Msg, cancelMsg func() tea.Msg, doneMsg func([]string) tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		picker.SetSize(msg.Width, msg.Height)
		return nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return cancelMsg
		case "enter":
			selected := picker.SelectedIDs()
			sort.Strings(selected)
			return func() tea.Msg { return doneMsg(selected) }
		default:
			_, cmd := picker.HandleKeyMsg(msg)
			return cmd
		}
	}
	return nil
}

// filterSelectorView renders the common View for filter selectors.
// itemLabel should be "tag" or "section" (will be pluralized automatically).
func filterSelectorView[T PickerItem](picker *BasePicker[T], itemLabel string) string {
	selectedCount := len(picker.Selected)
	var headerContent string
	if selectedCount == 0 {
		headerContent = MutedStyle.Render(fmt.Sprintf("No %ss selected (showing all)", itemLabel))
	} else {
		headerContent = MutedStyle.Render(fmt.Sprintf("%d %s%s selected", selectedCount, itemLabel, pluralize(selectedCount, "", "s")))
	}
	return picker.RenderInBox(picker.Config.Title, headerContent, 0)
}
