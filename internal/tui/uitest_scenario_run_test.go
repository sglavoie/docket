package tui

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- Execution: golden-ish result --------------------------------------------

func TestUITestScenarioRunFixtureGolden(t *testing.T) {
	raw := `{
		"name": "rich Home renders stable content",
		"fixture": "rich",
		"size": "120x40",
		"actions": [
			{"type": "command", "command": "outliner root"},
			{"type": "assert", "assert": "view", "value": "CrossProjectOutliner"},
			{"type": "assert", "assert": "contains", "value": "[rich-project]"},
			{"type": "assert", "assert": "no_layout_issues"}
		]
	}`
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{IncludeObservations: true})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		var screen string
		if len(result.Observations) > 0 {
			screen = result.Observations[len(result.Observations)-1].PlainScreen
		}
		t.Fatalf("scenario should pass, got error: %+v\nlast screen:\n%s", result.Error, screen)
	}
	if result.Scenario != "rich Home renders stable content" {
		t.Fatalf("result scenario name = %q", result.Scenario)
	}
	if result.ActionsRun != 4 {
		t.Fatalf("actions_run = %d, want 4", result.ActionsRun)
	}
	if len(result.Observations) != 4 {
		t.Fatalf("observations = %d, want 4", len(result.Observations))
	}
	if result.FinalObservation == nil {
		t.Fatal("final_observation should be populated")
	}
	if result.FinalObservation.Width != 120 || result.FinalObservation.Height != 40 {
		t.Fatalf("final observation size = %dx%d, want 120x40",
			result.FinalObservation.Width, result.FinalObservation.Height)
	}

	// Golden-ish: the normalized screen carries stable fixture content and no
	// ANSI escapes.
	screen := result.FinalObservation.PlainScreen
	for _, want := range []string{"[rich-project]", "Launch platform"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("final screen missing stable token %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "\x1b") {
		t.Fatal("final screen still contains ANSI escape sequences")
	}
}

func TestUITestScenarioHelpMinimumHeightNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "help minimum height",
		Fixture: "rich",
		Size:    "72x20",
		Actions: []UITestAction{
			{Type: UITestActionCommand, Command: "help"},
			{Type: UITestActionAssert, Assert: "view", Value: "Help"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("help layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioDashboardNarrowLayoutNoOverflow(t *testing.T) {
	// Regression: at narrow widths (<100 cols) the dashboard stacks its info
	// cards vertically. Each card used to be padded to the full section height,
	// so N cards overflowed the terminal. Fixtures with 0 info cards (empty)
	// never regressed, which is why the earlier regression set missed this.
	fixtures := []string{"rich", "narrow", "busy", "workflow"}
	sizes := []string{"80x24", "72x20"}

	for _, fixture := range fixtures {
		for _, size := range sizes {
			t.Run(fixture+"_"+size, func(t *testing.T) {
				sc := UITestScenario{
					Name:    "dashboard narrow layout " + fixture + " " + size,
					Fixture: fixture,
					Size:    size,
					Actions: []UITestAction{
						{Type: UITestActionAssert, Assert: "view", Value: "CrossProjectOutliner"},
						{Type: UITestActionAssert, Assert: "no_layout_issues"},
					},
				}

				result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
				if err != nil {
					t.Fatalf("RunScenarioFixture error: %v", err)
				}
				t.Cleanup(func() { _ = ws.Cleanup() })

				if !result.OK {
					t.Fatalf("dashboard %s at %s should pass, got error: %+v", fixture, size, result.Error)
				}
			})
		}
	}
}

func TestUITestScenarioDashboardSearchBusyMinimumLayoutNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "dashboard search busy minimum layout",
		Fixture: "busy",
		Size:    "60x20",
		Actions: []UITestAction{
			{Type: UITestActionAssert, Assert: "view", Value: "CrossProjectOutliner"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("dashboard search busy layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioMinimumHeightViewLayoutsNoOverflow(t *testing.T) {
	tests := []struct {
		name    string
		prefix  []UITestAction
		command string
		view    string
	}{
		{"project creator", nil, "new project", "ProjectCreator"},
		// rename project now targets the project under the visible Outliner
		// cursor (not the stale current project), so reset to All and land the
		// cursor on a task row before opening the project editor.
		{"project editor", []UITestAction{
			{Type: UITestActionCommand, Command: "outliner root"},
			// Step past the shared section header to land on the first task row;
			// the project group below it draws no heading while it is open.
			{Type: UITestActionKey, Key: "down"},
		}, "rename project", "ProjectEditor"},
		// Project Inbox mode needs a project under the cursor in the global
		// Outliner, so switch to All first and land on a task row.
		{"project Inbox mode", []UITestAction{
			{Type: UITestActionCommand, Command: "outliner root"},
			// Step past the shared section header to land on the first task row;
			// the project group below it draws no heading while it is open.
			{Type: UITestActionKey, Key: "down"},
		}, "project inbox mode", "CrossProjectOutliner"},
		{"global Inbox mode", nil, "global inbox mode", "CrossProjectOutliner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := append([]UITestAction{}, tt.prefix...)
			actions = append(actions,
				UITestAction{Type: UITestActionCommand, Command: tt.command},
				UITestAction{Type: UITestActionAssert, Assert: "view", Value: tt.view},
				UITestAction{Type: UITestActionAssert, Assert: "no_layout_issues"},
			)
			sc := UITestScenario{
				Name:    tt.name + " minimum height",
				Fixture: "rich",
				Size:    "72x20",
				Actions: actions,
			}

			result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
			if err != nil {
				t.Fatalf("RunScenarioFixture error: %v", err)
			}
			t.Cleanup(func() { _ = ws.Cleanup() })

			if !result.OK {
				t.Fatalf("%s layout should pass, got error: %+v", tt.name, result.Error)
			}
		})
	}
}

func TestUITestScenarioOutlinerCommandRoutes(t *testing.T) {
	tests := []struct {
		name    string
		command string
		view    string
	}{
		{"outliner root", "outliner root", "CrossProjectOutliner"},
		{"outliner", "outliner", "CrossProjectOutliner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := UITestScenario{
				Name:    tt.name + " command route",
				Fixture: "rich",
				Size:    "60x20",
				Actions: []UITestAction{
					{Type: UITestActionCommand, Command: tt.command},
					{Type: UITestActionAssert, Assert: "view", Value: tt.view},
					{Type: UITestActionAssert, Assert: "no_layout_issues"},
				},
			}

			result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
			if err != nil {
				t.Fatalf("RunScenarioFixture error: %v", err)
			}
			t.Cleanup(func() { _ = ws.Cleanup() })

			if !result.OK {
				t.Fatalf("%s command route should pass, got error: %+v", tt.name, result.Error)
			}
		})
	}
}

func TestUITestScenarioLongInlineInputsNoOverflow(t *testing.T) {
	longValue := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 4)
	tests := []struct {
		name    string
		size    string
		actions []UITestAction
	}{
		{
			name: "project creator slug preview",
			size: "60x20",
			actions: []UITestAction{
				{Type: UITestActionCommand, Command: "new project"},
				{Type: UITestActionText, Text: longValue},
				{Type: UITestActionAssert, Assert: "view", Value: "ProjectCreator"},
				{Type: UITestActionAssert, Assert: "no_layout_issues"},
			},
		},
		{
			name: "project editor changed slug preview",
			size: "60x35",
			actions: []UITestAction{
				// rename project targets the cursor project, so land the cursor on
				// a task row via all-tasks before opening the editor. The rows lead
				// with a shared section header and the project group below it draws
				// no heading while it is open, so one step reaches the first task.
				{Type: UITestActionCommand, Command: "outliner root"},
				{Type: UITestActionKey, Key: "down"},
				{Type: UITestActionCommand, Command: "rename project"},
				{Type: UITestActionText, Text: longValue},
				{Type: UITestActionAssert, Assert: "view", Value: "ProjectEditor"},
				{Type: UITestActionAssert, Assert: "no_layout_issues"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := UITestScenario{
				Name:    tt.name + " long input layout",
				Fixture: "rich",
				Size:    tt.size,
				Actions: tt.actions,
			}

			result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
			if err != nil {
				t.Fatalf("RunScenarioFixture error: %v", err)
			}
			t.Cleanup(func() { _ = ws.Cleanup() })

			if !result.OK {
				t.Fatalf("%s layout should pass, got error: %+v", tt.name, result.Error)
			}
		})
	}
}

func TestUITestScenarioTaskSubviewLongTitleLayoutsNoOverflow(t *testing.T) {
	views := []struct {
		name    string
		command string
		view    string
	}{
		{"attachments", "task attachments", "TaskAttachments"},
		{"comments", "task comments", "TaskComments"},
	}

	for _, size := range []string{"100x35", "72x20", "60x20"} {
		for _, tt := range views {
			t.Run(size+"_"+tt.name, func(t *testing.T) {
				sc := UITestScenario{
					Name:    "task subview long title layout",
					Fixture: "narrow",
					Size:    size,
					Actions: []UITestAction{
						{Type: UITestActionCommand, Command: "outliner root"},
						// All-tasks rows lead with a shared section header and the
						// project group below it draws no heading while it is open,
						// so one step lands the cursor on the first task before
						// opening its subview.
						{Type: UITestActionKey, Key: "down"},
						{Type: UITestActionCommand, Command: tt.command},
						{Type: UITestActionAssert, Assert: "view", Value: tt.view},
						{Type: UITestActionAssert, Assert: "no_layout_issues"},
					},
				}

				result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
				if err != nil {
					t.Fatalf("RunScenarioFixture error: %v", err)
				}
				t.Cleanup(func() { _ = ws.Cleanup() })

				if !result.OK {
					t.Fatalf("%s at %s should pass, got error: %+v", tt.name, size, result.Error)
				}
			})
		}
	}
}

func TestUITestScenarioRunFixtureDefaults(t *testing.T) {
	// No fixture or size in the scenario: RunScenarioFixture falls back to the
	// package defaults (rich fixture at 100x30).
	sc := UITestScenario{
		Name:    "defaults",
		Actions: []UITestAction{{Type: UITestActionAssert, Assert: "view", Value: "CrossProjectOutliner"}},
	}
	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("scenario should pass, got %+v", result.Error)
	}
	if result.FinalObservation == nil ||
		result.FinalObservation.Width != defaultScenarioWidth ||
		result.FinalObservation.Height != defaultScenarioHeight {
		t.Fatalf("default size not applied: %+v", result.FinalObservation)
	}
}

