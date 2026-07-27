package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// legacyFixtureFile is the sanitized, hand-authored pre-migration workspace used
// as a stable input baseline for the WorkflowState migration sessions. It must
// never contain live task content; see testdata/legacy_workspace/README.md.
const legacyFixtureFile = "testdata/legacy_workspace/project-alpha.tasks.md"

// TestLegacyWorkspaceFixtureParsesLegacyFields pins the legacy schema signals
// (status, section, stage) that later sessions migrate to a single
// WorkflowState. If the legacy parser stops round-tripping these fields, the
// migration mapping loses its documented inputs and this baseline fails loudly.
func TestLegacyWorkspaceFixtureParsesLegacyFields(t *testing.T) {
	content, err := os.ReadFile(legacyFixtureFile)
	if err != nil {
		t.Fatalf("read legacy fixture: %v", err)
	}

	result, err := ParseTasksMarkdownDocument(string(content))
	if err != nil {
		t.Fatalf("parse legacy fixture: %v", err)
	}
	tasks := result.Tasks
	if result.ProjectName != "Alpha Sandbox" {
		t.Fatalf("project name = %q, want %q", result.ProjectName, "Alpha Sandbox")
	}

	byID := make(map[string]*models.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}

	// The retained legacy lifecycle fields (status/section/stage) are no
	// longer stored on the task: the parser derives a single canonical
	// WorkflowState and captures the raw legacy values in the migration audit.
	auditByID := make(map[string]MigrationAuditEntry, len(result.MigrationAudit))
	for _, entry := range result.MigrationAudit {
		auditByID[entry.TaskID] = entry
	}

	// Each expectation corresponds to one branch of the legacy -> WorkflowState
	// migration precedence documented in the fixture README: state is the derived
	// canonical WorkflowState, and status/section/stage are the retained legacy
	// signals captured in the migration audit.
	cases := testutil.DailyLoopLegacyLifecycleFixtures()

	if len(cases) != len(tasks) {
		t.Fatalf("fixture has %d tasks but %d expectations; keep them in sync", len(tasks), len(cases))
	}

	for _, tc := range cases {
		task, ok := byID[tc.ID]
		if !ok {
			t.Errorf("fixture missing task %q", tc.ID)
			continue
		}
		if task.State() != tc.State {
			t.Errorf("%s: state = %q, want %q", tc.ID, task.State(), tc.State)
		}

		entry, ok := auditByID[tc.ID]
		if !ok {
			t.Errorf("%s: missing migration audit entry", tc.ID)
			continue
		}
		if entry.Status != tc.Status {
			t.Errorf("%s: audit status = %q, want %q", tc.ID, entry.Status, tc.Status)
		}
		if entry.Section != tc.Section {
			t.Errorf("%s: audit section = %q, want %q", tc.ID, entry.Section, tc.Section)
		}
		if entry.Stage != tc.Stage {
			t.Errorf("%s: audit stage = %q, want %q", tc.ID, entry.Stage, tc.Stage)
		}
	}
}

// TestLegacyWorkspaceFixtureCanonicalRoundTrip characterizes the storage seam
// migration sessions inherit: schema-v1 inputs load with project sections and
// audit data at the decode boundary, while the canonical write is schema v2 and
// retains every derived state without re-emitting retired task or project
// lifecycle fields.
func TestLegacyWorkspaceFixtureCanonicalRoundTrip(t *testing.T) {
	content, err := os.ReadFile(legacyFixtureFile)
	if err != nil {
		t.Fatalf("read legacy fixture: %v", err)
	}
	doc, err := ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse legacy document: %v", err)
	}
	if doc.SchemaVersion != SchemaVersionLegacy {
		t.Fatalf("input schema = %d, want legacy v1", doc.SchemaVersion)
	}

	canonical := doc.WriteV2()
	if !strings.Contains(canonical, "<!-- tt-schema: 2 -->") {
		t.Fatal("canonical write is missing the schema-v2 marker")
	}
	for _, retired := range []string{"status:", "section:", "stage:", "active:"} {
		if strings.Contains(canonical, retired) {
			t.Errorf("canonical write retained retired task field %q", retired)
		}
	}
	if strings.Contains(canonical, "<!-- sections:") {
		t.Fatalf("canonical write retained retired project sections:\n%s", canonical)
	}

	roundTrip, err := ParseProjectDocument(canonical)
	if err != nil {
		t.Fatalf("parse canonical document: %v", err)
	}
	if roundTrip.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("round-trip schema = %d, want v2", roundTrip.SchemaVersion)
	}
	if len(roundTrip.Tasks) != len(doc.Tasks) {
		t.Fatalf("round-trip tasks = %d, want %d", len(roundTrip.Tasks), len(doc.Tasks))
	}
	if len(roundTrip.legacySections) != 0 || len(roundTrip.legacySectionColors) != 0 {
		t.Fatalf("canonical round-trip retained legacy project metadata")
	}

	wantState := make(map[string]models.WorkflowState, len(doc.Tasks))
	for _, task := range doc.Tasks {
		wantState[task.ID] = task.State()
	}
	for _, task := range roundTrip.Tasks {
		if got, want := task.State(), wantState[task.ID]; got != want {
			t.Errorf("%s round-trip state = %q, want %q", task.ID, got, want)
		}
	}
	if second := roundTrip.WriteV2(); second != canonical {
		t.Fatalf("canonical rewrite is not stable:\n--- first ---\n%s\n--- second ---\n%s", canonical, second)
	}
}

