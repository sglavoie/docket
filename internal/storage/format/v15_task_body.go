package format

import (
	"fmt"
	"strings"
)

// This file owns the schema-15 task body, split from the metadata comment in
// v15_task_codec.go for the same reason schema 6 is split: the metadata line is
// a schema's own grammar, while the blocks beneath it are shared with every
// schema that changed nothing about them.
//
// Both halves below delegate to writeTaskBody and parseTaskBodyBlocks in
// v6_task_body.go, which are frozen. A schema that admits a block those helpers
// do not know about gets its own body file beside this one and leaves theirs
// alone: teaching the shared reader a newer block would make every historical
// schema accept it too, and a frozen reader that accepts more than it was
// written for is no longer a frozen reader.

// WriteV15TaskBody renders metadata plus the retained description and comment
// blocks. The project document owns the generic Task heading.
func WriteV15TaskBody(task V15TaskRecord) (string, error) {
	if err := ValidateV15TaskRecord(task); err != nil {
		return "", err
	}
	metadata, err := WriteV15TaskMetadata(task)
	if err != nil {
		return "", err
	}
	return writeTaskBody(metadata, task.Description, task.Comments), nil
}

// ParseV15TaskBody parses the body below one generic Task heading.
func ParseV15TaskBody(title, body string) (V15TaskRecord, error) {
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return V15TaskRecord{}, fmt.Errorf("v15 task %q is missing metadata", title)
	}
	task, err := ParseV15TaskMetadata(lines[0])
	if err != nil {
		return V15TaskRecord{}, fmt.Errorf("v15 task %q: %w", title, err)
	}
	task.Title = title

	task.Description, task.Comments, err = parseTaskBodyBlocks("v15", title, lines)
	if err != nil {
		return V15TaskRecord{}, err
	}
	if err := ValidateV15TaskRecord(task); err != nil {
		return V15TaskRecord{}, err
	}
	return task, nil
}
