package taskform

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// repeatSeed is the seed a repeat draft carries, spelled once so every test
// below starts from the same complete value.
func repeatSeed() TaskComposerSeed {
	return TaskComposerSeed{
		HostTitle:      "Repeat Task",
		Title:          "Ship the quarterly report",
		Description:    "Pull the numbers, then circulate.",
		Priority:       2,
		Tags:           []string{"finance", "recurring"},
		Context:        "office",
		ChecklistItems: []string{"Pull the numbers", "Draft the summary", "Circulate"},
		Destination:    app.CaptureDestinationBacklog,
	}
}

// TestComposerSeedOpensExpandedWithEveryRetainedField covers the retained half
// of the matrix at the surface, including the live input values: the draft
// fields alone would not prove it, because commitAllTextInputs overwrites title
// and context from the inputs at save time.
func TestComposerSeedOpensExpandedWithEveryRetainedField(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := repeatSeed()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)

	if composer.Mode != ComposerExpanded || composer.Expanded == nil {
		t.Fatal("a seeded preset did not open the expanded form")
	}
	form := composer.Expanded
	if form.hostTitle != "Repeat Task" {
		t.Errorf("hostTitle = %q, want Repeat Task", form.hostTitle)
	}
	if form.draft.title != seed.Title || form.titleInput.Value() != seed.Title {
		t.Errorf("title draft/input = %q/%q, want %q", form.draft.title, form.titleInput.Value(), seed.Title)
	}
	if form.draft.desc != seed.Description {
		t.Errorf("description = %q, want %q", form.draft.desc, seed.Description)
	}
	if form.draft.priority != seed.Priority {
		t.Errorf("priority = %d, want %d", form.draft.priority, seed.Priority)
	}
	if !slices.Equal(form.draft.tags, seed.Tags) {
		t.Errorf("tags = %v, want %v", form.draft.tags, seed.Tags)
	}
	if form.draft.context != seed.Context || form.contextInput.Value() != seed.Context {
		t.Errorf("context draft/input = %q/%q, want %q", form.draft.context, form.contextInput.Value(), seed.Context)
	}
	if !form.checklistExpanded {
		t.Error("a seeded checklist opened collapsed; the copied items are invisible without a keystroke")
	}
}

// TestComposerSeedResetsTheChecklistInOrder is the one retained field with a
// reset inside it: the texts arrive in stored order and every tick is dropped.
func TestComposerSeedResetsTheChecklistInOrder(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := repeatSeed()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)

	got := composer.Expanded.draft.checklist
	if len(got) != len(seed.ChecklistItems) {
		t.Fatalf("checklist has %d items, want %d", len(got), len(seed.ChecklistItems))
	}
	for i, item := range got {
		if item.Text != seed.ChecklistItems[i] {
			t.Errorf("checklist[%d] = %q, want %q (stored order)", i, item.Text, seed.ChecklistItems[i])
		}
		if item.Done {
			t.Errorf("checklist[%d] %q arrived ticked; a repeated checklist starts unticked", i, item.Text)
		}
	}
}

// TestComposerSeedClearsEveryOccurrenceField covers the cleared half at the
// surface. These are the form's created-draft defaults rather than assignments,
// which is exactly why they are asserted: a seeding path that started writing
// them would break this without breaking anything it touched.
func TestComposerSeedClearsEveryOccurrenceField(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := repeatSeed()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)
	form := composer.Expanded

	if form.draft.parentID != nil {
		t.Errorf("parentID = %v, want nil: a repetition starts at the project root", *form.draft.parentID)
	}
	if form.draft.state != models.StateBacklog {
		t.Errorf("state = %q, want backlog", form.draft.state)
	}
	if form.draft.dueDate != nil || form.draft.dueInput != "" || form.dueTextInput.Value() != "" {
		t.Errorf("due date = %v / %q / %q, want cleared",
			form.draft.dueDate, form.draft.dueInput, form.dueTextInput.Value())
	}
	if form.draft.pinned || form.draft.review {
		t.Errorf("pinned/review = %v/%v, want both cleared", form.draft.pinned, form.draft.review)
	}
	if len(form.draft.subtasks) != 0 {
		t.Errorf("subtasks = %v, want none: children are not copied", form.draft.subtasks)
	}
	if len(form.draft.attachments) != 0 || len(form.draft.comments) != 0 {
		t.Errorf("attachments/comments = %v/%v, want none", form.draft.attachments, form.draft.comments)
	}
	// presetDestination is Backlog rather than a planning destination, which is
	// what stops saveCreate from routing the new task into Today or Next.
	if form.presetDestination.IsPlanning() {
		t.Errorf("presetDestination = %q, want a non-planning destination", form.presetDestination)
	}
}

