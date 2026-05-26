package tui

// stageKeyForIndex returns the shortcut key for a stage at the given index.
// Indices 0-8 map to "1"-"9", indices 9-34 map to "a"-"z", beyond returns "".
func stageKeyForIndex(i int) string {
	if i >= 0 && i < 9 {
		return string(rune('1' + i))
	}
	if i >= 9 && i < 35 {
		return string(rune('a' + i - 9))
	}
	return ""
}

// stageIndexForKey returns the stage index for a shortcut key, or -1 if unrecognized.
// "1"-"9" map to 0-8, "a"-"z" map to 9-34.
func stageIndexForKey(key string) int {
	if len(key) != 1 {
		return -1
	}
	ch := key[0]
	if ch >= '1' && ch <= '9' {
		return int(ch - '1')
	}
	if ch >= 'a' && ch <= 'z' {
		return int(ch-'a') + 9
	}
	return -1
}

// StageForPrefixKey maps a key press to a stage name from the configured stages.
// Keys 1-9 map to stages[0..8], a-z map to stages[9..34], key 0 clears (empty string).
// Returns nil if the key is not recognized.
func StageForPrefixKey(key string, stages []string) *string {
	if key == "0" {
		s := ""
		return &s
	}
	idx := stageIndexForKey(key)
	if idx >= 0 && idx < len(stages) {
		return &stages[idx]
	}
	return nil
}

// StageKeyForName maps a configured stage name back to its prefix shortcut key.
// Returns "0" for clear-stage and "" when the stage is not configured.
func StageKeyForName(stage string, stages []string) string {
	if stage == "" {
		return "0"
	}
	for i, candidate := range stages {
		if candidate == stage {
			return stageKeyForIndex(i)
		}
	}
	return ""
}
