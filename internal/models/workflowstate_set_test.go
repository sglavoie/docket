package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWorkflowStateSetCanonicalOrderingAndMembership(t *testing.T) {
	set, err := NewWorkflowStateSet(
		StateCanceled,
		StateNext,
		StateInbox,
		StateNext,
		StateInProgress,
	)
	if err != nil {
		t.Fatalf("NewWorkflowStateSet: %v", err)
	}

	want := []WorkflowState{StateInbox, StateNext, StateInProgress, StateCanceled}
	if got := set.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Values() = %v, want %v", got, want)
	}
	if set.Len() != len(want) {
		t.Errorf("Len() = %d, want %d", set.Len(), len(want))
	}
	for _, state := range want {
		if !set.Contains(state) {
			t.Errorf("Contains(%q) = false, want true", state)
		}
	}
	if set.Contains(StateDone) {
		t.Error("Contains(done) = true, want false")
	}
}

func TestWorkflowStateSetRejectsInvalidState(t *testing.T) {
	if _, err := NewWorkflowStateSet(StateInbox, WorkflowState("todo")); err == nil {
		t.Fatal("NewWorkflowStateSet accepted invalid state")
	}
}

func TestWorkflowStateSetZeroValue(t *testing.T) {
	var set WorkflowStateSet
	if !set.IsEmpty() || set.Len() != 0 {
		t.Fatalf("zero value = len %d empty %v, want empty", set.Len(), set.IsEmpty())
	}
	if got := set.Values(); got == nil || len(got) != 0 {
		t.Fatalf("zero Values() = %#v, want non-nil empty slice", got)
	}
	if !set.Equal(WorkflowStateSet{}) {
		t.Error("zero-value sets should be equal")
	}
}

func TestWorkflowStateSetAlgebra(t *testing.T) {
	left, err := NewWorkflowStateSet(StateDone, StateInbox, StateNext)
	if err != nil {
		t.Fatalf("left set: %v", err)
	}
	right, err := NewWorkflowStateSet(StateWaiting, StateNext, StateDone)
	if err != nil {
		t.Fatalf("right set: %v", err)
	}

	intersection, err := NewWorkflowStateSet(StateNext, StateDone)
	if err != nil {
		t.Fatalf("intersection set: %v", err)
	}
	union, err := NewWorkflowStateSet(StateInbox, StateNext, StateWaiting, StateDone)
	if err != nil {
		t.Fatalf("union set: %v", err)
	}

	if !left.Intersects(right) {
		t.Error("Intersects() = false, want true")
	}
	if got := left.Intersection(right); !got.Equal(intersection) {
		t.Errorf("Intersection() = %v, want %v", got.Values(), intersection.Values())
	}
	if got := left.Union(right); !got.Equal(union) {
		t.Errorf("Union() = %v, want %v", got.Values(), union.Values())
	}
	if left.Intersection(WorkflowStateSet{}).Len() != 0 {
		t.Error("intersection with empty set should be empty")
	}
	if !left.Union(WorkflowStateSet{}).Equal(left) {
		t.Error("union with empty set should preserve the original set")
	}
}

func TestAllWorkflowStateSet(t *testing.T) {
	set := AllWorkflowStateSet()
	if got, want := set.Values(), AllWorkflowStates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("AllWorkflowStateSet().Values() = %v, want %v", got, want)
	}
}

func TestWorkflowStateSetJSONRoundTripIsCanonical(t *testing.T) {
	var set WorkflowStateSet
	if err := json.Unmarshal([]byte(`["done","inbox","done","waiting"]`), &set); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got, want := set.Values(), []WorkflowState{StateInbox, StateWaiting, StateDone}; !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded Values() = %v, want %v", got, want)
	}

	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(data), `["inbox","waiting","done"]`; got != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestWorkflowStateSetJSONRejectsInvalidWithoutMutation(t *testing.T) {
	set, err := NewWorkflowStateSet(StateNext)
	if err != nil {
		t.Fatalf("NewWorkflowStateSet: %v", err)
	}
	before := set

	if err := json.Unmarshal([]byte(`["inbox","todo"]`), &set); err == nil {
		t.Fatal("Unmarshal accepted invalid state")
	}
	if !set.Equal(before) {
		t.Fatalf("invalid JSON mutated set to %v, want %v", set.Values(), before.Values())
	}
}

func TestWorkflowStateSetJSONNullAndZeroMarshalAsEmpty(t *testing.T) {
	set, err := NewWorkflowStateSet(StateDone)
	if err != nil {
		t.Fatalf("NewWorkflowStateSet: %v", err)
	}
	if err := json.Unmarshal([]byte(`null`), &set); err != nil {
		t.Fatalf("Unmarshal(null): %v", err)
	}
	if !set.IsEmpty() {
		t.Fatalf("Unmarshal(null) = %v, want empty", set.Values())
	}

	data, err := json.Marshal(WorkflowStateSet{})
	if err != nil {
		t.Fatalf("Marshal(zero): %v", err)
	}
	if string(data) != `[]` {
		t.Fatalf("Marshal(zero) = %s, want []", data)
	}
}
