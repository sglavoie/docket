package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestTaskReferenceLocatorRoundTrips(t *testing.T) {
	reference := TaskReference{ProjectID: "project-alpha", TaskID: "task-42"}
	locator := reference.Locator()
	if locator != "project-alpha:task-42" {
		t.Fatalf("Locator() = %q, want %q", locator, "project-alpha:task-42")
	}
	parsed, err := ParseTaskReference(locator)
	if err != nil {
		t.Fatalf("ParseTaskReference: %v", err)
	}
	if !parsed.Equal(reference) {
		t.Fatalf("round trip = %+v, want %+v", parsed, reference)
	}
}

func TestParseTaskReferenceRefusesMalformedLocators(t *testing.T) {
	for _, locator := range []string{
		"",                     // nothing at all
		"project-alpha",        // no separator
		":task-42",             // empty project id
		"project-alpha:",       // empty task id
		"a:b:c",                // extra separator lands inside the task id
		"project alpha:task-1", // whitespace inside a component
		"project;alpha:task-1", // the durable list delimiter
		"project,alpha:task-1", // the metadata pair delimiter
		"project-->:task-1",    // would close the metadata comment
	} {
		if _, err := ParseTaskReference(locator); err == nil {
			t.Fatalf("ParseTaskReference(%q) succeeded, want refusal", locator)
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

func TestDedupeTaskReferencesKeepsFirstOccurrenceOrder(t *testing.T) {
	deduped := DedupeTaskReferences([]TaskReference{
		{ProjectID: "p", TaskID: "b"},
		{ProjectID: "p", TaskID: "a"},
		{ProjectID: "p", TaskID: "b"},
		{ProjectID: "q", TaskID: "a"},
		{ProjectID: "p", TaskID: "a"},
	})
	want := []TaskReference{
		{ProjectID: "p", TaskID: "b"},
		{ProjectID: "p", TaskID: "a"},
		{ProjectID: "q", TaskID: "a"},
	}
	if !TaskReferencesEqual(deduped, want) {
		t.Fatalf("dedupe = %+v, want %+v", deduped, want)
	}
	if DedupeTaskReferences(nil) != nil {
		t.Fatal("an empty list must dedupe to nil")
	}
}

func TestValidateTaskReferencesRefusesDuplicatesAndBadComponents(t *testing.T) {
	duplicate := []TaskReference{{ProjectID: "p", TaskID: "a"}, {ProjectID: "p", TaskID: "a"}}
	err := ValidateTaskReferences(duplicate)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate validation = %v, want a duplicate refusal", err)
	}
	if err := ValidateTaskReferences([]TaskReference{{ProjectID: "", TaskID: "a"}}); err == nil {
		t.Fatal("an empty project id was accepted")
	}
	if err := ValidateTaskReferences(nil); err != nil {
		t.Fatalf("an absent list must validate: %v", err)
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
