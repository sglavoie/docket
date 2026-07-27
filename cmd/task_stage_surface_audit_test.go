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

	for name, typ := range map[string]reflect.Type{
		"task list": reflect.TypeOf(taskJSONEntry{}),
		"task show": reflect.TypeOf(taskShowJSONEntry{}),
	} {
		for _, retired := range []string{"Status", "Section", "Stage", "Queued"} {
			if _, ok := typ.FieldByName(retired); ok {
				t.Errorf("%s JSON still exposes retired %s field", name, retired)
			}
		}
	}
}
