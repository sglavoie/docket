package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestUnsetPriorityRoundTripsAsZero pins the durable half of the unset-priority
// contract: zero is written verbatim and parses back as zero. The parsed task is
// seeded at PriorityDefault the way the storage parser seeds a disk-loaded task,
// so a rejected zero would silently rehydrate as P3.
func TestUnsetPriorityRoundTripsAsZero(t *testing.T) {
	task := &models.Task{
		ID:            "unset-priority",
		WorkflowState: models.StateReady,
		Priority:      models.PriorityUnset,
	}

	metaStr := WriteMetadataV2(task)
	if !strings.Contains(metaStr, "priority: 0") {
		t.Fatalf("metadata %q does not persist the unset priority", metaStr)
	}

	parsed := &models.Task{Priority: models.PriorityDefault}
	warnings := parseWrittenMetadata(task, parsed)
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none for an unset priority", warnings)
	}
	if parsed.Priority != models.PriorityUnset {
		t.Errorf("Priority = %d, want %d (unset)", parsed.Priority, models.PriorityUnset)
	}
}

// TestOffScalePriorityStillWarnsAndKeepsTheDefault proves accepting zero did not
// widen the range: a value outside the scale is still rejected with a warning and
// leaves the task at the default the parser seeded.
func TestOffScalePriorityStillWarnsAndKeepsTheDefault(t *testing.T) {
	for _, metaStr := range []string{"id: t1, state: ready, priority: -1", "id: t1, state: ready, priority: 9"} {
		parsed := &models.Task{Priority: models.PriorityDefault}
		warnings := ParseMetadata(parsed, metaStr)
		if !hasWarningField(warnings, "priority") {
			t.Errorf("%q produced no priority warning", metaStr)
		}
		if parsed.Priority != models.PriorityDefault {
			t.Errorf("%q: Priority = %d, want the seeded default %d", metaStr, parsed.Priority, models.PriorityDefault)
		}
	}
}
