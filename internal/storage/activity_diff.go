package storage

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// diffAndRecordActivity compares old and new task states and records activity entries.
// Tasks are matched by ID. New tasks (not in oldTasks) get a "created" entry.
func diffAndRecordActivity(oldTasks, newTasks []*models.Task, maxEntries int) {
	oldByID := make(map[string]*models.Task, len(oldTasks))
	for _, t := range oldTasks {
		oldByID[t.ID] = t
	}

	for _, newTask := range newTasks {
		old, exists := oldByID[newTask.ID]
		if !exists {
			// New task - add "created" entry if it doesn't already have one
			if !hasCreatedEntry(newTask) {
				newTask.Activity = append(newTask.Activity, &models.ActivityEntry{
					Timestamp: newTask.CreatedAt,
					Field:     "created",
				})
			}
			newTask.PruneActivity(maxEntries)
			continue
		}

		diffTask(old, newTask)
		newTask.PruneActivity(maxEntries)
	}
}

// diffTask compares two versions of the same task and records changes.
func diffTask(old, new *models.Task) {
	now := time.Now()

	// record appends an activity entry to the new task.
	record := func(field, oldVal, newVal string) {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     field,
			OldValue:  oldVal,
			NewValue:  newVal,
		})
	}

	// recordIfDiff records a change when two pre-computed strings differ.
	recordIfDiff := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			record(field, oldVal, newVal)
		}
	}

	// Workflow state replaces the legacy status/section/stage triplet: one
	// canonical entry per lifecycle move instead of parallel legacy-field diffs.
	recordIfDiff("state", string(old.State()), string(new.State()))
	recordIfDiff("priority", fmt.Sprintf("%d", old.Priority), fmt.Sprintf("%d", new.Priority))

	recordIfDiff("title", old.Title, new.Title)
	recordIfDiff("type", string(old.Type), string(new.Type))

	// Tags: record as +added -removed
	if tagChanges := diffTags(old.Tags, new.Tags); tagChanges != "" {
		record("tags", "", tagChanges)
	}

	recordIfDiff("due", formatDuePtr(old.DueDate, old.HasTime), formatDuePtr(new.DueDate, new.HasTime))
	recordIfDiff("snoozed", formatTimePtr(old.SnoozedUntil), formatTimePtr(new.SnoozedUntil))

	// Boolean fields. The retired legacy Active flag has no task-level meaning,
	// so it is intentionally not part of the diff surface.
	recordIfDiff("pinned", fmt.Sprintf("%v", old.Pinned), fmt.Sprintf("%v", new.Pinned))
	recordIfDiff("dimmed", fmt.Sprintf("%v", old.Dimmed), fmt.Sprintf("%v", new.Dimmed))

	// Description (hash-based, don't store full text)
	if hashString(old.Description) != hashString(new.Description) {
		record("description", "changed", "updated")
	}

	recordIfDiff("comments", fmt.Sprintf("%d", len(old.Comments)), fmt.Sprintf("%d", len(new.Comments)))

	if !stringSlicesEqual(old.DependsOn, new.DependsOn) {
		record("dependencies", "", "updated")
	}

	if !stringSlicesEqual(old.RelatedTo, new.RelatedTo) {
		record("related", "", "updated")
	}

	recordIfDiff("attachments", fmt.Sprintf("%d", len(old.Attachments)), fmt.Sprintf("%d", len(new.Attachments)))
	recordIfDiff("parent", ptrStringVal(old.ParentID), ptrStringVal(new.ParentID))
	recordIfDiff("recurrence", recurrenceString(old.Recurrence), recurrenceString(new.Recurrence))

	// Checklist
	oldChecklist, newChecklist := len(old.Checklist), len(new.Checklist)
	oldDone, newDone := countChecklistDone(old.Checklist), countChecklistDone(new.Checklist)
	if oldChecklist != newChecklist || oldDone != newDone {
		record("checklist",
			fmt.Sprintf("%d/%d", oldDone, oldChecklist),
			fmt.Sprintf("%d/%d", newDone, newChecklist))
	}

	if !scoreDimensionsEqual(old.ScoreDimensions, new.ScoreDimensions) {
		record("score", "", "updated")
	}

	// String fields with empty -> "none" normalization
	recordIfDiff("url", orDefault(old.URL, "none"), orDefault(new.URL, "none"))
	recordIfDiff("source", orDefault(old.Source, "none"), orDefault(new.Source, "none"))
}

// orDefault returns fallback if s is empty, otherwise s.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// diffTags returns a string like "+backend +api -frontend" for tag changes.
// Duplicates in either input are collapsed so the output never repeats the
// same +tag or -tag entry.
func diffTags(oldTags, newTags []string) string {
	oldSet := make(map[string]bool, len(oldTags))
	for _, t := range oldTags {
		oldSet[t] = true
	}
	newSet := make(map[string]bool, len(newTags))
	for _, t := range newTags {
		newSet[t] = true
	}

	var parts []string

	// Added tags (deduplicated by iterating the new-set keys via newTags order)
	addedSeen := make(map[string]bool, len(newTags))
	added := make([]string, 0)
	for _, t := range newTags {
		if oldSet[t] || addedSeen[t] {
			continue
		}
		addedSeen[t] = true
		added = append(added, t)
	}
	sort.Strings(added)
	for _, t := range added {
		parts = append(parts, "+"+t)
	}

	// Removed tags (deduplicated similarly)
	removedSeen := make(map[string]bool, len(oldTags))
	removed := make([]string, 0)
	for _, t := range oldTags {
		if newSet[t] || removedSeen[t] {
			continue
		}
		removedSeen[t] = true
		removed = append(removed, t)
	}
	sort.Strings(removed)
	for _, t := range removed {
		parts = append(parts, "-"+t)
	}

	return strings.Join(parts, " ")
}

func formatTimePtr(d *time.Time) string {
	if d == nil {
		return "none"
	}
	return d.Format(util.DateFormat)
}

// formatDuePtr formats a due date for activity diffing. When hasTime is true,
// the clock component is included so that an hour/minute change on the same
// day still produces a distinct value (and therefore an activity entry).
// Without this, formatTimePtr's date-only format would silently swallow
// time-of-day edits on time-bearing due dates.
func formatDuePtr(d *time.Time, hasTime bool) string {
	if d == nil {
		return "none"
	}
	if hasTime {
		return d.Format(util.DateTimeDueFormat)
	}
	return d.Format(util.DateFormat)
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func hasCreatedEntry(t *models.Task) bool {
	for _, e := range t.Activity {
		if e.Field == "created" {
			return true
		}
	}
	return false
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ptrStringVal(p *string) string {
	if p == nil {
		return "none"
	}
	return *p
}

// recurrenceString returns a human-readable string for a recurrence pattern, or "none".
func recurrenceString(r *models.RecurrencePattern) string {
	if r == nil {
		return "none"
	}
	return r.String()
}

// countChecklistDone returns the number of completed checklist items.
func countChecklistDone(items []models.ChecklistItem) int {
	n := 0
	for _, item := range items {
		if item.Done {
			n++
		}
	}
	return n
}

// scoreDimensionsEqual compares two score dimension maps for equality.
func scoreDimensionsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}
