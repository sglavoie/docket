package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// unwrapMetadata strips the leading "<!-- " and trailing " -->" from a rendered
// metadata comment, yielding the payload ParseMetadata consumes.
func unwrapMetadata(t *testing.T, meta string) string {
	t.Helper()
	if !strings.HasPrefix(meta, "<!-- ") || !strings.HasSuffix(meta, " -->") {
		t.Fatalf("metadata not wrapped as expected: %q", meta)
	}
	return meta[len("<!-- ") : len(meta)-len(" -->")]
}

// TestWriteMetadataV2_EmitsStateNotLegacyFields verifies the v2 writer replaces
// the legacy lifecycle fields with the canonical `state` field.
func TestWriteMetadataV2_EmitsStateNotLegacyFields(t *testing.T) {
	task := &models.Task{
		ID:            "state-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateWaiting,
		Priority:      2,
	}

	meta := WriteMetadataV2(task)

	if !strings.Contains(meta, "state: waiting") {
		t.Errorf("v2 metadata missing canonical state: %q", meta)
	}
	for _, legacy := range []string{"status:", "section:", "stage:", "active:"} {
		if strings.Contains(meta, legacy) {
			t.Errorf("v2 metadata still contains legacy field %q: %q", legacy, meta)
		}
	}
}

// TestStateCodecRoundTrip verifies that every canonical state written by the v2
// writer parses back to a task whose derived state matches.
func TestStateCodecRoundTrip(t *testing.T) {
	for _, state := range models.AllWorkflowStates() {
		task := &models.Task{ID: "rt", Type: models.TaskTypeTask, Priority: 1}
		task.ApplyWorkflowState(state)

		meta := WriteMetadataV2(task)
		parsed := &models.Task{}
		warnings := ParseMetadata(parsed, unwrapMetadata(t, meta))

		if len(warnings) != 0 {
			t.Errorf("state %q: unexpected warnings %v", state, warnings)
		}
		if got := parsed.State(); got != state {
			t.Errorf("state %q round-tripped to %q", state, got)
		}
	}
}

// TestStateCodecMalformedWarns verifies an unrecognized state value is reported
// as a warning and does not mutate the task's lifecycle fields.
func TestStateCodecMalformedWarns(t *testing.T) {
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, "id: bad, state: nonsense, priority: 1")

	if !hasWarningField(warnings, "state") {
		t.Errorf("expected a warning with Field=state, got %v", warnings)
	}
	// An unrecognized state value is ignored: the parser records a warning and
	// the task falls back to the default derived state rather than adopting the
	// malformed value.
	if parsed.State() != models.StateReady {
		t.Errorf("malformed state set State to %q, want fallback %q", parsed.State(), models.StateReady)
	}
}
