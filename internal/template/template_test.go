package template

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNewTemplate_BasicFields(t *testing.T) {
	tmpl := NewTemplate("My Template", models.TaskTypeStory)

	if tmpl.Name != "My Template" {
		t.Errorf("Name: got %q, want %q", tmpl.Name, "My Template")
	}
	if tmpl.Type != models.TaskTypeStory {
		t.Errorf("Type: got %q, want %q", tmpl.Type, models.TaskTypeStory)
	}
	if !tmpl.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if tmpl.Priority != 0 {
		t.Errorf("Priority: got %d, want 0", tmpl.Priority)
	}
	if tmpl.State != models.StateInbox {
		t.Errorf("State: got %q, want %q", tmpl.State, models.StateInbox)
	}
	if tmpl.Tags == nil || len(tmpl.Tags) != 0 {
		t.Errorf("Tags: got %v, want empty slice", tmpl.Tags)
	}
	if tmpl.Children == nil || len(tmpl.Children) != 0 {
		t.Errorf("Children: got %v, want empty slice", tmpl.Children)
	}
}

func TestTaskTemplateJSONMigratesLegacySectionAndPreservesContent(t *testing.T) {
	data := []byte(`{
		"id":"custom-1",
		"name":"Legacy",
		"enabled":false,
		"source":"imported",
		"category":"process",
		"type":"story",
		"priority":2,
		"tags":["second","first"],
		"section":"Next",
		"description":"kept",
		"children":[
			{"title":"child-b","type":"bug","optional":true},
			{"title":"child-a","type":"task","optional":false}
		],
		"checklist":["check-b","check-a"],
		"use_count":17
	}`)

	var got TaskTemplate
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != "custom-1" || got.Name != "Legacy" || got.Enabled ||
		got.Source != "imported" || got.Category != "process" ||
		got.Type != models.TaskTypeStory || got.Priority != 2 ||
		got.Description != "kept" || got.UseCount != 17 {
		t.Fatalf("legacy metadata changed: %+v", got)
	}
	if got.State != models.StateNext {
		t.Fatalf("State = %q, want %q", got.State, models.StateNext)
	}
	if !reflect.DeepEqual(got.Tags, []string{"second", "first"}) ||
		!reflect.DeepEqual(got.Checklist, []string{"check-b", "check-a"}) ||
		!reflect.DeepEqual(got.Children, []TemplateChild{
			{Title: "child-b", Type: models.TaskTypeBug, Optional: true},
			{Title: "child-a", Type: models.TaskTypeTask, Optional: false},
		}) {
		t.Fatalf("ordered template content changed: %+v", got)
	}
	if !got.NormalizeState() {
		t.Fatal("legacy section decode did not request a canonical rewrite")
	}
	if got.NormalizeState() {
		t.Fatal("canonical state normalization was not idempotent")
	}

	canonical, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &fields); err != nil {
		t.Fatalf("decode canonical fields: %v", err)
	}
	if _, ok := fields["section"]; ok {
		t.Fatalf("canonical JSON retained legacy section: %s", canonical)
	}
	if string(fields["state"]) != `"next"` {
		t.Fatalf("canonical state = %s, want next", fields["state"])
	}
}

func TestTaskTemplateJSONStateDefaultsAndCanonicalPrecedence(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		want        models.WorkflowState
		wantRewrite bool
	}{
		{"omitted defaults inbox", `{"name":"x","type":"task"}`, models.StateInbox, true},
		{"empty canonical defaults inbox", `{"name":"x","type":"task","state":""}`, models.StateInbox, true},
		{"empty legacy defaults inbox", `{"name":"x","type":"task","section":""}`, models.StateInbox, true},
		{"legacy custom section falls back ready", `{"name":"x","type":"task","section":"Custom"}`, models.StateReady, true},
		{"canonical wins mixed input", `{"name":"x","type":"task","state":"paused","section":"Next"}`, models.StatePaused, true},
		{"canonical only", `{"name":"x","type":"task","state":"review"}`, models.StateReview, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TaskTemplate
			if err := json.Unmarshal([]byte(tt.payload), &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if got.State != tt.want {
				t.Fatalf("State = %q, want %q", got.State, tt.want)
			}
			if changed := got.NormalizeState(); changed != tt.wantRewrite {
				t.Fatalf("NormalizeState changed = %v, want %v", changed, tt.wantRewrite)
			}
		})
	}
}

