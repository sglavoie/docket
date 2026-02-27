package config

// DefaultMaxPinnedBarItems is the default max pinned tasks shown in the status bar.
const DefaultMaxPinnedBarItems = 5

// GetMaxPinnedBarItems returns the maximum number of pinned tasks shown in the status bar.
// Returns 5 if not set.
func (c *Config) GetMaxPinnedBarItems() int {
	if c.MaxPinnedBarItems == nil {
		return DefaultMaxPinnedBarItems
	}
	return *c.MaxPinnedBarItems
}
