package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TagAdder provides shared add-tag mode state and logic.
// Embed this in any model that has a ListSurfaceAdapter[TagInfo] and needs add-tag functionality.
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

func (a *TagAdder) leaveAddMode() {
	a.AddTagMode = false
	a.AddTagError = ""
	a.AddTagInput.Blur()
}

// handleAddTagInputKey owns the shared input lifecycle for every tag-creation
// surface. The caller supplies picker-specific insertion and selection behavior
// so each surface can retain its own stable IDs and selection semantics.
func (a *TagAdder) handleAddTagInputKey(
	msg tea.KeyPressMsg,
	submit func(string) error,
) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.leaveAddMode()
		return true, nil

	case "enter":
		tagName, err := models.NormalizeTaskTag(a.AddTagInput.Value())
		if err != nil {
			a.AddTagError = err.Error()
			return true, nil
		}
		if err := submit(tagName); err != nil {
			a.AddTagError = err.Error()
			return true, nil
		}
		a.leaveAddMode()
		return true, nil

	default:
		var cmd tea.Cmd
		a.AddTagInput, cmd = a.AddTagInput.Update(msg)
		return true, cmd
	}
}

// HandleAddTagKey processes a key message while in add-tag mode.
// The picker parameter is used to manipulate the item list and selection.
// Returns (handled, cmd). The caller should return (m, cmd) when handled is true.
func (a *TagAdder) HandleAddTagKey(msg tea.KeyPressMsg, picker *ListSurfaceAdapter[TagInfo]) (bool, tea.Cmd) {
	return a.handleAddTagInputKey(msg, func(tagName string) error {
		// Check for duplicate
		for _, item := range picker.SourceItems() {
			if item.Name == tagName {
				picker.PreSelectByID(tagName)
				picker.SetCursorToID(tagName)
				return nil
			}
		}

		// Add new tag
		items := append(picker.SourceItems(), TagInfo{Name: tagName, TaskCount: 0})
		sort.Slice(items, func(i, j int) bool {
			return items[i].Name < items[j].Name
		})
		if err := picker.ReplaceItems(items); err != nil {
			return err
		}
		picker.Filter(picker.SearchQuery())
		picker.PreSelectByID(tagName)
		picker.SetCursorToID(tagName)
		return nil
	})
}

// RenderAddTagMode renders the add tag input UI as a fullscreen view.
func (a *TagAdder) RenderAddTagMode(width, height int) string {
	var b strings.Builder

	b.WriteString(renderFullscreenTitle("Add New Tag", ""))
	b.WriteString("\n\n")

	b.WriteString(MutedStyle.Render(fmt.Sprintf(
		"Enter a tag name (max %d characters)",
		a.AddTagInput.CharLimit,
	)))
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
func RenderTagSelectorView(picker *ListSurfaceAdapter[TagInfo], adder *TagAdder, title string, width, height int) string {
	if adder.AddTagMode {
		return adder.RenderAddTagMode(width, height)
	}

	var b strings.Builder

	selectedCount := len(picker.SelectionMap())
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
	itemCount := MutedStyle.Render(fmt.Sprintf("%d items", len(picker.FilteredItems())))
	b.WriteString(HelpStyle.Render(renderWrappingHelpLine(footerText, width)))
	b.WriteString("\n")
	b.WriteString(itemCount)

	return b.String()
}

// renderTagSelectorFooter renders the standard footer with 'a' key for adding tags.
func renderTagSelectorFooter(picker *ListSurfaceAdapter[TagInfo]) string {
	if picker.IsSearchMode() {
		return "enter done · esc clear"
	}
	if len(picker.FilteredItems()) == 0 {
		return "a add tag · esc cancel"
	}
	return "space toggle · a add tag · / search · c clear · enter apply · esc cancel"
}
