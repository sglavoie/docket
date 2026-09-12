package format

import (
	"fmt"
	"strconv"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// V18TaskRecord is the complete schema-18 storage shape: the schema-17 fields
// plus the check-in schedule and the review history instant. A check-in is when
// this task is next meant to be looked at; the repeat is how far forward
// acknowledging it moves that date; the review instant is when it was last
// answered. The first two are owed work and the third is history, which is why
// completion clears the pair and keeps the instant.
//
// The check-in date is stored as a calendar date, in the same YYYY-MM-DD
// spelling the deadline uses, and deliberately has no companion clock key the
// way "due" has "due_time": a check-in arrives when its day arrives, so an hour
// would be a value with no reader and two zones would disagree about the same
// schedule. The review instant is stored as an instant for the opposite reason —
// it records a moment that happened rather than a day that is owed.
//
// All three keys are optional and written only when set, so a task with no
// schedule and no review history stays byte-identical to its schema-17
// predecessor. That is the same additive discipline depends, due_time, review,
// and the resume note were added under.
type V18TaskRecord struct {
	V17TaskRecord
	// NextCheckIn is the canonical YYYY-MM-DD date, empty when absent.
	NextCheckIn string
	// CheckInRepeatDays is the cadence in whole calendar days, 0 for off.
	CheckInRepeatDays int
	// LastReviewedAt is the instant of the last acknowledgement, nil for none.
	LastReviewedAt *time.Time
}

// The three keys schema 18 adds.
const (
	v18CheckInMetadataKey       = "check_in"
	v18CheckInRepeatMetadataKey = "check_in_repeat"
	v18ReviewedMetadataKey      = "reviewed"
)

// v18CheckInDateLayout is the canonical stored spelling of a check-in date. It
// is the deadline's layout on purpose: two date-valued fields in one metadata
// line that disagreed about their format would be a grammar nobody could read by
// eye.
const v18CheckInDateLayout = "2006-01-02"

var v18MetadataKeys = v18TaskMetadataKeys()

func v18TaskMetadataKeys() map[string]bool {
	keys := make(map[string]bool, len(v15MetadataKeys)+3)
	for key := range v15MetadataKeys {
		keys[key] = true
	}
	keys[v18CheckInMetadataKey] = true
	keys[v18CheckInRepeatMetadataKey] = true
	keys[v18ReviewedMetadataKey] = true
	return keys
}

// All three keys are optional, so schema 18 has the same required metadata as
// schema 15, which is the last schema that changed the metadata line at all.
var v18RequiredMetadataKeys = v15RequiredMetadataKeys

// ValidateV18TaskRecord validates a complete schema-18 task record.
func ValidateV18TaskRecord(task V18TaskRecord) error {
	return validateV18TaskRecord(task, true)
}

// validateV18TaskRecord validates the inherited fields and then the three
// schema 18 adds. The repeat bound and the "a repeat needs a date" rule are the
// model's, read through models.ValidateCheckInSchedule, so the storage writer
// and every editing surface refuse exactly the same schedules. What is checked
// here and not there is the stored spelling: the date has to be the canonical
// layout, and the review instant has to be a real moment, because those are
// properties of the record rather than of the schedule.
func validateV18TaskRecord(task V18TaskRecord, requireTitle bool) error {
	if requireTitle {
		if err := ValidateV17TaskRecord(task.V17TaskRecord); err != nil {
			return err
		}
	} else if err := validateV16TaskRecord(task.V16TaskRecord, false); err != nil {
		return err
	} else if err := validateChecklistOrigin(task.ChecklistOrigin); err != nil {
		return err
	}
	next, err := v18CheckInDate(task.NextCheckIn)
	if err != nil {
		return err
	}
	if err := models.ValidateCheckInSchedule(models.CheckInSchedule{
		Next: next, RepeatDays: task.CheckInRepeatDays,
	}); err != nil {
		return err
	}
	if task.LastReviewedAt != nil && task.LastReviewedAt.IsZero() {
		return fmt.Errorf("v18 %s must be a real instant when present", v18ReviewedMetadataKey)
	}
	return nil
}

// v18CheckInDate reads the stored check-in date back as the calendar day it
// names. An empty value is no date; anything the canonical layout does not
// round-trip is refused rather than repaired, because a date this reader had to
// guess at is a date the writer never produced.
func v18CheckInDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(v18CheckInDateLayout, value)
	if err != nil {
		return nil, fmt.Errorf("parse v18 %s: %w", v18CheckInMetadataKey, err)
	}
	if parsed.Format(v18CheckInDateLayout) != value {
		return nil, fmt.Errorf("v18 %s %q is not a canonical %s value",
			v18CheckInMetadataKey, value, v18CheckInDateLayout)
	}
	return &parsed, nil
}

// WriteV18TaskMetadata writes the canonical schema-18 metadata comment. Each of
// the three keys is omitted when its value is absent, which is what keeps a
// migrated record byte-equal to the schema-17 record it came from.
func WriteV18TaskMetadata(task V18TaskRecord) (string, error) {
	if err := validateV18TaskRecord(task, false); err != nil {
		return "", err
	}
	parts := v15MetadataParts(task.V15TaskRecord)
	if task.NextCheckIn != "" {
		parts = append(parts, v18CheckInMetadataKey+": "+task.NextCheckIn)
	}
	if task.CheckInRepeatDays > 0 {
		parts = append(parts, v18CheckInRepeatMetadataKey+": "+strconv.Itoa(task.CheckInRepeatDays))
	}
	if task.LastReviewedAt != nil {
		parts = append(parts, v18ReviewedMetadataKey+": "+formatV6Time(*task.LastReviewedAt))
	}
	return wrapTaskMetadata(parts), nil
}

// ParseV18TaskMetadata strictly parses one schema-18 metadata comment.
func ParseV18TaskMetadata(comment string) (V18TaskRecord, error) {
	values, err := parseTaskMetadataFields("v18", comment, v18MetadataKeys, v18RequiredMetadataKeys)
	if err != nil {
		return V18TaskRecord{}, err
	}
	base, err := v15RecordFromMetadataValues(values)
	if err != nil {
		return V18TaskRecord{}, err
	}
	task := V18TaskRecord{V17TaskRecord: V17TaskRecord{V16TaskRecord: V16TaskRecord{V15TaskRecord: base}}}
	task.NextCheckIn = values[v18CheckInMetadataKey]
	if raw, present := values[v18CheckInRepeatMetadataKey]; present {
		task.CheckInRepeatDays, err = strconv.Atoi(raw)
		if err != nil {
			return V18TaskRecord{}, fmt.Errorf("parse v18 %s: %w", v18CheckInRepeatMetadataKey, err)
		}
		// The writer omits the key when the repeat is off, so a stored zero is a
		// file this writer did not produce rather than a schedule that does not
		// repeat.
		if task.CheckInRepeatDays == 0 {
			return V18TaskRecord{}, fmt.Errorf("parse v18 %s: value must be positive when present",
				v18CheckInRepeatMetadataKey)
		}
	}
	if raw, present := values[v18ReviewedMetadataKey]; present {
		reviewed, parseErr := parseV6Time(raw)
		if parseErr != nil {
			return V18TaskRecord{}, fmt.Errorf("parse v18 %s: %w", v18ReviewedMetadataKey, parseErr)
		}
		task.LastReviewedAt = &reviewed
	}
	if err := validateV18TaskRecord(task, false); err != nil {
		return V18TaskRecord{}, err
	}
	return task, nil
}
