package cmd

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProjectShow(t *testing.T) {
	t.Run("runs without error for current project", func(t *testing.T) {
		setupProjectTest(t)

		err := runProjectShow(projectShowCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("archived project with nil ArchivedAt does not panic", func(t *testing.T) {
		deps, project := setupProjectTest(t)
		// Simulate legacy archived project with nil ArchivedAt
		project.Archived = true
		project.ArchivedAt = nil
		deps.Config.Machine().CurrentProject = project.ID

		// This should not panic
		err := runProjectShow(projectShowCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestProjectShowJSON_UsesCanonicalProjectFields(t *testing.T) {
	pinnedAt := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	_, project := setupProjectTest(t)
	project.Pinned = true
	project.PinnedAt = &pinnedAt
	project.StatusIcon = "rocket"
	project.StatusIconSource = "user"

	stdout, err := captureArchiveAccessStdout(t, func() error {
		return displayProjectShowJSON(project)
	})
	if err != nil {
		t.Fatalf("displayProjectShowJSON() error = %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["pinned_at"]; !ok {
		t.Error("pinned_at field missing from project show JSON")
	}
	if v, ok := parsed["status_icon_source"]; !ok || v != "user" {
		t.Errorf("expected status_icon_source=user, got %v", v)
	}
	for _, field := range []string{"sections", "section_colors"} {
		if _, ok := parsed[field]; ok {
			t.Errorf("did not expect retired %s field in project show JSON", field)
		}
	}
}
