package config

// FocusQueueItem is one entry in the Next order index. Since the queue was
// folded into the StateNext workflow state, membership is derived from a task's
// State()==StateNext, not from presence in this list: the list now only records
// the user's preferred ordering among Next tasks. Prune-on-load drops entries
// whose task is no longer StateNext (see the TUI hydrateFocusQueue and the CLI
// focus-queue mutations). The struct shape is unchanged for on-disk
// compatibility.
type FocusQueueItem struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
}

type DailyPlanItem struct {
	TaskID      string `json:"task_id"`
	ProjectID   string `json:"project_id"`
	PlannedDate string `json:"planned_date"`
}

const maxFocusDismissedIDs = 200