// TestComposerSeedKeepsTitleLiteral is the parser-avoidance claim at the
// surface: a seeded title must not travel through the capture grammar, so a
// literal `#123` stays in the title instead of becoming a tag.
func TestComposerSeedKeepsTitleLiteral(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := TaskComposerSeed{
		HostTitle:      "Repeat Task",
		Title:          "Fix bug #123 $today @tomorrow *pin !1 — café ☕ 日本語",
		Description:    "Mentions %billing and ~task-9",
		ChecklistItems: []string{"Review !1 items ✓"},
	}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)
	form := composer.Expanded

	if form.draft.title != seed.Title {
		t.Errorf("title = %q, want the literal %q", form.draft.title, seed.Title)
	}
	if form.titleInput.Value() != seed.Title {
		t.Errorf("title input = %q, want the literal %q", form.titleInput.Value(), seed.Title)
	}
	if len(form.draft.tags) != 0 {
		t.Errorf("tags = %v, want none: the sigils in the title were parsed", form.draft.tags)
	}
	if form.draft.priority != models.PriorityUnset {
		t.Errorf("priority = %d, want unset: `!1` in the title was parsed", form.draft.priority)
	}
	if form.draft.pinned {
		t.Error("`*pin` in the title was parsed")
	}
	if form.draft.dueDate != nil {
		t.Error("`@tomorrow` in the title was parsed")
	}
	if form.draft.checklist[0].Text != "Review !1 items ✓" {
		t.Errorf("checklist item = %q, want the literal text", form.draft.checklist[0].Text)
	}
}

// TestComposerSeedOwnsItsSlices pins the memory-independence claim across the
// last hop: the form must not share backing arrays with the seed, or a later
// edit to either would reach the other.
func TestComposerSeedOwnsItsSlices(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := repeatSeed()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)
	form := composer.Expanded

	seed.Tags[0] = "mutated"
	seed.ChecklistItems[0] = "mutated"
	if form.draft.tags[0] != "finance" {
		t.Errorf("form tag = %q, want finance: the form aliases the seed's tags", form.draft.tags[0])
	}
	if form.draft.checklist[0].Text != "Pull the numbers" {
		t.Errorf("form checklist item = %q, want the seeded text: the form aliases the seed",
			form.draft.checklist[0].Text)
	}
}

// TestComposerSeedDefaultsToBacklog states the empty-destination rule: a seed is
// a considered starting point, so it is never Inbox the way a bare capture is.
func TestComposerSeedDefaultsToBacklog(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := TaskComposerSeed{Title: "No destination named"}
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)

	if composer.Expanded.draft.state != models.StateBacklog {
		t.Errorf("state = %q, want backlog", composer.Expanded.draft.state)
	}
}

