package route

import (
	"fmt"
	"image/color"

	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/theme"
)

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
func presentationScope(view presentation.PresentationID) RouteScope {
	switch view {
	case presentation.ViewGlobalNotes, presentation.ViewCrossProjectOutliner, presentation.ViewProjectHub:
		return ScopeApp
	case presentation.ViewTaskComments, presentation.ViewTaskAttachments:
		return ScopeTask
	default:
		return ScopeProject
	}
}

// presentationAccentCategory assigns a visual accent without introducing a
// second navigation registry.
func presentationAccentCategory(view presentation.PresentationID) AccentCategory {
	switch view {
	case presentation.ViewTaskComposer:
		return AccentCatCapture
	case presentation.ViewCrossProjectOutliner:
		return AccentCatTriage
	case presentation.ViewGlobalNotes, presentation.ViewNotes, presentation.ViewProjectHub:
		return AccentCatSupport
	default:
		return AccentCatNone
	}
}

// accentColorForCategory resolves an accent category against the active theme.
func accentColorForCategory(category AccentCategory) color.Color {
	switch category {
	case AccentCatCapture:
		return theme.AccentCapture
	case AccentCatTriage:
		return theme.AccentTriage
	case AccentCatExecute:
		return theme.AccentExecute
	case AccentCatSupport:
		return theme.AccentSupport
	default:
		return nil
	}
}

// presentationCapability scopes mature renderer commands without affecting
// typed route identity or history.
func presentationCapability(view presentation.PresentationID) operation.Capability {
	return operation.Capability(fmt.Sprintf("presentation:%d", view))
}

// PresentationCommandContext builds discovery capabilities for one renderer.
func PresentationCommandContext(view presentation.PresentationID) operation.CommandContext {
	scope := presentationScope(view)
	capabilities := operation.NewCapabilitySet(operation.CapabilityCommandDiscovery, presentationCapability(view))
	switch scope {
	case ScopeApp:
		capabilities.Add(operation.CapabilityAppScope)
	case ScopeTask:
		capabilities.Add(operation.CapabilityTaskScope)
	default:
		capabilities.Add(operation.CapabilityProjectScope)
	}
	return operation.CommandContext{
		View:         view,
		Capabilities: capabilities,
	}
}
