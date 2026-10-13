package chrome

// Layout constants for TUI rendering dimensions and spacing.
// These replace hardcoded magic numbers throughout the view code.

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
