package tui

import (
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/route"
)

// viewNavigationController is the shell's view and navigation cluster: the
// authoritative route and overlay stacks, the registry every renderer is
// installed in, and the terminal geometry both are sized against. It grows the
// NavigationState precedent — that field keeps its own type and stays named,
// with the registry and geometry it was already paired with joining it rather
// than replacing it.
//
// It owns the machinery, not the renderers. The live view instances
// (crossOutliner, helpView, manageHub, taskComposer, lenses.picker,
// conflictBrowser) stay AppModel fields: foundation_invariants_test.go pins
// "AppModel has exactly one field mentioning OutlinerModel" against the
// AppModel struct itself, and a pointer to a view is not navigation state.
//
// Registration remains a separate path: showView is the single site that sizes,
// wires, registers, caches, pushes, and sets accent, and the paint-time
// injection in View() is its own pass over the active renderer. This type is
// deliberately not a seam where those two could merge.
type viewNavigationController struct {
	// navigation is the sole live shell authority. PresentationID values attached
	// to routes and overlays select renderers; they never form a second
	// navigation stack.
	navigation route.NavigationState

	// views is the centralized registry of installed renderers.
	views *route.ViewRegistry

	// width and height are the terminal dimensions the shell composes its frame
	// against. They are distinct from the InteractiveViewState/BaseView fields of
	// the same name on views: those are per-renderer budgets, these are the
	// terminal's own size.
	width  int
	height int

	// tooNarrow and tooShort record that the terminal cannot fit the minimum
	// frame, which the shell renders instead of any view.
	tooNarrow bool
	tooShort  bool
}

// newViewNavigationController builds the cluster at its startup defaults, which
// stand in until the first WindowSizeMsg arrives.
func newViewNavigationController() viewNavigationController {
	return viewNavigationController{
		navigation: route.NewNavigationState(),
		views:      route.NewViewRegistry(),
		width:      80,
		height:     24,
	}
}

// size returns the terminal dimensions.
func (c *viewNavigationController) size() (int, int) { return c.width, c.height }

// viewSize returns the dimensions every renderer is sized to. All views —
// full-height ones included — get height-2, because ViewRegistry.Register
// re-applies its stored height-2 and would clobber anything else anyway.
func (c *viewNavigationController) viewSize() (int, int) { return c.width, c.height - 2 }

// tooSmall reports whether the terminal is below the minimum frame in either
// axis, so the shell renders the size notice instead of a view.
func (c *viewNavigationController) tooSmall() bool { return c.tooNarrow || c.tooShort }

// resize records new terminal dimensions and re-sizes every registered
// renderer to the view budget.
func (c *viewNavigationController) resize(width, height int) {
	c.width = width
	c.height = height
	c.tooNarrow = width < chrome.MinTerminalWidth
	c.tooShort = height < chrome.MinTerminalHeight
	c.views.SetSize(c.viewSize())
}

// atRoot reports whether navigation is the bare Outliner root: one primary
// frame and no overlay. Its negation is what makes an Esc have something to
// unwind.
func (c *viewNavigationController) atRoot() bool {
	return c.navigation.Primary.Depth() == 1 && !c.navigation.Overlays.Active()
}
