package cmd

import (
	"encoding/json"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskOutputStructsExposeOneLifecycleField(t *testing.T) {
	outputs := map[string]any{
		"list":     taskJSONEntry{State: models.StateWaiting.String()},
		"show":     taskShowJSONEntry{State: models.StateWaiting.String()},
		"mutation": mutateTaskJSONEntry{State: models.StateWaiting.String()},
		"archive":  archiveSummaryJSONEntry{State: models.StateWaiting.String()},
	}
	for name, output := range outputs {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(output)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if got := string(fields["state"]); got != `"waiting"` {
				t.Fatalf("state = %s, want waiting", got)
			}
			for _, legacy := range []string{"status", "section", "stage", "queued"} {
				if _, ok := fields[legacy]; ok {
					t.Fatalf("state-only output unexpectedly contains %q: %s", legacy, data)
				}
			}
		})
	}
}