func TestTaskTemplateJSONRejectsInvalidCanonicalState(t *testing.T) {
	var got TaskTemplate
	if err := json.Unmarshal([]byte(`{"name":"x","type":"task","state":"Backlog"}`), &got); err == nil {
		t.Fatal("Unmarshal accepted invalid canonical state")
	}
	got = TaskTemplate{Name: "x", Type: models.TaskTypeTask, State: models.WorkflowState("Backlog")}
	if _, err := json.Marshal(got); err == nil {
		t.Fatal("Marshal accepted invalid canonical state")
	}
}

func TestNewTemplate_GeneratesID(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)

	if tmpl.ID == "" {
		t.Fatal("ID should not be empty")
	}
	if len(tmpl.ID) != 8 {
		t.Errorf("ID length: got %d, want 8", len(tmpl.ID))
	}
}

func TestNewTemplate_UniqueIDs(t *testing.T) {
	tmpl1 := NewTemplate("A", models.TaskTypeTask)
	tmpl2 := NewTemplate("B", models.TaskTypeTask)

	if tmpl1.ID == tmpl2.ID {
		t.Errorf("two templates should have different IDs, both got %q", tmpl1.ID)
	}
}

func TestNewTemplate_AllTaskTypes(t *testing.T) {
	types := []models.TaskType{
		models.TaskTypeEpic,
		models.TaskTypeStory,
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	for _, tt := range types {
		tmpl := NewTemplate("Test", tt)
		if tmpl.Type != tt {
			t.Errorf("Type: got %q, want %q", tmpl.Type, tt)
		}
	}
}

// --- Validate ---

func TestValidate_ValidTemplate(t *testing.T) {
	tmpl := NewTemplate("Valid", models.TaskTypeTask)
	if err := tmpl.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_NilTemplate(t *testing.T) {
	var tmpl *TaskTemplate
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for nil template")
	}
	if !strings.Contains(err.Error(), "template is nil") {
		t.Errorf("error %q should mention nil template", err.Error())
	}
}

func TestValidate_EmptyName(t *testing.T) {
	tmpl := NewTemplate("", models.TaskTypeTask)
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error %q should mention name is required", err.Error())
	}
}

func TestValidate_WhitespaceName(t *testing.T) {
	tmpl := NewTemplate("   ", models.TaskTypeTask)
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("error %q should mention name is required", err.Error())
	}
}

func TestValidate_InvalidVariableTokenInName(t *testing.T) {
	tmpl := NewTemplate("{{ bad }}", models.TaskTypeTask)
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for invalid variable token")
	}
	if !strings.Contains(err.Error(), "invalid variable token") {
		t.Fatalf("error %q should mention invalid variable token", err)
	}
}

func TestValidate_RejectsInvalidTaskTitleName(t *testing.T) {
	tmpl := NewTemplate("Bad\nName", models.TaskTypeTask)
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for invalid template name")
	}
	if !strings.Contains(err.Error(), "template name:") {
		t.Fatalf("error %q should identify template name", err)
	}
	if !strings.Contains(err.Error(), "task title must not contain newlines") {
		t.Fatalf("error %q should include task-title validation", err)
	}
}

func TestValidate_InvalidTaskType(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskType("invalid"))
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for invalid task type")
	}
	if !strings.Contains(err.Error(), "invalid task type") {
		t.Errorf("error %q should mention invalid task type", err.Error())
	}
}

func TestValidate_AllValidTaskTypes(t *testing.T) {
	types := []models.TaskType{
		models.TaskTypeEpic,
		models.TaskTypeStory,
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	for _, tt := range types {
		tmpl := NewTemplate("Test", tt)
		if err := tmpl.Validate(); err != nil {
			t.Errorf("type %q: unexpected error: %v", tt, err)
		}
	}
}

func TestValidate_PriorityZero(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Priority = 0
	if err := tmpl.Validate(); err != nil {
		t.Errorf("priority 0 should be valid: %v", err)
	}
}

func TestValidate_PriorityValidRange(t *testing.T) {
	for p := 0; p <= models.PriorityMax; p++ {
		tmpl := NewTemplate("Test", models.TaskTypeTask)
		tmpl.Priority = p
		if err := tmpl.Validate(); err != nil {
			t.Errorf("priority %d should be valid: %v", p, err)
		}
	}
}

func TestValidate_PriorityNegative(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Priority = -1
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for negative priority")
	}
	if !strings.Contains(err.Error(), "priority must be") {
		t.Errorf("error %q should mention priority range", err.Error())
	}
}

