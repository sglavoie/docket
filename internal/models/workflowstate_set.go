package models

import (
	"encoding/json"
	"fmt"
)

// WorkflowStateSet is a validated set of canonical workflow states.
//
// The zero value is an empty set. Values always render in AllWorkflowStates
// order, regardless of insertion or JSON input order, so persisted filters and
// rule definitions remain deterministic.
type WorkflowStateSet struct {
	states map[WorkflowState]struct{}
}

// NewWorkflowStateSet validates states and returns a set with duplicates
// removed. An empty argument list returns the usable zero-value set.
func NewWorkflowStateSet(states ...WorkflowState) (WorkflowStateSet, error) {
	set := WorkflowStateSet{}
	for _, state := range states {
		if !state.IsValid() {
			return WorkflowStateSet{}, fmt.Errorf("invalid workflow state %q", state)
		}
		if set.states == nil {
			set.states = make(map[WorkflowState]struct{}, len(states))
		}
		set.states[state] = struct{}{}
	}
	return set, nil
}

// AllWorkflowStateSet returns a set containing all ten canonical states.
func AllWorkflowStateSet() WorkflowStateSet {
	set, err := NewWorkflowStateSet(AllWorkflowStates()...)
	if err != nil {
		panic(err)
	}
	return set
}

// Len returns the number of states in the set.
func (s WorkflowStateSet) Len() int {
	return len(s.states)
}

// IsEmpty reports whether the set contains no states.
func (s WorkflowStateSet) IsEmpty() bool {
	return s.Len() == 0
}

// Contains reports whether state belongs to the set.
func (s WorkflowStateSet) Contains(state WorkflowState) bool {
	_, ok := s.states[state]
	return ok
}

// Values returns the set members in canonical AllWorkflowStates order.
func (s WorkflowStateSet) Values() []WorkflowState {
	values := make([]WorkflowState, 0, len(s.states))
	for _, state := range AllWorkflowStates() {
		if s.Contains(state) {
			values = append(values, state)
		}
	}
	return values
}

// Equal reports whether s and other contain exactly the same states.
func (s WorkflowStateSet) Equal(other WorkflowStateSet) bool {
	if s.Len() != other.Len() {
		return false
	}
	for state := range s.states {
		if !other.Contains(state) {
			return false
		}
	}
	return true
}

// Intersects reports whether s and other share at least one state.
func (s WorkflowStateSet) Intersects(other WorkflowStateSet) bool {
	if s.Len() > other.Len() {
		s, other = other, s
	}
	for state := range s.states {
		if other.Contains(state) {
			return true
		}
	}
	return false
}

// Intersection returns the states shared by s and other.
func (s WorkflowStateSet) Intersection(other WorkflowStateSet) WorkflowStateSet {
	result := WorkflowStateSet{}
	for _, state := range AllWorkflowStates() {
		if s.Contains(state) && other.Contains(state) {
			if result.states == nil {
				result.states = make(map[WorkflowState]struct{})
			}
			result.states[state] = struct{}{}
		}
	}
	return result
}

// Union returns every state contained by either s or other.
func (s WorkflowStateSet) Union(other WorkflowStateSet) WorkflowStateSet {
	result := WorkflowStateSet{}
	for _, state := range AllWorkflowStates() {
		if s.Contains(state) || other.Contains(state) {
			if result.states == nil {
				result.states = make(map[WorkflowState]struct{})
			}
			result.states[state] = struct{}{}
		}
	}
	return result
}

// MarshalJSON emits a JSON array in canonical state order.
func (s WorkflowStateSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Values())
}

// UnmarshalJSON validates a JSON array of canonical states. Duplicate entries
// are accepted and collapsed according to set semantics. A null value decodes
// as the empty set.
func (s *WorkflowStateSet) UnmarshalJSON(data []byte) error {
	var states []WorkflowState
	if err := json.Unmarshal(data, &states); err != nil {
		return fmt.Errorf("decode workflow state set: %w", err)
	}
	set, err := NewWorkflowStateSet(states...)
	if err != nil {
		return err
	}
	*s = set
	return nil
}