func TestUITestScenarioRunResultOnFailure(t *testing.T) {
	// First action passes, second fails: the result must name the failing step.
	sc := UITestScenario{
		Name:    "failing assert",
		Fixture: "rich",
		Actions: []UITestAction{
			{Type: UITestActionAssert, Assert: "view", Value: "CrossProjectOutliner"},
			{Type: UITestActionAssert, Assert: "contains", Value: "zzz-absent-token", Label: "expects missing token"},
		},
	}
	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if result.OK {
		t.Fatal("scenario should have failed")
	}
	if result.ActionsRun != 1 {
		t.Fatalf("actions_run = %d, want 1 (one action passed before failure)", result.ActionsRun)
	}
	if result.Error == nil {
		t.Fatal("result.Error should be populated on failure")
	}
	if result.Error.Index != 1 {
		t.Fatalf("error index = %d, want 1", result.Error.Index)
	}
	if result.Error.Label != "expects missing token" {
		t.Fatalf("error label = %q, want %q", result.Error.Label, "expects missing token")
	}
	if result.FinalObservation == nil {
		t.Fatal("final_observation should carry the failure screen")
	}
}

func TestUITestScenarioRunFixtureRejectsInvalidScenario(t *testing.T) {
	// Validation happens before any driver is built, so no workspace leaks.
	_, ws, err := RunScenarioFixture(UITestScenario{}, RunScenarioOptions{})
	if err == nil {
		t.Fatal("empty scenario should fail before execution")
	}
	if ws != nil {
		_ = ws.Cleanup()
		t.Fatal("no workspace should be returned when validation fails")
	}
}

// --- Result JSON contract ----------------------------------------------------

func TestUITestScenarioResultJSONShape(t *testing.T) {
	sc := UITestScenario{
		Name:    "shape",
		Actions: []UITestAction{{Type: UITestActionAssert, Assert: "view", Value: "CrossProjectOutliner"}},
	}
	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{IncludeObservations: true})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	for _, key := range []string{"ok", "scenario", "actions_run", "final_observation", "observations"} {
		if _, present := decoded[key]; !present {
			t.Fatalf("result JSON missing %q key: %s", key, data)
		}
	}
	// error is omitted on success.
	if _, present := decoded["error"]; present {
		t.Fatalf("result JSON should omit error on success: %s", data)
	}
}
