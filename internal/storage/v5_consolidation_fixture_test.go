package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/storage/format"
)

const v5ConsolidationFixtureRoot = "testdata/v5_consolidation"

type v5ConsolidationMatrix struct {
	SchemaVersion    int                           `json:"schema_version"`
	StateMappings    []v5ConsolidationStateMapping `json:"state_mappings"`
	PriorityMappings []v5ConsolidationPriorityMap  `json:"priority_mappings"`
	TaskFields       []v5ConsolidationFieldMap     `json:"task_fields"`
	DomainMappings   []v5ConsolidationFieldMap     `json:"domain_mappings"`
	ArtifactMappings []v5ConsolidationFieldMap     `json:"artifact_mappings"`
	ConversionRules  []v5ConsolidationRule         `json:"conversion_rules"`
}

type v5ConsolidationStateMapping struct {
	Source          string `json:"source"`
	TargetLifecycle string `json:"target_lifecycle"`
	TargetNext      bool   `json:"target_next"`
}

type v5ConsolidationPriorityMap struct {
	Source      int    `json:"source"`
	Target      int    `json:"target"`
	TargetLabel string `json:"target_label"`
}

type v5ConsolidationFieldMap struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Action string `json:"action"`
}

type v5ConsolidationRule struct {
	ID          string `json:"id"`
	Expectation string `json:"expectation"`
}

func TestV5ConsolidationTaskFixtureCoversEverySourceFieldStateAndPriority(t *testing.T) {
	tasks := loadV5ConsolidationTasks(t)
	if len(tasks) < 12 {
		t.Fatalf("fixture task count = %d, want at least 12 representative tasks", len(tasks))
	}

	states := make(map[string]bool)
	priorities := make(map[int]bool)
	coveredFields := make(map[string]bool)
	taskType := reflect.TypeOf(format.V5TaskRecord{})
	for _, task := range tasks {
		states[task.State] = true
		priorities[task.Priority] = true
		value := reflect.ValueOf(task)
		for i := 0; i < taskType.NumField(); i++ {
			if !value.Field(i).IsZero() {
				coveredFields[taskType.Field(i).Name] = true
			}
		}
	}

	wantStates := []string{
		"inbox", "ready", "next", "in_progress", "waiting", "paused", "someday", "done", "canceled",
	}
	for _, state := range wantStates {
		if !states[state] {
			t.Errorf("fixture is missing schema-5 state %q", state)
		}
	}
	if len(states) != len(wantStates) {
		t.Errorf("fixture states = %v, want exactly %v", states, wantStates)
	}
	for priority := 0; priority <= 5; priority++ {
		if !priorities[priority] {
			t.Errorf("fixture is missing schema-5 priority %d", priority)
		}
	}
	if len(priorities) != 6 {
		t.Errorf("fixture priorities = %v, want exactly 0..5", priorities)
	}
	for i := 0; i < taskType.NumField(); i++ {
		name := taskType.Field(i).Name
		if !coveredFields[name] {
			t.Errorf("fixture never populates V5TaskRecord.%s", name)
		}
	}

	rich := findV5ConsolidationTask(t, tasks, "task-rich")
	if rich.Description == "" || len(rich.Tags) != 2 || rich.DueDate == "" || rich.SnoozedUntil == "" ||
		!rich.Pinned || rich.PinnedAt == nil {
		t.Fatalf("rich fixture lost retained scalar coverage: %+v", rich)
	}
	if len(rich.DependsOn) != 2 || rich.Recurrence == nil || rich.Recurrence.Interval != 2 || rich.Recurrence.Unit != "week" {
		t.Fatalf("rich fixture lost retired dependency/recurrence coverage: %+v", rich)
	}
	if len(rich.Comments) != 2 || len(rich.Decisions) != 2 ||
		!rich.Comments[1].CreatedAt.Equal(rich.Decisions[1].CreatedAt) {
		t.Fatalf("rich fixture lost comment/decision merge-order coverage: %+v", rich)
	}
	if len(rich.Attachments) != 3 || len(rich.Checklist) != 2 {
		t.Fatalf("rich fixture lost attachment/checklist coverage: %+v", rich)
	}
	nested := findV5ConsolidationTask(t, tasks, "task-next")
	if nested.ParentID != "task-inbox" {
		t.Fatalf("nested fixture parent = %q, want task-inbox", nested.ParentID)
	}
}