func TestValidate_PriorityTooHigh(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Priority = models.PriorityMax + 1
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for priority above max")
	}
	if !strings.Contains(err.Error(), "priority must be") {
		t.Errorf("error %q should mention priority range", err.Error())
	}
}

func TestValidate_ChecklistItemWithEmptyText(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Checklist = []string{"Valid item", ""}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for checklist item with empty text")
	}
	if !strings.Contains(err.Error(), "checklist item 2") {
		t.Errorf("error %q should reference checklist item 2", err.Error())
	}
	if !strings.Contains(err.Error(), "text is required") {
		t.Errorf("error %q should mention text is required", err.Error())
	}
}

func TestValidate_ChecklistItemWithWhitespaceText(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Checklist = []string{"  \t  "}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for checklist item with whitespace-only text")
	}
	if !strings.Contains(err.Error(), "checklist item 1") {
		t.Errorf("error %q should reference checklist item 1", err.Error())
	}
}

func TestValidate_ChildWithEmptyTitle(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Valid child"},
		{Title: ""},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for child with empty title")
	}
	if !strings.Contains(err.Error(), "item 2") {
		t.Errorf("error %q should reference item 2", err.Error())
	}
	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("error %q should mention title is required", err.Error())
	}
}

func TestValidate_ChildWithWhitespaceTitle(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "  \t  "},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for child with whitespace-only title")
	}
	if !strings.Contains(err.Error(), "item 1") {
		t.Errorf("error %q should reference item 1", err.Error())
	}
}

func TestValidate_ChildTitleUsesTaskTitleValidation(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Bad\nChild"},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for invalid child title")
	}
	if !strings.Contains(err.Error(), "item 1") {
		t.Fatalf("error %q should reference item 1", err)
	}
	if !strings.Contains(err.Error(), "task title must not contain newlines") {
		t.Fatalf("error %q should include task-title validation", err)
	}
}

func TestValidate_ChildWithInvalidType(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Child", Type: models.TaskType("bogus")},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for child with invalid type")
	}
	if !strings.Contains(err.Error(), "item 1") {
		t.Errorf("error %q should reference item 1", err.Error())
	}
	if !strings.Contains(err.Error(), "invalid task type") {
		t.Errorf("error %q should mention invalid task type", err.Error())
	}
}

func TestValidate_ChildWithEmptyType(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Child", Type: ""},
	}
	// Empty type on a child is allowed (defaults to Task at usage time)
	if err := tmpl.Validate(); err != nil {
		t.Errorf("child with empty type should be valid: %v", err)
	}
}

func TestValidate_ChildWithValidType(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeEpic)
	tmpl.Children = []TemplateChild{
		{Title: "Child", Type: models.TaskTypeStory},
	}
	if err := tmpl.Validate(); err != nil {
		t.Errorf("child with valid type should be valid: %v", err)
	}
}

func TestValidate_ChildTypeIncompatibleWithParent(t *testing.T) {
	// An Epic-under-Epic violates the type hierarchy. Validate should reject
	// this so we don't persist a structurally invalid task tree.
	tmpl := NewTemplate("Test", models.TaskTypeEpic)
	tmpl.Children = []TemplateChild{
		{Title: "Nested Epic", Type: models.TaskTypeEpic},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for epic-under-epic child")
	}
	if !strings.Contains(err.Error(), "item 1") {
		t.Errorf("error %q should reference item 1", err.Error())
	}
}

func TestValidate_ChildrenOnLeafType(t *testing.T) {
	// Tasks/Bugs/Ideas cannot have children. Validate should reject a template
	// of a leaf type that nevertheless declares Children — those children
	// would be silently dropped at use time.
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Children = []TemplateChild{
		{Title: "Sub", Type: models.TaskTypeTask},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for children on leaf-type template")
	}
}

func TestValidate_DuplicateChildTitlesRejected(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Do the thing"},
		{Title: "Do the thing"},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate child title")
	}
	if !strings.Contains(err.Error(), "item 2") {
		t.Errorf("error %q should reference item 2", err.Error())
	}
	if !strings.Contains(err.Error(), "duplicate child title") {
		t.Errorf("error %q should mention duplicate child title", err.Error())
	}
	if !strings.Contains(err.Error(), "item 1") {
		t.Errorf("error %q should cite the first occurrence (item 1)", err.Error())
	}
}

