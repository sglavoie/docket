package cmd

import (
	"reflect"
	"testing"
)

func TestRetiredTaskStageCommandSurfaceStaysRemoved(t *testing.T) {
	supported := map[string]bool{"state": false, "ready": false, "next": false}
	for _, child := range taskCmd.Commands() {
		if child.Name() == "stage" {
			t.Fatal("task command still exposes retired stage subcommand")
		}
		for _, alias := range child.Aliases {
			if alias == "stage" {
				t.Fatalf("task command %q still exposes retired stage alias", child.Name())
			}
		}
		if _, ok := supported[child.Name()]; ok {
			supported[child.Name()] = true
		}
	}
	for name, found := range supported {
		if !found {
			t.Errorf("task command is missing canonical workflow-state command %q", name)
		}
	}

	if flag := taskAddCmd.Flags().Lookup("stage"); flag != nil {
		t.Fatalf("task add still exposes retired --stage flag: %+v", flag)
	}

	// Backward-readable JSON keeps the optional field even though no command can
	// mutate it and current task output leaves it empty.
	for name, typ := range map[string]reflect.Type{
		"task list": reflect.TypeOf(taskJSONEntry{}),
		"task show": reflect.TypeOf(taskShowJSONEntry{}),
	} {
		field, ok := typ.FieldByName("Stage")
		if !ok || field.Tag.Get("json") != "stage,omitempty" {
			t.Errorf("%s JSON lost legacy stage compatibility field", name)
		}
	}
}
