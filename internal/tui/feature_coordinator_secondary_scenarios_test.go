package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// This file exercises the secondary-view feature coordinators
// (feature_coordinator_secondary.go) end to end. The task/artifact coordinators
// (attachment, bulk) run through the real transactional storage stack and undo
// ledger — the same mutation→reload→render→undo cycle the primary scenarios honor
// — while the config coordinators (settings, workflow) persist through a temp-dir
// config manager and reload from disk. Together they cover the four transactional
// flows session 32 introduced: attachment, bulk, workflow, and settings changes.

// secondaryEnv extends scenarioEnv with the secondary coordinators bound to the
// same application service.
func newSecondaryEnv(t *testing.T, slug string, tasks ...*models.Task) (*scenarioEnv, SecondaryCoordinators) {
	t.Helper()
	env := newScenarioEnv(t, slug, tasks...)
	return env, newSecondaryCoordinators(env.svc, nil)
}

// runBatchResult executes a coordinator command and asserts it produced a
// committed batch-transaction message, returning it.
func runBatchResult(t *testing.T, cmd tea.Cmd) BatchTransactionResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case BatchTransactionResultMsg:
		return msg
	case TransactionErrorMsg:
		t.Fatalf("expected a committed batch, got error: %v", msg.Err)
	default:
		t.Fatalf("expected BatchTransactionResultMsg, got %T", msg)
	}
	return BatchTransactionResultMsg{}
}

// runConfigResult executes a config coordinator command and asserts it produced a
// committed config-transaction message.
func runConfigResult(t *testing.T, cmd tea.Cmd) ConfigTransactionResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case ConfigTransactionResultMsg:
		return msg
	case TransactionErrorMsg:
		t.Fatalf("expected a committed config write, got error: %v", msg.Err)
	default:
		t.Fatalf("expected ConfigTransactionResultMsg, got %T", msg)
	}
	return ConfigTransactionResultMsg{}
}

// TestAttachmentScenario covers the transactional attachment overlay: attaching a
// URL commits the metadata, the reloaded task shows it, and undo removes it.
func TestAttachmentScenario(t *testing.T) {
	task := seedTask(t, 1, "Attach here", models.StateReady)
	env, features := newSecondaryEnv(t, "attach", task)

	res := runResult(t, features.Attachment.AttachURL(task.ID, "https://example.com/spec"))
	if res.Revision == "" {
		t.Fatal("attachment result carried no committed revision")
	}

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Attachments) != 1 {
		t.Fatalf("reloaded task attachments = %v, want 1", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Attachments) != 0 {
		t.Fatalf("undo did not remove the attachment: %v", got)
	}
}

// TestBulkTransitionScenario covers the transactional bulk overlay: transitioning
// two tasks commits one batch, the reloaded project shows both moved, the summary
// counts them, and undo restores their prior states.
func TestBulkTransitionScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateReady)
	b := seedTask(t, 2, "Second", models.StateReady)
	env, features := newSecondaryEnv(t, "bulk", a, b)

	msg := runBatchResult(t, features.Bulk.Transition([]string{a.ID, b.ID}, models.StateInProgress))
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}
	if msg.Summary == "" {
		t.Fatal("batch result carried no summary")
	}

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateInProgress {
			t.Fatalf("task %s state = %v, want in_progress", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateReady {
			t.Fatalf("undo state for %s = %v, want ready", id, got)
		}
	}
}

