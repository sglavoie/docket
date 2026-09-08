package listsurface

import (
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// ListSource is the validated input shape shared by List Surface adapters and
// the retained list surfaces built on top of them.
type ListSource struct {
	Items      []ListItem
	Categories []ListCategory
}

// NewModel creates a ListSurfaceModel from the adapted source.
func (source ListSource) NewModel(config ListSurfaceConfig) (*ListSurfaceModel, error) {
	return NewListSurfaceModel(source.Items, source.Categories, config)
}

// RetainKnownIDs keeps the given IDs this source actually offers, in the order
// they were given. It is the seed step for a picker whose preselection is read
// from somewhere other than the rows: a filter facet seeds from the narrowing
// already in force, and that narrowing can legitimately name a value the current
// vocabulary does not hold — a tag the scope you are standing in does not use,
// or one a rename has just moved to another spelling. Such a facet must still
// open on the vocabulary it has, so the seed is an intersection rather than an
// assertion. SetSelectedIDs stays strict for every caller whose seed is drawn
// from the same population as the rows, where a missing ID is a real defect.
func (source ListSource) RetainKnownIDs(ids []ListItemID) []ListItemID {
	if len(ids) == 0 {
		return nil
	}
	known := make(map[ListItemID]struct{}, len(source.Items))
	for _, item := range source.Items {
		known[item.ID] = struct{}{}
	}
	retained := make([]ListItemID, 0, len(ids))
	for _, id := range ids {
		if _, ok := known[id]; ok {
			retained = append(retained, id)
		}
	}
	if len(retained) == 0 {
		return nil
	}
	return retained
}

// ListSurfaceDisabledPickerItem optionally exposes why a legacy picker item is
// visible but unavailable.
type ListSurfaceDisabledPickerItem interface {
	ListSurfaceDisabledReason() string
}

// ListSurfacePreviewPickerItem optionally supplies rich preview content.
type ListSurfacePreviewPickerItem interface {
	ListSurfacePreview() ListPreview
}

// ListSurfaceOperationPickerItem optionally supplies contextual operation IDs.
type ListSurfaceOperationPickerItem interface {
	ListSurfaceOperationIDs() []operation.OperationID
}

// ListSurfaceStyledPickerItem optionally colors an adapted row's primary text.
// The style is resolved on every adaptation rather than cached on the payload,
// so a source rebuilt after ApplyTheme wears the current theme's color. A pinned
// PinnablePickerItem composes with it cleanly: the pin rides PrimaryMarker,
// which sits beside the primary text rather than wrapping it.
type ListSurfaceStyledPickerItem interface {
	ListSurfacePrimaryStyle() lipgloss.Style
}

// The concrete constructors that build a ListSource from each established picker
// vocabulary live in list_surface_sources.go.