func TestValidate_DuplicateChildTitleThirdItem(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeEpic)
	tmpl.Children = []TemplateChild{
		{Title: "Alpha", Type: models.TaskTypeStory},
		{Title: "Beta", Type: models.TaskTypeStory},
		{Title: "Alpha", Type: models.TaskTypeStory},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate child title at item 3")
	}
	if !strings.Contains(err.Error(), "item 3") {
		t.Errorf("error %q should reference item 3", err.Error())
	}
	if !strings.Contains(err.Error(), `"Alpha"`) {
		t.Errorf("error %q should quote the duplicate title", err.Error())
	}
}

func TestValidate_DuplicateChildTitlesCaseSensitive(t *testing.T) {
	// "Task" and "task" are different titles — must not be rejected.
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Task"},
		{Title: "task"},
	}
	if err := tmpl.Validate(); err != nil {
		t.Errorf("case-distinct titles should be valid: %v", err)
	}
}

func TestValidate_DuplicateChildTitlesIgnoreSurroundingWhitespace(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Deploy"},
		{Title: " Deploy \t"},
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate child title after trimming whitespace")
	}
	if !strings.Contains(err.Error(), "item 2") {
		t.Errorf("error %q should reference item 2", err.Error())
	}
	if !strings.Contains(err.Error(), `"Deploy"`) {
		t.Errorf("error %q should quote the normalized duplicate title", err.Error())
	}
}

func TestValidate_UniqueChildTitlesPass(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "Alpha"},
		{Title: "Beta"},
		{Title: "Gamma"},
	}
	if err := tmpl.Validate(); err != nil {
		t.Errorf("unique child titles should be valid: %v", err)
	}
}

func TestValidate_MultipleErrors_FirstReturned(t *testing.T) {
	// Template with multiple issues: bad name takes precedence
	tmpl := &TaskTemplate{
		Name:     "",
		Type:     models.TaskType("invalid"),
		Priority: -1,
	}
	err := tmpl.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Errorf("first validation error should be about name, got: %v", err)
	}
}

// --- Summary ---

func TestSummary_TypeOnly(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	got := tmpl.Summary()
	if got != "task, →inbox" {
		t.Errorf("Summary: got %q, want %q", got, "task, →inbox")
	}
}

func TestSummary_WithPriority(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeBug)
	tmpl.Priority = 2
	got := tmpl.Summary()
	if got != "bug, P2, →inbox" {
		t.Errorf("Summary: got %q, want %q", got, "bug, P2, →inbox")
	}
}

func TestSummary_PriorityZeroOmitted(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Priority = 0
	got := tmpl.Summary()
	if strings.Contains(got, "P0") {
		t.Errorf("Summary should not include P0 for default priority, got %q", got)
	}
}

func TestSummary_WithTags(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Tags = []string{"backend", "api"}
	got := tmpl.Summary()
	if got != "story, #backend #api, →inbox" {
		t.Errorf("Summary: got %q, want %q", got, "story, #backend #api, →inbox")
	}
}

func TestSummary_WithState(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.State = models.StateNext
	got := tmpl.Summary()
	// Expect the arrow prefix
	if !strings.Contains(got, "→next") {
		t.Errorf("Summary should contain →next, got %q", got)
	}
}

func TestSummary_WithChecklist(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Checklist = []string{"Step 1", "Step 2", "Step 3"}
	got := tmpl.Summary()
	if !strings.Contains(got, "3 checklist items") {
		t.Errorf("Summary should mention checklist items, got %q", got)
	}
}

func TestSummary_WithChildren(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeStory)
	tmpl.Children = []TemplateChild{
		{Title: "A"},
		{Title: "B"},
	}
	got := tmpl.Summary()
	if !strings.Contains(got, "2 items") {
		t.Errorf("Summary should mention 2 items, got %q", got)
	}
}

func TestSummary_ChecklistTakesPrecedenceOverChildren(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Checklist = []string{"X"}
	tmpl.Children = []TemplateChild{{Title: "Y"}}
	got := tmpl.Summary()
	if !strings.Contains(got, "1 checklist item") {
		t.Errorf("Summary should show checklist count, got %q", got)
	}
	if strings.Contains(got, "1 item") && !strings.Contains(got, "checklist") {
		t.Errorf("Summary should not show children count when checklist present, got %q", got)
	}
}

