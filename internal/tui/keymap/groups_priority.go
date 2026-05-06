package keymap

// PriorityDeps holds action closures for the "!" priority prefix group.
type PriorityDeps struct {
	// ChangePriority returns an Action for the given priority key ("1"-"5").
	ChangePriority func(key string) Action
}

// PriorityGroup returns entries for the "!" prefix: priority levels 1-5.
func PriorityGroup(deps PriorityDeps) []Entry {
	cat := "Priority"
	descriptions := map[string]string{
		"1": "Set priority: Critical",
		"2": "Set priority: High",
		"3": "Set priority: Normal",
		"4": "Set priority: Low",
		"5": "Set priority: Someday",
	}
	children := make([]Entry, 5)
	for i, k := range []string{"1", "2", "3", "4", "5"} {
		key := k
		children[i] = Bind(key, descriptions[key], cat, deps.ChangePriority(key))
	}
	return []Entry{Prefix("!", "Priority", cat, children)}
}
