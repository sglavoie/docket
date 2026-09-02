package format

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// ValidateV5TaskRecord validates the complete target record without consulting
// any legacy task type or project tag catalog.
func ValidateV5TaskRecord(task V5TaskRecord) error {
	return validateV5TaskRecord(task, true)
}

//nolint:gocyclo // Canonical record validation intentionally centralizes every schema invariant.
func validateV5TaskRecord(task V5TaskRecord, requireTitle bool) error {
	if strings.TrimSpace(task.ID) == "" || task.ID != strings.TrimSpace(task.ID) {
		return fmt.Errorf("v5 task id is required and must be trimmed")
	}
	if strings.ContainsAny(task.ID, ",;\n\r") {
		return fmt.Errorf("v5 task id %q contains a metadata delimiter", task.ID)
	}
	if task.ShortID <= 0 {
		return fmt.Errorf("v5 task %q has invalid short id %d", task.ID, task.ShortID)
	}
	if requireTitle {
		if strings.TrimSpace(task.Title) == "" {
			return fmt.Errorf("v5 task %q has an empty title", task.ID)
		}
		if task.Title != strings.TrimSpace(task.Title) {
			return fmt.Errorf("v5 task %q title must be trimmed", task.ID)
		}
		if err := models.ValidateTaskTitle(task.Title); err != nil {
			return fmt.Errorf("v5 task %q title: %w", task.ID, err)
		}
	}
	if !isV5State(task.State) {
		return fmt.Errorf("v5 task %q has invalid state %q", task.ID, task.State)
	}
	if task.Priority < 0 || task.Priority > 5 {
		return fmt.Errorf("v5 task %q priority must be between 0 and 5", task.ID)
	}
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return fmt.Errorf("v5 task %q requires created and updated timestamps", task.ID)
	}
	if task.CompletedAt != nil && task.CompletedAt.IsZero() {
		return fmt.Errorf("v5 task %q has a zero completed timestamp", task.ID)
	}
	if err := validateV5Date("due", task.DueDate); err != nil {
		return fmt.Errorf("v5 task %q: %w", task.ID, err)
	}
	if err := validateV5Date("snoozed", task.SnoozedUntil); err != nil {
		return fmt.Errorf("v5 task %q: %w", task.ID, err)
	}
	if task.ParentID != "" {
		if err := validateV5Reference("parent", task.ParentID); err != nil {
			return fmt.Errorf("v5 task %q: %w", task.ID, err)
		}
		if task.ParentID == task.ID {
			return fmt.Errorf("v5 task %q cannot parent itself", task.ID)
		}
	}
	if err := validateV5StringSet("depends", task.ID, task.DependsOn); err != nil {
		return err
	}
	if err := validateCanonicalV5Tags(task.ID, task.Tags); err != nil {
		return err
	}
	if task.PinnedAt != nil && task.PinnedAt.IsZero() {
		return fmt.Errorf("v5 task %q has a zero pin timestamp", task.ID)
	}
	if task.Recurrence != nil {
		if task.Recurrence.Interval <= 0 || task.Recurrence.Interval > v5MaxRecurrenceInterval {
			return fmt.Errorf("v5 task %q recurrence interval must be between 1 and %d", task.ID, v5MaxRecurrenceInterval)
		}
		switch task.Recurrence.Unit {
		case "day", "week", "month":
		default:
			return fmt.Errorf("v5 task %q has invalid recurrence unit %q", task.ID, task.Recurrence.Unit)
		}
	}
	for i, attachment := range task.Attachments {
		if err := validateV5Attachment(attachment); err != nil {
			return fmt.Errorf("v5 task %q attachment %d: %w", task.ID, i+1, err)
		}
	}
	for i, item := range task.Checklist {
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("v5 task %q checklist item %d has empty text", task.ID, i+1)
		}
	}
	for i, comment := range task.Comments {
		if strings.TrimSpace(comment.Text) == "" {
			return fmt.Errorf("v5 task %q comment %d has empty text", task.ID, i+1)
		}
		if comment.CreatedAt.IsZero() {
			return fmt.Errorf("v5 task %q comment %d has a zero timestamp", task.ID, i+1)
		}
	}
	for i, decision := range task.Decisions {
		if strings.TrimSpace(decision.Text) == "" {
			return fmt.Errorf("v5 task %q decision %d has empty text", task.ID, i+1)
		}
		if decision.CreatedAt.IsZero() {
			return fmt.Errorf("v5 task %q decision %d has a zero timestamp", task.ID, i+1)
		}
	}
	return nil
}

