package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func childIDsOf(result *quickEditResult) []string {
	ids := make([]string, len(result.children))
	for i, child := range result.children {
		ids[i] = child.ID
	}
	return ids
}

func TestParseQuickEditChildrenAreOrderedUniqueAndAdditive(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", WorkflowState: models.StateBacklog}
	existing := &models.Task{ID: "existing", ShortID: 2, Title: "Existing child", ParentID: &task.ID, WorkflowState: models.StateBacklog}
	login := &models.Task{ID: "login", ShortID: 3, Title: "Login Flow", WorkflowState: models.StateBacklog}
	other := &models.Task{ID: "other", ShortID: 4, Title: "Other work", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task, existing, login, other}}
	project.BuildTaskTree()

	result, err := parseQuickEdit("Owner <4 <Login <3", task, project)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	if got := childIDsOf(result); len(got) != 2 || got[0] != other.ID || got[1] != login.ID {
		t.Fatalf("children = %v, want [other login]", got)
	}
	if login.ParentID != nil || other.ParentID != nil {
		t.Fatalf("resolution reparented live tasks: login=%v other=%v", login.ParentID, other.ParentID)
	}
	if existing.ParentID == nil || *existing.ParentID != task.ID {
		t.Fatalf("existing child lost its parent: %v", existing.ParentID)
	}
	previewIDs := make([]string, len(result.preview.Children))
	for i, child := range result.preview.Children {
		previewIDs[i] = child.ID
	}
	if len(previewIDs) != 3 || previewIDs[0] != existing.ID || previewIDs[1] != other.ID || previewIDs[2] != login.ID {
		t.Fatalf("preview children = %v, want [existing other login]", previewIDs)
	}
	if len(task.Children) != 1 || task.Children[0] != existing {
		t.Fatalf("live child collection mutated: %+v", task.Children)
	}
}

func TestParseQuickEditChildResolutionAndConflictErrors(t *testing.T) {
	grandparent := &models.Task{ID: "grandparent", ShortID: 8, Title: "Grandparent", WorkflowState: models.StateBacklog}
	parent := &models.Task{ID: "parent", ShortID: 9, Title: "Parent", ParentID: &grandparent.ID, WorkflowState: models.StateBacklog}
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", ParentID: &parent.ID, WorkflowState: models.StateBacklog}
	sharedA := &models.Task{ID: "shared-a", ShortID: 3, Title: "Shared alpha", WorkflowState: models.StateBacklog}
	sharedB := &models.Task{ID: "shared-b", ShortID: 4, Title: "Shared beta", WorkflowState: models.StateBacklog}
	free := &models.Task{ID: "free", ShortID: 5, Title: "Free", WorkflowState: models.StateBacklog}
	remote := &models.Task{ID: "remote", ShortID: 6, Title: "Remote", ParentID: &free.ID, WorkflowState: models.StateBacklog}
	project := &models.Project{
		ID:    "project",
		Slug:  "project",
		Tasks: []*models.Task{grandparent, parent, task, sharedA, sharedB, free, remote},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "unknown", input: "Owner <999", want: "child not found"},
		{name: "zero is not a clear", input: "Owner <0", want: `child not found: "0"`},
		{name: "ambiguous", input: "Owner <Shared", want: "ambiguous"},
		{name: "self", input: "Owner <1", want: "task cannot be its own child"},
		{name: "parent and child", input: "Owner >5 <5", want: "#5 cannot be both the parent and a child"},
		{name: "direct ancestor", input: "Owner <9", want: "child #9 is an ancestor of the edited task"},
		{name: "distant ancestor", input: "Owner <8", want: "child #8 is an ancestor of the edited task"},
		{name: "ancestor only through the parent patch", input: "Owner >6 <5", want: "child #5 is an ancestor of the edited task"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseQuickEdit(tt.input, task, project)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseQuickEdit(%q) error = %v, want %q", tt.input, err, tt.want)
			}
		})
	}
}

