package config

import "sort"

type ProjectReferences struct {
	WasActionInbox    bool                 `json:"was_action_inbox,omitempty"`
	Today             []ProjectTaskLocator `json:"today,omitempty"`
	NextOrder         []ProjectTaskLocator `json:"next_order,omitempty"`
	CustomContextKeys []string             `json:"custom_context_keys,omitempty"`
}

func contextKeysContaining(m map[string][]string, matches func(string) bool) []string {
	var keys []string
	for key, ids := range m {
		for _, pid := range ids {
			if matches(pid) {
				keys = append(keys, key)
				break
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func addProjectToContextMap(m *map[string][]string, key, projectID string) {
	if *m == nil {
		*m = make(map[string][]string)
	}
	if containsString((*m)[key], projectID) {
		return
	}
	(*m)[key] = append((*m)[key], projectID)
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
