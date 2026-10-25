package listsurface

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
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
	searchActive bool
	searchQuery  string
	searchInput  textinput.Model
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
	theme.ApplyThemedTextInput(&v.searchInput)
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

// ClearSearch clears the search state and resets the input.
func (v *BaseSearchableView) ClearSearch() {
	v.searchActive = false
	v.searchQuery = ""
	v.searchInput.SetValue("")
	v.searchInput.Blur()
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

// SearchInputSnapshot is the whole of what a restoration point owes the search
// input: the confirmed query, the text still standing in the box, and whether
// the box has focus. The three travel together because they disagree
// legitimately — a confirmed search holds a query with the input closed, and a
// search still being typed holds a value the query has not caught up with — so
// restoring any one of them alone reinstalls a state the user was never in.
type SearchInputSnapshot struct {
	Query  string
	Value  string
	Active bool
}

// CaptureSearchInput reads that state without disturbing it.
func (v *BaseSearchableView) CaptureSearchInput() SearchInputSnapshot {
	return SearchInputSnapshot{
		Query:  v.searchQuery,
		Value:  v.searchInput.Value(),
		Active: v.searchActive,
	}
}

// RestoreSearchInput writes it back. It assigns the query rather than routing
// through SetSearchQuery, because an embedder that overrides that setter to open
// a projection of its own would reopen one here, and the caller restoring this
// already holds the projection the query belongs to.
func (v *BaseSearchableView) RestoreSearchInput(snapshot SearchInputSnapshot) {
	v.searchQuery = snapshot.Query
	v.searchInput.SetValue(snapshot.Value)
	v.searchActive = snapshot.Active
}

// SearchInputCursor returns the caret's rune index inside the search input.
// Cursor-aware completion needs it so a suggestion accepted mid-query replaces
// only the token under the caret.
func (v *BaseSearchableView) SearchInputCursor() int {
	return v.searchInput.Position()
}

// SetSearchInputCursor moves the caret to a rune index, clamped by the input.
func (v *BaseSearchableView) SetSearchInputCursor(position int) {
	v.searchInput.SetCursor(position)
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
}

// ConfirmSearch exits search mode but keeps the current query.
func (v *BaseSearchableView) ConfirmSearch() {
	v.searchActive = false
	v.searchInput.Blur()
	v.searchQuery = v.searchInput.Value()
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

	switch key {
	case "esc":
		v.DeactivateSearch()
		return true, nil, true
	case "enter":
		v.ConfirmSearch()
		return true, nil, false
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
