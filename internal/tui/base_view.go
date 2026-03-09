package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/config"
)

// ViewBundle groups the config and config manager fields shared by all views.
// Embed this anonymously so that m.config and m.configManager work unchanged.
type ViewBundle struct {
	config        *config.Config
	configManager *config.Manager
}

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

// IsFullHeight returns false by default.
func (v *BaseView) IsFullHeight() bool {
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
	cursorFlash   bool
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

// ClampCursorTo ensures cursor is within [0, length-1] range.
func (s *InteractiveViewState) ClampCursorTo(length int) {
	s.cursor = ClampCursor(s.cursor, length)
}

// ResetCursor sets the cursor to position 0.
func (s *InteractiveViewState) ResetCursor() {
	s.cursor = 0
}

// JumpToTop resets the cursor to 0 and returns a flash cmd if the jump
// was more than 1 position.
func (s *InteractiveViewState) JumpToTop() tea.Cmd {
	old := s.cursor
	s.cursor = 0
	return flashIfJumped(old, s.cursor, &s.cursorFlash)
}

// GetConfirmDialog returns a pointer to the embedded confirm dialog.
func (s *InteractiveViewState) GetConfirmDialog() *ConfirmDialog {
	return &s.confirm
}

// showDeleteConfirm activates the delete confirmation dialog for the given task.
func (s *InteractiveViewState) showDeleteConfirm(shortID int) {
	s.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", shortID))
}

// HandleConfirmKey checks if a confirm dialog is active and processes the key.
// Returns (confirmed, handled). On cancel, clears the status message.
// When handled is false, no confirm dialog was active and the caller should
// continue normal key processing.
func (s *InteractiveViewState) HandleConfirmKey(key string) (confirmed, handled bool) {
	if !s.confirm.IsActive() {
		return false, false
	}
	c, h := s.confirm.HandleKey(key)
	return c, h
}

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
	v.searchInput.Width = SearchInputWidth(w)
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

// HandleSearchMode processes a key message when search mode is active.
// Returns (model, cmd, true) if search is active and the key was consumed.
// Calls onChanged when the search input value changes.
func (v *BaseSearchableView) HandleSearchMode(
	msg tea.KeyMsg, model tea.Model, onChanged func(),
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

// UndoContext tracks the action label and description for the next undo snapshot.
// Embed this in views that support undo to avoid duplicating fields and methods.
type UndoContext struct {
	Action  string // e.g. "Complete Task", "Set Priority"
	Context string // e.g. task title or "3 tasks"
}

// Set records the action and context for the next undo snapshot.
func (u *UndoContext) Set(action, ctx string) {
	u.Action = action
	u.Context = ctx
}

// Clear resets the undo context after it has been consumed.
func (u *UndoContext) Clear() {
	u.Action = ""
	u.Context = ""
}

// DetailsPaneState manages state for a split-pane details view.
// Embed this in view models that show a scrollable details/preview pane.
type DetailsPaneState struct {
	Focused   bool
	Scroll    int
	MaxScroll int
	Height    int
}

// HandleScrollKey processes scroll-related keys when the details pane is focused.
// Returns true if the key was consumed (j/k, ctrl+d/ctrl+u, pgdown/pgup, g/G).
func (d *DetailsPaneState) HandleScrollKey(key string) bool {
	switch key {
	case "j", "down":
		d.Scroll++
		return true
	case "k", "up":
		if d.Scroll > 0 {
			d.Scroll--
		}
		return true
	case "ctrl+d", "pgdown":
		halfPage := max(d.Height/2, 1)
		d.Scroll += halfPage
		return true
	case "ctrl+u", "pgup":
		halfPage := max(d.Height/2, 1)
		d.Scroll -= halfPage
		if d.Scroll < 0 {
			d.Scroll = 0
		}
		return true
	case "g":
		d.Scroll = 0
		return true
	case "G":
		d.Scroll = 99999 // Will be clamped by rendering
		return true
	}
	return false
}

// ClampScroll ensures the scroll offset is within valid bounds.
func (d *DetailsPaneState) ClampScroll() {
	if d.Scroll > d.MaxScroll {
		d.Scroll = d.MaxScroll
	}
	if d.Scroll < 0 {
		d.Scroll = 0
	}
}

// ResetScroll resets the scroll offset to the top.
func (d *DetailsPaneState) ResetScroll() {
	d.Scroll = 0
}

// TabState holds the tab index and names for views with a tab bar.
// Embed or include this in view models to avoid repeating activeTab int
// plus a module-level tabNames slice.
type TabState struct {
	Active int
	Names  []string
}

// Cycle advances the active tab by delta with wrapping.
func (t *TabState) Cycle(delta int) {
	CycleTab(&t.Active, len(t.Names), delta)
}

// Count returns the number of tabs.
func (t *TabState) Count() int { return len(t.Names) }

// RenderBar renders the tab bar at the given width.
func (t *TabState) RenderBar(width int) string {
	return renderWrappingTabBar(t.Names, t.Active, width)
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

// ToggleSection flips a section's collapsed state.
func ToggleSection[K comparable](collapsed map[K]bool, key K) {
	collapsed[key] = !collapsed[key]
}
