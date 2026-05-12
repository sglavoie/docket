package tui

// Icon constants for consistent glyph usage across views.
// IconExpanded and IconCollapsed use soft triangles (▾/▸) for tree nodes and
// collapsible sections. IconScrollLeft/Right and IconInProgress use filled
// triangles (◀/▶) for scroll hints and the kanban in-progress status.
// IconCursorRight shares the same glyph as IconCollapsed but signals cursor
// position or selection rather than a collapsed state.
const (
	IconExpanded    = "▾" // soft downward: expanded tree node / open section
	IconCollapsed   = "▸" // soft rightward: collapsed tree node / closed section
	IconCursorRight = "▸" // soft rightward: cursor / selection indicator
	IconScrollLeft  = "◀" // filled leftward: scroll-left hint
	IconScrollRight = "▶" // filled rightward: scroll-right hint
	IconInProgress  = "▶" // filled rightward: kanban in-progress status icon
	IconTodayMarker = "▼" // filled downward: today position marker in roadmap

	IconComment    = "▪" // solid square: comment indicator
	IconDecision   = "▫" // hollow square: decision indicator
	IconAttachment = "⊕" // circle-plus: attachment indicator
	IconChecklist  = "☑" // checkbox: checklist progress indicator
)
