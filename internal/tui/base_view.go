package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// BaseView provides common functionality for TUI views.
// Embed this in view models to get standard dimension and error handling.
type BaseView struct {
	width  int
	height int
	err    error
}

// SetSize updates the view dimensions.
func (v *BaseView) SetSize(w, h int) {
	v.width = w
	v.height = h
}

// Width returns the current width.
func (v *BaseView) Width() int {
	return v.width
}

// Height returns the current height.
func (v *BaseView) Height() int {
	return v.height
}

// SetError sets the current error.
func (v *BaseView) SetError(err error) {
	v.err = err
}

// ClearError clears the current error.
func (v *BaseView) ClearError() {
	v.err = nil
}

// HasError returns true if there is an error.
func (v *BaseView) HasError() bool {
	return v.err != nil
}

// Error returns the current error.
func (v *BaseView) Error() error {
	return v.err
}

// ShouldCaptureEsc returns false by default.
func (v *BaseView) ShouldCaptureEsc() bool {
	return false
}

// HasTextInput returns false by default.
func (v *BaseView) HasTextInput() bool {
	return false
}

// ContentWidth returns width minus standard padding (2 on each side).
func (v *BaseView) ContentWidth() int {
	return max(v.width-4, 20)
}

// ContentHeight returns height minus standard padding (1 on each side).
func (v *BaseView) ContentHeight() int {
	return max(v.height-2, 5)
}

// InteractiveViewState provides common interactive state for views that
// support cursor navigation, prefix key sequences, status messages, and
// confirmation dialogs. Embed this in view models to eliminate duplicate
// field declarations and HasPendingPrefix() methods.
type InteractiveViewState struct {
	cursor        int
	pendingPrefix string
	statusMessage string
	confirm       ConfirmDialog
}

// HasPendingPrefix returns true if waiting for second key in a prefix sequence.
func (s *InteractiveViewState) HasPendingPrefix() bool {
	return s.pendingPrefix != ""
}

// DefaultShouldCaptureEsc returns true if a pending prefix or confirm dialog is active.
// Views should call this in their ShouldCaptureEsc() override and OR in their own conditions.
// Views that also embed BaseSearchableView should additionally OR in m.SearchShouldCaptureEsc().
func (s *InteractiveViewState) DefaultShouldCaptureEsc() bool {
	return s.pendingPrefix != "" || s.confirm.IsActive()
}

// ResetCursor sets the cursor to position 0.
func (s *InteractiveViewState) ResetCursor() {
	s.cursor = 0
}

// showDeleteConfirm activates the delete confirmation dialog for the given task.
func (s *InteractiveViewState) showDeleteConfirm(shortID int) {
	s.confirm.Show("delete", fmt.Sprintf("Delete task #%d? (y/n)", shortID))
	s.statusMessage = s.confirm.Prompt()
}

// BaseSearchableView extends BaseView with search functionality.
type BaseSearchableView struct {
	BaseView
	searchActive bool
	searchQuery  string
	searchInput  textinput.Model
}

// InitSearchInput initializes the search input with the given placeholder.
// Call this in the constructor of views that embed BaseSearchableView.
func (v *BaseSearchableView) InitSearchInput(placeholder string, charLimit, width int) {
	v.searchInput = textinput.New()
	v.searchInput.Placeholder = placeholder
	v.searchInput.CharLimit = charLimit
	v.searchInput.Width = width
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

// HandleSearchInput processes a key message while in search mode.
// Returns (handled, cmd, valueChanged) where:
//   - handled: true if the key was consumed (esc or enter)
//   - cmd: any command to return (e.g., from textinput update)
//   - valueChanged: true if the search input value changed
//
// The caller should check handled first. If true, no further processing is needed.
// If valueChanged is true, the caller should update their filtered list.
func (v *BaseSearchableView) HandleSearchInput(msg tea.KeyMsg) (handled bool, cmd tea.Cmd, valueChanged bool) {
	switch msg.String() {
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

// WrapCursor wraps a cursor position within list bounds.
// delta is typically 1 (down) or -1 (up).
func WrapCursor(current, listLen, delta int) int {
	if listLen == 0 {
		return 0
	}
	result := current + delta
	if result < 0 {
		return listLen - 1
	}
	if result >= listLen {
		return 0
	}
	return result
}

// CycleTab advances a tab index by delta with wrapping.
// delta is typically +1 (right) or -1 (left).
func CycleTab(current *int, count, delta int) {
	if count <= 0 {
		return
	}
	*current = ((*current+delta)%count + count) % count
}

// ExpandAllSections clears the collapsed map so all sections are visible.
func ExpandAllSections[K comparable](collapsed map[K]bool) {
	for k := range collapsed {
		delete(collapsed, k)
	}
}

// CollapseAllSections marks all given keys as collapsed.
func CollapseAllSections[K comparable](collapsed map[K]bool, keys []K) {
	for _, k := range keys {
		collapsed[k] = true
	}
}
