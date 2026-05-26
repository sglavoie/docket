package tui

import (
	"regexp"
	"strconv"
	"strings"
)

// KeySyncView is implemented by views that expose HandledKeys for sync testing.
// HandledKeys returns all keys the view's Update method can process,
// regardless of current mode or cursor state.
type KeySyncView interface {
	HandledKeys() []string
}

// keysExcludedFromSync contains registry display keys that should be skipped
// in bidirectional sync checks. These are informational display entries or
// keys handled entirely at the AppModel level (not by the view's Update),
// or intentional handler/registry mismatches documented here.
var keysExcludedFromSync = map[string]bool{
	// @ popup sub-keys — handled by AppModel's project cycle manager
	"@ j/k":      true,
	"@ space":    true,
	"@ 1-9":      true,
	"@ enter/g":  true,
	"@ p":        true,
	"@ c/C":      true,
	"@ u":        true,
	"@ ctrl+j/k": true,
	"@ ctrl+h/l": true,

	// Legacy entries removed (replaced by shift+arrow keys)

	// Recurrence keys: only implemented in primary views (ProjectTree/Outliner)
	// via handleDeferRecurrence. Secondary views silently consume the prefix.
	",k":  true,
	",Dt": true,
	",D3": true,
	",Dw": true,
	",DW": true,
	",Dm": true,

	// mS (move to Standby) is handled by sectionForMoveKey but not registered
	// in movePrefixBindings — intentionally undocumented alias.
	"mS": true,

	// Sidebar display entries for Outliner — handled by sidebar.HandleKeys when
	// sidebar is focused; the constituent keys duplicate main navigation entries.
	"j / ↓": true,
	"k / ↑": true,
	"h / ←": true,
	"l / →": true,

	// shift+tab is an undocumented alias for backward tab cycling in FocusBoard.
	"shift+tab": true,

	// Sidebar display entries for ProjectTree/Outliner — these are shown in the
	// help popup but triggered while sidebar is focused, not via primary key dispatch.
	"j/k (sidebar)":   true,
	"h/l (sidebar)":   true,
	"Enter (sidebar)": true,
	"Esc (sidebar)":   true,
	"x (sidebar)":     true,

	// Documentation-only display strings that don't correspond to single key events.
	"n / e": true,
	"i + ~": true,
}

// registryKeyExpansions maps display key strings that cannot be split naively
// by "/" because the "/" is embedded inside a modifier expression, or because
// the slash-split result needs normalization (e.g. "gg/G" → ["g","G"]).
var registryKeyExpansions = map[string][]string{
	"Ctrl+D/U":                {"ctrl+d", "ctrl+u"},
	"Ctrl+U/D":                {"ctrl+u", "ctrl+d"},
	"ctrl+u/d":                {"ctrl+u", "ctrl+d"},
	"Tab/j":                   {"tab", "j"},
	"Shift+Tab/k":             {"shift+tab", "k"},
	"Tab/Shift+Tab":           {"tab", "shift+tab"},
	"gg/G":                    {"g", "G"},
	"ctrl+j/k":                {"ctrl+j", "ctrl+k"},
	"ctrl+h/l":                {"ctrl+h", "ctrl+l"},
	"n/N":                     {"n", "N"},
	"J/K":                     {"J", "K"},
	"a/n":                     {"a", "n"},
	"d/x":                     {"d", "x"},
	"shift+← / shift+→":       {"shift+left", "shift+right"},
	"shift+↑ / shift+↓":       {"shift+up", "shift+down"},
	"shift+←/→":               {"shift+left", "shift+right"},
	"shift+↑/↓":               {"shift+up", "shift+down"},
	"Tab / shift+← / shift+→": {"tab", "shift+left", "shift+right"},
	"+/-":                     {"+", "-"},
	"h/l":                     {"h", "l"},
	"t/T":                     {"t", "T"},
	"zR/zM":                   {"zR", "zM"},
	"zo/zc":                   {"zo", "zc"},
	"ctrl+/":                  {"ctrl+/"},
	"Space/Enter":             {"space", "enter"},
	"Esc":                     {"esc"},
	"o/Enter":                 {"o", "enter"},
	"p/Esc":                   {"p", "esc"},
	"ctrl+U/D":                {"ctrl+u", "ctrl+d"},
	"a/A":                     {"a", "A"},
	"d/D":                     {"d", "D"},
	"{ / }":                   {"{", "}"},
	".1-.9":                   {".1", ".2", ".3", ".4", ".5", ".6", ".7", ".8", ".9"},
	"ctrl+d/u":                {"ctrl+d", "ctrl+u"},
	"P0-P9":                   {"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9"},
}

// numRangeRe matches patterns like "1-5" or "1-9".
var numRangeRe = regexp.MustCompile(`^(\d)-(\d)$`)

// expandRegistryKey expands a display key string from the KeybindingRegistry to
// the individual actionable key strings returned by tea.KeyPressMsg.String().
// Returns nil for excluded/informational display entries.
func expandRegistryKey(displayKey string) []string {
	if keysExcludedFromSync[displayKey] {
		return nil
	}

	// Direct lookup for ambiguous formats
	if expanded, ok := registryKeyExpansions[displayKey]; ok {
		return expanded
	}

	// Numeric range like "1-5", "1-9"
	if m := numRangeRe.FindStringSubmatch(displayKey); m != nil {
		start, _ := strconv.Atoi(m[1])
		end, _ := strconv.Atoi(m[2])
		var keys []string
		for i := start; i <= end; i++ {
			keys = append(keys, strconv.Itoa(i))
		}
		return keys
	}

	// Simple slash-separated like "j/k", "g/G", "Space/Enter" (caught above), etc.
	if strings.Contains(displayKey, "/") {
		parts := strings.Split(displayKey, "/")
		var result []string
		for _, p := range parts {
			if k := normalizeDisplayKey(strings.TrimSpace(p)); k != "" {
				result = append(result, k)
			}
		}
		if len(result) > 0 {
			return result
		}
	}

	// Single key — normalize display name and return
	if k := normalizeDisplayKey(displayKey); k != "" {
		return []string{k}
	}
	return nil
}

// normalizeDisplayKey converts a display key string to the tea.KeyPressMsg.String()
// format used in switch-case handlers.
func normalizeDisplayKey(k string) string {
	switch k {
	case "↑":
		return "up"
	case "↓":
		return "down"
	case "←":
		return "left"
	case "→":
		return "right"
	case "Enter", "enter":
		return "enter"
	case "Esc", "esc":
		return "esc"
	case "Tab", "tab":
		return "tab"
	case "Space", "space":
		return "space"
	case "Backspace", "backspace":
		return "backspace"
	case "Delete", "delete":
		return "delete"
	case "":
		return ""
	}

	lowered := strings.ToLower(k)
	// Ctrl/Shift/Alt modifier combos are always lowercase in BubbleTea
	if strings.HasPrefix(lowered, "ctrl+") ||
		strings.HasPrefix(lowered, "shift+") ||
		strings.HasPrefix(lowered, "alt+") {
		return lowered
	}

	// Preserve original case for bare key names (j, k, G, J, K, etc.)
	return k
}
