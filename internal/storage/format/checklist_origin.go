package format

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// This file owns the one-line checklist-origin record: where it sits in a task
// body, how it is spelled, and how strictly it is read back. It is shared by
// every schema that carries the line rather than copied into each, because the
// line has one canonical spelling and two readers that disagreed about it would
// be two schemas claiming the same bytes mean different things. The schema name
// is a parameter so each reader still reports its own version.
//
// The line is written directly under the metadata comment rather than as a
// block, because it is machine-written JSON of bounded shape: the block grammar
// exists to escape prose, and there is no prose here.

const checklistOriginPrefix = "<!-- checklist-origin: "
const checklistOriginSuffix = " -->"

// insertChecklistOriginLine writes the origin line under the metadata line of an
// already-rendered body. A nil origin writes nothing at all, which is what keeps
// a task with no origin byte-identical to its predecessor schema's rendering.
func insertChecklistOriginLine(body string, origin *models.ChecklistOrigin) (string, error) {
	if origin == nil {
		return body, nil
	}
	encoded, err := json.Marshal(origin)
	if err != nil {
		return "", err
	}
	newline := strings.IndexByte(body, '\n')
	return body[:newline+1] + checklistOriginPrefix + string(encoded) + checklistOriginSuffix + "\n" + body[newline+1:], nil
}

// splitChecklistOriginLine peels the optional origin line off a task body and
// returns the body without it, so the caller hands its own block reader exactly
// what that reader was written for.
//
// Reading is strict in both directions: an unterminated line, a JSON object with
// a field the model does not declare, and a value that is not the canonical
// encoding of what it decodes to are each an error rather than a best guess,
// because every accepted shape is one insertChecklistOriginLine can produce.
func splitChecklistOriginLine(schema, title, body string) (*models.ChecklistOrigin, string, error) {
	lines := strings.SplitN(body, "\n", 3)
	if len(lines) < 2 || !strings.HasPrefix(lines[1], checklistOriginPrefix) {
		return nil, body, nil
	}
	if !strings.HasSuffix(lines[1], checklistOriginSuffix) {
		return nil, "", fmt.Errorf("%s task %q has malformed checklist origin", schema, title)
	}
	value := strings.TrimSuffix(strings.TrimPrefix(lines[1], checklistOriginPrefix), checklistOriginSuffix)
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	var parsed models.ChecklistOrigin
	if err := decoder.Decode(&parsed); err != nil {
		return nil, "", fmt.Errorf("%s task %q checklist origin: %w", schema, title, err)
	}
	canonical, _ := json.Marshal(parsed)
	if value != string(canonical) {
		return nil, "", fmt.Errorf("%s task %q has noncanonical checklist origin", schema, title)
	}
	remainder := lines[0] + "\n"
	if len(lines) == 3 {
		remainder += lines[2]
	}
	return &parsed, remainder, nil
}

// validateChecklistOrigin holds the origin to the one shape a promoted item can
// have come from. It is shared for the same reason the line grammar is: a
// schema that admitted an origin another schema refuses would not be reading the
// same record.
func validateChecklistOrigin(origin *models.ChecklistOrigin) error {
	if origin == nil {
		return nil
	}
	if models.ValidateTaskReference(origin.ProjectID, origin.TaskID) != nil || origin.Index < 0 {
		return fmt.Errorf("invalid checklist origin")
	}
	return nil
}
