package route

import "github.com/sglavoie/docket/internal/tui/presentation"

// OverlayForPresentation maps only transient renderer identities to the typed
// overlay stack. Primary and secondary destinations are installed through
// their typed route activation methods, never inferred from a renderer ID.
func OverlayForPresentation(view presentation.PresentationID) (Overlay, bool, error) {
	switch view {
	case presentation.ViewLastMessage:
		return referenceOverlayFor(ReferenceLastMessage)
	case presentation.ViewHelp:
		return referenceOverlayFor(ReferenceHelp)
	case presentation.ViewOutlineErrors:
		return referenceOverlayFor(ReferenceOutlineErrors)
	default:
	}

	kind, mapped := transientPresentationOverlayKind(view)
	if !mapped {
		return Overlay{}, false, nil
	}
	overlay, err := NewOverlay(kind, view.String())
	if err != nil {
		return Overlay{}, true, err
	}
	overlay.presentation, overlay.hasPresentation = view, true
	return overlay, true, nil
}

// referenceOverlayFor builds the read-only overlay class: content the user
// reads and dismisses, whose identity lives on a typed Reference route rather
// than on the renderer ID.
func referenceOverlayFor(kind ReferenceKind) (Overlay, bool, error) {
	route, err := NewRoute(Reference, ReferenceParams{Kind: kind})
	if err != nil {
		return Overlay{}, true, err
	}
	overlay, err := NewReferenceOverlay(route)
	return overlay, true, err
}

func transientPresentationOverlayKind(view presentation.PresentationID) (OverlayKind, bool) {
	switch view {
	case presentation.ViewProjectCreator, presentation.ViewProjectEditor, presentation.ViewTextEditor:
		return OverlayForm, true
	case presentation.ViewTaskComments, presentation.ViewTaskAttachments:
		return OverlayTaskTool, true
	case presentation.ViewPriorityPicker, presentation.ViewParentSelector, presentation.ViewTagSelector:
		return OverlayPicker, true
	default:
		return 0, false
	}
}