func TestParseQuickEditCombinedParentAndChildInversionIsAccepted(t *testing.T) {
	parent := &models.Task{ID: "parent", ShortID: 2, Title: "Parent", WorkflowState: models.StateBacklog}
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", ParentID: &parent.ID, WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{parent, task}}

	if _, err := parseQuickEdit("Owner <2", task, project); err == nil {
		t.Fatal("adopting the current parent without clearing it must be a cycle")
	}
	result, err := parseQuickEdit("Owner >0 <2", task, project)
	if err != nil {
		t.Fatalf("hierarchy inversion rejected: %v", err)
	}
	if result.parentMode != quickEditParentRoot {
		t.Fatalf("parent mode = %v, want root", result.parentMode)
	}
	if got := childIDsOf(result); len(got) != 1 || got[0] != parent.ID {
		t.Fatalf("children = %v, want [parent]", got)
	}
	if task.ParentID == nil || *task.ParentID != parent.ID || parent.ParentID != nil {
		t.Fatalf("inversion mutated live tasks: task=%v parent=%v", task.ParentID, parent.ParentID)
	}
}

func TestParseQuickEditChildDepthUsesFinalGraph(t *testing.T) {
	var chain []*models.Task
	var previous *models.Task
	for depth := 0; depth <= 5; depth++ {
		node := &models.Task{
			ID:            fmt.Sprintf("depth-%d", depth),
			ShortID:       depth + 1,
			Title:         fmt.Sprintf("Depth %d", depth),
			WorkflowState: models.StateBacklog,
		}
		if previous != nil {
			node.ParentID = &previous.ID
		}
		chain = append(chain, node)
		previous = node
	}
	task := previous
	leaf := &models.Task{ID: "leaf", ShortID: 20, Title: "Leaf", WorkflowState: models.StateBacklog}
	tall := &models.Task{ID: "tall", ShortID: 21, Title: "Tall", WorkflowState: models.StateBacklog}
	tallChild := &models.Task{ID: "tall-child", ShortID: 22, Title: "Tall child", ParentID: &tall.ID, WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: append(chain, leaf, tall, tallChild)}

	if _, err := parseQuickEdit("Depth 5 <20", task, project); err != nil {
		t.Fatalf("leaf adoption at the depth limit rejected: %v", err)
	}
	_, err := parseQuickEdit("Depth 5 <21", task, project)
	if err == nil || !strings.Contains(err.Error(), "child #21 would make the subtree exceed maximum nesting depth of 6 levels") {
		t.Fatalf("tall adoption error = %v, want depth refusal", err)
	}
}

func TestParseQuickEditCrossProjectChildDepthAndScope(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Movable", WorkflowState: models.StateBacklog}
	sourceOnly := &models.Task{ID: "source-only", ShortID: 7, Title: "Source only", WorkflowState: models.StateBacklog}
	source := &models.Project{ID: "source", Slug: "source", Tasks: []*models.Task{task, sourceOnly}}

	var chain []*models.Task
	var previous *models.Task
	for depth := 0; depth <= 4; depth++ {
		node := &models.Task{
			ID:            fmt.Sprintf("anchor-%d", depth),
			ShortID:       depth + 1,
			Title:         fmt.Sprintf("Anchor %d", depth),
			WorkflowState: models.StateBacklog,
		}
		if previous != nil {
			node.ParentID = &previous.ID
		}
		chain = append(chain, node)
		previous = node
	}
	leaf := &models.Task{ID: "leaf", ShortID: 20, Title: "Leaf", WorkflowState: models.StateBacklog}
	tall := &models.Task{ID: "tall", ShortID: 21, Title: "Tall", WorkflowState: models.StateBacklog}
	tallChild := &models.Task{ID: "tall-child", ShortID: 22, Title: "Tall child", ParentID: &tall.ID, WorkflowState: models.StateBacklog}
	destination := &models.Project{ID: "destination", Slug: "destination", Tasks: append(chain, leaf, tall, tallChild)}
	projects := []*models.Project{source, destination}

	result, err := parseQuickEdit("Movable %destination >5 <20", task, source, projects)
	if err != nil {
		t.Fatalf("cross-project leaf adoption rejected: %v", err)
	}
	if got := childIDsOf(result); len(got) != 1 || got[0] != leaf.ID {
		t.Fatalf("children = %v, want [leaf]", got)
	}

	_, err = parseQuickEdit("Movable %destination >5 <21", task, source, projects)
	if err == nil || !strings.Contains(err.Error(), "exceed maximum nesting depth") {
		t.Fatalf("cross-project tall adoption error = %v, want depth refusal", err)
	}

	_, err = parseQuickEdit("Movable %destination <7", task, source, projects)
	if err == nil || !strings.Contains(err.Error(), "child not found") {
		t.Fatalf("source-project child error = %v, want refusal in the effective project", err)
	}
}

