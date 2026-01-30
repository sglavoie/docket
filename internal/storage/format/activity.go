package format

import (
	"fmt"
	"regexp"
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
	idx := strings.Index(payload, ": ")
	if idx < 0 {
		return nil
	}

	field := payload[:idx]
	value := payload[idx+2:]

	// Check for "old > new" pattern
	if parts := strings.SplitN(value, " > ", 2); len(parts) == 2 {
		return &models.ActivityEntry{
			Timestamp: timestamp,
			Field:     field,
			OldValue:  parts[0],
			NewValue:  parts[1],
		}
	}

	// Tags or other single-value entries
	return &models.ActivityEntry{
		Timestamp: timestamp,
		Field:     field,
		OldValue:  "",
		NewValue:  value,
	}
}

// WriteActivity generates the activity block for a task.
func WriteActivity(task *models.Task) string {
	if len(task.Activity) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<!-- ACTIVITY -->\n")

	// Write entries in reverse chronological order (newest first)
	for i := len(task.Activity) - 1; i >= 0; i-- {
		entry := task.Activity[i]
		ts := entry.Timestamp.Format(util.DateTimeFormat)

		if entry.Field == "created" {
			b.WriteString(fmt.Sprintf("<!-- act: %s, created -->\n", ts))
			continue
		}

		if entry.OldValue == "" {
			// Single-value entry (e.g., tags: +backend +api)
			b.WriteString(fmt.Sprintf("<!-- act: %s, %s: %s -->\n", ts, entry.Field, entry.NewValue))
		} else {
			b.WriteString(fmt.Sprintf("<!-- act: %s, %s: %s > %s -->\n", ts, entry.Field, entry.OldValue, entry.NewValue))
		}
	}

	b.WriteString("<!-- /ACTIVITY -->\n")
	return b.String()
}
