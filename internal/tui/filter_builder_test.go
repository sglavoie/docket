package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestFilterBuilderSupportsParkedCriterion(t *testing.T) {
	fieldIdx := smartListFieldIndex("parked")
	if fieldIdx < 0 {
		t.Fatal("smart list fields should include parked")
	}
	field := smartListFields[fieldIdx]
	if field.ValueType != "enum" || len(field.EnumValues) != 2 || field.EnumValues[0] != "true" || field.EnumValues[1] != "false" {
		t.Fatalf("parked field = %+v, want boolean true/false enum", field)
	}

	builder := NewFilterBuilderModel(nil)
	builder.nameInput.SetValue("Parked")
	builder.criteria[0].fieldCursor = fieldIdx
	builder.criteria[0].opCursor = 0
	builder.criteria[0].valueCursor = 1

	filter := builder.buildSavedFilter()
	if len(filter.Criteria) != 1 {
		t.Fatalf("criteria len = %d, want 1", len(filter.Criteria))
	}
	criterion := filter.Criteria[0]
	if criterion.Field != "parked" || criterion.Operation != "equals" || criterion.Value != "false" {
		t.Fatalf("criterion = %+v, want parked equals false", criterion)
	}
}

func TestFilterBuilderSupportsCanonicalStateSets(t *testing.T) {
	fieldIdx := smartListFieldIndex("state")
	if fieldIdx < 0 {
		t.Fatal("smart list fields should include state")
	}
	if smartListFieldIndex("status") >= 0 || smartListFieldIndex("section") >= 0 {
		t.Fatal("smart list fields should not expose legacy lifecycle criteria")
	}

	builder := NewFilterBuilderModel(nil)
	builder.nameInput.SetValue("Planning states")
	builder.criteria[0].fieldCursor = fieldIdx
	builder.criteria[0].opCursor = 0
	builder.criteria[0].valueInput.SetValue("waiting, inbox, next")

	filter := builder.buildSavedFilter()
	criterion := filter.Criteria[0]
	want := testWorkflowStateSet(models.StateInbox, models.StateNext, models.StateWaiting)
	if criterion.Field != "state" || criterion.Operation != "in" || !criterion.States.Equal(want) {
		t.Fatalf("criterion = field %q op %q states %v, want canonical state set %v", criterion.Field, criterion.Operation, criterion.States.Values(), want.Values())
	}

	editor := NewFilterEditorModel(nil, &filter)
	got := editor.criteria[0].valueInput.Value()
	for _, state := range []string{"inbox", "next", "waiting"} {
		if !strings.Contains(got, state) {
			t.Fatalf("editor state input = %q, missing %q", got, state)
		}
	}
}

func TestFilterEditorLoadsParkedCriterion(t *testing.T) {
	filter := &config.SavedFilter{
		ID:   "parked",
		Name: "Parked",
		Mode: "criteria",
		Criteria: []config.SmartListCriterion{
			{Field: "parked", Operation: "equals", Value: "true"},
		},
	}

	builder := NewFilterEditorModel(nil, filter)
	if got := builder.currentField(0).Name; got != "parked" {
		t.Fatalf("currentField = %q, want parked", got)
	}
	field := builder.currentField(0)
	if builder.criteria[0].valueCursor >= len(field.EnumValues) || field.EnumValues[builder.criteria[0].valueCursor] != "true" {
		t.Fatalf("value cursor = %d on %+v, want true", builder.criteria[0].valueCursor, field)
	}
}

func smartListFieldIndex(name string) int {
	for i, field := range smartListFields {
		if field.Name == name {
			return i
		}
	}
	return -1
}
