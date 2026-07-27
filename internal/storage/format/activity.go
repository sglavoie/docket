package format

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Matches activity block start marker "<!-- ACTIVITY -->"
var activityStartRegex = regexp.MustCompile(`^<!--\s*ACTIVITY\s*-->$`)

// Matches activity block end marker "<!-- /ACTIVITY -->"
var activityEndRegex = regexp.MustCompile(`^<!--\s*/ACTIVITY\s*-->$`)

// Matches activity entry like "<!-- act: 2025-01-29 14:30, status: todo > in_progress -->"
// or "<!-- act: 2025-01-29 14:30, created -->"
var activityEntryRegex = regexp.MustCompile(`^<!--\s*act:\s*(.+?),\s*(.+?)\s*-->$`)

// emptyValueMarker is the sentinel emitted by the writer when an activity entry
// value (typically the new value of a cleared field) is empty. Without it, the
// regex's trailing \s* would consume the trailing space the writer leaves
// before "-->", making "field: old >  -->" parse identically to a single-value
// entry with NewValue "old >". The marker is unambiguous because escapeActivityValue
// doubles any literal backslash, so a real "\e" in a value is always serialized
// as "\\e" and never collides with the bare sentinel.
const emptyValueMarker = `\e`

// ExtractActivityFromDescription parses the ACTIVITY block from task description.
// It extracts activity entries and returns the cleaned description (without the activity block).
func ExtractActivityFromDescription(task *models.Task) {
	if task.Description == "" {
		return
	}

	blockLines, cleanDesc := ExtractBlock(task.Description, activityStartRegex, activityEndRegex)
	task.Description = cleanDesc

	for _, bl := range blockLines {
		if matches := activityEntryRegex.FindStringSubmatch(bl.Trimmed); matches != nil {
			timestamp, ok := ParseTimeField(matches[1])
			if !ok {
				continue
			}
			entry := parseActivityPayload(matches[2], timestamp)
			if entry != nil {
				task.Activity = append(task.Activity, entry)
			}
		}
	}

	// WriteActivity writes entries newest-first for human readability.
	// Reverse them back to chronological order (oldest-first) which is the
	// in-memory convention expected by diffAndRecordActivity and PruneActivity.
	slices.Reverse(task.Activity)
}

// parseActivityPayload parses the payload portion of an activity entry.
// Handles "created", "field: old > new", "tags: +added -removed", "title: old > new".
func parseActivityPayload(payload string, timestamp time.Time) *models.ActivityEntry {
	payload = strings.TrimSpace(payload)

	if payload == "created" {
		return &models.ActivityEntry{
			Timestamp: timestamp,
			Field:     "created",
		}
	}

	// Split on first ": "
	before, after, ok := strings.Cut(payload, ": ")
	if !ok {
		return nil
	}

	field := before
	value := after

	// Check for "old > new" pattern using the first unescaped " > " separator.
	// Escaped occurrences (" \> ") in values are unescaped after splitting.
	if idx := findUnescapedSeparator(value); idx >= 0 {
		rawNew := value[idx+3:]
		var newVal string
		if rawNew == emptyValueMarker {
			newVal = ""
		} else {
			newVal = unescapeActivityValue(rawNew)
		}
		return normalizeLegacyLifecycleActivity(&models.ActivityEntry{
			Timestamp: timestamp,
			Field:     field,
			OldValue:  unescapeActivityValue(value[:idx]),
			NewValue:  newVal,
		})
	}

	// Tags or other single-value entries
	return normalizeLegacyLifecycleActivity(&models.ActivityEntry{
		Timestamp: timestamp,
		Field:     field,
		OldValue:  "",
		NewValue:  unescapeActivityValue(value),
	})
}

func normalizeLegacyLifecycleActivity(entry *models.ActivityEntry) *models.ActivityEntry {
	if entry == nil || entry.Field != "status" {
		return entry
	}
	entry.Field = "state"
	entry.OldValue = legacyActivityState(entry.OldValue)
	entry.NewValue = legacyActivityState(entry.NewValue)
	return entry
}

func legacyActivityState(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "todo":
		return string(models.StateReady)
	case "in_progress":
		return string(models.StateInProgress)
	case "blocked":
		return string(models.StateWaiting)
	case "standby":
		return string(models.StatePaused)
	case "review":
		return string(models.StateReview)
	case "maybe":
		return string(models.StateSomeday)
	case "done":
		return string(models.StateDone)
	case "canceled":
		return string(models.StateCanceled)
	default:
		return value
	}
}

// findUnescapedSeparator returns the index of the first unescaped " > " in s,
// or -1 if none is found. An escaped separator is " \> ".
// To distinguish an escaped separator from a literal backslash before a real
// separator (e.g., "foo\\\\ > bar"), we count consecutive backslashes: an odd
// count means the last backslash escapes the ">", an even count means all
// backslashes are literal and the " > " is a real separator.
func findUnescapedSeparator(s string) int {
	searchFrom := 0
	for {
		idx := strings.Index(s[searchFrom:], " > ")
		if idx == -1 {
			return -1
		}
		idx += searchFrom
		// Count consecutive backslashes immediately before the " > "
		backslashes := 0
		for i := idx - 1; i >= 0 && s[i] == '\\'; i-- {
			backslashes++
		}
		// Odd number of backslashes means the ">" is escaped; even means it's real
		if backslashes%2 == 1 {
			searchFrom = idx + 3
			continue
		}
		return idx
	}
}

// escapeActivityValue escapes literal " > ", backslashes, and line breaks in
// activity values so each activity entry remains a single parseable comment.
func escapeActivityValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], " > ") {
			b.WriteString(` \> `)
			i += len(" > ") - 1
			continue
		}
		switch s[i] {
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// unescapeActivityValue reverses escapeActivityValue.
func unescapeActivityValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
			case '>':
				b.WriteByte('>')
				i++
			case 'n':
				b.WriteByte('\n')
				i++
			case 'r':
				b.WriteByte('\r')
				i++
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// WriteActivity generates the activity block for a task.
func WriteActivity(task *models.Task) string {
	if len(task.Activity) == 0 {
		return ""
	}

	var b strings.Builder
	headerWritten := false

	// Write entries in reverse chronological order (newest first)
	for i := len(task.Activity) - 1; i >= 0; i-- {
		entry := task.Activity[i]
		if entry == nil {
			continue
		}
		if !headerWritten {
			b.WriteString("<!-- ACTIVITY -->\n")
			headerWritten = true
		}
		ts := entry.Timestamp.Format(util.DateTimeFormat)

		if entry.Field == "created" {
			fmt.Fprintf(&b, "<!-- act: %s, created -->\n", ts)
			continue
		}

		if entry.OldValue == "" {
			// Single-value entry (e.g., tags: +backend +api)
			fmt.Fprintf(&b, "<!-- act: %s, %s: %s -->\n", ts, entry.Field, escapeActivityValue(entry.NewValue))
		} else {
			newSerialized := escapeActivityValue(entry.NewValue)
			if entry.NewValue == "" {
				newSerialized = emptyValueMarker
			}
			fmt.Fprintf(&b, "<!-- act: %s, %s: %s > %s -->\n", ts, entry.Field, escapeActivityValue(entry.OldValue), newSerialized)
		}
	}

	if !headerWritten {
		return ""
	}
	b.WriteString("<!-- /ACTIVITY -->\n")
	return b.String()
}
