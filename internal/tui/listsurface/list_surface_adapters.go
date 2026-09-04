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
