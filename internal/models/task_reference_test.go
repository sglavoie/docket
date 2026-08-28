package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestTaskReferenceLocatorIsTheDurableSpelling(t *testing.T) {
	reference := TaskReference{ProjectID: "project-alpha", TaskID: "task-42"}
	if locator := reference.Locator(); locator != "project-alpha:task-42" {
		t.Fatalf("Locator() = %q, want %q", locator, "project-alpha:task-42")
	}
	if err := reference.Validate(); err != nil {
		t.Fatalf("a well-formed reference was refused: %v", err)
	}
}

// The reserved runes are the ones that would make a persisted record
// unparseable rather than merely unusual: the metadata pair separator, the
// dependency list separator, the locator separator itself, and the sequence that
// would close the metadata comment the list rides inside.
func TestTaskReferenceValidationRefusesUnpersistableComponents(t *testing.T) {
	for _, pair := range [][2]string{
		{"", "task-42"},             // empty project id
		{"project-alpha", ""},       // empty task id
		{"project-alpha", "a:b"},    // the locator separator
		{"project alpha", "task-1"}, // whitespace inside a component
		{"project;alpha", "task-1"}, // the durable list delimiter
		{"project,alpha", "task-1"}, // the metadata pair delimiter
		{"project-->", "task-1"},    // would close the metadata comment
	} {
		if err := ValidateTaskReference(pair[0], pair[1]); err == nil {
			t.Fatalf("ValidateTaskReference(%q, %q) succeeded, want refusal", pair[0], pair[1])
		}
	}
}

func TestTaskReferenceEqualityIgnoresNothingButIdentity(t *testing.T) {
	base := TaskReference{ProjectID: "p", TaskID: "t"}
	if !base.Equal(TaskReference{ProjectID: "p", TaskID: "t"}) {
		t.Fatal("identical references are not equal")
	}
	if base.Equal(TaskReference{ProjectID: "p", TaskID: "other"}) {
		t.Fatal("different task ids compared equal")
	}
	if base.Equal(TaskReference{ProjectID: "other", TaskID: "t"}) {
		t.Fatal("different project ids compared equal")
	}
	if base.IsZero() || !(TaskReference{}).IsZero() {
		t.Fatal("IsZero does not report the empty reference")
	}
}

func TestCloneTaskReferencesDetachesBackingArray(t *testing.T) {
	source := []TaskReference{{ProjectID: "p", TaskID: "a"}, {ProjectID: "p", TaskID: "b"}}
	cloned := CloneTaskReferences(source)
	if !TaskReferencesEqual(cloned, source) {
		t.Fatalf("clone = %+v, want %+v", cloned, source)
	}
	cloned[0].TaskID = "mutated"
	if source[0].TaskID != "a" {
		t.Fatal("mutating the clone reached the source slice")
	}
	if CloneTaskReferences(nil) != nil || CloneTaskReferences([]TaskReference{}) != nil {
		t.Fatal("an empty list must clone to nil so absence has one shape")
	}
}

// TestTaskDependenciesAreOrderedAndPersisted pins the two properties the whole
// feature rests on: the field is persisted (it carries a json name, so the
// project merger's reflective inventory admits it) and the list preserves
// insertion order rather than sorting.
func TestTaskDependenciesAreOrderedAndPersisted(t *testing.T) {
	field, found := reflect.TypeOf(Task{}).FieldByName("Dependencies")
	if !found {
		t.Fatal("Task has no Dependencies field")
	}
	if name := strings.Split(field.Tag.Get("json"), ",")[0]; name != "dependencies" {
		t.Fatalf("Dependencies json name = %q, want %q", name, "dependencies")
	}
	task := &Task{Dependencies: []TaskReference{{ProjectID: "p", TaskID: "z"}, {ProjectID: "p", TaskID: "a"}}}
	if task.Dependencies[0].TaskID != "z" || task.Dependencies[1].TaskID != "a" {
		t.Fatal("dependency order was not preserved")
	}
}
