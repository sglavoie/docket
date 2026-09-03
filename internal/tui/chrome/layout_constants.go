package chrome

// Layout constants for TUI rendering dimensions and spacing.
// These replace hardcoded magic numbers throughout the view code.

// ColumnLayout holds pre-computed fixed-width column measurements for a single
// render pass. When non-nil, badge columns are padded to fixed widths so
// metadata aligns vertically across all rows. Nil = current variable behavior.
type ColumnLayout struct {
	PriorityWidth int // 1 when any row shows priority badge; 0 if all hidden
	StatusWidth   int // max rendered status badge visual width; 0 when column inactive
	ProjectWidth  int // max rendered "[project]" badge visual width (cross-project views); 0 when not in use
	HasPriority   bool
	HasStatus     bool
}

const (
	// CreatorTabBarPadding is the horizontal padding subtracted from width
	// when rendering tab bars in creator/editor views.
	CreatorTabBarPadding = 4

	// MinColumnAlignWidth is the minimum render width at which column alignment
	// is applied. Below this threshold prefix padding is skipped to preserve space.
	MinColumnAlignWidth = 60
)

// WorstCaseColumnLayout stands in for the real layout on the viewport
// measurement pass, which cannot have one: ComputeOutlinerColumnLayout reads the
// row window computeOutlinerViewport is still deriving. Padding only ever widens
// a badge and every status/priority glyph is single-width, so assuming it is
// present over-reserves the Peek height by at most two columns of title budget
// instead of under-reserving it and overflowing terminal height.
func WorstCaseColumnLayout(rowWidth int) *ColumnLayout {
	if rowWidth < MinColumnAlignWidth {
		return nil
	}
	return &ColumnLayout{
		PriorityWidth: 1,
		StatusWidth:   1,
		HasPriority:   true,
		HasStatus:     true,
	}
}
