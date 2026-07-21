package tui

import "image/color"

// viewAccentMap maps PresentationID to its category accent color.
// Unmapped views (pickers, editors, selectors) inherit from parent via stack walk.
var viewAccentMap map[PresentationID]color.Color

// buildViewAccentMap initializes the accent map. Called from ApplyTheme so that
// palette overrides are reflected on every theme change. The view→category
// assignment is independent from route identity; this resolves each
// presentation's category against the active theme.
func buildViewAccentMap() {
	m := make(map[PresentationID]color.Color)
	for _, vt := range allPresentations() {
		if c := accentColorForCategory(presentationAccentCategory(vt)); c != nil {
			m[vt] = c
		}
	}
	viewAccentMap = m
}

// ViewAccentColor returns the accent color for a view type, or nil if unmapped.
func ViewAccentColor(vt PresentationID) color.Color {
	if viewAccentMap == nil {
		return nil
	}
	return viewAccentMap[vt]
}
