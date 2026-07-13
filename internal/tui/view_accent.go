package tui

import "image/color"

// viewAccentMap maps ViewType to its category accent color.
// Unmapped views (pickers, editors, selectors) inherit from parent via stack walk.
var viewAccentMap map[ViewType]color.Color

// buildViewAccentMap initializes the accent map. Called from ApplyTheme so that
// palette overrides are reflected on every theme change. The view→category
// assignment lives declaratively in the route table (see route.go); this
// resolves each route's accent category to the active theme color.
func buildViewAccentMap() {
	m := make(map[ViewType]color.Color)
	for _, vt := range allViewTypes() {
		if c := accentColorForCategory(routeFor(vt).Accent); c != nil {
			m[vt] = c
		}
	}
	viewAccentMap = m
}

// ViewAccentColor returns the accent color for a view type, or nil if unmapped.
func ViewAccentColor(vt ViewType) color.Color {
	if viewAccentMap == nil {
		return nil
	}
	return viewAccentMap[vt]
}
