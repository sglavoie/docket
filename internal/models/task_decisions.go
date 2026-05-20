package models

import (
	"strings"
	"time"
)

// AddDecision adds a new timestamped decision to the task. Decisions with
// empty/whitespace-only text are silently dropped because they cannot be
// round-tripped through the markdown decisions block (the parser uses an
// empty text to detect the end of a decision and would lose the entry).
func (t *Task) AddDecision(text, rationale string, category DecisionCategory) {
	if strings.TrimSpace(text) == "" {
		return
	}
	decision := &Decision{
		Text:      text,
		Rationale: rationale,
		Category:  category,
		CreatedAt: time.Now(),
	}
	t.Decisions = append(t.Decisions, decision)
	t.UpdatedAt = time.Now()
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

// ToggleDecisionResolved flips the Resolved state of the decision at the given index.
// Returns false if the index is out of range.
func (t *Task) ToggleDecisionResolved(index int) bool {
	if index < 0 || index >= len(t.Decisions) {
		return false
	}
	t.Decisions[index].Resolved = !t.Decisions[index].Resolved
	t.UpdatedAt = time.Now()
	return true
}

// UnresolvedDecisionCount returns the number of decisions where Resolved is false.
func (t *Task) UnresolvedDecisionCount() int {
	count := 0
	for _, d := range t.Decisions {
		if !d.Resolved {
			count++
		}
	}
	return count
}
