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

// ParseTaskReference reads one canonical locator. It refuses a missing or extra
// separator so a malformed value can never be silently reinterpreted as a
// different task.
func ParseTaskReference(locator string) (TaskReference, error) {
	projectID, taskID, found := strings.Cut(locator, TaskReferenceSeparator)
	if !found {
		return TaskReference{}, fmt.Errorf("task reference %q is missing the %q separator", locator, TaskReferenceSeparator)
	}
	reference := TaskReference{ProjectID: projectID, TaskID: taskID}
	if err := reference.Validate(); err != nil {
		return TaskReference{}, fmt.Errorf("task reference %q: %w", locator, err)
	}
	return reference, nil
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

// DedupeTaskReferences drops repeated locators while preserving the order in
// which each one was first seen. Insertion order is the stored order, so the
// first occurrence wins and a later duplicate never moves an entry.
func DedupeTaskReferences(references []TaskReference) []TaskReference {
	if len(references) == 0 {
		return nil
	}
	seen := make(map[TaskReference]bool, len(references))
	deduped := make([]TaskReference, 0, len(references))
	for _, reference := range references {
		if seen[reference] {
			continue
		}
		seen[reference] = true
		deduped = append(deduped, reference)
	}
	if len(deduped) == 0 {
		return nil
	}
	return deduped
}

// ValidateTaskReferences validates every locator in a list and refuses a
// duplicate. Self-reference and target existence are workspace-aware questions
// the application layer answers; a single list cannot see either.
func ValidateTaskReferences(references []TaskReference) error {
	seen := make(map[TaskReference]bool, len(references))
	for index, reference := range references {
		if err := reference.Validate(); err != nil {
			return fmt.Errorf("dependency %d: %w", index, err)
		}
		if seen[reference] {
			return fmt.Errorf("dependency %d: duplicate task reference %q", index, reference.Locator())
		}
		seen[reference] = true
	}
	return nil
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