func TestV5ConsolidationFixtureRetainsNotesAttachmentsAndUndoRedoHistory(t *testing.T) {
	projectNote, err := LoadNoteFrom(
		filepath.Join(v5ConsolidationFixtureRoot, "workspace", "projects", "alpha", "notes"),
		"project-brief.md",
	)
	if err != nil {
		t.Fatalf("load project-note fixture: %v", err)
	}
	globalNote, err := LoadNoteFrom(
		filepath.Join(v5ConsolidationFixtureRoot, "workspace", ".docket", "global-notes"),
		"global-brief.md",
	)
	if err != nil {
		t.Fatalf("load global-note fixture: %v", err)
	}
	if projectNote.Title != "Synthetic project brief" || globalNote.Title != "Synthetic global brief" {
		t.Fatalf("note fixture titles = %q / %q", projectNote.Title, globalNote.Title)
	}

	attachment, err := os.ReadFile(filepath.Join(
		v5ConsolidationFixtureRoot,
		"workspace", "projects", "alpha", ".attachments", "task-rich", "specification.txt",
	))
	if err != nil {
		t.Fatalf("read attachment fixture: %v", err)
	}
	if !strings.Contains(string(attachment), "retained through schema-6 activation") {
		t.Fatalf("attachment fixture content = %q", attachment)
	}

	ledger := NewWorkspaceLedger(
		filepath.Join(v5ConsolidationFixtureRoot, "workspace", ".docket", "undo"),
		[]string{"/synthetic/workspace"},
	)
	status, err := ledger.Validate()
	if err != nil {
		t.Fatalf("validate undo/redo fixture: %v", err)
	}
	if status.UndoEntries != 1 || status.RedoEntries != 1 {
		t.Fatalf("fixture ledger status = %+v, want one undo and one redo entry", status)
	}
	persisted, err := ledger.load()
	if err != nil {
		t.Fatalf("load undo/redo fixture: %v", err)
	}
	if persisted.Entries[0].Action != "edit task" || persisted.RedoEntries[0].Action != "create note" {
		t.Fatalf("fixture ledger actions = %q / %q", persisted.Entries[0].Action, persisted.RedoEntries[0].Action)
	}
}

