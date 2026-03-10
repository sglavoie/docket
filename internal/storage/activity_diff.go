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

// DiffAndRecordActivity compares old and new task states and records activity entries.
// Tasks are matched by ID. New tasks (not in oldTasks) get a "created" entry.
func DiffAndRecordActivity(oldTasks, newTasks []*models.Task, maxEntries int) {
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

	if old.Status != new.Status {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "status",
			OldValue:  string(old.Status),
			NewValue:  string(new.Status),
		})
	}

	if old.Priority != new.Priority {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "priority",
			OldValue:  fmt.Sprintf("%d", old.Priority),
			NewValue:  fmt.Sprintf("%d", new.Priority),
		})
	}

	if old.Section != new.Section {
		oldSection := old.Section
		if oldSection == "" {
			oldSection = "Active"
		}
		newSection := new.Section
		if newSection == "" {
			newSection = "Active"
		}
		if oldSection != newSection {
			new.Activity = append(new.Activity, &models.ActivityEntry{
				Timestamp: now,
				Field:     "section",
				OldValue:  oldSection,
				NewValue:  newSection,
			})
		}
	}

	if old.Stage != new.Stage {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "stage",
			OldValue:  old.Stage,
			NewValue:  new.Stage,
		})
	}

	if old.Title != new.Title {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "title",
			OldValue:  old.Title,
			NewValue:  new.Title,
		})
	}

	if string(old.Type) != string(new.Type) {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "type",
			OldValue:  string(old.Type),
			NewValue:  string(new.Type),
		})
	}

	// Tags: record as +added -removed
	tagChanges := diffTags(old.Tags, new.Tags)
	if tagChanges != "" {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "tags",
			NewValue:  tagChanges,
		})
	}

	// Due date
	oldDue := formatDueDate(old.DueDate)
	newDue := formatDueDate(new.DueDate)
	if oldDue != newDue {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "due",
			OldValue:  oldDue,
			NewValue:  newDue,
		})
	}

	// Snoozed
	oldSnoozed := formatSnoozedDate(old.SnoozedUntil)
	newSnoozed := formatSnoozedDate(new.SnoozedUntil)
	if oldSnoozed != newSnoozed {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "snoozed",
			OldValue:  oldSnoozed,
			NewValue:  newSnoozed,
		})
	}

	// Pinned
	if old.Pinned != new.Pinned {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "pinned",
			OldValue:  fmt.Sprintf("%v", old.Pinned),
			NewValue:  fmt.Sprintf("%v", new.Pinned),
		})
	}

	// Active
	if old.Active != new.Active {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "active",
			OldValue:  fmt.Sprintf("%v", old.Active),
			NewValue:  fmt.Sprintf("%v", new.Active),
		})
	}

	// Description (hash-based, don't store full text)
	if hashString(old.Description) != hashString(new.Description) {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "description",
			OldValue:  "changed",
			NewValue:  "updated",
		})
	}

	// Comments
	oldComments := len(old.Comments)
	newComments := len(new.Comments)
	if oldComments != newComments {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "comments",
			OldValue:  fmt.Sprintf("%d", oldComments),
			NewValue:  fmt.Sprintf("%d", newComments),
		})
	}

	// Dependencies (DependsOn)
	if !stringSlicesEqual(old.DependsOn, new.DependsOn) {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "dependencies",
			NewValue:  "updated",
		})
	}

	// Related tasks
	if !stringSlicesEqual(old.RelatedTo, new.RelatedTo) {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "related",
			NewValue:  "updated",
		})
	}

	// Attachments
	oldAttach := len(old.Attachments)
	newAttach := len(new.Attachments)
	if oldAttach != newAttach {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "attachments",
			OldValue:  fmt.Sprintf("%d", oldAttach),
			NewValue:  fmt.Sprintf("%d", newAttach),
		})
	}

	// ParentID
	oldParent := ptrStringVal(old.ParentID)
	newParent := ptrStringVal(new.ParentID)
	if oldParent != newParent {
		new.Activity = append(new.Activity, &models.ActivityEntry{
			Timestamp: now,
			Field:     "parent",
			OldValue:  oldParent,
			NewValue:  newParent,
		})
	}
}

// diffTags returns a string like "+backend +api -frontend" for tag changes.
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

	// Added tags
	added := make([]string, 0)
	for _, t := range newTags {
		if !oldSet[t] {
			added = append(added, t)
		}
	}
	sort.Strings(added)
	for _, t := range added {
		parts = append(parts, "+"+t)
	}

	// Removed tags
	removed := make([]string, 0)
	for _, t := range oldTags {
		if !newSet[t] {
			removed = append(removed, t)
		}
	}
	sort.Strings(removed)
	for _, t := range removed {
		parts = append(parts, "-"+t)
	}

	return strings.Join(parts, " ")
}

func formatSnoozedDate(d *time.Time) string {
	if d == nil {
		return "none"
	}
	return d.Format(util.DateFormat)
}

func formatDueDate(d *time.Time) string {
	if d == nil {
		return "none"
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
