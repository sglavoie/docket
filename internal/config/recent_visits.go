package config

// DefaultMaxRecentTaskVisits is the default max tasks shown in the quick-switch overlay.
const DefaultMaxRecentTaskVisits = 15

// GetMaxRecentTaskVisits returns the maximum number of tasks shown in the quick-switch overlay.
// Returns 15 if not set.
func (c *Config) GetMaxRecentTaskVisits() int {
	if c.MaxRecentTaskVisits == nil {
		return DefaultMaxRecentTaskVisits
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
