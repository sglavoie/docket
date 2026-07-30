package format

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	legacyV4AlphaFixture = "../testdata/v4_migration/workspace/projects/alpha/tasks.md"
	legacyV4AlphaGolden  = "../testdata/v4_migration/expected-alpha-task-conversion.json"
)

func TestLegacyV4AlphaTaskConversionGolden(t *testing.T) {
	location, err := time.LoadLocation("America/Mexico_City")
	if err != nil {
		t.Fatalf("load migration location: %v", err)
	}
	content, err := os.ReadFile(legacyV4AlphaFixture)
	if err != nil {
		t.Fatalf("read legacy v4 fixture: %v", err)
	}
	document, err := DecodeLegacyV4TaskDocument(string(content), location)
	if err != nil {
		t.Fatalf("decode legacy v4 fixture: %v", err)
	}
	got, err := ConvertLegacyV4TaskDocument(document, location)
	if err != nil {
		t.Fatalf("convert legacy v4 fixture: %v", err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal conversion result: %v", err)
	}
	gotJSON = append(gotJSON, '\n')

	wantJSON, err := os.ReadFile(legacyV4AlphaGolden)
	if err != nil {
		t.Fatalf("read golden conversion result: %v\ncandidate golden:\n%s", err, gotJSON)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("conversion differs from golden %s\n--- got ---\n%s\n--- want ---\n%s",
			legacyV4AlphaGolden, gotJSON, wantJSON)
	}
}

func TestLegacyV4TaskConversionUsesWorkspaceTimezoneForDateReduction(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load migration location: %v", err)
	}
	content := "# Project: Timezone Fixture\n" +
		"<!-- tt-schema: 2 -->\n\n" +
		"## Task: Offset dates\n" +
		"<!-- id: timezone-task, state: ready, priority: 3, created: 2026-01-01T00:00:00Z, due: 2026-02-04T01:30:00Z, snoozed: 2026-02-05T07:30:00Z -->\n"

	document, err := DecodeLegacyV4TaskDocument(content, location)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	result, err := ConvertLegacyV4TaskDocument(document, location)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	task := result.Tasks[0]
	if task.DueDate != "2026-02-03" {
		t.Errorf("due date = %q, want 2026-02-03", task.DueDate)
	}
	if task.SnoozedUntil != "2026-02-04" {
		t.Errorf("snooze date = %q, want 2026-02-04", task.SnoozedUntil)
	}
	if len(result.Audit) != 2 {
		t.Fatalf("date conversion audit entries = %d, want 2: %+v", len(result.Audit), result.Audit)
	}
}

func TestLegacyV4ArchiveTaskDocumentUsesArchiveProjectHeader(t *testing.T) {
	content := "## Archived: 2026-01-28 13:33\n" +
		"<!-- project: Archived Fixture, count: 1 -->\n\n" +
		"### Task: Preserve archived work\n" +
		"<!-- id: archived-task, state: done, priority: 3, created: 2026-01-28 12:00, completed: 2026-01-28 13:00 -->\n"

	document, err := DecodeLegacyV4TaskDocument(content, time.UTC)
	if err != nil {
		t.Fatalf("decode archive document: %v", err)
	}
	if document.ProjectName != "Archived Fixture" {
		t.Fatalf("project name = %q, want archive header name", document.ProjectName)
	}
	if len(document.Tasks) != 1 || document.Tasks[0].Title != "Preserve archived work" {
		t.Fatalf("tasks = %+v, want the archived task", document.Tasks)
	}
}

func TestLegacyV4WeekdayRecurrenceRequiresDueDateAnchor(t *testing.T) {
	location := time.UTC
	content := "# Project: Recurrence Fixture\n" +
		"<!-- tt-schema: 2 -->\n\n" +
		"## Task: Missing recurrence anchor\n" +
		"<!-- id: missing-anchor, state: ready, priority: 3, created: 2026-01-01 08:00, recur: 2w:mon,fri -->\n"

	document, err := DecodeLegacyV4TaskDocument(content, location)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	_, err = ConvertLegacyV4TaskDocument(document, location)
	if err == nil || !strings.Contains(err.Error(), "has no due date to anchor it") {
		t.Fatalf("conversion error = %v, want missing due-date anchor", err)
	}
}

func TestLegacyV4TaskConverterSourceDoesNotDependOnRuntimeModels(t *testing.T) {
	for _, name := range []string{"legacy_v4_task.go", "legacy_v4_task_convert.go"} {
		source, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(source), "internal/models") {
			t.Errorf("%s imports runtime models", name)
		}
	}
}
