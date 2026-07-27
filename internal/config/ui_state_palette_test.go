package config

import (
	"fmt"
	"testing"
)

func TestGetRecentPaletteItems_Empty(t *testing.T) {
	cfg := NewMachineState()

	items := cfg.GetRecentPaletteItems()

	if items == nil {
		t.Error("GetRecentPaletteItems should return empty slice, not nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestSetRecentPaletteItems(t *testing.T) {
	cfg := NewMachineState()

	items := []RecentPaletteItem{
		{Category: "Commands", ID: "task-add"},
		{Category: "Projects", ID: "my-project"},
	}
	cfg.SetRecentPaletteItems(items)

	result := cfg.GetRecentPaletteItems()
	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
	if result[0].Category != "Commands" || result[0].ID != "task-add" {
		t.Error("first item mismatch")
	}
}

func TestRecentDestinationQualifiedIdentity(t *testing.T) {
	tests := []struct {
		name        string
		destination RecentDestination
		want        string
	}{
		{
			name:        "task",
			destination: RecentDestination{Kind: RecentDestinationTask, ProjectID: "project", ID: "task"},
			want:        "task:project:task",
		},
		{
			name:        "project note",
			destination: RecentDestination{Kind: RecentDestinationNote, ProjectID: "project", ID: "note.md"},
			want:        "note:project:note.md",
		},
		{
			name:        "global note",
			destination: RecentDestination{Kind: RecentDestinationNote, ID: "note.md"},
			want:        "note:global:note.md",
		},
		{
			name:        "project",
			destination: RecentDestination{Kind: RecentDestinationProject, ID: "project"},
			want:        "project:project",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.destination.QualifiedID(); got != test.want {
				t.Fatalf("QualifiedID() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMachineStateRecentDestinationsNormalizeRecordAndBound(t *testing.T) {
	machine := NewMachineState()
	destinations := []RecentDestination{
		{Kind: RecentDestinationTask, ProjectID: "alpha", ID: "shared"},
		{Kind: RecentDestinationTask, ProjectID: "beta", ID: "shared"},
		{Kind: RecentDestinationTask, ProjectID: "alpha", ID: "shared", Count: 9},
		{Kind: RecentDestinationTask, ID: "unqualified"},
	}
	for index := range MaxRecentDestinations {
		destinations = append(destinations, RecentDestination{
			Kind: RecentDestinationTag,
			ID:   fmt.Sprintf("tag-%02d", index),
		})
	}
	machine.SetRecentDestinations(destinations)

	recents := machine.GetRecentDestinations()
	if got := len(recents); got != MaxRecentDestinations {
		t.Fatalf("recents = %d, want %d", got, MaxRecentDestinations)
	}
	if recents[0].QualifiedID() != "task:alpha:shared" ||
		recents[1].QualifiedID() != "task:beta:shared" {
		t.Fatalf("qualified task recents = %+v", recents[:2])
	}
	recents[0].ID = "mutated"
	if machine.RecentDestinations[0].ID != "shared" {
		t.Fatal("GetRecentDestinations returned mutable backing storage")
	}

	if !machine.RecordRecentDestination(RecentDestination{
		Kind:      RecentDestinationTask,
		ProjectID: "beta",
		ID:        "shared",
	}) {
		t.Fatal("RecordRecentDestination returned false")
	}
	if got := machine.RecentDestinations[0]; got.QualifiedID() != "task:beta:shared" || got.Count != 2 {
		t.Fatalf("recorded recent = %+v, want beta task with count 2", got)
	}
	if machine.RecordRecentDestination(RecentDestination{Kind: RecentDestinationTask, ID: "unqualified"}) {
		t.Fatal("unqualified task destination should be rejected")
	}
}
