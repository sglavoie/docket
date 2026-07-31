package models

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTaskTagsCanonicalizesMarkers(t *testing.T) {
	got, err := NormalizeTaskTags([]string{"backend", "type:BUG", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"backend", "bug"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
	}
}

func TestNormalizeTaskTagsRejectsMarkerConflicts(t *testing.T) {
	_, err := NormalizeTaskTags([]string{"bug", "idea"})
	if err == nil || !strings.Contains(err.Error(), "only one type marker") {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestTaskMarkerFromLegacyTag(t *testing.T) {
	task, err := NewGenericTask("t", 1, "Legacy marker")
	if err != nil {
		t.Fatal(err)
	}
	task.Tags = []string{"type:story"}
	marker, ok := task.Marker()
	if !ok || marker != TaskMarkerStory || marker.Icon() != "S" {
		t.Fatalf("marker = %q, %v", marker, ok)
	}
	if !task.HasTag("story") {
		t.Fatal("legacy marker did not match canonical raw tag")
	}
}