// TestComposerSeedRendersAndSizesThroughTheExpandedForm proves a composer
// constructed straight into expanded mode needs no extra wiring: the delegating
// accessors all reach the embedded form the way ExpandSingle's do.
func TestComposerSeedRendersAndSizesThroughTheExpandedForm(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	seed := repeatSeed()
	composer := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project, Seed: &seed}, runtime)

	composer.SetSize(100, 30)
	if composer.Expanded.width != 100 || composer.Expanded.height != 30 {
		t.Fatalf("expanded form size = %dx%d, want 100x30", composer.Expanded.width, composer.Expanded.height)
	}
	rendered := ansi.Strip(composer.View().Content)
	if !strings.Contains(rendered, "Repeat Task") {
		t.Errorf("the seeded overlay does not name itself:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Ship the quarterly report") {
		t.Errorf("the seeded title is not rendered:\n%s", rendered)
	}
	// The expanded form draws no footer: its keys reach the shell through the
	// composer, and they are the form's rather than the compact panel's.
	if strings.Contains(rendered, "Ctrl+S save") {
		t.Errorf("the seeded form drew its own footer:\n%s", rendered)
	}
	spec, guided := composer.ContextualGuidance()
	if !guided {
		t.Fatal("a seeded composer published no guidance, so the shell footer would fall back to a breadcrumb")
	}
	published := ansi.Strip(chrome.RenderGuidanceText(spec, 100))
	if !strings.Contains(published, "Ctrl+S save") || strings.Contains(published, "Enter create") {
		t.Errorf("a seeded composer published %q; want the expanded form's keys", published)
	}
}

// TestComposerSeedSavesOneNewTaskWithFreshIdentity is the end of the contract:
// the ordinary creation transaction mints identity, short ID, timestamps, and
// the checklist, and it does so exactly once.
func TestComposerSeedSavesOneNewTaskWithFreshIdentity(t *testing.T) {
	env := newScenarioEnv(t, "repeat")
	cfg := newTestConfig()
	seed := repeatSeed()
	composer := NewTaskComposerModel(
		TaskComposerPreset{Kind: ComposerProject, Project: env.project, Seed: &seed},
		testRuntimeAt(cfg.DataDir(), cfg),
	)
	composer.SetTaskFormCoordinator(env.features.TaskForm)

	msg := runCreate(t, composer.Expanded)

	loaded := env.reload(t)
	if len(loaded.Tasks) != 1 {
		t.Fatalf("project holds %d tasks, want exactly the one the save created", len(loaded.Tasks))
	}
	task := loaded.GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("the created task is absent from the reloaded project")
	}
	if task.ID == "" || task.ShortID == 0 {
		t.Errorf("identity = %q/#%d, want freshly generated values", task.ID, task.ShortID)
	}
	if task.CreatedAt.IsZero() || task.StateChangedAt.IsZero() {
		t.Errorf("timestamps = %v/%v, want the creation transaction's", task.CreatedAt, task.StateChangedAt)
	}
	if task.Title != seed.Title || task.Description != seed.Description {
		t.Errorf("title/description = %q/%q, want the seeded values", task.Title, task.Description)
	}
	if task.State() != models.StateBacklog {
		t.Errorf("state = %q, want backlog", task.State())
	}
	if task.DueDate != nil || task.DeferredUntil != nil || task.Pinned || task.Review || task.ParentID != nil {
		t.Error("the created task carried an occurrence field the seed cleared")
	}
	if len(task.Comments) != 0 || len(task.Attachments) != 0 || len(task.Dependencies) != 0 {
		t.Error("the created task carried comments, attachments, or dependencies")
	}
	if len(task.Checklist) != len(seed.ChecklistItems) {
		t.Fatalf("checklist has %d items, want %d persisted", len(task.Checklist), len(seed.ChecklistItems))
	}
	for i, item := range task.Checklist {
		if item.Text != seed.ChecklistItems[i] || item.Done {
			t.Errorf("persisted checklist[%d] = %q done=%v, want %q unticked",
				i, item.Text, item.Done, seed.ChecklistItems[i])
		}
	}

	// One undo entry covers the whole creation, and redo puts it back: the draft
	// added no ledger entry of its own on the way in.
	env.undo(t)
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("after undo the project holds %d tasks, want none", got)
	}
	env.redo(t)
	if got := len(env.reload(t).Tasks); got != 1 {
		t.Fatalf("after redo the project holds %d tasks, want one", got)
	}
}

