package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/workflow"
)

// statelessTagRule fires on the task's CURRENT status (no prior state needed), so
// it exercises the stateless evaluation branch (Contextual=false).
func statelessTagRule() workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      "r-status-tag",
		Name:    "Tag done tasks",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "shipped"}},
	}
}

// contextualTagRule fires only on a status transition into done, so it requires
// the pre-mutation state (Contextual=true) to trigger.
func contextualTagRule() workflow.WorkflowRule {
	return workflow.WorkflowRule{
		ID:      "r-status-changed-tag",
		Name:    "Tag on completion",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatusChanged, Operation: workflow.OpEquals, Value: "*->done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "shipped"}},
	}
}

func TestStateWithWorkflowStatelessEvaluationFiresRule(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules:      []workflow.WorkflowRule{statelessTagRule()},
		Contextual: false,
		UndoAction: "Complete Task",
	})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if t1.State() != models.StateDone {
		t.Errorf("state = %s, want done", t1.State())
	}
	if !t1.HasTag("shipped") {
		t.Errorf("stateless rule did not apply tag; tags = %v", t1.Tags)
	}
	if res.RuleTriggerCounts["r-status-tag"] != 1 {
		t.Errorf("trigger count = %d, want 1", res.RuleTriggerCounts["r-status-tag"])
	}
	if len(res.Tasks) != 1 || res.Tasks[0].TaskID != "t1" {
		t.Errorf("result tasks = %+v, want single t1", res.Tasks)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestStateWithWorkflowContextualEvaluationFiresChangeRule(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules:      []workflow.WorkflowRule{contextualTagRule()},
		Contextual: true,
	}); err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if !t1.HasTag("shipped") {
		t.Errorf("contextual status-changed rule did not fire; tags = %v", t1.Tags)
	}
}

func TestStateWithWorkflowStatelessDoesNotFireChangeRule(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// A status-changed trigger needs the prior state; with Contextual=false no
	// prior state is captured, so the rule must NOT fire — proving the seam honors
	// the stateless/contextual distinction the legacy secondary-view path relied on.
	if _, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules:      []workflow.WorkflowRule{contextualTagRule()},
		Contextual: false,
	}); err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if t1.HasTag("shipped") {
		t.Errorf("status-changed rule fired under stateless evaluation; tags = %v", t1.Tags)
	}
}

func TestStateWithWorkflowDeferredMoveCommitsOneTransaction(t *testing.T) {
	t1 := newTask("t1", 1, "movable")
	projA := newProject("proj-a", "Alpha", t1)
	projB := newProject("proj-b", "Beta")
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-move-done",
		Name:    "Archive done to Beta",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionMoveToProject, Value: "Beta"}},
	}}

	res, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{Rules: rules})
	if err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if projB.GetTaskByID("t1") == nil {
		t.Fatalf("task not moved to destination; projB tasks = %d", len(projB.Tasks))
	}
	if projA.GetTaskByID("t1") != nil {
		t.Errorf("task still present in source after deferred move")
	}
	if len(repo.commits) != 1 {
		t.Fatalf("Commit called %d times, want 1 (move rides the single-task transaction)", len(repo.commits))
	}
	if got := len(repo.commits[0]); got != 2 {
		t.Errorf("changesets = %d, want 2 (source + destination)", got)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Errorf("TouchedProjectIDs = %v, want two projects", res.TouchedProjectIDs)
	}
}

func TestStateWithWorkflowFollowupAppended(t *testing.T) {
	t1 := newTask("t1", 1, "deploy")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-followup",
		Name:    "Follow up",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionCreateFollowup, Value: "verify {task}"}},
	}}

	if _, err := svc.StateWithWorkflow("t1", models.StateDone, app.SingleTaskWorkflowOptions{Rules: rules}); err != nil {
		t.Fatalf("StateWithWorkflow: %v", err)
	}
	if len(projA.Tasks) != 2 {
		t.Fatalf("project tasks = %d, want 2 (original + follow-up)", len(projA.Tasks))
	}
}

func TestStateWithWorkflowGuardRejectionCommitsNothing(t *testing.T) {
	parent := newTask("p", 1, "parent")
	child := newTask("c", 2, "child")
	pid := "p"
	child.ParentID = &pid
	projA := newProject("proj-a", "Alpha", parent, child)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	// Completing a parent with an incomplete child must be rejected before any
	// commit (the incomplete-children guard).
	_, err := svc.StateWithWorkflow("p", models.StateDone, app.SingleTaskWorkflowOptions{
		Rules: []workflow.WorkflowRule{statelessTagRule()},
	})
	if err == nil {
		t.Fatal("expected the completion to be rejected by the children guard")
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (rejected mutation must not persist)", len(repo.commits))
	}
	if parent.State() == models.StateDone {
		t.Error("parent should not have transitioned when the guard rejects it")
	}
}

func TestSetPriorityWithWorkflowFiresRuleAndCommits(t *testing.T) {
	t1 := newTask("t1", 1, "urgent")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	rules := []workflow.WorkflowRule{{
		ID:      "r-high-pri",
		Name:    "Tag high priority",
		Enabled: true,
		Trigger: workflow.RuleTrigger{Field: workflow.FieldPriority, Operation: workflow.OpEquals, Value: "1"},
		Actions: []workflow.RuleAction{{ActionType: workflow.ActionAddTag, Value: "hot"}},
	}}

	if _, err := svc.SetPriorityWithWorkflow("t1", 1, app.SingleTaskWorkflowOptions{Rules: rules}); err != nil {
		t.Fatalf("SetPriorityWithWorkflow: %v", err)
	}
	if t1.Priority != 1 {
		t.Errorf("priority = %d, want 1", t1.Priority)
	}
	if !t1.HasTag("hot") {
		t.Errorf("priority rule did not fire on a non-state single-task mutation; tags = %v", t1.Tags)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
}

func TestWithWorkflowNoRulesCommitsBareMutation(t *testing.T) {
	t1 := newTask("t1", 1, "plain")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	res, err := svc.SetSectionWithWorkflow("t1", models.SectionSomeday, app.SingleTaskWorkflowOptions{UndoAction: "Move to Section"})
	if err != nil {
		t.Fatalf("SetSectionWithWorkflow: %v", err)
	}
	if res.RuleTriggerCounts != nil {
		t.Errorf("RuleTriggerCounts = %v, want nil with no rules", res.RuleTriggerCounts)
	}
	if len(repo.commits) != 1 {
		t.Errorf("Commit called %d times, want 1", len(repo.commits))
	}
	if got := repo.commits[0][0].UndoAction; got != "Move to Section" {
		t.Errorf("UndoAction = %q, want Move to Section", got)
	}
}
