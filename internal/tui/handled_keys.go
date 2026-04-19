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

// listNavigationKeys returns keys handled by views that use handleListNavigation.
// pgup/pgdown are intentional aliases for ctrl+u/ctrl+d and are excluded here
// since they don't appear separately in the registry.
func listNavigationKeys() []string {
	return []string{"j", "k", "up", "down", "G", "ctrl+u", "ctrl+d"}
}

// inboxProcessorKeys returns the key set shared by InboxProcessorModel and GlobalInboxProcessorModel.
func inboxProcessorKeys() []string {
	return []string{
		"j", "k", "g", "G",
		"enter", "n",
		",s", ",d", ",b", ",x", ",o",
		"s", "1", "2", "3", "4", "5", "t", "c", "e", "d", "D", "b", "w", "u",
	}
}

// statusPrefixKeys returns the status prefix sequences handled by dispatchCommonPrefix.
// Does not include recurrence keys (,k ,Dt ,D3 ,Dw ,DW ,Dm) — those are
// intentionally excluded (keysExcludedFromSync) since only primary views handle them.
func statusPrefixKeys() []string {
	return []string{",s", ",d", ",b", ",S", ",o", ",x", ",r"}
}

// priorityPrefixKeys returns the priority prefix sequences (!1–!5).
func priorityPrefixKeys() []string {
	return []string{"!1", "!2", "!3", "!4", "!5"}
}

// dueDatePrefixKeys returns the due-date prefix sequences (d+).
func dueDatePrefixKeys() []string {
	return []string{"dt", "dm", "dw", "dn", "d0", "dd"}
}

// snoozePrefixKeys returns the snooze prefix sequences (s+).
func snoozePrefixKeys() []string {
	return []string{"st", "sw", "sm", "sn", "s0", "sd"}
}

// linkedTaskPrefixKeys returns the linked-task prefix sequences (c+).
func linkedTaskPrefixKeys() []string {
	return []string{"cb", "cd", "cs", "cf", "cn", "cp"}
}

// stagePrefixKeys returns stage prefix sequences (.1–.9, .0, F.).
func stagePrefixKeys() []string {
	return []string{".1", ".2", ".3", ".4", ".5", ".6", ".7", ".8", ".9", ".0", "F."}
}

// sortKeys returns sort key sequences (1–5 and 0).
func sortKeys() []string {
	return []string{"1", "2", "3", "4", "5", "0"}
}

// foldPrefixKeys returns fold prefix sequences (z+).
func foldPrefixKeys() []string {
	return []string{"za", "zo", "zc", "zR", "zM", "zv", "zp"}
}

// typeChangeKeys returns type-change key sequences (<, >, H, L).
func typeChangeKeys() []string {
	return []string{"<", ">", "H", "L"}
}

// blockerPrefixKeySet returns blocker prefix sequences (bb, bm).
func blockerPrefixKeySet() []string {
	return []string{"bb", "bm"}
}

// taskPrefixKeySet returns task prefix sequences (t+).
func taskPrefixKeySet() []string {
	return []string{"tn", "tN", "tr", "ts", "to", "tp", "td", "tt", "tc", "ta", "tl", "te", "tm", "tD"}
}

// movePrefixKeys returns move-prefix sequences registered by movePrefixBindings.
// mS (Standby) is handled by sectionForMoveKey but not registered; see keysExcludedFromSync.
func movePrefixKeys() []string {
	return []string{"ma", "mn", "mi", "mw", "mr", "ms", "mK", "mJ"}
}

// filterSubKeys returns the common F-prefix sub-key sequences registered by
// primary views (Fp, Ft, Fs, F#, Fa, FS, Fr, Fc, Fh, F@, Fx, Fd, Fvs, Fvl, Fvd).
func filterSubKeys() []string {
	return []string{
		"Fp", "FP", "Ft", "Fs", "F#", "Fa", "FS",
		"Fr", "Fc", "Fh", "F@", "Fx", "Fd",
		"Fvs", "Fvl", "Fvd",
	}
}

// commonActionKeys returns keys handled by handleCommonTaskAction, shared
// across all secondary task list views.
func commonActionKeys() []string {
	return []string{
		"o", "ctrl+.", "Z", "D", "e", "r",
		"y", "Y", "ctrl+y", "?",
		"*", "S", "O", "ctrl+b",
	}
}

// commonPrimaryViewKeys returns keys handled by handleCommonPrimaryViewKey, shared
// across ProjectTree and Outliner.
func commonPrimaryViewKeys() []string {
	return []string{
		"ctrl+p", "ctrl+o",
		"`", "~", "@",
		"B", "p", "w", "W",
		"i", "I",
		"\\", "{", "}",
		"R", "ctrl+r", "ctrl+l",
		"u",
		"S", "ctrl+.", "ctrl+m",
	}
}

// concatKeys concatenates multiple key-set slices into one.
func concatKeys(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// globalAppKeys returns keys handled at the AppModel level for all views.
// These are excluded from per-view invisible-binding checks.
func globalAppKeys() []string {
	return []string{
		"q", "ctrl+c", "alt+o", "alt+i", "|", "#", "esc",
		"ctrl+x", "ctrl+z", ":", ";", "ctrl+,", "ctrl+i", "ctrl+/", `ctrl+\`,
		"shift+space", "?", "alt+l",
		// Semicolon prefix dispatches
		";n", ";d", ";w", ";N", ";q", ";f", ";l", ";B", ";D", ";m", ";a", ";W", ";o", ";k", ";i", ";I", ";{", ";,", ";c", ";tN",
	}
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

	// Complex compound display entries that don't map to single key events
	"h/l or [/]": true,
	"(/) ":       true,

	// Recurrence keys: registered via statusBindings() for all views but only
	// implemented in primary views (ProjectTree/Outliner) via handleDeferRecurrence.
	// Secondary views silently consume the prefix without acting.
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
	"Ctrl+D/U":      {"ctrl+d", "ctrl+u"},
	"Ctrl+U/D":      {"ctrl+u", "ctrl+d"},
	"ctrl+u/d":      {"ctrl+u", "ctrl+d"},
	"Tab/j":         {"tab", "j"},
	"Shift+Tab/k":   {"shift+tab", "k"},
	"Tab/Shift+Tab": {"tab", "shift+tab"},
	"gg/G":          {"g", "G"},
	"ctrl+j/k":      {"ctrl+j", "ctrl+k"},
	"ctrl+h/l":      {"ctrl+h", "ctrl+l"},
	"n/N":           {"n", "N"},
	"J/K":           {"J", "K"},
	"a/n":           {"a", "n"},
	"d/x":           {"d", "x"},
	"[/]":           {"[", "]"},
	"+/-":           {"+", "-"},
	"h/l":           {"h", "l"},
	"t/T":           {"t", "T"},
	"zR/zM":         {"zR", "zM"},
	"zo/zc":         {"zo", "zc"},
	"ctrl+/":        {"ctrl+/"},
	"Space/Enter":   {"space", "enter"},
	"Esc":           {"esc"},
	"o/Enter":       {"o", "enter"},
	"p/Esc":         {"p", "esc"},
	"ctrl+U/D":      {"ctrl+u", "ctrl+d"},
	"a/A":           {"a", "A"},
	"d/D":           {"d", "D"},
	"{/}":           {"{", "}"},
	".1-.9":         {".1", ".2", ".3", ".4", ".5", ".6", ".7", ".8", ".9"},
	"ctrl+d/u":      {"ctrl+d", "ctrl+u"},
	"P0-P9":         {"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9"},
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
