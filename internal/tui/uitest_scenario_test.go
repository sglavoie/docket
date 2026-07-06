package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Parsing -----------------------------------------------------------------

func TestUITestScenarioParseAndValidate(t *testing.T) {
	raw := `{
		"name": "open dashboard",
		"fixture": "rich",
		"size": "120x40",
		"actions": [
			{"type": "assert", "assert": "view", "value": "Dashboard", "label": "starts on dashboard"},
			{"type": "resize", "width": 100, "height": 30},
			{"type": "key", "key": "ctrl+i"},
			{"type": "wait"}
		]
	}`

	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}
	if sc.Name != "open dashboard" || sc.Fixture != "rich" || sc.Size != "120x40" {
		t.Fatalf("parsed header = %q/%q/%q", sc.Name, sc.Fixture, sc.Size)
	}
	if len(sc.Actions) != 4 {
		t.Fatalf("parsed %d actions, want 4", len(sc.Actions))
	}
	if sc.Actions[0].Label != "starts on dashboard" {
		t.Fatalf("action label = %q, want %q", sc.Actions[0].Label, "starts on dashboard")
	}
	if err := sc.Validate(); err != nil {
		t.Fatalf("Validate() error on well-formed scenario: %v", err)
	}
}

func TestUITestScenarioParseRejectsUnknownField(t *testing.T) {
	// A typo'd field name must fail rather than silently drop.
	raw := `{"actions": [{"type": "wait", "kye": "x"}]}`
	if _, err := ParseScenario([]byte(raw)); err == nil {
		t.Fatal("ParseScenario should reject unknown fields")
	}
}

func TestUITestScenarioParseRejectsTrailingContent(t *testing.T) {
	raw := `{"actions": [{"type": "wait"}]} {"extra": true}`
	if _, err := ParseScenario([]byte(raw)); err == nil {
		t.Fatal("ParseScenario should reject trailing content after the first object")
	}
}

func TestUITestScenarioLoadFile(t *testing.T) {
	raw := `{"name": "from file", "actions": [{"type": "wait"}]}`
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	sc, err := LoadScenario(path)
	if err != nil {
		t.Fatalf("LoadScenario error: %v", err)
	}
	if sc.Name != "from file" || len(sc.Actions) != 1 {
		t.Fatalf("LoadScenario got %q/%d actions", sc.Name, len(sc.Actions))
	}
}

// --- Strict validation -------------------------------------------------------

func TestUITestScenarioValidateEmpty(t *testing.T) {
	if err := (UITestScenario{}).Validate(); err == nil {
		t.Fatal("empty scenario should fail validation")
	}
}

func TestUITestScenarioValidateUnknownActionType(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: "teleport"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("unknown action type should fail validation")
	}
}

func TestUITestScenarioValidateMalformedSize(t *testing.T) {
	// Malformed top-level size.
	sc := UITestScenario{Size: "wide", Actions: []UITestAction{{Type: UITestActionWait}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("malformed top-level size should fail validation")
	}

	// Non-positive resize dimensions.
	sc = UITestScenario{Actions: []UITestAction{{Type: UITestActionResize, Width: 0, Height: 30}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("resize with zero width should fail validation")
	}
}

func TestUITestScenarioValidateInvalidKey(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionKey, Key: "ctrl+"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("invalid key should fail validation before execution")
	}
}

func TestUITestScenarioValidateUnknownAssertKind(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionAssert, Assert: "bogus"}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("unknown assertion kind should fail validation")
	}
	sc = UITestScenario{Actions: []UITestAction{{Type: UITestActionAssert}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("missing assertion kind should fail validation")
	}
}

func TestUITestScenarioValidateEmptyText(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionText, Text: ""}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("empty text action should fail validation")
	}
}

func TestUITestScenarioValidateRejectsEmptyCommand(t *testing.T) {
	sc := UITestScenario{Actions: []UITestAction{{Type: UITestActionCommand, Command: "  "}}}
	if err := sc.Validate(); err == nil {
		t.Fatal("command action with blank command should fail validation")
	}
}

// --- Size resolution ---------------------------------------------------------

