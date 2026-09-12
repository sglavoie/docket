package format

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// V17TaskRecord adds the optional origin of a promoted checklist item.
type V17TaskRecord struct {
	V16TaskRecord
	ChecklistOrigin *models.ChecklistOrigin
}

func ValidateV17TaskRecord(task V17TaskRecord) error {
	if err := ValidateV16TaskRecord(task.V16TaskRecord); err != nil {
		return err
	}
	if o := task.ChecklistOrigin; o != nil {
		if models.ValidateTaskReference(o.ProjectID, o.TaskID) != nil || o.Index < 0 {
			return fmt.Errorf("invalid checklist origin")
		}
	}
	return nil
}

func WriteV17TaskBody(task V17TaskRecord) (string, error) {
	if err := ValidateV17TaskRecord(task); err != nil {
		return "", err
	}
	body, err := WriteV16TaskBody(task.V16TaskRecord)
	if err != nil || task.ChecklistOrigin == nil {
		return body, err
	}
	encoded, err := json.Marshal(task.ChecklistOrigin)
	if err != nil {
		return "", err
	}
	newline := strings.IndexByte(body, '\n')
	return body[:newline+1] + "<!-- checklist-origin: " + string(encoded) + " -->\n" + body[newline+1:], nil
}

func ParseV17TaskBody(title, body string) (V17TaskRecord, error) {
	lines := strings.SplitN(body, "\n", 3)
	var origin *models.ChecklistOrigin
	if len(lines) >= 2 && strings.HasPrefix(lines[1], "<!-- checklist-origin: ") {
		if !strings.HasSuffix(lines[1], " -->") {
			return V17TaskRecord{}, fmt.Errorf("v17 task %q has malformed checklist origin", title)
		}
		value := strings.TrimSuffix(strings.TrimPrefix(lines[1], "<!-- checklist-origin: "), " -->")
		decoder := json.NewDecoder(strings.NewReader(value))
		decoder.DisallowUnknownFields()
		var parsed models.ChecklistOrigin
		if err := decoder.Decode(&parsed); err != nil {
			return V17TaskRecord{}, fmt.Errorf("v17 task %q checklist origin: %w", title, err)
		}
		canonical, _ := json.Marshal(parsed)
		if value != string(canonical) {
			return V17TaskRecord{}, fmt.Errorf("v17 task %q has noncanonical checklist origin", title)
		}
		origin = &parsed
		body = lines[0] + "\n"
		if len(lines) == 3 {
			body += lines[2]
		}
	}
	base, err := ParseV16TaskBody(title, body)
	if err != nil {
		return V17TaskRecord{}, err
	}
	task := V17TaskRecord{V16TaskRecord: base, ChecklistOrigin: origin}
	return task, ValidateV17TaskRecord(task)
}
