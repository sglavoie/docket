package tui

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// PerspectiveInactive is the sentinel value used by views (Cross-Kanban, Today)
// that treat perspectives as an optional lens. -1 means "no perspective
// filter active"; valid perspective indices are 0..9.
const PerspectiveInactive = -1

// PerspectivePrefixDeps wires the "P" prefix perspective bindings to a view.
//
// SelectByID is invoked for P0..P9 with the shortcut's stable perspective ID.
// CycleNext / CyclePrev advance the active perspective by ±1 with view-defined
// semantics (e.g. wrapping, skipping the inactive sentinel).
type PerspectivePrefixDeps struct {
	SelectByID func(perspID int) (tea.Model, tea.Cmd)
	CycleNext  func() (tea.Model, tea.Cmd)
	CyclePrev  func() (tea.Model, tea.Cmd)
	Gate       keymap.Gate // optional; nil = always active
}

// BuildPerspectivePrefixEntry returns the "P" prefix entry shared by views that
// expose perspectives as a lens (Cross-Kanban, Today). Children:
//   - 0..9      direct preset selection (P0..P9)
//   - P         cycle to next perspective (PP)
//   - N         cycle to previous perspective (PN)
func BuildPerspectivePrefixEntry(deps PerspectivePrefixDeps) keymap.Entry {
	cat := "Perspectives"
	children := make([]keymap.Entry, 0, 12)
	for key := range 10 {
		perspective, _ := PerspectiveByKey(key)
		id := perspective.ID
		children = append(children, keymap.Bind(
			strconv.Itoa(key),
			perspective.Name+" perspective",
			cat,
			func() (tea.Model, tea.Cmd) { return deps.SelectByID(id) },
		))
	}
	children = append(children,
		keymap.Bind("P", "Cycle perspective next", cat, deps.CycleNext),
		keymap.Bind("N", "Cycle perspective previous", cat, deps.CyclePrev),
	)
	var opts []keymap.PrefixOption
	if deps.Gate != nil {
		opts = append(opts, keymap.WithPrefixGate(deps.Gate))
	}
	return keymap.Prefix("P", "Switch perspective", cat, children, opts...)
}

// CyclePerspectiveLens advances `current` by `delta` (±1) for views that use
// the PerspectiveInactive sentinel. Cycling from PerspectiveInactive lands on
// All for delta>0 and Completed for delta<0; otherwise it follows the shared
// canonical rotation. The sentinel state is exited but never re-entered.
func CyclePerspectiveLens(current, delta int) int {
	return CyclePerspective(current, delta)
}

// PerspectiveLensName returns the display name for `idx`, or "" when the lens
// is inactive (idx == PerspectiveInactive or out of range).
func PerspectiveLensName(idx int) string {
	if idx < 0 || idx > 9 {
		return ""
	}
	p, _ := PerspectiveByID(idx)
	return p.Name
}

// PerspectiveLensFilter returns the predicate for `idx`, or nil when the lens
// is inactive. Callers should treat nil as "pass all tasks".
func PerspectiveLensFilter(idx int) func(*models.Task) bool {
	if idx < 0 || idx > 9 {
		return nil
	}
	p, ok := PerspectiveByID(idx)
	if !ok {
		return nil
	}
	return p.Filter
}