func TestV5ConsolidationTransformationMatrixIsExhaustive(t *testing.T) {
	matrix := loadV5ConsolidationMatrix(t)
	if matrix.SchemaVersion != 1 {
		t.Fatalf("transformation matrix schema = %d, want 1", matrix.SchemaVersion)
	}

	wantStates := map[string]v5ConsolidationStateMapping{
		"inbox":       {Source: "inbox", TargetLifecycle: "inbox"},
		"ready":       {Source: "ready", TargetLifecycle: "backlog"},
		"next":        {Source: "next", TargetLifecycle: "backlog", TargetNext: true},
		"in_progress": {Source: "in_progress", TargetLifecycle: "doing"},
		"waiting":     {Source: "waiting", TargetLifecycle: "waiting"},
		"paused":      {Source: "paused", TargetLifecycle: "paused"},
		"someday":     {Source: "someday", TargetLifecycle: "someday"},
		"done":        {Source: "done", TargetLifecycle: "done"},
		"canceled":    {Source: "canceled", TargetLifecycle: "canceled"},
	}
	gotStates := make(map[string]v5ConsolidationStateMapping, len(matrix.StateMappings))
	for _, mapping := range matrix.StateMappings {
		gotStates[mapping.Source] = mapping
	}
	if !reflect.DeepEqual(gotStates, wantStates) {
		t.Fatalf("state transformation matrix = %#v, want %#v", gotStates, wantStates)
	}

	wantPriorities := map[int]v5ConsolidationPriorityMap{
		0: {Source: 0, Target: 0, TargetLabel: "unset"},
		1: {Source: 1, Target: 1, TargetLabel: "high"},
		2: {Source: 2, Target: 1, TargetLabel: "high"},
		3: {Source: 3, Target: 0, TargetLabel: "unset"},
		4: {Source: 4, Target: 3, TargetLabel: "low"},
		5: {Source: 5, Target: 3, TargetLabel: "low"},
	}
	gotPriorities := make(map[int]v5ConsolidationPriorityMap, len(matrix.PriorityMappings))
	for _, mapping := range matrix.PriorityMappings {
		gotPriorities[mapping.Source] = mapping
	}
	if !reflect.DeepEqual(gotPriorities, wantPriorities) {
		t.Fatalf("priority transformation matrix = %#v, want %#v", gotPriorities, wantPriorities)
	}

	fieldActions := make(map[string]v5ConsolidationFieldMap, len(matrix.TaskFields))
	for _, mapping := range matrix.TaskFields {
		if _, duplicate := fieldActions[mapping.Source]; duplicate {
			t.Fatalf("transformation matrix repeats task field %q", mapping.Source)
		}
		fieldActions[mapping.Source] = mapping
	}
	taskType := reflect.TypeOf(format.V5TaskRecord{})
	var sourceFields []string
	for i := 0; i < taskType.NumField(); i++ {
		jsonName := strings.Split(taskType.Field(i).Tag.Get("json"), ",")[0]
		sourceFields = append(sourceFields, jsonName)
		if _, ok := fieldActions[jsonName]; !ok {
			t.Errorf("transformation matrix has no action for V5TaskRecord field %q", jsonName)
		}
	}
	if len(fieldActions) != len(sourceFields) {
		t.Errorf("transformation matrix task fields = %d, V5TaskRecord fields = %d", len(fieldActions), len(sourceFields))
	}
	for source, wantAction := range map[string]string{
		"depends_on": "drop-and-audit",
		"recurrence": "drop-and-audit",
		"comments":   "retain-and-merge",
		"decisions":  "convert-to-comments-and-audit",
	} {
		if got := fieldActions[source].Action; got != wantAction {
			t.Errorf("task field %s action = %q, want %q", source, got, wantAction)
		}
	}

	domainActions := make(map[string]string, len(matrix.DomainMappings))
	for _, mapping := range matrix.DomainMappings {
		domainActions[mapping.Source] = mapping.Action
	}
	for source, want := range map[string]string{
		"workspace.system_inbox_project_id":     "rename-project-and-reference-preserving-id",
		"project.context":                       "retain",
		"planning.today":                        "remove-dangling-terminal-and-next-conflicts",
		"planning.next_order + task.state=next": "rebuild-membership-and-order",
		"machine.custom_contexts":               "drop-and-audit-name-and-member-count",
		"machine.theme":                         "retain",
	} {
		if got := domainActions[source]; got != want {
			t.Errorf("domain field %s action = %q, want %q", source, got, want)
		}
	}

	artifactActions := make(map[string]string, len(matrix.ArtifactMappings))
	for _, mapping := range matrix.ArtifactMappings {
		artifactActions[mapping.Source] = mapping.Action
	}
	if got := artifactActions["undo/redo ledger"]; got != "snapshot-then-clear-live-stacks" {
		t.Errorf("undo/redo artifact action = %q", got)
	}

	wantRules := map[string]string{
		"decision-comment-format": "Decision: <text> followed by Rationale: <rationale> on a second line when rationale is present",
		"decision-comment-order":  "merge chronologically; equal timestamps keep existing comments first then decisions in source order",
		"recurrence-audit":        "record the task ID and discarded recurrence value",
		"dependency-audit":        "record the task ID and every discarded dependency reference",
		"next-membership":         "every schema-5 StateNext task becomes a schema-6 Next member",
		"next-order":              "keep valid legacy NextOrder members first then append missing members in catalog-project and document-task order",
		"today-normalization":     "Next wins Today conflicts; remove and audit terminal and dangling Today entries",
		"unfiled-identity":        "preserve the protected project ID while renaming Inbox/inbox and its catalog reference to Unfiled/unfiled",
		"custom-context-audit":    "drop custom contexts and audit each name and member count",
		"undo-boundary":           "snapshot schema-5 undo and redo history then clear both live stacks",
	}
	gotRules := make(map[string]string, len(matrix.ConversionRules))
	for _, rule := range matrix.ConversionRules {
		if _, duplicate := gotRules[rule.ID]; duplicate {
			t.Fatalf("transformation matrix repeats conversion rule %q", rule.ID)
		}
		gotRules[rule.ID] = rule.Expectation
	}
	if !reflect.DeepEqual(gotRules, wantRules) {
		t.Fatalf("conversion rules = %#v, want %#v", gotRules, wantRules)
	}
}

func loadV5ConsolidationTasks(t *testing.T) []format.V5TaskRecord {
	t.Helper()
	var tasks []format.V5TaskRecord
	for _, rel := range []string{
		"workspace/projects/alpha/tasks.md",
		"workspace/projects/beta/tasks.md",
		"workspace/projects/inbox/tasks.md",
	} {
		content, err := os.ReadFile(filepath.Join(v5ConsolidationFixtureRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read fixture %s: %v", rel, err)
		}
		document, err := ParseV5ProjectDocument(string(content))
		if err != nil {
			t.Fatalf("parse fixture %s: %v", rel, err)
		}
		tasks = append(tasks, document.Tasks...)
	}
	return tasks
}

func findV5ConsolidationTask(t *testing.T, tasks []format.V5TaskRecord, id string) format.V5TaskRecord {
	t.Helper()
	index := slices.IndexFunc(tasks, func(task format.V5TaskRecord) bool { return task.ID == id })
	if index < 0 {
		t.Fatalf("fixture task %q not found", id)
	}
	return tasks[index]
}

func loadV5ConsolidationMatrix(t *testing.T) v5ConsolidationMatrix {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(v5ConsolidationFixtureRoot, "transformation-matrix.json"))
	if err != nil {
		t.Fatalf("read transformation matrix: %v", err)
	}
	var matrix v5ConsolidationMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode transformation matrix: %v", err)
	}
	return matrix
}
