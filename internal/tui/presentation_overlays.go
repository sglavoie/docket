package tui

// overlayForPresentation maps only transient renderer identities to the typed
// overlay stack. Primary and secondary destinations are installed through
// their typed route activation methods, never inferred from a renderer ID.
func overlayForPresentation(view PresentationID) (Overlay, bool, error) {
	switch view {
	case ViewHelp, ViewAsk, ViewTutor:
		kind := ReferenceHelp
		switch view {
		case ViewHelp:
		case ViewAsk:
			kind = ReferenceAsk
		case ViewTutor:
			kind = ReferenceTutor
		default:
			return Overlay{}, false, nil
		}
		route, err := NewRoute(Reference, ReferenceParams{Kind: kind})
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
	case ViewFinder, ViewGlobalSearch:
		return OverlayFinder, true
	case ViewTaskForm, ViewTemplateVarFiller:
		return OverlayComposer, true
	case ViewProjectList, ViewProjectSelector:
		return OverlayProjectPicker, true
	case ViewColumnPicker:
		return OverlayViewMenu, true
	case ViewTagFilterSelector, ViewNotesSearch:
		return OverlaySearch, true
	case ViewTaskJumper:
		return OverlayTaskJump, true
	case ViewProjectPreview, ViewRecentActivityPreview:
		return OverlayPeek, true
	case ViewProjectCreator, ViewProjectEditor, ViewWorkflowRuleCreator, ViewWorkflowRuleEditor, ViewWorkflowWizard,
		ViewTaskTemplateCreator, ViewTaskTemplateEditor, ViewTextEditor:
		return OverlayForm, true
	case ViewConflictResolution:
		return OverlayConfirmation, true
	case ViewDateInput, ViewDependencySelector, ViewDependencyViewer,
		ViewPriorityPicker, ViewParentSelector, ViewTagSelector,
		ViewBulkTagSelector, ViewRecurrenceEditor,
		ViewLinkedTaskSelector, ViewCrossProjectLinker, ViewNoteTaskLinker,
		ViewNoteTagSelector, ViewSnoozeInput, ViewParkInput, ViewTaskTemplatePicker,
		ViewScoreEditor, ViewScoringProfilePicker,
		ViewWorkflowDryRun, ViewStatusInput, ViewStatusIconPicker:
		return OverlayPicker, true
	default:
		return 0, false
	}
}