func TestUITestScenarioResolveSize(t *testing.T) {
	// Explicit size wins over defaults.
	w, h, err := (UITestScenario{Size: "80x24"}).ResolveSize(100, 30)
	if err != nil || w != 80 || h != 24 {
		t.Fatalf("ResolveSize(explicit) = %dx%d err=%v, want 80x24", w, h, err)
	}
	// Empty size falls back to defaults.
	w, h, err = (UITestScenario{}).ResolveSize(100, 30)
	if err != nil || w != 100 || h != 30 {
		t.Fatalf("ResolveSize(default) = %dx%d err=%v, want 100x30", w, h, err)
	}
	// Malformed size errors.
	if _, _, err := (UITestScenario{Size: "100"}).ResolveSize(100, 30); err == nil {
		t.Fatal("ResolveSize(malformed) should error")
	}
}

// --- Execution: golden-ish result --------------------------------------------

func TestUITestScenarioRunFixtureGolden(t *testing.T) {
	raw := `{
		"name": "rich dashboard renders stable content",
		"fixture": "rich",
		"size": "120x40",
		"actions": [
			{"type": "assert", "assert": "view", "value": "Dashboard"},
			{"type": "assert", "assert": "contains", "value": "Rich Project"},
			{"type": "assert", "assert": "contains", "value": "Launch platform"},
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
		t.Fatalf("scenario should pass, got error: %+v", result.Error)
	}
	if result.Scenario != "rich dashboard renders stable content" {
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
	for _, want := range []string{"Rich Project", "Launch platform"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("final screen missing stable token %q:\n%s", want, screen)
		}
	}
	if strings.Contains(screen, "\x1b") {
		t.Fatal("final screen still contains ANSI escape sequences")
	}
}

func TestUITestScenarioRoadmapLayoutNoOverflow(t *testing.T) {
	raw := `{
		"name": "rich roadmap layout",
		"fixture": "rich",
		"size": "100x35",
		"actions": [
			{"type": "command", "command": "roadmap"},
			{"type": "assert", "assert": "view", "value": "Roadmap"},
			{"type": "assert", "assert": "no_layout_issues"}
		]
	}`
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("roadmap layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioRoadmapEmptyForecastsNoOverflow(t *testing.T) {
	raw := `{
		"name": "narrow roadmap empty forecasts layout",
		"fixture": "narrow",
		"size": "72x20",
		"actions": [
			{"type": "command", "command": "roadmap"},
			{"type": "assert", "assert": "view", "value": "Roadmap"},
			{"type": "assert", "assert": "no_layout_issues"},
			{"type": "key", "key": "tab"},
			{"type": "wait"},
			{"type": "assert", "assert": "no_layout_issues"},
			{"type": "key", "key": "tab"},
			{"type": "wait"},
			{"type": "assert", "assert": "no_layout_issues"}
		]
	}`
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("roadmap empty forecasts layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioTaskDetailFromGlobalInboxNoOverflow(t *testing.T) {
	raw := `{
		"name": "task detail from global inbox layout",
		"fixture": "rich",
		"size": "100x35",
		"actions": [
			{"type": "command", "command": "global inbox processor"},
			{"type": "command", "command": "view task details"},
			{"type": "assert", "assert": "view", "value": "TaskDetail"},
			{"type": "assert", "assert": "no_layout_issues"}
		]
	}`
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario error: %v", err)
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("task detail layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioKeybindingExplorerMinimumHeightNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "keybinding explorer minimum height",
		Fixture: "rich",
		Size:    "72x20",
		Actions: []UITestAction{
			{Type: UITestActionCommand, Command: "keybinding explorer"},
			{Type: UITestActionAssert, Assert: "view", Value: "KeybindingExplorer"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("keybinding explorer layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioDashboardNarrowLayoutNoOverflow(t *testing.T) {
	// Regression: at narrow widths (<100 cols) the dashboard stacks its info
	// cards vertically. Each card used to be padded to the full section height,
	// so N cards overflowed the terminal. Fixtures with 0 info cards (empty)
	// never regressed, which is why the earlier regression set missed this.
	fixtures := []string{"rich", "narrow", "alerts", "workflow"}
	sizes := []string{"80x24", "72x20"}

	for _, fixture := range fixtures {
		for _, size := range sizes {
			t.Run(fixture+"_"+size, func(t *testing.T) {
				sc := UITestScenario{
					Name:    "dashboard narrow layout " + fixture + " " + size,
					Fixture: fixture,
					Size:    size,
					Actions: []UITestAction{
						{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"},
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

func TestUITestScenarioPipelineHubMinimumLayoutNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "pipeline hub minimum layout",
		Fixture: "rich",
		Size:    "72x20",
		Actions: []UITestAction{
			{Type: UITestActionKey, Key: ";"},
			{Type: UITestActionKey, Key: ";"},
			{Type: UITestActionAssert, Assert: "view", Value: "PipelineHub"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("pipeline hub layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioMinimumHeightViewLayoutsNoOverflow(t *testing.T) {
	tests := []struct {
		name    string
		command string
		view    string
	}{
		{"settings", "settings", "Settings"},
		{"metrics dashboard", "metrics dashboard", "MetricsDashboard"},
		{"task template creator", "create task template", "TaskTemplateCreator"},
		{"project creator", "new project", "ProjectCreator"},
		{"project editor", "rename project", "ProjectEditor"},
		{"workflow rule creator", "new workflow rule", "WorkflowRuleCreator"},
		{"inbox processor", "triage inbox", "InboxProcessor"},
		{"global inbox processor", "global inbox processor", "GlobalInboxProcessor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := UITestScenario{
				Name:    tt.name + " minimum height",
				Fixture: "rich",
				Size:    "72x20",
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
				t.Fatalf("%s layout should pass, got error: %+v", tt.name, result.Error)
			}
		})
	}
}

func TestUITestScenarioMinimumTerminalEmptyStatesNoOverflow(t *testing.T) {
	tests := []struct {
		name    string
		command string
		view    string
	}{
		{"backlog queue empty state", "backlog", "FocusView"},
		{"workflow rules empty state", "workflow rules", "WorkflowRules"},
		{"task templates empty state", "task templates", "TaskTemplates"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := UITestScenario{
				Name:    tt.name + " minimum terminal layout",
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
				t.Fatalf("%s layout should pass, got error: %+v", tt.name, result.Error)
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
			name: "today quick add",
			size: "60x20",
			actions: []UITestAction{
				{Type: UITestActionCommand, Command: "today"},
				{Type: UITestActionKey, Key: "a"},
				{Type: UITestActionText, Text: longValue},
				{Type: UITestActionAssert, Assert: "view", Value: "Today"},
				{Type: UITestActionAssert, Assert: "no_layout_issues"},
			},
		},
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

func TestUITestScenarioScoreProfilePresetPickerMinimumLayoutNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "score profile preset picker minimum layout",
		Fixture: "rich",
		Size:    "72x20",
		Actions: []UITestAction{
			{Type: UITestActionCommand, Command: "scoring profiles"},
			{Type: UITestActionKey, Key: "a"},
			{Type: UITestActionAssert, Assert: "view", Value: "ScoreProfileManager"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("score profile preset picker layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioScoreProfileDetailPaneBoundaryNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "score profile detail pane boundary layout",
		Fixture: "rich",
		Size:    "100x35",
		Actions: []UITestAction{
			{Type: UITestActionCommand, Command: "scoring profiles"},
			{Type: UITestActionKey, Key: "a"},
			{Type: UITestActionKey, Key: "enter"},
			{Type: UITestActionResize, Width: 81, Height: 20},
			{Type: UITestActionAssert, Assert: "view", Value: "ScoreProfileManager"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("score profile detail pane boundary layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioTodayQuickAddMinimumLayoutNoOverflow(t *testing.T) {
	tests := []struct {
		name    string
		actions []UITestAction
	}{
		{
			name: "syntax hint",
			actions: []UITestAction{
				{Type: UITestActionCommand, Command: "today"},
				{Type: UITestActionCommand, Command: "today quick add"},
			},
		},
		{
			name: "metadata preview",
			actions: []UITestAction{
				{Type: UITestActionCommand, Command: "today"},
				{Type: UITestActionCommand, Command: "today quick add"},
				{Type: UITestActionText, Text: "New item !1 #reallylongtagname @tomorrow -s external -u https://example.com/very/long/path"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := append([]UITestAction{}, tt.actions...)
			actions = append(actions,
				UITestAction{Type: UITestActionAssert, Assert: "view", Value: "Today"},
				UITestAction{Type: UITestActionAssert, Assert: "no_layout_issues"},
			)
			sc := UITestScenario{
				Name:    "today quick add " + tt.name + " minimum layout",
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
				t.Fatalf("today quick add %s layout should pass, got error: %+v", tt.name, result.Error)
			}
		})
	}
}

func TestUITestScenarioLongNoMatchSearchQueriesMinimumLayoutNoOverflow(t *testing.T) {
	tests := []struct {
		name    string
		view    string
		actions []UITestAction
	}{
		{
			name: "command palette",
			view: "CommandPalette",
			actions: []UITestAction{
				{Type: UITestActionKey, Key: ":"},
				{Type: UITestActionText, Text: strings.Repeat("a", 56)},
			},
		},
		{
			name: "global search",
			view: "GlobalSearch",
			actions: []UITestAction{
				{Type: UITestActionCommand, Command: "global search"},
				{Type: UITestActionText, Text: strings.Repeat("a", 60)},
			},
		},
		{
			name: "notes search",
			view: "NotesSearch",
			actions: []UITestAction{
				{Type: UITestActionKey, Key: "enter"},
				{Type: UITestActionKey, Key: "{"},
				{Type: UITestActionKey, Key: "/"},
				{Type: UITestActionText, Text: strings.Repeat("a", 72)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := append([]UITestAction{}, tt.actions...)
			actions = append(actions,
				UITestAction{Type: UITestActionAssert, Assert: "view", Value: tt.view},
				UITestAction{Type: UITestActionAssert, Assert: "no_layout_issues"},
			)
			sc := UITestScenario{
				Name:    tt.name + " long no-match query minimum layout",
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

func TestUITestScenarioTaskSubviewLongTitleLayoutsNoOverflow(t *testing.T) {
	views := []struct {
		name    string
		command string
		view    string
	}{
		{"attachments", "task attachments", "TaskAttachments"},
		{"comments", "task comments", "TaskComments"},
		{"decisions", "manage decisions", "TaskDecisions"},
		{"activity", "task activity", "TaskActivity"},
	}

	for _, size := range []string{"100x35", "72x20", "60x20"} {
		for _, tt := range views {
			t.Run(size+"_"+tt.name, func(t *testing.T) {
				sc := UITestScenario{
					Name:    "task subview long title layout",
					Fixture: "narrow",
					Size:    size,
					Actions: []UITestAction{
						{Type: UITestActionKey, Key: "enter"},
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

func TestUITestScenarioExportProjectStatusLayoutNoOverflow(t *testing.T) {
	sc := UITestScenario{
		Name:    "export project status layout",
		Fixture: "rich",
		Size:    "100x35",
		Actions: []UITestAction{
			{Type: UITestActionCommand, Command: "export project"},
			{Type: UITestActionAssert, Assert: "no_layout_issues"},
		},
	}

	result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
	if err != nil {
		t.Fatalf("RunScenarioFixture error: %v", err)
	}
	t.Cleanup(func() { _ = ws.Cleanup() })

	if !result.OK {
		t.Fatalf("export project status layout should pass, got error: %+v", result.Error)
	}
}

func TestUITestScenarioWorkflowRuleCreatorWideLayoutsNoOverflow(t *testing.T) {
	for _, size := range []string{"100x30", "100x35", "120x40"} {
		t.Run(size, func(t *testing.T) {
			sc := UITestScenario{
				Name:    "workflow rule creator wide layout",
				Fixture: "rich",
				Size:    size,
				Actions: []UITestAction{
					{Type: UITestActionCommand, Command: "new workflow rule"},
					{Type: UITestActionAssert, Assert: "view", Value: "WorkflowRuleCreator"},
					{Type: UITestActionAssert, Assert: "no_layout_issues"},
				},
			}

			result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
			if err != nil {
				t.Fatalf("RunScenarioFixture error: %v", err)
			}
			t.Cleanup(func() { _ = ws.Cleanup() })

			if !result.OK {
				t.Fatalf("workflow rule creator layout should pass, got error: %+v", result.Error)
			}
		})
	}
}

func TestUITestScenarioRunFixtureDefaults(t *testing.T) {
	// No fixture or size in the scenario: RunScenarioFixture falls back to the
	// package defaults (rich fixture at 100x30).
	sc := UITestScenario{
		Name:    "defaults",
		Actions: []UITestAction{{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"}},
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
			{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"},
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
		Actions: []UITestAction{{Type: UITestActionAssert, Assert: "view", Value: "Dashboard"}},
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
