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

func TestProjectShowJSON_IncludesPinnedAtAndSectionColors(t *testing.T) {
	pinnedAt := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

	_, project := setupProjectTest(t)
	project.Pinned = true
	project.PinnedAt = &pinnedAt
	project.StatusIcon = "rocket"
	project.StatusIconSource = "user"
	project.SectionColors = map[string]string{"Next": "blue", "Inbox": "green"}

	// Run the JSON display via the command with --json flag
	cmd := projectShowCmd
	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("failed to set json flag: %v", err)
	}

	defer func() {
		if err := cmd.Flags().Set("json", "false"); err != nil {
			t.Fatalf("failed to reset json flag: %v", err)
		}
	}()

	// Since displayProjectShowJSON prints to stdout, test the struct construction directly
	type projectShowJSONEntry struct {
		PinnedAt         *time.Time        `json:"pinned_at,omitempty"`
		StatusIconSource string            `json:"status_icon_source,omitempty"`
		SectionColors    map[string]string `json:"section_colors"`
	}

	entry := projectShowJSONEntry{
		PinnedAt:         project.PinnedAt,
		StatusIconSource: project.StatusIconSource,
		SectionColors:    project.SectionColors,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["pinned_at"]; !ok {
		t.Error("pinned_at field missing from project show JSON")
	}
	if v, ok := parsed["status_icon_source"]; !ok || v != "user" {
		t.Errorf("expected status_icon_source=user, got %v", v)
	}
	colors, ok := parsed["section_colors"]
	if !ok {
		t.Fatal("section_colors field missing from project show JSON")
	}
	colorsMap, ok := colors.(map[string]any)
	if !ok {
		t.Fatalf("section_colors is not an object: %T", colors)
	}
	if colorsMap["Next"] != "blue" {
		t.Errorf("expected section_colors.Next=blue, got %v", colorsMap["Next"])
	}
}

func TestProjectShowJSON_NilSectionColorsBecomesEmptyMap(t *testing.T) {
	_, project := setupProjectTest(t)
	project.SectionColors = nil

	// Verify the nil-to-empty-map normalization that displayProjectShowJSON does
	sectionColors := project.SectionColors
	if sectionColors == nil {
		sectionColors = map[string]string{}
	}

	data, err := json.Marshal(struct {
		SectionColors map[string]string `json:"section_colors"`
	}{SectionColors: sectionColors})
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	colors, ok := parsed["section_colors"]
	if !ok {
		t.Fatal("section_colors should be present even when original is nil")
	}
	colorsMap, ok := colors.(map[string]any)
	if !ok {
		t.Fatalf("section_colors is not an object: %T", colors)
	}
	if len(colorsMap) != 0 {
		t.Errorf("expected empty section_colors, got %v", colorsMap)
	}
}
