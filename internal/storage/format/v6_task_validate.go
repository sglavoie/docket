package format

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// ValidateV6TaskRecord validates a complete schema-6 task record.
func ValidateV6TaskRecord(task V6TaskRecord) error {
	return validateV6TaskRecord(task, true, deferDateLegacy)
}

// deferral spells the deferral value the way the caller's own schema does, so a
// rejected value is checked against that schema's layout and reported under the
// key that schema actually writes.
//
//nolint:gocyclo // Keeping all durable field invariants together makes the schema boundary auditable.
func validateV6TaskRecord(task V6TaskRecord, requireTitle bool, deferral deferEncoding) error {
	if strings.TrimSpace(task.ID) == "" || task.ID != strings.TrimSpace(task.ID) {
		return fmt.Errorf("v6 task id is required and must be trimmed")
	}
	if strings.ContainsAny(task.ID, ",;\n\r") {
		return fmt.Errorf("v6 task id %q contains a metadata delimiter", task.ID)
	}
	if task.ShortID <= 0 {
		return fmt.Errorf("v6 task %q has invalid short id %d", task.ID, task.ShortID)
	}
	if requireTitle {
		if strings.TrimSpace(task.Title) == "" {
			return fmt.Errorf("v6 task %q has an empty title", task.ID)
		}
		if task.Title != strings.TrimSpace(task.Title) {
			return fmt.Errorf("v6 task %q title must be trimmed", task.ID)
		}
		if err := models.ValidateTaskTitle(task.Title); err != nil {
			return fmt.Errorf("v6 task %q title: %w", task.ID, err)
		}
	}
	if !isV6State(task.State) {
		return fmt.Errorf("v6 task %q has invalid state %q", task.ID, task.State)
	}
	if task.Priority < 0 || task.Priority > 3 {
		return fmt.Errorf("v6 task %q priority must be between 0 and 3", task.ID)
	}
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		return fmt.Errorf("v6 task %q requires created and updated timestamps", task.ID)
	}
	if task.CompletedAt != nil && task.CompletedAt.IsZero() {
		return fmt.Errorf("v6 task %q has a zero completed timestamp", task.ID)
	}
	if err := validateV6Date("due", task.DueDate); err != nil {
		return fmt.Errorf("v6 task %q: %w", task.ID, err)
	}
	if err := validateDeferValue(deferral, task.SnoozedUntil); err != nil {
		return fmt.Errorf("v6 task %q: %w", task.ID, err)
	}
	if task.ParentID != "" {
		if err := validateV6Reference("parent", task.ParentID); err != nil {
			return fmt.Errorf("v6 task %q: %w", task.ID, err)
		}
		if task.ParentID == task.ID {
			return fmt.Errorf("v6 task %q cannot parent itself", task.ID)
		}
	}
	if err := validateCanonicalV6Tags(task.ID, task.Tags); err != nil {
		return err
	}
	if task.PinnedAt != nil && task.PinnedAt.IsZero() {
		return fmt.Errorf("v6 task %q has a zero pin timestamp", task.ID)
	}
	for i, attachment := range task.Attachments {
		if err := validateV6Attachment(attachment); err != nil {
			return fmt.Errorf("v6 task %q attachment %d: %w", task.ID, i+1, err)
		}
	}
	for i, item := range task.Checklist {
		if strings.TrimSpace(item.Text) == "" {
			return fmt.Errorf("v6 task %q checklist item %d has empty text", task.ID, i+1)
		}
	}
	for i, comment := range task.Comments {
		if strings.TrimSpace(comment.Text) == "" {
			return fmt.Errorf("v6 task %q comment %d has empty text", task.ID, i+1)
		}
		if comment.CreatedAt.IsZero() {
			return fmt.Errorf("v6 task %q comment %d has a zero timestamp", task.ID, i+1)
		}
	}
	return nil
}

func isV6State(state string) bool {
	switch state {
	case "inbox", "backlog", "doing", "waiting", "paused", "someday", "done", "canceled":
		return true
	default:
		return false
	}
}

func validateV6Date(field, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(v6DateFormat, value)
	if err != nil || parsed.Format(v6DateFormat) != value {
		return fmt.Errorf("%s date %q is not canonical YYYY-MM-DD", field, value)
	}
	return nil
}

// validateDeferValue rejects a deferral value the encoding's layout cannot
// round-trip, which is the same canonicality bar validateV6Date and parseV6Time
// enforce for their one fixed layout.
func validateDeferValue(enc deferEncoding, value string) error {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(enc.layout, value)
	if err != nil || parsed.Format(enc.layout) != value {
		canonical := enc.layout
		switch enc.layout {
		case v6DateFormat:
			canonical = "YYYY-MM-DD"
		case time.RFC3339Nano:
			canonical = "RFC3339Nano"
		}
		return fmt.Errorf("%s date %q is not canonical %s", enc.key, value, canonical)
	}
	return nil
}

func validateV6Reference(field, value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s reference is empty or untrimmed", field)
	}
	if strings.ContainsAny(value, ",;\n\r") {
		return fmt.Errorf("%s reference %q contains a metadata delimiter", field, value)
	}
	return nil
}

func validateCanonicalV6Tags(taskID string, tags []string) error {
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		// Schema 6 predates the lowercase-kebab contract. Keep this reader
		// frozen so schema-8 activation can ingest and audit legacy spellings.
		if tag == "" || strings.HasPrefix(tag, "#") || tag != strings.TrimSpace(tag) || len(tag) > models.MaxTagLength ||
			strings.ContainsAny(tag, ",;\n\r") {
			return fmt.Errorf("v6 task %q has invalid legacy tag %q", taskID, tag)
		}
		if seen[tag] {
			return fmt.Errorf("v6 task %q has duplicate tag %q", taskID, tag)
		}
		seen[tag] = true
	}
	return nil
}

// ValidateV7TaskRecord applies the schema-7 lowercase-kebab tag contract to
// the otherwise unchanged schema-6 task record shape.
func ValidateV7TaskRecord(task V6TaskRecord) error {
	return validateV7TaskRecord(task, deferDateLegacy)
}

func validateV7TaskRecord(task V6TaskRecord, deferral deferEncoding) error {
	if err := validateV6TaskRecord(task, true, deferral); err != nil {
		return err
	}
	for _, tag := range task.Tags {
		if err := models.ValidateTag(tag); err != nil {
			return fmt.Errorf("v7 task %q tags: %w", task.ID, err)
		}
	}
	return nil
}

func validateV6Attachment(attachment V6TaskAttachment) error {
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
