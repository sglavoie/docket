package format

import (
	"fmt"
	"strings"
)

// This file owns the schema-18 task body. The block set is schema 16's
// unchanged — the optional RESUME block, then DESCRIPTION and COMMENTS — and the
// optional checklist-origin line is schema 17's, so what schema 18 actually
// changes is the metadata line alone. Both halves below therefore delegate: the
// blocks to writeTaskBodyV16 and parseTaskBodyBlocksV16, the origin line to the
// shared helpers in checklist_origin.go, and only the metadata to this schema's
// own writer and reader.
//
// It is written as its own pair rather than by widening schema 17's for the
// reason every schema here is: a frozen reader taught a newer metadata key would
// start accepting documents that did not exist when its own were written, and
// the archives a migration validates against would stop being a fixed target.

// WriteV18TaskBody renders metadata plus the optional checklist-origin line and
// the retained resume note, description, and comment blocks. The project
// document owns the generic Task heading.
func WriteV18TaskBody(task V18TaskRecord) (string, error) {
	if err := ValidateV18TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV18TaskMetadata(task)
	if err != nil {
		return "", err
	}
	body := writeTaskBodyV16(metadata, task.ResumeNote, task.Description, task.Comments)
	return insertChecklistOriginLine(body, task.ChecklistOrigin)
}

// ParseV18TaskBody parses the body below one generic Task heading.
func ParseV18TaskBody(title, body string) (V18TaskRecord, error) {
	origin, remainder, err := splitChecklistOriginLine("v18", title, body)
	if err != nil {
		return V18TaskRecord{}, err
	}
	lines := strings.Split(strings.TrimSuffix(remainder, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V18TaskRecord{}, fmt.Errorf("v18 task %q is missing metadata", title)
	}
	task, err := ParseV18TaskMetadata(lines[0])
	if err != nil {
		return V18TaskRecord{}, fmt.Errorf("v18 task %q: %w", title, err)
	}
	task.Title = title
	task.ChecklistOrigin = origin

	task.ResumeNote, task.Description, task.Comments, err = parseTaskBodyBlocksV16("v18", title, lines)
	if err != nil {
		return V18TaskRecord{}, err
	}
	if err := ValidateV18TaskRecord(task); err != nil {
		return V18TaskRecord{}, err
	}
	return task, nil
}
