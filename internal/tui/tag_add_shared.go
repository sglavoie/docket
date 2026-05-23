package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// TagAdder provides shared add-tag mode state and logic.
// Embed this in any model that has a BasePicker[TagInfo] and needs add-tag functionality.
type TagAdder struct {
	AddTagMode  bool
	AddTagInput textinput.Model
	AddTagError string
}

// NewTagAdder creates a TagAdder with a configured text input.
func NewTagAdder() TagAdder {
	input := textinput.New()
	input.CharLimit = 50
	applyThemedTextInput(&input)
	return TagAdder{AddTagInput: input}
}

// HasTextInput returns true when the add-tag input is active.
func (a *TagAdder) HasTextInput() bool {
	return a.AddTagMode
}

// ShouldCaptureEsc returns true when the add-tag input is active.
func (a *TagAdder) ShouldCaptureEsc() bool {
	return a.AddTagMode
}

// EnterAddMode activates add-tag mode and returns the blink command.
func (a *TagAdder) EnterAddMode() tea.Cmd {
	a.AddTagMode = true
	a.AddTagError = ""
	a.AddTagInput.Reset()
	a.AddTagInput.SetValue("")
	a.AddTagInput.Focus()
	return textinput.Blink
}

// HandleAddTagKey processes a key message while in add-tag mode.
// The picker parameter is used to manipulate the item list and selection.
// Returns (handled, cmd). The caller should return (m, cmd) when handled is true.
func (a *TagAdder) HandleAddTagKey(msg tea.KeyPressMsg, picker *BasePicker[TagInfo]) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.AddTagMode = false
		a.AddTagError = ""
		a.AddTagInput.Blur()
		return true, nil

	case "enter":
		// Strip a leading '#' so users who type quick-capture syntax
		// (#backend) don't end up with tags literally containing a hash;
		// tag rendering always prepends '#' itself.
		tagName := strings.TrimSpace(a.AddTagInput.Value())
		tagName = strings.TrimSpace(strings.TrimPrefix(tagName, "#"))
		if tagName == "" {
			a.AddTagError = "Tag name cannot be empty"
			return true, nil
		}

		// Check for duplicate
		for _, item := range picker.Items {
			if item.Name == tagName {
				picker.PreSelectByID(tagName)
				picker.SetCursorToID(tagName)
				a.AddTagError = fmt.Sprintf("Tag '%s' already exists - selected it", tagName)
				a.AddTagMode = false
				a.AddTagInput.Blur()
				return true, nil
			}
		}

		// Add new tag
		picker.Items = append(picker.Items, TagInfo{Name: tagName, TaskCount: 0})
		sort.Slice(picker.Items, func(i, j int) bool {
			return picker.Items[i].Name < picker.Items[j].Name
		})
		picker.Filter(picker.SearchQuery())
		picker.PreSelectByID(tagName)
		picker.SetCursorToID(tagName)

		a.AddTagMode = false
		a.AddTagError = ""
		a.AddTagInput.Blur()
		return true, nil

	default:
		var cmd tea.Cmd
		a.AddTagInput, cmd = a.AddTagInput.Update(msg)
		return true, cmd
	}
}

// RenderAddTagMode renders the add tag input UI as a fullscreen view.
func (a *TagAdder) RenderAddTagMode(width, height int) string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Add New Tag", ""))
	b.WriteString("\n\n")

	b.WriteString(MutedStyle.Render("Enter a tag name (max 50 characters)"))
	b.WriteString("\n\n")
	b.WriteString(a.AddTagInput.View())
	b.WriteString("\n\n")

	if a.AddTagError != "" {
		errorStyle := BoldStyle.Foreground(ColorError)
		b.WriteString(errorStyle.Render(a.AddTagError))
		b.WriteString("\n")
	}

	left := HelpStyle.Render("enter add tag · esc cancel")
	b.WriteString(renderStatusBarFooter(left, "", width))

	return b.String()
}

// RenderTagSelectorView renders the standard tag selector view as a fullscreen layout.
func RenderTagSelectorView(picker *BasePicker[TagInfo], adder *TagAdder, title string, width, height int) string {
	if adder.AddTagMode {
		return adder.RenderAddTagMode(width, height)
	}

	var b strings.Builder

	selectedCount := len(picker.Selected)
	var subtitle string
	if selectedCount == 0 {
		subtitle = "No tags selected"
	} else {
		subtitle = fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s"))
	}

	b.WriteString(renderFullscreenTitle(title, subtitle))
	b.WriteString("\n\n")

	b.WriteString(picker.RenderSearchBar())

	maxVisible := 15
	if height > 0 {
		maxVisible = height - 8
	}
	b.WriteString(picker.RenderList(maxVisible))

	footerText := renderTagSelectorFooter(picker)
	itemCount := MutedStyle.Render(fmt.Sprintf("%d items", len(picker.Filtered)))
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(footerText, width)))
	b.WriteString("\n")
	b.WriteString(itemCount)

	return b.String()
}

// renderTagSelectorFooter renders the standard footer with 'a' key for adding tags.
func renderTagSelectorFooter(picker *BasePicker[TagInfo]) string {
	if picker.IsSearchMode() {
		return "enter done · esc clear"
	}
	if len(picker.Filtered) == 0 {
		return "a add tag · esc cancel"
	}
	return "space toggle · a add tag · / search · c clear · enter apply · esc cancel"
}
