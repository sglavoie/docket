package models

import (
	"fmt"
	"strings"
)

// TaskReferenceSeparator joins the two components of a canonical task locator.
// It is the same rune in the durable encoding and in every in-memory locator, so
// a value that round-trips through storage is byte-identical to one built here.
const TaskReferenceSeparator = ":"

// taskReferenceReservedRunes are the runes a locator component may never carry.
// The durable encoding rides inside one task metadata comment: metadata pairs
// split on ", ", the dependency list splits on ";", a locator splits on ":", and
// the comment itself ends at "-->". A component holding any of those would make
// the record unparseable rather than merely unusual, so the domain refuses it
// before storage ever sees it.
const taskReferenceReservedRunes = " \t\r\n:;,|<>"

// TaskReference is a stable, project-qualified locator for exactly one task.
// Both components are durable identities rather than display values: renaming a
// project or retitling a task never rewrites a reference, and a task that moves
// between projects is relocated by rewriting ProjectID alone.
type TaskReference struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

// ValidateTaskReference reports whether a project/task identity pair can be
// persisted as a locator. It takes strings so the storage codecs can validate
// their own record shape without first building a domain value.
func ValidateTaskReference(projectID, taskID string) error {
	if err := validateTaskReferenceComponent("project id", projectID); err != nil {
		return err
	}
	return validateTaskReferenceComponent("task id", taskID)
}

func validateTaskReferenceComponent(label, value string) error {
	if value == "" {
		return fmt.Errorf("task reference %s cannot be empty", label)
	}
	if strings.ContainsAny(value, taskReferenceReservedRunes) {
		return fmt.Errorf("task reference %s %q contains a reserved character", label, value)
	}
	return nil
}

// Validate reports whether this reference is a persistable locator.
func (r TaskReference) Validate() error {
	return ValidateTaskReference(r.ProjectID, r.TaskID)
}

// IsZero reports whether neither component is set. A partially filled reference
// is not zero; it is invalid, and Validate is what says so.
func (r TaskReference) IsZero() bool {
	return r.ProjectID == "" && r.TaskID == ""
}

// Equal reports identity equality. References are compared by their durable
// components alone, which is what makes deduplication stable across a rename.
func (r TaskReference) Equal(other TaskReference) bool {
	return r.ProjectID == other.ProjectID && r.TaskID == other.TaskID
}

// Locator renders the canonical "project-id:task-id" form.
func (r TaskReference) Locator() string {
	return r.ProjectID + TaskReferenceSeparator + r.TaskID
}

// CloneTaskReferences returns a detached copy of a reference list. An empty
// input yields nil rather than an empty slice, so an absent dependency list has
// exactly one in-memory shape and reflective field comparison (the project
// merger's equality) never reports a spurious change.
func CloneTaskReferences(references []TaskReference) []TaskReference {
	if len(references) == 0 {
		return nil
	}
	cloned := make([]TaskReference, len(references))
	copy(cloned, references)
	return cloned
}

// TaskReferencesEqual reports whether two lists carry the same locators in the
// same order.
func TaskReferencesEqual(left, right []TaskReference) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if !left[index].Equal(right[index]) {
			return false
		}
	}
	return true
}
