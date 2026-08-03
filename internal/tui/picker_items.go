package tui

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

// CountDisplayer is implemented by PickerItems that want custom styled count
// text instead of the default "(N)" format. The returned string is pre-rendered
// and should not be wrapped in additional styles.
type CountDisplayer interface {
	CountDisplayText() string
}
