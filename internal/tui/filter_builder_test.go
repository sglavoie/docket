package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
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
