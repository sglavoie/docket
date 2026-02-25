package config

import "fmt"

// SmartListCriterion represents a single filter criterion in a smart list.
type SmartListCriterion struct {
	Field     string `json:"field"`     // type, status, priority, tags, due_date, section, score_min, actionable, created_days, updated_days, completed_days
	Operation string `json:"operation"` // equals, not_equals, contains, greater_than, less_than
	Value     string `json:"value"`
}

// SmartList is a user-defined, auto-updating tab with compound filter criteria.
type SmartList struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Criteria         []SmartListCriterion `json:"criteria"`
	Logic            string               `json:"logic,omitempty"` // "and" (default) or "or"
	SortField        int                  `json:"sort_field,omitempty"`
	SortAscending    bool                 `json:"sort_ascending"`
	IncludeCompleted bool                 `json:"include_completed,omitempty"`
}

// EffectiveLogic returns the logic operator, defaulting to "and".
func (s *SmartList) EffectiveLogic() string {
	if s.Logic == "or" {
		return "or"
	}
	return "and"
}

// AddSmartList adds a smart list to the config.
// Returns an error if a smart list with the same ID already exists.
func (c *Config) AddSmartList(sl SmartList) error {
	for _, existing := range c.SmartLists {
		if existing.ID == sl.ID {
			return fmt.Errorf("smart list %q already exists", sl.ID)
		}
	}
	c.SmartLists = append(c.SmartLists, sl)
	return nil
}

// GetSmartList returns a pointer to the smart list with the given ID, or nil.
func (c *Config) GetSmartList(id string) *SmartList {
	for i := range c.SmartLists {
		if c.SmartLists[i].ID == id {
			return &c.SmartLists[i]
		}
	}
	return nil
}

// UpdateSmartList replaces a smart list by ID.
// Returns false if the smart list was not found.
func (c *Config) UpdateSmartList(sl SmartList) bool {
	for i := range c.SmartLists {
		if c.SmartLists[i].ID == sl.ID {
			c.SmartLists[i] = sl
			return true
		}
	}
	return false
}

// RemoveSmartList removes a smart list by ID and cleans up AllTasksTabOrder.
// Returns false if the smart list was not found.
func (c *Config) RemoveSmartList(id string) bool {
	found := false
	for i := range c.SmartLists {
		if c.SmartLists[i].ID == id {
			c.SmartLists = append(c.SmartLists[:i], c.SmartLists[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return false
	}

	// Remove from tab order if present
	var newOrder []string
	for _, entry := range c.AllTasksTabOrder {
		if entry != id {
			newOrder = append(newOrder, entry)
		}
	}
	c.AllTasksTabOrder = newOrder
	return true
}