// TestBulkDeleteScenario covers the transactional bulk delete overlay: deleting two
// subtrees commits one batch, the reloaded project loses both, and undo restores
// them.
func TestBulkDeleteScenario(t *testing.T) {
	a := seedTask(t, 1, "Doomed A", models.StateReady)
	b := seedTask(t, 2, "Doomed B", models.StateReady)
	env, features := newSecondaryEnv(t, "bulkdel", a, b)

	msg := runBatchResult(t, features.Bulk.Delete([]string{a.ID, b.ID}))
	if len(msg.TouchedProjectIDs) == 0 {
		t.Fatal("bulk delete touched no projects")
	}

	reloaded := env.reload(t)
	if reloaded.GetTaskByID(a.ID) != nil || reloaded.GetTaskByID(b.ID) != nil {
		t.Fatal("bulk delete left a task behind")
	}

	env.undo(t)
	reloaded = env.reload(t)
	if reloaded.GetTaskByID(a.ID) == nil || reloaded.GetTaskByID(b.ID) == nil {
		t.Fatal("undo did not restore both deleted tasks")
	}
}

// TestBulkSetPriorityScenario covers the transactional bulk priority overlay:
// setting the priority on two tasks commits one batch, the reloaded project shows
// both changed, and undo restores their prior priorities.
func TestBulkSetPriorityScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateReady)
	b := seedTask(t, 2, "Second", models.StateReady)
	a.Priority = 3
	b.Priority = 3
	env, features := newSecondaryEnv(t, "bulk-priority", a, b)

	msg := runBatchResult(t, features.Bulk.SetPriority([]string{a.ID, b.ID}, 1))
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.Priority != 1 {
			t.Fatalf("task %s priority = %v, want 1", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.Priority != 3 {
			t.Fatalf("undo priority for %s = %v, want 3", id, got)
		}
	}
}

// TestBulkDueDateScenario covers the transactional bulk due-date overlay: setting
// a due date on two tasks commits one batch, clearing it commits another, and undo
// restores the set date.
func TestBulkDueDateScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateReady)
	b := seedTask(t, 2, "Second", models.StateReady)
	env, features := newSecondaryEnv(t, "bulk-due", a, b)

	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	runBatchResult(t, features.Bulk.SetDueDate([]string{a.ID, b.ID}, due, false))

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-08-01" {
			t.Fatalf("task %s due date = %v, want 2026-08-01", id, got)
		}
	}

	runBatchResult(t, features.Bulk.ClearDueDate([]string{a.ID, b.ID}))
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate != nil {
			t.Fatalf("task %s due date not cleared: %v", id, got)
		}
	}

	// Undo the clear: the set date returns.
	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-08-01" {
			t.Fatalf("undo did not restore due date for %s: %v", id, got)
		}
	}
}

// TestBulkSnoozeScenario covers the transactional bulk snooze overlay: snoozing
// two tasks commits one batch, clearing the snooze commits another, and undo
// restores the snooze.
func TestBulkSnoozeScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateReady)
	b := seedTask(t, 2, "Second", models.StateReady)
	env, features := newSecondaryEnv(t, "bulk-snooze", a, b)

	until := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	runBatchResult(t, features.Bulk.Snooze([]string{a.ID, b.ID}, &until))

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.SnoozedUntil == nil || got.SnoozedUntil.Format("2006-01-02") != "2026-08-10" {
			t.Fatalf("task %s snoozed = %v, want 2026-08-10", id, got)
		}
	}

	runBatchResult(t, features.Bulk.Snooze([]string{a.ID, b.ID}, nil))
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.SnoozedUntil != nil {
			t.Fatalf("task %s snooze not cleared: %v", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.SnoozedUntil == nil || got.SnoozedUntil.Format("2006-01-02") != "2026-08-10" {
			t.Fatalf("undo did not restore snooze for %s: %v", id, got)
		}
	}
}

