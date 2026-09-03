package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TagInfo implements PickerItem for tag selection.
// FilterText returns the tag name for filtering.
func (t TagInfo) FilterText() string {
	return t.Name
}

// DisplayText returns the tag name for display.
func (t TagInfo) DisplayText() string {
	return t.Name
}

// CountValue returns the task count for this tag.
func (t TagInfo) CountValue() int {
	return t.TaskCount
}

// ItemID returns the tag name as its unique identifier.
func (t TagInfo) ItemID() string {
	return t.Name
}

// ListSurfacePrimaryStyle gives an adapted tag row the one chip color its tag
// wears everywhere else. A tag row names a tag rather than an effect, so it
// resolves through TagChipStyle even when the tag is semantic: the title color
// belongs to the `#` picker's SEMANTIC section, which builds its own rows. This
// hangs off the payload rather than off one call site because the retained tag
// selector adapts its items again on every ReplaceItems, so a minted row would
// otherwise come back plain.
func (t TagInfo) ListSurfacePrimaryStyle() lipgloss.Style {
	return theme.TagChipStyle(t.Name)
}

// CountDisplayer is implemented by PickerItems that want custom styled count
// text instead of the default "(N)" format. The returned string is pre-rendered
// and should not be wrapped in additional styles.
type CountDisplayer interface {
	CountDisplayText() string
}
