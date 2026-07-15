package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// sectionColumnDefs returns ordered KanbanColumn definitions for Section mode.
func sectionColumnDefs(order []string) []KanbanColumn {
	defaults := []string{
		models.SectionInbox,
		models.SectionNext,
		models.SectionWaiting,
		models.SectionSomeday,
		models.SectionReview,
		models.SectionStandby,
		models.SectionCompleted,
	}
	if len(order) == 0 {
		order = defaults
	}
	cols := make([]KanbanColumn, 0, len(order))
	for _, s := range order {
		cols = append(cols, KanbanColumn{
			ID:    sectionColumnID(s),
			Label: s,
			Icon:  sectionColumnIcon(s),
		})
	}
	return cols
}

func sectionColumnID(section string) string {
	switch section {
	case models.SectionInbox:
		return "inbox"
	case models.SectionNext:
		return "next"
	case models.SectionWaiting:
		return "waiting"
	case models.SectionSomeday:
		return "someday"
	case models.SectionReview:
		return "review"
	case models.SectionStandby:
		return "standby"
	case models.SectionCompleted:
		return "completed"
	default:
		return section
	}
}

func sectionColumnIcon(_ string) string {
	return "@"
}

// statusColumnDefs returns ordered KanbanColumn definitions for Status mode.
func statusColumnDefs(order []string) []KanbanColumn {
	defaults := []string{"todo", "in_progress", "review", "blocked", "standby", "maybe", "done", "canceled"}
	if len(order) == 0 {
		order = defaults
	}
	labels := map[string]string{
		"todo":        "Todo",
		"in_progress": "In Progress",
		"review":      "Review",
		"blocked":     "Blocked",
		"standby":     "Standby",
		"maybe":       "Maybe",
		"done":        "Done",
		"canceled":    "Canceled",
	}
	colIcons := map[string]string{
		"todo":        icons.Get("kanban.todo"),
		"in_progress": icons.Get("status.in_progress"),
		"review":      icons.Get("kanban.review"),
		"blocked":     icons.Get("kanban.blocked"),
		"standby":     icons.Get("status.standby"),
		"maybe":       icons.Get("status.maybe"),
		"done":        icons.Get("kanban.done"),
		"canceled":    icons.Get("kanban.canceled"),
	}
	cols := make([]KanbanColumn, 0, len(order))
	for _, id := range order {
		label := labels[id]
		if label == "" {
			label = id
		}
		cols = append(cols, KanbanColumn{
			ID:    id,
			Label: label,
			Icon:  colIcons[id],
		})
	}
	return cols
}

// priorityColumnDefs returns ordered KanbanColumn definitions for Priority mode.
func priorityColumnDefs() []KanbanColumn {
	return []KanbanColumn{
		{ID: "p1", Label: "P1", Icon: "!"},
		{ID: "p2", Label: "P2", Icon: "!"},
		{ID: "p3", Label: "P3", Icon: "!"},
		{ID: "p4", Label: "P4", Icon: "!"},
		{ID: "p5", Label: "P5", Icon: "!"},
		{ID: "unset", Label: "Unset", Icon: "·"},
	}
}

// scoreColumnDefs returns ordered KanbanColumn definitions for ScoreBuckets mode.
func scoreColumnDefs() []KanbanColumn {
	return []KanbanColumn{
		{ID: "0-25", Label: "0–25%", Icon: icons.Get("score_bucket")},
		{ID: "26-50", Label: "26–50%", Icon: icons.Get("score_bucket")},
		{ID: "51-75", Label: "51–75%", Icon: icons.Get("score_bucket")},
		{ID: "76-100", Label: "76–100%", Icon: icons.Get("score_bucket")},
	}
}

// legacyStatusColumnForState maps a workflow state onto the Status-mode column
// vocabulary (statusColumnDefs keys columns by the legacy status strings). The
// three todo-ish planning states share the todo column, waiting shares the
// blocked column, paused maps to standby, and someday to maybe, mirroring the
// state collapse.
func legacyStatusColumnForState(state models.WorkflowState) string {
	switch state {
	case models.StateInbox, models.StateReady, models.StateNext:
		return "todo"
	case models.StateInProgress:
		return "in_progress"
	case models.StateWaiting:
		return "blocked"
	case models.StatePaused:
		return "standby"
	case models.StateReview:
		return "review"
	case models.StateSomeday:
		return "maybe"
	case models.StateDone:
		return "done"
	case models.StateCanceled:
		return "canceled"
	default:
		return string(state)
	}
}