// TestBulkSetTagsScenario covers the transactional bulk tags overlay: replacing
// the tag set on two tasks commits one batch, the reloaded project shows the new
// tags, and undo restores the originals.
func TestBulkSetTagsScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateReady)
	b := seedTask(t, 2, "Second", models.StateReady)
	a.AddTag("old")
	env, features := newSecondaryEnv(t, "bulk-tags", a, b)

	runBatchResult(t, features.Bulk.SetTags([]string{a.ID, b.ID}, []string{"backend", "urgent"}))

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		got := reloaded.GetTaskByID(id)
		if got == nil || len(got.Tags) != 2 || got.Tags[0] != "backend" || got.Tags[1] != "urgent" {
			t.Fatalf("task %s tags = %v, want [backend urgent]", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	if got := reloaded.GetTaskByID(a.ID); got == nil || len(got.Tags) != 1 || got.Tags[0] != "old" {
		t.Fatalf("undo did not restore task a tags: %v", reloaded.GetTaskByID(a.ID))
	}
	if got := reloaded.GetTaskByID(b.ID); got == nil || len(got.Tags) != 0 {
		t.Fatalf("undo did not restore task b tags: %v", reloaded.GetTaskByID(b.ID))
	}
}

// TestSubviewCommentScenario covers the transactional comments panel: adding a
// comment commits it, the reloaded task shows it, and undo removes it.
func TestSubviewCommentScenario(t *testing.T) {
	task := seedTask(t, 1, "Comment here", models.StateReady)
	env, features := newSecondaryEnv(t, "subview-comment", task)

	res := runResult(t, features.Subview.AddComment(task.ID, "found the root cause"))
	if res.Revision == "" {
		t.Fatal("comment result carried no committed revision")
	}

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Comments) != 1 || got.Comments[0].Text != "found the root cause" {
		t.Fatalf("reloaded task comments = %v, want the added comment", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Comments) != 0 {
		t.Fatalf("undo did not remove the comment: %v", got)
	}
}

// TestSubviewChecklistScenario covers the transactional checklist panel: adding
// and toggling an item commit separately, the reloaded task shows the toggled
// item, and undo reverts the toggle while keeping the item.
func TestSubviewChecklistScenario(t *testing.T) {
	task := seedTask(t, 1, "Check here", models.StateReady)
	env, features := newSecondaryEnv(t, "subview-checklist", task)

	runResult(t, features.Subview.AddChecklistItem(task.ID, "write the test"))
	runResult(t, features.Subview.ToggleChecklistItem(task.ID, 0))

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Checklist) != 1 || !got.Checklist[0].Done {
		t.Fatalf("reloaded checklist = %v, want one done item", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Checklist) != 1 || got.Checklist[0].Done {
		t.Fatalf("undo did not revert the toggle: %v", got)
	}
}

// TestTagCatalogScenario covers the transactional tag manager: renaming a
// task-used tag commits catalog and task usages together, and undo restores the
// original name everywhere.
func TestTagCatalogScenario(t *testing.T) {
	task := seedTask(t, 1, "Tagged", models.StateReady)
	task.AddTag("backend")
	env, features := newSecondaryEnv(t, "tag-catalog", task)

	res, err := features.Tag.RenameCatalog(env.project.ID, "backend", "platform")
	if err != nil {
		t.Fatalf("RenameCatalog: %v", err)
	}
	if !res.Changed || res.Tag != "platform" {
		t.Fatalf("rename result = %+v, want changed platform", res)
	}

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Tags) != 1 || got.Tags[0] != "platform" {
		t.Fatalf("reloaded task tags = %v, want [platform]", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Tags) != 1 || got.Tags[0] != "backend" {
		t.Fatalf("undo did not restore the tag name: %v", got)
	}
}

// TestAttachmentReorderScenario covers the transactional attachment reorder: the
// swap commits, the reloaded task shows the new order, and undo restores it.
func TestAttachmentReorderScenario(t *testing.T) {
	task := seedTask(t, 1, "Reorder here", models.StateReady)
	env, features := newSecondaryEnv(t, "attach-reorder", task)

	runResult(t, features.Attachment.AttachURL(task.ID, "https://example.com/a"))
	runResult(t, features.Attachment.AttachURL(task.ID, "https://example.com/b"))
	runResult(t, features.Attachment.Reorder(task.ID, 0, 1))

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Attachments[0].Filename != "https://example.com/b" {
		t.Fatalf("reloaded attachments = %v, want b first", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Attachments[0].Filename != "https://example.com/a" {
		t.Fatalf("undo did not restore the order: %v", got)
	}
}

// TestLinkScenario covers the transactional link surfaces: a bidirectional
// related link commits both sides atomically, unlink removes one side, and undo
// restores it.
func TestLinkScenario(t *testing.T) {
	a := seedTask(t, 1, "Source", models.StateReady)
	b := seedTask(t, 2, "Target", models.StateReady)
	env, features := newSecondaryEnv(t, "links", a, b)

	msg := runBatchResult(t, features.Link.RelateBidirectional(a.ID, b.ID))
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}

	reloaded := env.reload(t)
	if got := reloaded.GetTaskByID(a.ID); got == nil || len(got.RelatedTo) != 1 {
		t.Fatalf("source related = %v, want one link", got)
	}
	if got := reloaded.GetTaskByID(b.ID); got == nil || len(got.RelatedTo) != 1 {
		t.Fatalf("target related = %v, want reciprocal link", got)
	}

	runResult(t, features.Link.RemoveRelated(a.ID, b.ID))
	if got := env.reload(t).GetTaskByID(a.ID); got == nil || len(got.RelatedTo) != 0 {
		t.Fatalf("unlink left source related = %v", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(a.ID); got == nil || len(got.RelatedTo) != 1 {
		t.Fatalf("undo did not restore the link: %v", got)
	}
}

// newConfigEnv builds a temp-dir config manager over a fresh config so the config
// coordinators can persist and reload.
func newConfigEnv(t *testing.T) (*config.Manager, *config.Config, string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), ".tt.json")
	mgr := config.NewManager(configPath)
	cfg := &config.Config{}
	return mgr, cfg, configPath
}

// TestSettingsScenario covers the transactional settings editor: saving a mutated
// editor setting through the settings coordinator persists it, and a fresh manager
// reloads the committed value.
func TestSettingsScenario(t *testing.T) {
	mgr, cfg, configPath := newConfigEnv(t)
	features := newSecondaryCoordinators(&app.Service{}, mgr)

	cfg.Editor = "hx"
	runConfigResult(t, features.Settings.Save(cfg))

	reloaded, err := config.NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Editor != "hx" {
		t.Fatalf("reloaded editor = %q, want hx", reloaded.Editor)
	}
}

// TestWorkflowScenario covers the transactional workflow editor: saving a config
// with an added workflow rule through the workflow coordinator persists it, and a
// fresh manager reloads the committed rule.
func TestWorkflowScenario(t *testing.T) {
	mgr, cfg, configPath := newConfigEnv(t)
	features := newSecondaryCoordinators(&app.Service{}, mgr)

	cfg.WorkflowRules = append(cfg.WorkflowRules, workflow.WorkflowRule{
		ID:      "rule-1",
		Name:    "Auto-ready on capture",
		Enabled: true,
	})
	runConfigResult(t, features.Workflow.Save(cfg))

	reloaded, err := config.NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	rules := reloaded.GetWorkflowRules()
	if len(rules) != 1 || rules[0].ID != "rule-1" {
		t.Fatalf("reloaded workflow rules = %+v, want one rule-1", rules)
	}
}

// TestSettingsSaveError covers the error leg: a failing gateway surfaces a
// TransactionErrorMsg rather than a committed result.
func TestSettingsSaveError(t *testing.T) {
	// Point the manager at a path whose parent is a file, so MkdirAll fails.
	bad := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	mgr := config.NewManager(filepath.Join(bad, "nested", ".tt.json"))
	features := newSecondaryCoordinators(&app.Service{}, mgr)

	if err := runError(t, features.Settings.Save(&config.Config{})); err == nil {
		t.Fatal("expected a save error through a broken gateway")
	}
}