// TestComposerSeedEditedBeforeSaveCommitsTheEdits is the "the draft is yours"
// half: what lands is the edited draft, not the seed it started from.
func TestComposerSeedEditedBeforeSaveCommitsTheEdits(t *testing.T) {
	env := newScenarioEnv(t, "repeat-edit")
	cfg := newTestConfig()
	seed := repeatSeed()
	composer := NewTaskComposerModel(
		TaskComposerPreset{Kind: ComposerProject, Project: env.project, Seed: &seed},
		testRuntimeAt(cfg.DataDir(), cfg),
	)
	composer.SetTaskFormCoordinator(env.features.TaskForm)

	form := composer.Expanded
	form.titleInput.SetValue("Ship the Q3 report")
	form.draft.priority = 1
	form.draft.checklist = append(form.draft.checklist, models.ChecklistItem{Text: "File it"})

	msg := runCreate(t, form)

	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("the created task is absent from the reloaded project")
	}
	if task.Title != "Ship the Q3 report" {
		t.Errorf("title = %q, want the edited value", task.Title)
	}
	if task.Priority != 1 {
		t.Errorf("priority = %d, want the edited value", task.Priority)
	}
	if len(task.Checklist) != 4 || task.Checklist[3].Text != "File it" {
		t.Errorf("checklist = %+v, want the appended item persisted", task.Checklist)
	}
}

// TestComposerSeedFailedSaveKeepsTheDraftAndRetriesOnce covers the two failure
// properties together: a refusal leaves the draft open and editable, and the
// guard that survives it still admits exactly one submission per attempt, so a
// retry cannot double-create.
func TestComposerSeedFailedSaveKeepsTheDraftAndRetriesOnce(t *testing.T) {
	env := newScenarioEnv(t, "repeat-retry")
	cfg := newTestConfig()
	seed := repeatSeed()
	composer := NewTaskComposerModel(
		TaskComposerPreset{Kind: ComposerProject, Project: env.project, Seed: &seed},
		testRuntimeAt(cfg.DataDir(), cfg),
	)
	composer.SetTaskFormCoordinator(env.features.TaskForm)
	form := composer.Expanded

	// An empty title is the cheapest refusal that reaches the same guard a
	// storage failure would.
	form.titleInput.SetValue("   ")
	if cmd := form.handleSave(); cmd != nil {
		t.Fatal("an empty title saved")
	}
	if form.err == nil {
		t.Fatal("the refused save reported no error")
	}
	if form.submissionInFlight {
		t.Fatal("a refused save armed the in-flight guard, so no retry is possible")
	}
	if !slices.Equal(checklistTexts(form.draft.checklist), seed.ChecklistItems) {
		t.Errorf("the refused save lost the draft's checklist: %+v", form.draft.checklist)
	}
	if form.draft.desc != seed.Description {
		t.Errorf("the refused save lost the draft's description: %q", form.draft.desc)
	}
	if got := len(env.reload(t).Tasks); got != 0 {
		t.Fatalf("the refused save created %d tasks", got)
	}

	form.titleInput.SetValue(seed.Title)
	if _, err := runCreate(t, form), form.err; err != nil {
		t.Fatalf("the retry failed: %v", err)
	}
	if cmd := form.handleSave(); cmd != nil {
		t.Fatal("a second save after a successful one was admitted; the creation would duplicate")
	}
	if got := len(env.reload(t).Tasks); got != 1 {
		t.Fatalf("project holds %d tasks, want exactly one", got)
	}
}

// checklistTexts projects a draft checklist onto its texts for comparison.
func checklistTexts(items []models.ChecklistItem) []string {
	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, item.Text)
	}
	return texts
}
