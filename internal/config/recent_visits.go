package config

// DefaultMaxRecentTaskVisits is the default max tasks shown in the quick-switch overlay.
const DefaultMaxRecentTaskVisits = 15

// MaxRecentTaskVisitsUpperBound matches the [1, 50] range enforced by the
// settings TUI validator and the loader's clampIntPtrRange. Values above this
// are clamped so an in-memory or pre-Load() Config can't bypass the limit.
const MaxRecentTaskVisitsUpperBound = 50

// RecentTaskVisit stores a task reference for the recent task quick-switch list.
type RecentTaskVisit struct {
	TaskID    string `json:"task_id"`
	ProjectID string `json:"project_id"`
	Pinned    bool   `json:"pinned,omitempty"`
}

// GetMaxRecentTaskVisits returns the maximum number of tasks shown in the quick-switch overlay.
// Returns 15 if not set or set below the minimum valid value of 1.
// Values above MaxRecentTaskVisitsUpperBound are clamped to the upper bound.
func (c *Config) GetMaxRecentTaskVisits() int {
	if c.MaxRecentTaskVisits == nil || *c.MaxRecentTaskVisits < 1 {
		return DefaultMaxRecentTaskVisits
	}
	if *c.MaxRecentTaskVisits > MaxRecentTaskVisitsUpperBound {
		return MaxRecentTaskVisitsUpperBound
	}
	return *c.MaxRecentTaskVisits
}

// GetRecentTaskVisits returns the persisted recent task visit items.
func (c *Config) GetRecentTaskVisits() []RecentTaskVisit {
	return c.RecentTaskVisits
}

// SetRecentTaskVisits replaces the entire recent task visits list.
func (c *Config) SetRecentTaskVisits(items []RecentTaskVisit) {
	c.RecentTaskVisits = items
}
