package models

import (
	"reflect"
	"testing"
)

func TestNormalizeTaskTagsTreatsSemanticSpellingsAsIndependentTags(t *testing.T) {
	got, err := NormalizeTaskTags([]string{"backend", "#type:BUG", "bug", "idea", "bug"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"backend", "type:BUG", "bug", "idea"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %v, want %v", got, want)
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
	if task.HasTag("story") || !task.HasTag("type:story") {
		t.Fatal("live tag lookup canonicalized a legacy marker spelling")
	}
}
