package format

import "strings"

// splitMetadataPairs splits metadata string into key-value pairs, handling values
// that may contain commas (e.g., "recur: 1w:mon,wed,fri").
//
// Fields whose values may legitimately contain commas (and therefore require this
// special handling rather than a naive strings.Split(s, ",")):
//   - recur: weekday lists use comma-separated day names, e.g. "1w:mon,wed,fri"
//   - url: URLs may contain commas in query strings
//   - bug.workaround, story.persona, epic.metrics: free-text that may contain commas
//
// The strategy: scan for ", knownKey:" delimiters instead of bare commas, and skip
// keys that have already been seen (each key appears at most once per metadata block)
// to avoid mis-splitting on key-like substrings inside free-text values.
//
// It looks for ", key:" patterns where key is a known metadata key or a "score.*" prefix.
// To prevent free-text values (URLs, descriptions) that happen to contain ", key:"
// patterns from being mis-split, it tracks already-seen keys and only splits on
// keys that haven't appeared yet (since each key appears at most once in metadata).
func splitMetadataPairs(metaStr string) []string {
	var pairs []string
	remaining := metaStr
	seenKeys := make(map[string]bool)

	for len(remaining) > 0 {
		currentKey := extractMetadataKey(remaining)
		if currentKey != "" {
			seenKeys[currentKey] = true
		}

		splitIdx := metadataFieldRegistry.nextSplitIndex(remaining, seenKeys)

		if splitIdx == -1 {
			pairs = append(pairs, strings.TrimSpace(remaining))
			break
		}

		pairs = append(pairs, strings.TrimSpace(remaining[:splitIdx]))
		remaining = remaining[splitIdx+2:]
	}

	return pairs
}

// extractMetadataKey returns the key portion from a "key: value" string,
// or empty string if no valid key is found.
func extractMetadataKey(s string) string {
	s = strings.TrimSpace(s)
	colonIdx := strings.Index(s, ":")
	if colonIdx <= 0 {
		return ""
	}
	return strings.TrimSpace(s[:colonIdx])
}

// extractScoreKey returns the full "score.<id>" key from a string starting with "score.<id>:".
func extractScoreKey(s string) string {
	if !strings.HasPrefix(s, "score.") {
		return ""
	}
	rest := s[6:]
	colonIdx := strings.Index(rest, ":")
	if colonIdx <= 0 {
		return ""
	}
	return "score." + rest[:colonIdx]
}

func metadataCommaIsEscaped(s string, commaIdx int) bool {
	backslashes := 0
	for i := commaIdx - 1; i >= 0 && s[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func findUnescapedMetadataDelimiter(s, pattern string) int {
	searchFrom := 0
	for {
		idx := strings.Index(s[searchFrom:], pattern)
		if idx == -1 {
			return -1
		}
		idx += searchFrom
		if !metadataCommaIsEscaped(s, idx) {
			return idx
		}
		searchFrom = idx + 1
		if searchFrom >= len(s) {
			return -1
		}
	}
}

// findScoreKeyIndex finds the index of the first ", score.<id>:" pattern in s.
// Returns -1 if not found. The score dimension ID must be non-empty and contain
// only alphanumeric characters and underscores.
func findScoreKeyIndex(s string) int {
	searchFrom := 0
	for {
		idx := strings.Index(s[searchFrom:], ", score.")
		if idx == -1 {
			return -1
		}
		idx += searchFrom
		if metadataCommaIsEscaped(s, idx) {
			searchFrom = idx + 1
			if searchFrom >= len(s) {
				return -1
			}
			continue
		}
		rest := s[idx+8:]
		colonIdx := strings.Index(rest, ":")
		if colonIdx > 0 {
			dimID := rest[:colonIdx]
			if isValidDimensionID(dimID) {
				return idx
			}
		}
		searchFrom = idx + 2
		if searchFrom >= len(s) {
			return -1
		}
	}
}

// isValidDimensionID checks that a dimension ID contains only [a-zA-Z0-9_]
// and does not exceed MaxDimensionIDLength.
func isValidDimensionID(id string) bool {
	if id == "" || len(id) > MaxDimensionIDLength {
		return false
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}
