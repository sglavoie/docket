package config

const DefaultMaxRecentTaskVisits = 15
const MaxRecentTaskVisitsUpperBound = 50

type RecentTaskVisit struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
	Pinned    bool   `json:"pinned,omitempty"`
}
