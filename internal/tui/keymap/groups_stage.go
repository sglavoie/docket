package keymap

// StageDeps holds action closures for the "." stage prefix group.
type StageDeps struct {
	// ChangeStage returns an Action that sets the named stage ("" = clear).
	ChangeStage func(stage string) Action
	// Stages is the ordered list of stage names from project config.
	// Keys 1-9 map to stages[0..8]; a-z map to stages[9..34].
	Stages []string
}

// stageKeyForIndex mirrors the function in the tui package.
// Indices 0-8 → "1"-"9"; indices 9-34 → "a"-"z"; beyond → "".
func stageKeyForIndex(i int) string {
	if i >= 0 && i < 9 {
		return string(rune('1' + i))
	}
	if i >= 9 && i < 35 {
		return string(rune('a' + i - 9))
	}
	return ""
}

// StageGroup returns entries for the "." prefix: workflow stage assignment.
// Children are built dynamically from deps.Stages; the group should be
// rebuilt when the project changes (e.g. via buildKeyMap on project switch).
func StageGroup(deps StageDeps) []Entry {
	cat := "Stage (. prefix)"
	children := []Entry{
		Bind("0", "Clear stage", cat, deps.ChangeStage("")),
	}
	for i, stage := range deps.Stages {
		key := stageKeyForIndex(i)
		if key == "" {
			break
		}
		name := stage
		children = append(children, Bind(key, "Set stage: "+name, cat, deps.ChangeStage(name)))
	}
	return []Entry{Prefix(".", "Stage", cat, children)}
}
