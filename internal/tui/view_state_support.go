package tui

import "image/color"

// UndoContext tracks the action label and description for the next undo snapshot.
// Embed this in views that support undo to avoid duplicating fields and methods.
type UndoContext struct {
	Action     string   // e.g. "Complete Task", "Set Priority"
	Context    string   // e.g. task title or "3 tasks"
	ProjectIDs []string // Affected project IDs (cross-project operations)
	TaskIDs    []string // Task IDs explicitly changed by user (enables targeted undo)
	// BackupPaths is intentionally omitted: storage.RestoreFromBackup derives
	// the backup path from the project automatically, so callers never need to
	// store it. Use crossHandleUndo → RestoreFromBackup for all undo operations.
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
	u.ProjectIDs = nil
	u.TaskIDs = nil
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
	Active    int
	Names     []string
	TabColors []color.Color // per-tab inactive foreground override (nil entry = default muted)
}

// Cycle advances the active tab by delta with wrapping.
func (t *TabState) Cycle(delta int) {
	CycleTab(&t.Active, len(t.Names), delta)
}

// Count returns the number of tabs.
func (t *TabState) Count() int { return len(t.Names) }

// RenderBar renders the tab bar at the given width.
// Pass an optional accent color to highlight the active tab with a distinct color.
func (t *TabState) RenderBar(width int, accent ...color.Color) string {
	return renderWrappingTabBarColored(t.Names, t.Active, t.TabColors, width, accent...)
}