func TestSummary_AllParts(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeEpic)
	tmpl.Priority = 1
	tmpl.Tags = []string{"urgent"}
	tmpl.State = models.StateReview
	tmpl.Children = []TemplateChild{{Title: "Child"}}

	got := tmpl.Summary()
	if got != "epic, P1, #urgent, →review, 1 item" {
		t.Errorf("Summary: got %q, want %q", got, "epic, P1, #urgent, →review, 1 item")
	}
}

// --- GetEffectivePriority ---

func TestGetEffectivePriority_Zero(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Priority = 0
	got := tmpl.GetEffectivePriority()
	if got != models.PriorityDefault {
		t.Errorf("GetEffectivePriority: got %d, want %d", got, models.PriorityDefault)
	}
}

func TestGetEffectivePriority_Set(t *testing.T) {
	for _, p := range []int{1, 2, 3, 4, 5} {
		tmpl := NewTemplate("Test", models.TaskTypeTask)
		tmpl.Priority = p
		got := tmpl.GetEffectivePriority()
		if got != p {
			t.Errorf("priority %d: GetEffectivePriority: got %d, want %d", p, got, p)
		}
	}
}

// --- GetChildType ---

func TestGetChildType_Empty(t *testing.T) {
	child := TemplateChild{Title: "Test", Type: ""}
	got := child.GetChildType()
	if got != models.TaskTypeTask {
		t.Errorf("GetChildType: got %q, want %q", got, models.TaskTypeTask)
	}
}

func TestGetChildType_Set(t *testing.T) {
	types := []models.TaskType{
		models.TaskTypeEpic,
		models.TaskTypeStory,
		models.TaskTypeTask,
		models.TaskTypeBug,
		models.TaskTypeIdea,
	}

	for _, tt := range types {
		child := TemplateChild{Title: "Test", Type: tt}
		got := child.GetChildType()
		if got != tt {
			t.Errorf("type %q: GetChildType: got %q, want %q", tt, got, tt)
		}
	}
}

// --- ContentEqual ---

func TestContentEqual_Identical(t *testing.T) {
	a := &TaskTemplate{
		Name:        "X",
		Category:    "feature",
		Type:        models.TaskTypeStory,
		Priority:    2,
		State:       models.StateNext,
		Description: "desc",
		Tags:        []string{"a", "b"},
		Checklist:   []string{"step1", "step2"},
		Children:    []TemplateChild{{Title: "c1", Type: models.TaskTypeTask, Optional: false}},
	}
	b := &TaskTemplate{
		Name:        "X",
		Category:    "feature",
		Type:        models.TaskTypeStory,
		Priority:    2,
		State:       models.StateNext,
		Description: "desc",
		Tags:        []string{"a", "b"},
		Checklist:   []string{"step1", "step2"},
		Children:    []TemplateChild{{Title: "c1", Type: models.TaskTypeTask, Optional: false}},
	}
	if !a.ContentEqual(b) {
		t.Error("identical templates should be equal")
	}
}

func TestContentEqual_MetadataIgnored(t *testing.T) {
	// ID, Source, Enabled, UseCount are metadata and must not affect equality.
	a := &TaskTemplate{
		ID:       "id-a",
		Source:   "default",
		Enabled:  true,
		UseCount: 100,
		Name:     "X",
		Type:     models.TaskTypeTask,
	}
	b := &TaskTemplate{
		ID:       "id-b",
		Source:   "user",
		Enabled:  false,
		UseCount: 0,
		Name:     "X",
		Type:     models.TaskTypeTask,
	}
	if !a.ContentEqual(b) {
		t.Error("templates differing only in metadata should be equal")
	}
}

func TestContentEqual_TagsReorderedNotEqual(t *testing.T) {
	// Regression: Tags must be compared as an ORDERED list, not a set.
	// "#backend #api" renders differently from "#api #backend" in Summary,
	// so order is user-visible content.
	a := &TaskTemplate{Name: "X", Type: models.TaskTypeTask, Tags: []string{"backend", "api"}}
	b := &TaskTemplate{Name: "X", Type: models.TaskTypeTask, Tags: []string{"api", "backend"}}
	if a.ContentEqual(b) {
		t.Error("templates with reordered Tags must not be equal (Tags are ordered)")
	}
}