func TestLegacyProjectMetadataStaysAtDecodeBoundary(t *testing.T) {
	content := "# Project: Decode Boundary\n" +
		"<!-- sections: Next:blue;Waiting:red;Inbox:green -->\n" +
		"<!-- tags: legacy;keep -->\n" +
		"<!-- pinned_tags: keep -->\n\n" +
		"## Task: Waiting task\n" +
		"<!-- id: waiting-1, status: todo, section: Waiting, priority: 2 -->\n\n" +
		"## Task: Custom section task\n" +
		"<!-- id: custom-1, status: todo, section: Sprint-1, priority: 2 -->\n"

	decoded, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("parse legacy metadata: %v", err)
	}
	if len(decoded.legacySections) != 3 || decoded.legacySectionColors["Waiting"] != "red" {
		t.Fatalf("decode boundary lost legacy metadata: sections=%v colors=%v", decoded.legacySections, decoded.legacySectionColors)
	}

	projectPath := filepath.Join(t.TempDir(), "decode-boundary")
	if mkdirErr := os.MkdirAll(projectPath, DirPermissions); mkdirErr != nil {
		t.Fatalf("create project path: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(tasksFilePath(projectPath), []byte(content), FilePermissions); writeErr != nil {
		t.Fatalf("write legacy project: %v", writeErr)
	}
	project := &models.Project{ID: "decode-boundary", Name: "Decode Boundary", Path: projectPath}
	load, err := LoadProjectWithWarnings(project)
	if err != nil {
		t.Fatalf("load legacy project: %v", err)
	}
	if len(project.Tags) != 2 || len(project.PinnedTags) != 1 {
		t.Fatalf("canonical project metadata was not retained: tags=%v pinned=%v", project.Tags, project.PinnedTags)
	}
	if len(project.Tasks) != 2 || project.Tasks[0].State() != models.StateWaiting || project.Tasks[1].State() != models.StateReady {
		t.Fatalf("legacy states were not derived canonically: %#v", project.Tasks)
	}
	if len(load.Warnings) != 1 || len(project.Warnings) != 1 {
		t.Fatalf("repair warnings were not retained: load=%v project=%v", load.Warnings, project.Warnings)
	}

	if saveErr := SaveProject(project); saveErr != nil {
		t.Fatalf("save canonical project: %v", saveErr)
	}
	canonical, err := os.ReadFile(tasksFilePath(projectPath))
	if err != nil {
		t.Fatalf("read canonical project: %v", err)
	}
	text := string(canonical)
	if !strings.Contains(text, "<!-- tt-schema: 2 -->") ||
		!strings.Contains(text, "<!-- tags: legacy;keep -->") ||
		!strings.Contains(text, "<!-- pinned_tags: keep -->") {
		t.Fatalf("canonical project metadata is incomplete:\n%s", text)
	}
	for _, retired := range []string{"<!-- sections:", "status:", "section:", "stage:", "active:"} {
		if strings.Contains(text, retired) {
			t.Fatalf("canonical project retained %q:\n%s", retired, text)
		}
	}
}

func TestMixedLifecycleMetadataCanonicalStateWins(t *testing.T) {
	content := "# Project: Mixed\n" +
		"<!-- tt-schema: 2 -->\n\n" +
		"## Task: Canonical wins\n" +
		"<!-- id: mixed-1, state: next, status: canceled, section: Completed, stage: Review, priority: 2 -->\n"

	doc, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("parse mixed lifecycle document: %v", err)
	}
	if got := doc.Tasks[0].State(); got != models.StateNext {
		t.Fatalf("mixed lifecycle state = %q, want canonical next", got)
	}

	canonical := doc.WriteV2()
	if !strings.Contains(canonical, "state: next") {
		t.Fatalf("canonical rewrite lost state:\n%s", canonical)
	}
	for _, retired := range []string{"status:", "section:", "stage:"} {
		if strings.Contains(canonical, retired) {
			t.Fatalf("canonical rewrite retained %q:\n%s", retired, canonical)
		}
	}
}

// TestLegacyWorkspaceFixtureIsSanitized guards the "never copy live task
// content" rule: the fixture path stays inside testdata and every task ID uses
// the synthetic "legacy-" prefix, so a future edit cannot quietly paste real
// workspace data into the repository.
func TestLegacyWorkspaceFixtureIsSanitized(t *testing.T) {
	if filepath.Base(filepath.Dir(legacyFixtureFile)) != "legacy_workspace" {
		t.Fatalf("legacy fixture must live under testdata/legacy_workspace, got %q", legacyFixtureFile)
	}

	content, err := os.ReadFile(legacyFixtureFile)
	if err != nil {
		t.Fatalf("read legacy fixture: %v", err)
	}
	_, tasks, err := ParseTasksMarkdown(string(content))
	if err != nil {
		t.Fatalf("parse legacy fixture: %v", err)
	}
	for _, task := range tasks {
		if len(task.ID) < len("legacy-") || task.ID[:len("legacy-")] != "legacy-" {
			t.Errorf("task ID %q is not a synthetic legacy- fixture id", task.ID)
		}
	}
}