func isV5State(state string) bool {
	switch state {
	case "inbox", "ready", "next", "in_progress", "waiting", "paused", "someday", "done", "canceled":
		return true
	default:
		return false
	}
}

func validateV5Date(field, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(v5DateFormat, value)
	if err != nil || parsed.Format(v5DateFormat) != value {
		return fmt.Errorf("%s date %q is not canonical YYYY-MM-DD", field, value)
	}
	return nil
}

func validateV5Reference(field, value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s reference is empty or untrimmed", field)
	}
	if strings.ContainsAny(value, ",;\n\r") {
		return fmt.Errorf("%s reference %q contains a metadata delimiter", field, value)
	}
	return nil
}

func validateV5StringSet(field, taskID string, values []string) error {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if err := validateV5Reference(field, value); err != nil {
			return fmt.Errorf("v5 task %q: %w", taskID, err)
		}
		if value == taskID {
			return fmt.Errorf("v5 task %q cannot depend on itself", taskID)
		}
		if seen[value] {
			return fmt.Errorf("v5 task %q has duplicate %s reference %q", taskID, field, value)
		}
		seen[value] = true
	}
	return nil
}

func validateCanonicalV5Tags(taskID string, tags []string) error {
	normalized, err := normalizeMigrationV5Tags(tags)
	if err != nil {
		return fmt.Errorf("v5 task %q tags: %w", taskID, err)
	}
	if !slices.Equal(tags, normalized) {
		return fmt.Errorf("v5 task %q tags are not canonical: got %v, want %v", taskID, tags, normalized)
	}
	return nil
}

// normalizeMigrationV5Tags preserves the historical schema-5 marker rules at
// the migration boundary. Live schema-6 tags use ordinary independent tag
// normalization; only legacy v5 decoding canonicalizes type:<marker> spellings
// and rejects the marker combinations that schema version could not represent.
func normalizeMigrationV5Tags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	var selected string
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			return nil, fmt.Errorf("tag cannot be empty")
		}
		if len(tag) > models.MaxTagLength {
			return nil, fmt.Errorf("tag exceeds %d characters", models.MaxTagLength)
		}
		if strings.ContainsAny(tag, ",;\n\r") {
			return nil, fmt.Errorf("tag contains a metadata delimiter")
		}
		if marker, ok := parseMigrationV5TaskMarkerTag(tag); ok {
			if selected != "" && selected != marker {
				return nil, fmt.Errorf("task can have only one type marker (%s and %s conflict)", selected, marker)
			}
			selected = marker
			tag = marker
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

// parseMigrationV5TaskMarkerTag owns the retired schema-5 marker vocabulary at
// its read boundary. Live task tags deliberately have no marker parser.
func parseMigrationV5TaskMarkerTag(tag string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(tag, "#")))
	normalized = strings.TrimPrefix(normalized, "type:")
	switch normalized {
	case "epic", "story", "bug", "idea":
		return normalized, true
	default:
		return "", false
	}
}

func validateV5Attachment(attachment V5TaskAttachment) error {
	switch attachment.Kind {
	case "file", "url", "code":
	default:
		return fmt.Errorf("invalid kind %q", attachment.Kind)
	}
	if attachment.Filename == "" {
		return fmt.Errorf("filename is required")
	}
	if attachment.LineNumber < 0 {
		return fmt.Errorf("line number cannot be negative")
	}
	if attachment.Kind != "code" && attachment.LineNumber != 0 {
		return fmt.Errorf("line number is only valid for code attachments")
	}
	if attachment.AddedAt != nil && attachment.AddedAt.IsZero() {
		return fmt.Errorf("added timestamp cannot be zero")
	}
	return nil
}
