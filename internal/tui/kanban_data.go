package tui

import (
	"slices"

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

func sectionColumnIcon(section string) string {
	switch section {
	case models.SectionInbox:
		return "⌐"
	case models.SectionNext:
		return "→"
	case models.SectionWaiting:
		return "⌛"
	case models.SectionSomeday:
		return "○"
	case models.SectionReview:
		return "◎"
	case models.SectionStandby:
		return "⏸"
	case models.SectionCompleted:
		return "✓"
	default:
		return "·"
	}
}

// statusColumnDefs returns ordered KanbanColumn definitions for Status mode.
func statusColumnDefs(order []string) []KanbanColumn {
	defaults := []string{"todo", "in_progress", "review", "blocked", "standby", "done", "canceled"}
	if len(order) == 0 {
		order = defaults
	}
	labels := map[string]string{
		"todo":        "Todo",
		"in_progress": "In Progress",
		"review":      "Review",
		"blocked":     "Blocked",
		"standby":     "Standby",
		"done":        "Done",
		"canceled":    "Canceled",
	}
	icons := map[string]string{
		"todo":        "○",
		"in_progress": "▶",
		"review":      "◎",
		"blocked":     "⊗",
		"standby":     "⏸",
		"done":        "✓",
		"canceled":    "✗",
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
			Icon:  icons[id],
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

// stageColumnDefs returns ordered KanbanColumn definitions for Stages mode.
func stageColumnDefs(stages []string) []KanbanColumn {
	// Always include an "Unset" bucket for tasks without a stage.
	cols := make([]KanbanColumn, 0, len(stages)+1)
	cols = append(cols, KanbanColumn{ID: "unset", Label: "Unset", Icon: "·"})
	for _, s := range stages {
		cols = append(cols, KanbanColumn{ID: s, Label: s, Icon: "▶"})
	}
	return cols
}

// customSectionColumnDefs returns ordered KanbanColumn definitions for CustomSections mode.
func customSectionColumnDefs(sections []string) []KanbanColumn {
	cols := make([]KanbanColumn, 0, len(sections)+1)
	cols = append(cols, KanbanColumn{ID: "unset", Label: "Unset", Icon: "·"})
	for _, s := range sections {
		cols = append(cols, KanbanColumn{ID: s, Label: s, Icon: "⊞"})
	}
	return cols
}

// scoreColumnDefs returns ordered KanbanColumn definitions for ScoreBuckets mode.
func scoreColumnDefs() []KanbanColumn {
	return []KanbanColumn{
		{ID: "0-25", Label: "0–25%", Icon: "◈"},
		{ID: "26-50", Label: "26–50%", Icon: "◈"},
		{ID: "51-75", Label: "51–75%", Icon: "◈"},
		{ID: "76-100", Label: "76–100%", Icon: "◈"},
	}
}

// columnForTask returns the column ID that the task belongs to for the given mode.
// For ScoreBuckets mode a scoring profile may be nil (task goes to "0-25").
func columnForTask(task *models.Task, mode KanbanMode, profile *scoring.ScoringProfile) string {
	switch mode {
	case KanbanModeSection:
		s := task.Section
		if s == "" {
			return "next" // empty section defaults to Next
		}
		return sectionColumnID(s)

	case KanbanModeStatus:
		return string(task.Status)

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

	case KanbanModeStages:
		if task.Stage == "" {
			return "unset"
		}
		return task.Stage

	case KanbanModeCustomSections:
		if task.CustomSection == "" {
			return "unset"
		}
		return task.CustomSection

	case KanbanModeScoreBuckets:
		if profile == nil {
			return "0-25"
		}
		score := scoring.ComputeScore(task, profile)
		switch {
		case score > 0.75:
			return "76-100"
		case score > 0.5:
			return "51-75"
		case score > 0.25:
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
	sectionOrder, statusOrder, stages, customSections []string,
	profile *scoring.ScoringProfile,
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
	case KanbanModeStages:
		cols = stageColumnDefs(stages)
	case KanbanModeCustomSections:
		cols = customSectionColumnDefs(customSections)
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
		colID := columnForTask(card.Task, mode, profile)
		if ci, ok := idx[colID]; ok {
			cols[ci].Tasks = append(cols[ci].Tasks, card)
		} else {
			// Dynamic column (e.g. unknown stage/custom-section) — create on the fly
			// and append at the end with a check for duplicates.
			newIdx := -1
			for i, c := range cols {
				if c.ID == colID {
					newIdx = i
					break
				}
			}
			if newIdx < 0 {
				cols = append(cols, KanbanColumn{ID: colID, Label: colID, Icon: "·"})
				idx[colID] = len(cols) - 1
				newIdx = len(cols) - 1
			}
			cols[newIdx].Tasks = append(cols[newIdx].Tasks, card)
		}
	}

	// Clamp cursors so they stay within bounds after data changes.
	for i, c := range cols {
		if len(c.Tasks) == 0 {
			cols[i].Cursor = 0
			cols[i].ScrollOffset = 0
		} else if c.Cursor >= len(c.Tasks) {
			cols[i].Cursor = len(c.Tasks) - 1
		}
	}

	return cols
}

// collectStages gathers all unique stage values across the provided project
// list, merging with the global stage list from config.
func collectStages(globalStages []string, projects []*models.Project) []string {
	seen := make(map[string]bool, len(globalStages))
	result := make([]string, 0, len(globalStages))
	for _, s := range globalStages {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, p := range projects {
		for _, t := range p.Tasks {
			if t.Stage != "" && !seen[t.Stage] {
				seen[t.Stage] = true
				result = append(result, t.Stage)
			}
		}
	}
	return result
}

// collectCustomSections gathers unique CustomSection values from the given task list.
func collectCustomSections(cards []KanbanCard) []string {
	seen := make(map[string]bool)
	var result []string
	for _, card := range cards {
		if s := card.Task.CustomSection; s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	slices.Sort(result)
	return result
}
