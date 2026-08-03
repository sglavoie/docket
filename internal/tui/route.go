package tui

import "image/color"

// RouteScope classifies the data owned by the active route or presentation.
type RouteScope int

const (
	ScopeApp RouteScope = iota
	ScopeProject
	ScopeTask
)

// AccentCategory names the workflow-stage accent painted by a presentation.
// Concrete colors are resolved from the active theme at render time.
type AccentCategory int

const (
	AccentCatNone AccentCategory = iota
	AccentCatCapture
	AccentCatTriage
	AccentCatExecute
	AccentCatSupport
)

// presentationScope derives command scope directly from the active renderer.
// Navigation identity is owned exclusively by RouteID and its typed params.
func presentationScope(view PresentationID) RouteScope {
	switch view {
	case ViewGlobalNotes, ViewCrossProjectOutliner, ViewContextHub, ViewProjectHub:
		return ScopeApp
	case ViewTaskComments, ViewTaskChecklist, ViewTaskAttachments:
		return ScopeTask
	default:
		return ScopeProject
	}
}

// presentationAccentCategory assigns a visual accent without introducing a
// second navigation registry.
func presentationAccentCategory(view PresentationID) AccentCategory {
	switch view {
	case ViewTaskComposer:
		return AccentCatCapture
	case ViewCrossProjectOutliner:
		return AccentCatTriage
	case ViewGlobalNotes, ViewNotes, ViewContextHub, ViewProjectHub:
		return AccentCatSupport
	default:
		return AccentCatNone
	}
}

// accentColorForCategory resolves an accent category against the active theme.
func accentColorForCategory(category AccentCategory) color.Color {
	switch category {
	case AccentCatCapture:
		return AccentCapture
	case AccentCatTriage:
		return AccentTriage
	case AccentCatExecute:
		return AccentExecute
	case AccentCatSupport:
		return AccentSupport
	default:
		return nil
	}
}

// allPresentations returns the contiguous renderer inventory.
func allPresentations() []PresentationID {
	presentations := make([]PresentationID, 0, int(ViewProjectHub))
	for presentation := ViewProjectCreator; presentation <= ViewProjectHub; presentation++ {
		presentations = append(presentations, presentation)
	}
	return presentations
}
