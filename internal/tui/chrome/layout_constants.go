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
	// ProjectColumn is the column the trailing project field begins in, measured
	// over one whole projection rather than over the rows a window happens to
	// hold. It is what makes the field a column instead of a segment: every row
	// pads its title and cues out to it, so scrolling moves rows past a fixed
	// column instead of moving the column. Zero means no row names a project, or
	// the row is too narrow to seat one, and the field follows each row's own
	// cues immediately.
	ProjectColumn int
	HasPriority   bool
	HasStatus     bool
}

const (
	// MinTerminalWidth is the minimum terminal width required to render the TUI.
	MinTerminalWidth = 60

	// MinTerminalHeight is the minimum terminal height required to render the
	// TUI. Fullscreen surfaces derive their own floor from it rather than
	// spelling a second literal, which is why it sits here rather than beside
	// the shell's window-size handler.
	MinTerminalHeight = 20

	// CreatorTabBarPadding is the horizontal padding subtracted from width
	// when rendering tab bars in creator/editor views.
	CreatorTabBarPadding = 4

	// MinColumnAlignWidth is the minimum render width at which column alignment
	// is applied. Below this threshold prefix padding is skipped to preserve space.
	MinColumnAlignWidth = 60
)
