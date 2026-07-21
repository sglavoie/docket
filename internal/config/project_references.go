package config

import "sort"

type ProjectReferences struct {
	WasCurrent        bool               `json:"was_current,omitempty"`
	WasActionInbox    bool               `json:"was_action_inbox,omitempty"`
	ScoringProfile    *string            `json:"scoring_profile,omitempty"`
	WorkflowEval      *WorkflowEvalState `json:"workflow_eval,omitempty"`
	FocusQueue        []FocusQueueItem   `json:"focus_queue,omitempty"`
	DailyPlan         []DailyPlanItem    `json:"daily_plan,omitempty"`
	RecentTaskVisits  []RecentTaskVisit  `json:"recent_task_visits,omitempty"`
	CustomContextKeys []string           `json:"custom_context_keys,omitempty"`
}

func lookupProjectKey[T any](values map[string]T, id, slug string) (T, bool) {
	if id != "" {
		if v, ok := values[id]; ok {
			return v, true
		}
	}
	if slug != "" {
		if v, ok := values[slug]; ok {
			return v, true
		}
	}
	var zero T
	return zero, false
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
