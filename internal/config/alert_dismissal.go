package config

import "time"

// GetAlertDismissals returns the persisted alert dismissals.
func (c *Config) GetAlertDismissals() []AlertDismissal {
	return c.AlertDismissals
}

// SetAlertDismissals replaces the entire alert dismissals list.
func (c *Config) SetAlertDismissals(d []AlertDismissal) {
	c.AlertDismissals = d
}

// DismissAlert records a dismissal for the given key if not already present.
func (c *Config) DismissAlert(key, category string) {
	for _, d := range c.AlertDismissals {
		if d.Key == key {
			return
		}
	}
	c.AlertDismissals = append(c.AlertDismissals, AlertDismissal{
		Key:         key,
		Category:    category,
		DismissedAt: time.Now(),
	})
}

// IsAlertDismissed returns true if an alert with the given key is currently dismissed.
func (c *Config) IsAlertDismissed(key string) bool {
	for _, d := range c.AlertDismissals {
		if d.Key == key {
			return true
		}
	}
	return false
}

// UndismissAlert removes the dismissal record for the given key.
// Returns true if the key was found and removed.
func (c *Config) UndismissAlert(key string) bool {
	for i, d := range c.AlertDismissals {
		if d.Key == key {
			c.AlertDismissals = append(c.AlertDismissals[:i], c.AlertDismissals[i+1:]...)
			return true
		}
	}
	return false
}

// ClearAlertDismissals removes all alert dismissals.
func (c *Config) ClearAlertDismissals() {
	c.AlertDismissals = nil
}
