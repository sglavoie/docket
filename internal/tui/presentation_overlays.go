package tui

// overlayForPresentation maps only transient renderer identities to the typed
// overlay stack. Primary and secondary destinations are installed through
// their typed route activation methods, never inferred from a renderer ID.
func overlayForPresentation(view PresentationID) (Overlay, bool, error) {
	switch view {
	case ViewHelp:
		route, err := NewRoute(Reference, ReferenceParams{Kind: ReferenceHelp})
		if err != nil {
			return Overlay{}, true, err
		}
		overlay, err := NewReferenceOverlay(route)
		return overlay, true, err
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

func transientPresentationOverlayKind(view PresentationID) (OverlayKind, bool) {
	switch view {
	case ViewTaskForm:
		return OverlayComposer, true
	case ViewProjectSelector:
		return OverlayProjectPicker, true
	case ViewTagFilterSelector:
		return OverlaySearch, true
	case ViewProjectCreator, ViewProjectEditor, ViewTextEditor:
		return OverlayForm, true
	case ViewConflictResolution:
		return OverlayConfirmation, true
	case ViewDateInput, ViewDependencySelector, ViewDependencyViewer,
		ViewPriorityPicker, ViewParentSelector, ViewTagSelector,
		ViewBulkTagSelector, ViewRecurrenceEditor,
		ViewCrossProjectLinker, ViewSnoozeInput:
		return OverlayPicker, true
	default:
		return 0, false
	}
}
