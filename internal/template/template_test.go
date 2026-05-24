package template

import (
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
	if tmpl.Tags == nil || len(tmpl.Tags) != 0 {
		t.Errorf("Tags: got %v, want empty slice", tmpl.Tags)
	}
	if tmpl.Children == nil || len(tmpl.Children) != 0 {
		t.Errorf("Children: got %v, want empty slice", tmpl.Children)
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
	if got != "task" {
		t.Errorf("Summary: got %q, want %q", got, "task")
	}
}

func TestSummary_WithPriority(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeBug)
	tmpl.Priority = 2
	got := tmpl.Summary()
	if got != "bug, P2" {
		t.Errorf("Summary: got %q, want %q", got, "bug, P2")
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
	if got != "story, #backend #api" {
		t.Errorf("Summary: got %q, want %q", got, "story, #backend #api")
	}
}

func TestSummary_WithSection(t *testing.T) {
	tmpl := NewTemplate("Test", models.TaskTypeTask)
	tmpl.Section = "Next"
	got := tmpl.Summary()
	// Expect the arrow prefix
	if !strings.Contains(got, "→Next") {
		t.Errorf("Summary should contain →Next, got %q", got)
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
	tmpl.Section = "Now"
	tmpl.Children = []TemplateChild{{Title: "Child"}}

	got := tmpl.Summary()
	if got != "epic, P1, #urgent, →Now, 1 item" {
		t.Errorf("Summary: got %q, want %q", got, "epic, P1, #urgent, →Now, 1 item")
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