func TestQuickEditPreviewChildAdoptionIsPure(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", WorkflowState: models.StateBacklog}
	adopted := &models.Task{ID: "adopted", ShortID: 2, Title: "Adopted", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task, adopted}}
	project.BuildTaskTree()
	updated := task.UpdatedAt

	result, err := parseQuickEdit("Renamed <2", task, project)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	if task.Title != "Owner" || task.UpdatedAt != updated || len(task.Children) != 0 {
		t.Fatalf("edited task mutated during preview: %+v", task)
	}
	if adopted.ParentID != nil || adopted.Parent != nil {
		t.Fatalf("adopted task mutated during preview: parentID=%v parent=%v", adopted.ParentID, adopted.Parent)
	}
	if len(result.preview.Children) != 1 || result.preview.Children[0] != adopted {
		t.Fatalf("preview children = %+v", result.preview.Children)
	}
}

func quickEditAdoptionFixture(taskState, childState models.WorkflowState) (*models.Task, *models.Project) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", WorkflowState: taskState}
	child := &models.Task{ID: "child", ShortID: 2, Title: "Adopted", WorkflowState: childState}
	return task, &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task, child}}
}

// TestQuickEditTerminalDestinationRefusesIncompleteAdoptedChild pins the one
// terminal-adoption policy on the inline-edit preview, including the case where
// the destination names the state the task already holds: an explicit $done is an
// explicit terminal claim whether or not it moves the state, so the preview must
// report the refusal the application layer will raise rather than predicting the
// reconciliation to Doing.
func TestQuickEditTerminalDestinationRefusesIncompleteAdoptedChild(t *testing.T) {
	for destination, terminal := range map[string]models.WorkflowState{
		"done":     models.StateDone,
		"canceled": models.StateCanceled,
	} {
		t.Run("open adopted child blocks "+destination, func(t *testing.T) {
			task, project := quickEditAdoptionFixture(models.StateBacklog, models.StateBacklog)
			_, err := parseQuickEdit("Owner <2 $"+destination, task, project)
			want := "has incomplete children and cannot move to " + destination
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %v, want %q", err, want)
			}
		})

		t.Run("open adopted child blocks a re-applied "+destination, func(t *testing.T) {
			task, project := quickEditAdoptionFixture(terminal, models.StateBacklog)
			_, err := parseQuickEdit("Owner <2 $"+destination, task, project)
			want := "has incomplete children and cannot move to " + destination
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("error = %v, want %q", err, want)
			}
		})

		t.Run("finished adopted child allows "+destination, func(t *testing.T) {
			task, project := quickEditAdoptionFixture(models.StateBacklog, models.StateDone)
			result, err := parseQuickEdit("Owner <2 $"+destination, task, project)
			if err != nil {
				t.Fatalf("completed child blocked a terminal destination: %v", err)
			}
			if result.preview.State() != terminal {
				t.Fatalf("preview state = %s, want %s", result.preview.State(), terminal)
			}

			task, project = quickEditAdoptionFixture(terminal, models.StateDone)
			result, err = parseQuickEdit("Owner <2 $"+destination, task, project)
			if err != nil {
				t.Fatalf("re-applied terminal destination: %v", err)
			}
			if result.preview.State() != terminal {
				t.Fatalf("re-applied preview state = %s, want %s", result.preview.State(), terminal)
			}
		})
	}
}

func TestQuickEditPreviewTerminalLifecycleWithAdoptedChildren(t *testing.T) {
	task, project := quickEditAdoptionFixture(models.StateDone, models.StateBacklog)
	result, err := parseQuickEdit("Owner <2", task, project)
	if err != nil {
		t.Fatalf("terminal parent adopting an open child: %v", err)
	}
	if result.preview.State() != models.StateDoing || result.state != models.StateDoing {
		t.Fatalf("preview state = %s result state = %s, want doing", result.preview.State(), result.state)
	}
	if task.State() != models.StateDone {
		t.Fatalf("reconciliation preview mutated the live task: %s", task.State())
	}
}

