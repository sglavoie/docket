package config

// GetRecentTaskVisits returns the persisted recent task visit items.
func (c *Config) GetRecentTaskVisits() []RecentTaskVisit {
	return c.RecentTaskVisits
}

// SetRecentTaskVisits replaces the entire recent task visits list.
func (c *Config) SetRecentTaskVisits(items []RecentTaskVisit) {
	c.RecentTaskVisits = items
}
