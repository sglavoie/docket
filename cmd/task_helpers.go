package cmd

import (
	"fmt"
	"strconv"
)

// parseItemIndex parses and validates an index argument against a collection length.
// Returns the parsed index or an error with a descriptive message using itemName.
func parseItemIndex(arg string, count int, itemName string) (int, error) {
	index, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("invalid %s index: %s", itemName, arg)
	}
	if count == 0 {
		return 0, fmt.Errorf("no %ss to index", itemName)
	}
	if index < 0 || index >= count {
		return 0, fmt.Errorf("%s index %d out of range (0-%d)", itemName, index, count-1)
	}
	return index, nil
}

// resolveEditor returns the flag value if set, otherwise falls back to the config editor.
func resolveEditor(flag, cfgEditor string) string {
	if flag != "" {
		return flag
	}
	return cfgEditor
}