func TestTaskLineEditRequestQualifiesChildLocators(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", WorkflowState: models.StateBacklog}
	sibling := &models.Task{ID: "sibling", ShortID: 2, Title: "Sibling", WorkflowState: models.StateBacklog}
	source := &models.Project{ID: "source", Slug: "source", Tasks: []*models.Task{task, sibling}}
	destinationChild := &models.Task{ID: "destination-child", ShortID: 7, Title: "Destination child", WorkflowState: models.StateBacklog}
	destination := &models.Project{ID: "destination", Slug: "destination", Tasks: []*models.Task{destinationChild}}
	identity := TaskIdentity{ProjectID: source.ID, TaskID: task.ID}

	result, err := parseQuickEdit("Owner <2", task, source, []*models.Project{source, destination})
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	req := taskLineEditRequest(identity, result)
	if len(req.Children) != 1 || req.Children[0].ProjectID != source.ID || req.Children[0].TaskID != sibling.ID {
		t.Fatalf("same-project child locators = %+v", req.Children)
	}

	result, err = parseQuickEdit("Owner %destination <7", task, source, []*models.Project{source, destination})
	if err != nil {
		t.Fatalf("cross-project parseQuickEdit: %v", err)
	}
	req = taskLineEditRequest(identity, result)
	if req.DestinationProjectID != destination.ID {
		t.Fatalf("destination project = %q", req.DestinationProjectID)
	}
	if len(req.Children) != 1 || req.Children[0].ProjectID != destination.ID || req.Children[0].TaskID != destinationChild.ID {
		t.Fatalf("cross-project child locators = %+v", req.Children)
	}

	result, err = parseQuickEdit("Owner", task, source, []*models.Project{source, destination})
	if err != nil {
		t.Fatalf("childless parseQuickEdit: %v", err)
	}
	if req = taskLineEditRequest(identity, result); req.Children != nil {
		t.Fatalf("childless request children = %+v, want nil", req.Children)
	}
}

func TestParseQuickEditInvalidChildDoesNotMutate(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", Priority: models.PriorityLow, WorkflowState: models.StateBacklog}
	sibling := &models.Task{ID: "sibling", ShortID: 2, Title: "Sibling", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task, sibling}}
	project.BuildTaskTree()
	updated := task.UpdatedAt

	if _, err := parseQuickEdit("Renamed !1 <2 <999", task, project); err == nil {
		t.Fatal("expected the unresolvable child to reject the whole draft")
	}
	if task.Title != "Owner" || task.Priority != models.PriorityLow || task.UpdatedAt != updated || len(task.Children) != 0 {
		t.Fatalf("edited task mutated by a rejected draft: %+v", task)
	}
	if sibling.ParentID != nil {
		t.Fatalf("sibling adopted by a rejected draft: %v", sibling.ParentID)
	}
}

func TestQuickEditChildZeroIsReservedFromTitlePrefixResolution(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Owner", WorkflowState: models.StateBacklog}
	zero := &models.Task{ID: "zero-day", ShortID: 2, Title: "0 day triage", WorkflowState: models.StateBacklog}
	nine := &models.Task{ID: "nine-lives", ShortID: 3, Title: "9 lives remaining", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task, zero, nine}}
	project.BuildTaskTree()

	_, err := parseQuickEdit("Owner <0", task, project)
	if err == nil || !strings.Contains(err.Error(), `child not found: "0"`) {
		t.Fatalf("<0 error = %v, want the reserved token to resolve nothing", err)
	}
	if zero.ParentID != nil {
		t.Fatalf("<0 adopted the zero-titled task: %v", zero.ParentID)
	}

	result, err := parseQuickEdit("Owner <9", task, project)
	if err != nil {
		t.Fatalf("<9 parse: %v", err)
	}
	if got := childIDsOf(result); len(got) != 1 || got[0] != nine.ID {
		t.Fatalf("<9 children = %v, want [%s]", got, nine.ID)
	}

	candidates, reason := quickEditChildCandidates("Owner <0", task, project, nil)
	if reason != "" {
		t.Fatalf("candidate reason = %q, want none", reason)
	}
	offered := false
	for _, candidate := range candidates {
		if candidate.ID == zero.ID {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("completion withheld the zero-titled task after <0: %d candidates", len(candidates))
	}
}
