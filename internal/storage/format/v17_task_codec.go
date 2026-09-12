package format

import (
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
	return validateChecklistOrigin(task.ChecklistOrigin)
}

func WriteV17TaskBody(task V17TaskRecord) (string, error) {
	if err := ValidateV17TaskRecord(task); err != nil {
		return "", err
	}
	body, err := WriteV16TaskBody(task.V16TaskRecord)
	if err != nil {
		return "", err
	}
	return insertChecklistOriginLine(body, task.ChecklistOrigin)
}

func ParseV17TaskBody(title, body string) (V17TaskRecord, error) {
	origin, remainder, err := splitChecklistOriginLine("v17", title, body)
	if err != nil {
		return V17TaskRecord{}, err
	}
	base, err := ParseV16TaskBody(title, remainder)
	if err != nil {
		return V17TaskRecord{}, err
	}
	task := V17TaskRecord{V16TaskRecord: base, ChecklistOrigin: origin}
	return task, ValidateV17TaskRecord(task)
}
