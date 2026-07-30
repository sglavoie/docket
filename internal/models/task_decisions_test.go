package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAddDecisionAtCreatesLeanTimestampedRecord(t *testing.T) {
	task := &Task{}
	now := time.Date(2026, 7, 28, 13, 45, 0, 0, time.UTC)

	task.AddDecisionAt("Use PostgreSQL", "Better for complex queries", now)

	if len(task.Decisions) != 1 {
		t.Fatalf("decisions = %d, want 1", len(task.Decisions))
	}
	decision := task.Decisions[0]
	if decision.Text != "Use PostgreSQL" || decision.Rationale != "Better for complex queries" {
		t.Fatalf("decision = %+v", decision)
	}
	if !decision.CreatedAt.Equal(now) || !task.UpdatedAt.Equal(now) {
		t.Errorf("timestamps = decision %v task %v, want %v", decision.CreatedAt, task.UpdatedAt, now)
	}

	encoded, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "category") || strings.Contains(string(encoded), "resolved") {
		t.Fatalf("canonical decision JSON contains retired fields: %s", encoded)
	}
}

func TestAddDecisionRejectsWhitespace(t *testing.T) {
	for _, text := range []string{"", "   ", "\t\n"} {
		task := &Task{}
		task.AddDecision(text, "rationale")
		if len(task.Decisions) != 0 {
			t.Fatalf("AddDecision(%q) created %d decisions", text, len(task.Decisions))
		}
	}
}

func TestRemoveDecision(t *testing.T) {
	task := &Task{}
	task.AddDecision("First", "")
	task.AddDecision("Second", "")

	if !task.RemoveDecision(0) {
		t.Fatal("RemoveDecision(0) returned false")
	}
	if len(task.Decisions) != 1 || task.Decisions[0].Text != "Second" {
		t.Fatalf("remaining decisions = %+v", task.Decisions)
	}
	if task.RemoveDecision(-1) || task.RemoveDecision(1) {
		t.Fatal("out-of-range removal succeeded")
	}
}

func TestClearDecisions(t *testing.T) {
	task := &Task{}
	task.AddDecision("First", "")
	task.AddDecision("Second", "")
	task.ClearDecisions()
	if len(task.Decisions) != 0 {
		t.Fatalf("decisions after clear = %d, want 0", len(task.Decisions))
	}
}
