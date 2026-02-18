package config

// GetFocusQueue returns the persisted focus queue items.
func (c *Config) GetFocusQueue() []FocusQueueItem {
	return c.FocusQueue
}

// SetFocusQueue replaces the entire focus queue.
func (c *Config) SetFocusQueue(items []FocusQueueItem) {
	c.FocusQueue = items
}

// GetFocusDismissedIDs returns the persisted dismissed suggestion IDs.
func (c *Config) GetFocusDismissedIDs() []string {
	return c.FocusDismissedIDs
}

// SetFocusDismissedIDs replaces the entire dismissed ID list.
func (c *Config) SetFocusDismissedIDs(ids []string) {
	c.FocusDismissedIDs = ids
}
