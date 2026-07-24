package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// SearchInputWidth returns a dynamic search input width based on the view width.
// The width is capped between 20 and 60 characters.
func SearchInputWidth(viewWidth int) int {
	w := min(max(viewWidth/3, 20), 60)
	return w
}

// BaseSearchableView extends BaseView with search functionality.
type BaseSearchableView struct {
	BaseView
	searchActive      bool
	searchQuery       string
	searchInput       textinput.Model
	searchHelpVisible bool
	ac                AutocompleteState
}

// SetSize updates the view dimensions and adjusts the search input width.
func (v *BaseSearchableView) SetSize(w, h int) {
	v.BaseView.SetSize(w, h)
	v.searchInput.SetWidth(SearchInputWidth(w))
}

// InitSearchInput initializes the search input with the given placeholder.
// Call this in the constructor of views that embed BaseSearchableView.
func (v *BaseSearchableView) InitSearchInput(placeholder string, charLimit, width int) {
	v.searchInput = textinput.New()
	v.searchInput.Placeholder = placeholder
	v.searchInput.CharLimit = charLimit
	v.searchInput.SetWidth(width)
	applyThemedTextInput(&v.searchInput)
}

// HasActiveSearch returns true if search mode is active.
func (v *BaseSearchableView) HasActiveSearch() bool {
	return v.searchActive
}

// IsSearchMode returns true if currently in search input mode.
func (v *BaseSearchableView) IsSearchMode() bool {
	return v.searchActive
}

// DefaultHasTextInput returns true if search mode is active.
// Views should call this in their HasTextInput() override and OR in their own conditions.
func (v *BaseSearchableView) DefaultHasTextInput() bool {
	return v.searchActive
}

// SearchShouldCaptureEsc returns true if search input is active.
// Include this in ShouldCaptureEsc() overrides of views that embed BaseSearchableView:
//
//	return m.InteractiveViewState.DefaultShouldCaptureEsc() || m.SearchShouldCaptureEsc()
func (v *BaseSearchableView) SearchShouldCaptureEsc() bool {
	return v.searchActive
}

// IsSearchHelpVisible returns true if the search help pane is visible.
func (v *BaseSearchableView) IsSearchHelpVisible() bool {
	return v.searchHelpVisible
}

// SearchAC returns a pointer to the autocomplete state for search.
func (v *BaseSearchableView) SearchAC() *AutocompleteState {
	return &v.ac
}

// ClearSearch clears the search state and resets the input.
func (v *BaseSearchableView) ClearSearch() {
	v.searchActive = false
	v.searchQuery = ""
	v.searchInput.SetValue("")
	v.searchInput.Blur()
	v.searchHelpVisible = false
	v.ac.Clear()
}

// SetSearchActive sets the search active state.
func (v *BaseSearchableView) SetSearchActive(active bool) {
	v.searchActive = active
}

// SearchQuery returns the current search query.
func (v *BaseSearchableView) SearchQuery() string {
	return v.searchQuery
}

// SetSearchQuery sets the search query.
func (v *BaseSearchableView) SetSearchQuery(query string) {
	v.searchQuery = query
}

// SearchInputValue returns the current value of the search input.
func (v *BaseSearchableView) SearchInputValue() string {
	return v.searchInput.Value()
}

// SetSearchInputValue sets the underlying search input's text, used when
// restoring a captured search state (e.g. an outliner scope pop).
func (v *BaseSearchableView) SetSearchInputValue(value string) {
	v.searchInput.SetValue(value)
}

// SearchInputView returns the rendered search input for display.
func (v *BaseSearchableView) SearchInputView() string {
	return v.searchInput.View()
}

// ActivateSearch enters search mode and focuses the input.
// Returns a command to start the cursor blinking.
func (v *BaseSearchableView) ActivateSearch() tea.Cmd {
	v.searchActive = true
	v.searchInput.Focus()
	return textinput.Blink
}

// DeactivateSearch exits search mode, blurs the input, and clears it.
// The search query is also cleared.
func (v *BaseSearchableView) DeactivateSearch() {
	v.searchActive = false
	v.searchInput.Blur()
	v.searchInput.SetValue("")
	v.searchQuery = ""
	v.searchHelpVisible = false
	v.ac.Clear()
}

// ConfirmSearch exits search mode but keeps the current query.
func (v *BaseSearchableView) ConfirmSearch() {
	v.searchActive = false
	v.searchInput.Blur()
	v.searchQuery = v.searchInput.Value()
	v.searchHelpVisible = false
	v.ac.Clear()
}

// HandleSearchMode processes a key message when search mode is active.
// Returns (model, cmd, true) if search is active and the key was consumed.
// Calls onChanged when the search input value changes.
func (v *BaseSearchableView) HandleSearchMode(
	msg tea.KeyPressMsg, model tea.Model, onChanged func(),
) (tea.Model, tea.Cmd, bool) {
	if !v.searchActive {
		return model, nil, false
	}
	_, cmd, valueChanged := v.HandleSearchInput(msg)
	if valueChanged && onChanged != nil {
		onChanged()
	}
	return model, cmd, true
}

// HandleSearchInput processes a key message while in search mode.
// Returns (handled, cmd, valueChanged) where:
//   - handled: true if the key was consumed (esc or enter)
//   - cmd: any command to return (e.g., from textinput update)
//   - valueChanged: true if the search input value changed
//
// The caller should check handled first. If true, no further processing is needed.
// If valueChanged is true, the caller should update their filtered list.
func (v *BaseSearchableView) HandleSearchInput(msg tea.KeyPressMsg) (handled bool, cmd tea.Cmd, valueChanged bool) {
	key := msg.String()

	// Handle autocomplete navigation when active (tab/enter/esc/up/down)
	if v.ac.HandleKey(key, func() {
		v.acceptSearchAutocomplete()
	}) {
		if key == "tab" || key == "enter" {
			return false, nil, true
		}
		return true, nil, false
	}

	switch key {
	case "esc":
		v.DeactivateSearch()
		return true, nil, true
	case "enter":
		v.ConfirmSearch()
		return true, nil, false
	case "?":
		if !v.ac.IsActive() {
			v.searchHelpVisible = !v.searchHelpVisible
			return true, nil, false
		}
		fallthrough
	default:
		oldVal := v.searchInput.Value()
		v.searchInput, cmd = v.searchInput.Update(msg)
		valueChanged = oldVal != v.searchInput.Value()
		if valueChanged {
			v.searchQuery = v.searchInput.Value()
		}
		return false, cmd, valueChanged
	}
}

// acceptSearchAutocomplete inserts the selected suggestion into the search input.
func (v *BaseSearchableView) acceptSearchAutocomplete() {
	if !v.ac.IsActive() {
		return
	}

	var newInput string
	if kw := keywordForMode(v.ac.Mode); kw != "" {
		newInput = v.ac.BuildKeywordAcceptedInput(v.searchInput.Value(), kw)
	} else {
		triggerChar := TriggerCharForMode(v.ac.Mode, searchTriggers)
		newInput = v.ac.BuildAcceptedInput(v.searchInput.Value(), triggerChar)
	}

	v.searchInput.SetValue(newInput)
	v.searchInput.SetCursor(len(newInput))
	v.ac.Clear()
}