func TestContentEqual_TagsDuplicatesSignificant(t *testing.T) {
	// A set-based comparison would treat ["a","a"] == ["a"]. Ordered comparison
	// (correct) treats them as distinct.
	a := &TaskTemplate{Name: "X", Type: models.TaskTypeTask, Tags: []string{"a", "a"}}
	b := &TaskTemplate{Name: "X", Type: models.TaskTypeTask, Tags: []string{"a"}}
	if a.ContentEqual(b) {
		t.Error("duplicate-tag template must not equal single-tag template")
	}
}

func TestContentEqual_FieldDifferences(t *testing.T) {
	base := func() *TaskTemplate {
		return &TaskTemplate{
			Name:        "X",
			Category:    "feature",
			Type:        models.TaskTypeTask,
			Priority:    2,
			State:       models.StateNext,
			Description: "d",
			Tags:        []string{"t"},
			Checklist:   []string{"c"},
			Children:    []TemplateChild{{Title: "c1"}},
		}
	}
	mutators := map[string]func(*TaskTemplate){
		"Name":           func(x *TaskTemplate) { x.Name = "Y" },
		"Category":       func(x *TaskTemplate) { x.Category = "bug" },
		"Type":           func(x *TaskTemplate) { x.Type = models.TaskTypeBug },
		"Priority":       func(x *TaskTemplate) { x.Priority = 3 },
		"State":          func(x *TaskTemplate) { x.State = models.StateInbox },
		"Description":    func(x *TaskTemplate) { x.Description = "d2" },
		"Tags len":       func(x *TaskTemplate) { x.Tags = []string{"t", "u"} },
		"Tags content":   func(x *TaskTemplate) { x.Tags = []string{"u"} },
		"Checklist len":  func(x *TaskTemplate) { x.Checklist = []string{"c", "d"} },
		"Checklist text": func(x *TaskTemplate) { x.Checklist = []string{"d"} },
		"Child Title":    func(x *TaskTemplate) { x.Children[0].Title = "c2" },
		"Child Type":     func(x *TaskTemplate) { x.Children[0].Type = models.TaskTypeStory },
		"Child Optional": func(x *TaskTemplate) { x.Children[0].Optional = true },
		"Children len":   func(x *TaskTemplate) { x.Children = append(x.Children, TemplateChild{Title: "c2"}) },
	}
	for name, mut := range mutators {
		a := base()
		b := base()
		mut(b)
		if a.ContentEqual(b) {
			t.Errorf("%s: mutated template must not equal base", name)
		}
	}
}

func TestContentEqual_ChildrenReorderedNotEqual(t *testing.T) {
	// Children are created in declared order at use time, so ordering is
	// content, not a set membership question.
	a := &TaskTemplate{
		Name: "X", Type: models.TaskTypeStory,
		Children: []TemplateChild{{Title: "first"}, {Title: "second"}},
	}
	b := &TaskTemplate{
		Name: "X", Type: models.TaskTypeStory,
		Children: []TemplateChild{{Title: "second"}, {Title: "first"}},
	}
	if a.ContentEqual(b) {
		t.Error("reordered Children must not be equal")
	}
}

func TestContentEqual_ChecklistReorderedNotEqual(t *testing.T) {
	a := &TaskTemplate{
		Name: "X", Type: models.TaskTypeTask,
		Checklist: []string{"A", "B"},
	}
	b := &TaskTemplate{
		Name: "X", Type: models.TaskTypeTask,
		Checklist: []string{"B", "A"},
	}
	if a.ContentEqual(b) {
		t.Error("reordered Checklist must not be equal")
	}
}

func TestContentEqual_NilHandling(t *testing.T) {
	var nilT *TaskTemplate
	if !nilT.ContentEqual(nil) {
		t.Error("two nil templates should be equal")
	}
	a := &TaskTemplate{Name: "X", Type: models.TaskTypeTask}
	if a.ContentEqual(nil) {
		t.Error("non-nil should not equal nil")
	}
	if nilT.ContentEqual(a) {
		t.Error("nil should not equal non-nil")
	}
}

func TestContentEqual_NilVsEmptySlice(t *testing.T) {
	// Nil and empty slices should be considered equivalent because they are
	// indistinguishable in user-visible output and in JSON (with omitempty).
	a := &TaskTemplate{Name: "X", Type: models.TaskTypeTask, Tags: nil}
	b := &TaskTemplate{Name: "X", Type: models.TaskTypeTask, Tags: []string{}}
	if !a.ContentEqual(b) {
		t.Error("nil and empty Tags should be equal (len 0 == len 0)")
	}
}
