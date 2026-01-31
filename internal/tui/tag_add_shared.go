package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	return TagAdder{AddTagInput: input}
}

// HasActiveInput returns true when in add tag mode.
func (a *TagAdder) HasActiveInput() bool {
	return a.AddTagMode
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
func (a *TagAdder) HandleAddTagKey(msg tea.KeyMsg, picker *BasePicker[TagInfo]) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.AddTagMode = false
		a.AddTagError = ""
		a.AddTagInput.Blur()
		return true, nil

	case "enter":
		tagName := strings.TrimSpace(a.AddTagInput.Value())
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

// RenderAddTagMode renders the add tag input UI as an overlay card.
func (a *TagAdder) RenderAddTagMode(width, height int) string {
	var content strings.Builder

	content.WriteString(MutedStyle.Render("Enter a tag name (max 50 characters)"))
	content.WriteString("\n\n")
	content.WriteString(a.AddTagInput.View())
	content.WriteString("\n\n")

	if a.AddTagError != "" {
		errorStyle := BoldStyle.Foreground(ColorError)
		content.WriteString(errorStyle.Render(a.AddTagError))
		content.WriteString("\n\n")
	}

	content.WriteString(MutedStyle.Render("enter add tag | esc cancel"))

	return renderOverlayCard(content.String(), width, height, OverlayCardConfig{
		Title:    "Add New Tag",
		Width:    50,
		Centered: true,
	})
}

// RenderTagSelectorView renders the standard tag selector view with search, list, footer, and overlay card.
func RenderTagSelectorView(picker *BasePicker[TagInfo], adder *TagAdder, title string, width, height int) string {
	if adder.AddTagMode {
		return adder.RenderAddTagMode(width, height)
	}

	boxWidth := 80
	if width > 0 && width < boxWidth+10 {
		boxWidth = width - 10
	}

	var content strings.Builder
	content.WriteString(picker.RenderSearchBar())

	maxVisible := 15
	if height > 0 {
		maxVisible = height - 14
	}
	picker.Config.BoxWidth = boxWidth
	content.WriteString(picker.RenderList(maxVisible))

	content.WriteString("\n")
	content.WriteString(renderTagSelectorFooter(picker))

	selectedCount := len(picker.Selected)
	var subtitle string
	if selectedCount == 0 {
		subtitle = "No tags selected"
	} else {
		subtitle = fmt.Sprintf("%d tag%s selected", selectedCount, pluralize(selectedCount, "", "s"))
	}

	return renderOverlayCard(content.String(), width, height, OverlayCardConfig{
		Title:    title,
		Subtitle: subtitle,
		Width:    boxWidth,
		Centered: true,
	})
}

// renderTagSelectorFooter renders the standard footer with 'a' key for adding tags.
func renderTagSelectorFooter(picker *BasePicker[TagInfo]) string {
	if picker.IsSearchMode() {
		return MutedStyle.Render("enter done | esc clear")
	}
	if len(picker.Filtered) == 0 {
		return MutedStyle.Render("a add tag | esc cancel")
	}
	return MutedStyle.Render("space toggle | a add tag | / search | c clear | enter apply | esc cancel")
}
