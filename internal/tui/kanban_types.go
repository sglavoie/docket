package tui

import "github.com/sglavoie/tt/internal/models"

// KanbanMode defines the axis by which tasks are distributed into columns.
type KanbanMode int

const (
	KanbanModeSection        KanbanMode = iota // GTD sections: Inbox, Next, Active, Waiting, Someday, Review, Standby, Completed
	KanbanModeStatus                           // Status: Todo, InProgress, Review, Blocked, Standby, Done, Canceled
	KanbanModePriority                         // Priority: P1–P5, Unset
	KanbanModeStages                           // Project workflow stages
	KanbanModeCustomSections                   // User-defined custom sections per project
	KanbanModeScoreBuckets                     // Score ranges: 0–25%, 26–50%, 51–75%, 76–100%
)

// KanbanModeNames maps each mode to a human-readable label.
var KanbanModeNames = map[KanbanMode]string{
	KanbanModeSection:        "Section",
	KanbanModeStatus:         "Status",
	KanbanModePriority:       "Priority",
	KanbanModeStages:         "Stages",
	KanbanModeCustomSections: "Custom Sections",
	KanbanModeScoreBuckets:   "Score Buckets",
}

// KanbanModeIcons maps each mode to a short icon/glyph used in the header.
var KanbanModeIcons = map[KanbanMode]string{
	KanbanModeSection:        "§",
	KanbanModeStatus:         "◎",
	KanbanModePriority:       "!",
	KanbanModeStages:         IconInProgress,
	KanbanModeCustomSections: "⊞",
	KanbanModeScoreBuckets:   "◈",
}

// KanbanModeID returns the stable string ID used for persistence.
func (m KanbanMode) ID() string {
	ids := map[KanbanMode]string{
		KanbanModeSection:        "section",
		KanbanModeStatus:         "status",
		KanbanModePriority:       "priority",
		KanbanModeStages:         "stages",
		KanbanModeCustomSections: "custom_sections",
		KanbanModeScoreBuckets:   "score_buckets",
	}
	if id, ok := ids[m]; ok {
		return id
	}
	return "section"
}

// KanbanModeFromID returns the KanbanMode for the given persistence ID.
func KanbanModeFromID(id string) KanbanMode {
	for m, mid := range map[KanbanMode]string{
		KanbanModeSection:        "section",
		KanbanModeStatus:         "status",
		KanbanModePriority:       "priority",
		KanbanModeStages:         "stages",
		KanbanModeCustomSections: "custom_sections",
		KanbanModeScoreBuckets:   "score_buckets",
	} {
		if mid == id {
			return m
		}
	}
	return KanbanModeSection
}

// AllKanbanModes returns all kanban modes in display order.
func AllKanbanModes() []KanbanMode {
	return []KanbanMode{
		KanbanModeSection,
		KanbanModeStatus,
		KanbanModePriority,
		KanbanModeStages,
		KanbanModeCustomSections,
		KanbanModeScoreBuckets,
	}
}

// KanbanCard is a single task displayed as a card within a kanban column.
type KanbanCard struct {
	Task    *models.Task
	Project *models.Project
}

// KanbanColumn holds all cards assigned to a single column bucket.
type KanbanColumn struct {
	ID           string       // Stable ID for persistence (e.g. "inbox", "todo", "p1")
	Label        string       // Display name shown in header
	Icon         string       // Short icon shown next to label
	Tasks        []KanbanCard // All cards in this column (unfiltered)
	Cursor       int          // Focused card index within this column
	ScrollOffset int          // First visible card index (for vertical scrolling)
}
