package route

import (
	"image/color"

	"github.com/sglavoie/docket/internal/tui/presentation"
)

// ViewAccentColor returns the accent color for a view type, or nil if unmapped.
// Unmapped views (pickers, editors, selectors) inherit from parent via stack
// walk, which is what the nil answer feeds.
//
// The view→category assignment is independent from route identity, and the
// category→color resolution reads the active theme's accent vars, so the answer
// already follows every ApplyTheme without being cached. It used to be a map
// rebuilt from ApplyTheme; that rebuild was the one call the theme layer made
// upward into route vocabulary, and the map only ever memoized two switches
// over a sixteen-value enum.
func ViewAccentColor(vt presentation.PresentationID) color.Color {
	return accentColorForCategory(presentationAccentCategory(vt))
}
