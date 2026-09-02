package tui

import (
	"image/color"

	"github.com/sglavoie/tt/internal/tui/presentation"
)

// viewAccentMap maps PresentationID to its category accent color.
// Unmapped views (pickers, editors, selectors) inherit from parent via stack walk.
var viewAccentMap map[presentation.PresentationID]color.Color

// buildViewAccentMap initializes the accent map. Called from ApplyTheme so that
// palette overrides are reflected on every theme change. The view→category
// assignment is independent from route identity; this resolves each
// presentation's category against the active theme.
func buildViewAccentMap() {
	m := make(map[presentation.PresentationID]color.Color)
	for _, vt := range allPresentations() {
		if c := accentColorForCategory(presentationAccentCategory(vt)); c != nil {
			m[vt] = c
		}
	}
	viewAccentMap = m
}

// ViewAccentColor returns the accent color for a view type, or nil if unmapped.
func ViewAccentColor(vt presentation.PresentationID) color.Color {
	if viewAccentMap == nil {
		return nil
	}
	return viewAccentMap[vt]
}
