package tui

import "github.com/sglavoie/tt/internal/icons"

// Structural icon constants — UI chrome that doesn't need emoji variants.
const (
	IconExpanded    = "▾"
	IconCollapsed   = "▸"
	IconCursorRight = "▸"
	IconScrollLeft  = "◀"
	IconScrollRight = "▶"
	IconInProgress  = "▶"
	IconTodayMarker = "▼"
)

func IconComment() string    { return icons.Get("comment") }
func IconDecision() string   { return icons.Get("decision") }
func IconAttachment() string { return icons.Get("attachment") }
func IconChecklist() string  { return icons.Get("checklist") }
