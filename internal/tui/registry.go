package tui

import tea "github.com/charmbracelet/bubbletea"

// ViewRegistry manages view instances and provides centralized view operations.
// It supports lazy initialization of views through factories.
type ViewRegistry struct {
	views  map[ViewType]View
	width  int
	height int
}

// NewViewRegistry creates a new view registry.
func NewViewRegistry() *ViewRegistry {
	return &ViewRegistry{
		views:  make(map[ViewType]View),
		width:  80,
		height: 24,
	}
}

// Register adds a view instance to the registry.
func (r *ViewRegistry) Register(vt ViewType, view View) {
	r.views[vt] = view
	if r.width > 0 && r.height > 0 {
		view.SetSize(r.width, r.height)
	}
}

// Get returns the view for the given type, or nil if not registered.
func (r *ViewRegistry) Get(vt ViewType) View {
	return r.views[vt]
}

// Has returns true if a view is registered for the given type.
func (r *ViewRegistry) Has(vt ViewType) bool {
	_, ok := r.views[vt]
	return ok
}

// Remove removes a view from the registry.
func (r *ViewRegistry) Remove(vt ViewType) {
	delete(r.views, vt)
}

// SetSize updates dimensions for all registered views.
func (r *ViewRegistry) SetSize(width, height int) {
	r.width = width
	r.height = height
	for _, view := range r.views {
		view.SetSize(width, height)
	}
}

// Update delegates the Update call to the specified view type.
// Returns the tea.Cmd from the view's Update method.
func (r *ViewRegistry) Update(vt ViewType, msg tea.Msg) tea.Cmd {
	view := r.views[vt]
	if view == nil {
		return nil
	}

	newModel, cmd := view.Update(msg)
	// Update the registry with the new model state
	if newView, ok := newModel.(View); ok {
		r.views[vt] = newView
	}
	return cmd
}

// View returns the View() output for the specified view type.
func (r *ViewRegistry) View(vt ViewType) string {
	view := r.views[vt]
	if view == nil {
		return ""
	}
	return view.View()
}

// Clear removes all views from the registry.
func (r *ViewRegistry) Clear() {
	r.views = make(map[ViewType]View)
}

// Count returns the number of registered views.
func (r *ViewRegistry) Count() int {
	return len(r.views)
}

// Types returns all registered view types.
func (r *ViewRegistry) Types() []ViewType {
	types := make([]ViewType, 0, len(r.views))
	for vt := range r.views {
		types = append(types, vt)
	}
	return types
}
