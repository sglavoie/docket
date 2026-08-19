package tui

import (
	"testing"
)

// TestAdoptChildrenPickerOmitsTheTasksItAlreadyParents is the structural half of
// the additive contract: with no row for an existing child, no selection state
// in the picker can ask for a detach, which `<child` has no form for.
func TestAdoptChildrenPickerOmitsTheTasksItAlreadyParents(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureRich)
	parent, project := taskContentFixtureTask(t, m, "User authentication")

	primary := make(map[string]ListItem)
	for _, item := range operationChildListSource(parent, project).Items {
		primary[item.PrimaryText] = item
	}

	for _, absent := range []string{
		"User authentication",
		"Build login form",
		"Add password validation",
	} {
		if _, ok := primary[absent]; ok {
			t.Errorf("Adopt Children offers %q, which the task is or already parents", absent)
		}
	}
	if _, ok := primary["Design login mockup"]; ok {
		t.Error("Adopt Children offers a completed task")
	}

	ancestor, ok := primary["Launch platform"]
	if !ok {
		t.Fatal("Adopt Children omits the ancestor instead of refusing it in place")
	}
	if ancestor.Enabled() {
		t.Error("adopting an ancestor is offered as an available row")
	}
	candidate, ok := primary["Triage support inbox"]
	if !ok {
		t.Fatal("Adopt Children omits an ordinary candidate")
	}
	if !candidate.Enabled() {
		t.Errorf("ordinary candidate is disabled: %s", candidate.DisabledReason)
	}
}

// TestAdoptChildrenWithNothingCheckedKeepsEveryExistingChild is the behavioral
// half: an empty checked set is a no-op, never the `<0` detach FieldChild
// refuses to declare.
func TestAdoptChildrenWithNothingCheckedKeepsEveryExistingChild(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureRich)
	parent, project := taskContentFixtureTask(t, m, "User authentication")
	child, _ := taskContentFixtureTask(t, m, "Build login form")

	cmd := m.adoptChildrenCmd(taskIdentityFor(parent, project), nil, nil)
	if cmd == nil {
		t.Fatal("adopting nothing produced no command")
	}
	msg, ok := cmd().(TaskLineEditedMsg)
	if !ok {
		t.Fatalf("adopting nothing = %T, want an edit result", cmd())
	}
	if msg.Result.Changed {
		t.Error("adopting nothing committed a transaction")
	}

	m.reloadProjectByID(project.ID)
	reloaded := m.projectByID(project.ID).GetTaskByID(child.ID)
	if reloaded.ParentID == nil || *reloaded.ParentID != parent.ID {
		t.Errorf("existing child parent = %v, want it still under %s", reloaded.ParentID, parent.ID)
	}
}