// columnForTask returns the column ID that the task belongs to for the given mode.
// For ScoreBuckets mode the profile is resolved per-project via the automation
// snapshot; if no profile is assigned the task falls into "0-25".
func columnForTask(task *models.Task, mode KanbanMode, auto *config.AutomationConfig, projectID string) string {
	switch mode {
	case KanbanModeSection:
		s := taskSectionBucket(task)
		if s == "" {
			return "next" // the active ready/in_progress bucket defaults to Next
		}
		return sectionColumnID(s)

	case KanbanModeStatus:
		return legacyStatusColumnForState(task.State())

	case KanbanModePriority:
		switch task.Priority {
		case 1:
			return "p1"
		case 2:
			return "p2"
		case 3:
			return "p3"
		case 4:
			return "p4"
		case 5:
			return "p5"
		default:
			return "unset"
		}

	case KanbanModeScoreBuckets:
		profile := auto.GetProjectScoringProfile(projectID)
		if profile == nil {
			return "0-25"
		}
		score := scoring.NormalizedScore(task, profile)
		switch {
		case score > 225:
			return "76-100"
		case score > 150:
			return "51-75"
		case score > 75:
			return "26-50"
		default:
			return "0-25"
		}
	}
	return "unset"
}

// loadKanbanItems loads all tasks from the given projects, returning a flat slice
// of KanbanCards suitable for column distribution.
func loadKanbanItems(projects []*models.Project, svc *ProjectService) []KanbanCard {
	var cards []KanbanCard
	for _, project := range projects {
		if project.Sandbox {
			continue
		}
		if err := svc.Load(project); err != nil {
			continue
		}
		for _, task := range project.Tasks {
			cards = append(cards, KanbanCard{
				Task:    task,
				Project: project,
			})
		}
	}
	return cards
}

// rebuildColumns distributes cards into columns according to the current mode.
// It returns a new set of KanbanColumns with Tasks populated; Cursor and
// ScrollOffset are preserved from the previous columns if the IDs match.
func rebuildColumns(
	cards []KanbanCard,
	mode KanbanMode,
	prev []KanbanColumn,
	sectionOrder, statusOrder []string,
	auto *config.AutomationConfig,
) []KanbanColumn {
	// Build base column set.
	var cols []KanbanColumn
	switch mode {
	case KanbanModeSection:
		cols = sectionColumnDefs(sectionOrder)
	case KanbanModeStatus:
		cols = statusColumnDefs(statusOrder)
	case KanbanModePriority:
		cols = priorityColumnDefs()
	case KanbanModeScoreBuckets:
		cols = scoreColumnDefs()
	default:
		cols = sectionColumnDefs(nil)
	}

	// Index columns by ID for O(1) dispatch.
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		idx[c.ID] = i
	}

	// Restore Cursor/ScrollOffset from previous columns where IDs match.
	if len(prev) > 0 {
		prevIdx := make(map[string]int, len(prev))
		for i, c := range prev {
			prevIdx[c.ID] = i
		}
		for i, c := range cols {
			if pi, ok := prevIdx[c.ID]; ok {
				cols[i].Cursor = prev[pi].Cursor
				cols[i].ScrollOffset = prev[pi].ScrollOffset
			}
		}
	}

	// Distribute cards.
	for _, card := range cards {
		colID := columnForTask(card.Task, mode, auto, card.Project.ID)
		if ci, ok := idx[colID]; ok {
			cols[ci].Tasks = append(cols[ci].Tasks, card)
		} else {
			// Dynamic column (for example, an unknown section) - create on the fly
			// and append at the end with a check for duplicates.
			newIdx := -1
			for i, c := range cols {
				if c.ID == colID {
					newIdx = i
					break
				}
			}
			if newIdx < 0 {
				col := KanbanColumn{ID: colID, Label: colID, Icon: "·"}
				if mode == KanbanModeSection && colID == sectionColumnID(models.SectionCompleted) {
					col.Label = models.SectionCompleted
					col.Icon = sectionColumnIcon(models.SectionCompleted)
				}
				cols = append(cols, col)
				idx[colID] = len(cols) - 1
				newIdx = len(cols) - 1
			}
			cols[newIdx].Tasks = append(cols[newIdx].Tasks, card)
		}
	}

	for i := range cols {
		sortKanbanCardsByProjectDisplayOrder(cols[i].Tasks)
	}

	// Clamp cursors so they stay within bounds after data changes.
	for i, c := range cols {
		if len(c.Tasks) == 0 {
			cols[i].Cursor = 0
			cols[i].ScrollOffset = 0
			continue
		}
		if c.Cursor >= len(c.Tasks) {
			cols[i].Cursor = len(c.Tasks) - 1
		}
		if c.Cursor < 0 {
			cols[i].Cursor = 0
		}
		if c.ScrollOffset < 0 {
			cols[i].ScrollOffset = 0
		}
		if c.ScrollOffset >= len(c.Tasks) {
			cols[i].ScrollOffset = len(c.Tasks) - 1
		}
	}

	return cols
}
