package main

import (
	"os"
	"strings"
	"testing"
)

func TestArchiveHotPathsUseSummaryIndex(t *testing.T) {
	files := map[string][]string{
		"cmd/roadmap.go": {
			"storage.LoadArchivedTaskSummaries(",
			"metrics.ComputeAllAtWithWeekStartAndSummaries(",
			"planner.WeightedVelocityWithSummaries(",
		},
		"internal/tui/metrics_dashboard.go": {
			"storage.LoadArchivedTaskSummaries(",
			"metrics.ComputeAllAtWithWeekStartAndSummaries(",
		},
		"internal/tui/roadmap.go": {
			"storage.LoadArchivedTaskSummaries(",
			"metrics.ComputeAllAtWithWeekStartAndSummaries(",
			"planner.WeightedVelocityWithSummaries(",
		},
		"cmd/task_archive_access.go": {
			"storage.LoadArchivedTaskSummaries(",
			"storage.FindArchivedTask(",
		},
	}

	for path, required := range files {
		content := readArchiveGuardFile(t, path)
		if strings.Contains(content, "storage.LoadArchivedTasks(") {
			t.Fatalf("%s reintroduced full archive parsing on an archive-aware hot path", path)
		}
		for _, needle := range required {
			if !strings.Contains(content, needle) {
				t.Fatalf("%s missing expected summary/index call %q", path, needle)
			}
		}
	}
}

func readArchiveGuardFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
