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
		task := &models.Task{ID: "rt", Priority: 1}
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

func TestRetiredStageMetadataRemainsReadOnlyMigrationInput(t *testing.T) {
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, "id: legacy-review, status: todo, stage: Review, priority: 2")
	if len(warnings) != 1 || warnings[0].Type != WarningDeprecation {
		t.Fatalf("legacy stage metadata warnings = %v, want one deprecation", warnings)
	}
	if got := parsed.State(); got != models.StateReady {
		t.Fatalf("legacy review stage migrated to %q, want %q", got, models.StateReady)
	}
	if !parsed.HasTag(models.LegacyReviewTag) {
		t.Fatalf("legacy review stage lost review tag: %v", parsed.Tags)
	}

	written := WriteMetadataV2(parsed)
	if strings.Contains(written, "stage:") {
		t.Fatalf("current metadata writer resurrected retired stage field: %q", written)
	}
	if !strings.Contains(written, "state: ready") || !strings.Contains(written, "tags: review") {
		t.Fatalf("current metadata writer lost normalized review meaning: %q", written)
	}
}

func TestRetiredV2ReviewStateNormalizesOnReadAndWrite(t *testing.T) {
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, "id: legacy-review, state: review, tags: existing, priority: 2")
	if len(warnings) != 1 || warnings[0].Type != WarningDeprecation {
		t.Fatalf("state: review warnings = %v, want one deprecation", warnings)
	}
	if parsed.State() != models.StateReady || !parsed.HasTag("existing") || !parsed.HasTag(models.LegacyReviewTag) {
		t.Fatalf("normalized task = state %q tags %v", parsed.State(), parsed.Tags)
	}

	directLegacy := &models.Task{
		ID:            "direct-review",
		WorkflowState: models.StateReview,
		Priority:      2,
		Tags:          []string{"existing"},
	}
	written := WriteMetadataV2(directLegacy)
	if !strings.Contains(written, "state: ready") || !strings.Contains(written, "tags: existing;review") {
		t.Fatalf("direct legacy output was not normalized: %q", written)
	}
	if directLegacy.State() != models.StateReview || directLegacy.HasTag(models.LegacyReviewTag) {
		t.Fatalf("writer mutated caller: state %q tags %v", directLegacy.State(), directLegacy.Tags)
	}
}
