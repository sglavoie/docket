package models

import (
	"strings"
	"time"
)

// AddDecision adds a new timestamped decision to the task. Decisions with
// empty/whitespace-only text are silently dropped because they cannot be
// round-tripped through the markdown decisions block (the parser uses an
// empty text to detect the end of a decision and would lose the entry).
func (t *Task) AddDecision(text, rationale string) {
	t.AddDecisionAt(text, rationale, time.Now())
}

// AddDecisionAt adds a decision using the caller-owned mutation timestamp.
func (t *Task) AddDecisionAt(text, rationale string, now time.Time) {
	if strings.TrimSpace(text) == "" {
		return
	}
	decision := &Decision{
		Text:      text,
		Rationale: rationale,
		CreatedAt: now,
	}
	t.Decisions = append(t.Decisions, decision)
	t.UpdatedAt = now
}

// RemoveDecision removes the decision at the given index.
// Returns true if a decision was removed.
func (t *Task) RemoveDecision(index int) bool {
	if index < 0 || index >= len(t.Decisions) {
		return false
	}
	t.Decisions = append(t.Decisions[:index], t.Decisions[index+1:]...)
	t.UpdatedAt = time.Now()
	return true
}

// ClearDecisions removes all decisions from the task.
func (t *Task) ClearDecisions() {
	if len(t.Decisions) > 0 {
		t.Decisions = []*Decision{}
		t.UpdatedAt = time.Now()
	}
}
