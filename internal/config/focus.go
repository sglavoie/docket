package config

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
