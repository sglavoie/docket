package models

import "testing"

func hiddenTestProject() (*Project, *Task, *Task, *Task) {
	project := queryTestProject()
	parent := queryTestTask()
	parent.ID, parent.ShortID = "task-parent", 1
	child := queryTestTask()
	child.ID, child.ShortID = "task-child", 2
	child.ParentID = &parent.ID
	grandchild := queryTestTask()
	grandchild.ID, grandchild.ShortID = "task-grandchild", 3
	grandchild.ParentID = &child.ID
	project.Tasks = []*Task{parent, child, grandchild}
	project.BuildTaskTree()
	return project, parent, child, grandchild
}

func TestTaskEffectivelyHiddenInheritsFromEveryAncestor(t *testing.T) {
	project, parent, child, grandchild := hiddenTestProject()
	for _, task := range project.Tasks {
		if TaskEffectivelyHidden(project, task) {
			t.Fatalf("%s is hidden with no flag stored anywhere", task.ID)
		}
	}
	parent.Hidden = true
	for _, task := range []*Task{parent, child, grandchild} {
		if !TaskEffectivelyHidden(project, task) {
			t.Fatalf("%s is not hidden under a hidden root", task.ID)
		}
	}
	parent.Hidden, child.Hidden = false, true
	if TaskEffectivelyHidden(project, parent) {
		t.Fatal("a hidden child hid its parent")
	}
	if !TaskEffectivelyHidden(project, grandchild) {
		t.Fatal("a grandchild under a hidden child is visible")
	}
}

func TestTaskEffectivelyHiddenResolvesADetachedClone(t *testing.T) {
	project, parent, _, grandchild := hiddenTestProject()
	parent.Hidden = true
	clone := CloneTaskDeep(grandchild)
	if clone.Parent != nil {
		t.Fatal("precondition: a deep clone carries no Parent link")
	}
	if !TaskEffectivelyHidden(project, clone) {
		t.Fatal("a clone did not resolve its stored ParentID inside the project")
	}
	if TaskEffectivelyHidden(nil, clone) {
		t.Fatal("a clone with no project and no link answered for an ancestor it cannot reach")
	}
	if TaskEffectivelyHidden(project, nil) {
		t.Fatal("a nil task is hidden")
	}
}

func TestQueryHiddenPopulationClauseAdmitsInheritedHiddenness(t *testing.T) {
	project, parent, child, _ := hiddenTestProject()
	parent.Hidden = true
	visible := queryTestTask()
	visible.ID = "task-visible"
	project.Tasks = append(project.Tasks, visible)
	now := queryTestNow()

	only := Query{Clauses: []QueryClause{positiveClause(t, QueryFieldHidden, QueryOperatorIs, QueryPopulationOnly)}}
	include := Query{Clauses: []QueryClause{positiveClause(t, QueryFieldHidden, QueryOperatorIs, QueryPopulationInclude)}}
	if !only.Admits(child, project, now) || !only.Admits(parent, project, now) {
		t.Fatal("+hidden:only refused an effectively hidden task")
	}
	if only.Admits(visible, project, now) {
		t.Fatal("+hidden:only admitted a visible task")
	}
	if !include.Admits(child, project, now) || !include.Admits(visible, project, now) {
		t.Fatal("+hidden narrowed rather than only opening the population")
	}
	if !only.OpensHiddenPopulation() || !include.OpensHiddenPopulation() {
		t.Fatal("a positive hidden clause does not open the hidden population")
	}
	if (Query{}).OpensHiddenPopulation() {
		t.Fatal("an empty query opens the hidden population")
	}
	if !only.Canonical().RequiresReload() {
		t.Fatal("a hidden clause carries no population obligation")
	}
}

func TestQueryHiddenPopulationRoundTripsCanonically(t *testing.T) {
	for _, text := range []string{"+hidden", "+hidden:only", "*pin +parked +hidden:only"} {
		parsed, err := ParseQuery(text)
		if err != nil {
			t.Fatalf("ParseQuery(%q): %v", text, err)
		}
		if got := parsed.String(); got != text {
			t.Fatalf("ParseQuery(%q).String() = %q", text, got)
		}
	}
	reordered, err := ParseQuery("+hidden:only +parked *pin")
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if got := reordered.String(); got != "*pin +parked +hidden:only" {
		t.Fatalf("canonical order = %q, want hidden after the project populations", got)
	}
	if _, err := ParseQuery("+hidden:all"); err == nil {
		t.Fatal("+hidden:all parsed")
	}
}

func TestQueryAttentionClausesNeverAdmitHiddenWork(t *testing.T) {
	project, parent, child, _ := hiddenTestProject()
	now := queryTestNow()
	parent.Hidden = true
	child.Review = true
	child.UpdatedAt = now.AddDate(0, 0, -(StaleTaskThresholdDays + 1))
	opened := positiveClause(t, QueryFieldHidden, QueryOperatorIs, QueryPopulationInclude)
	for _, field := range []QueryField{QueryFieldNeedsReview, QueryFieldStale} {
		query := Query{Clauses: []QueryClause{opened, positiveClause(t, field, QueryOperatorPresent, "")}}
		if query.Admits(child, project, now) {
			t.Fatalf("%s admitted a task under a hidden parent", field)
		}
		parent.Hidden = false
		if !query.Admits(child, project, now) {
			t.Fatalf("%s refused the same task once visible", field)
		}
		parent.Hidden = true
	}
}

func TestQueryHiddenClauseSurvivesALensPartitionAndJoin(t *testing.T) {
	clauses, remainder := PartitionQueryString("+hidden:only invoice *pin")
	if got := clauses.String(); got != "*pin +hidden:only" {
		t.Fatalf("partitioned clauses = %q, want the hidden clause lifted canonically", got)
	}
	if remainder != "invoice" {
		t.Fatalf("remainder = %q, want the prose alone", remainder)
	}
	joined := JoinQueryString(clauses, remainder)
	if joined != "*pin +hidden:only invoice" {
		t.Fatalf("joined = %q", joined)
	}
	if again := JoinQueryString(PartitionQueryString(joined)); again != joined {
		t.Fatalf("join is not idempotent: %q then %q", joined, again)
	}
	if !clauses.OpensHiddenPopulation() {
		t.Fatal("a lens carrying +hidden:only does not open the hidden population")
	}
}

func TestQueryNegatedHiddenClauseOpensNothing(t *testing.T) {
	negated := Query{Clauses: []QueryClause{negativeClause(t, QueryFieldHidden, QueryOperatorIs, QueryPopulationOnly)}}
	if negated.OpensHiddenPopulation() {
		t.Fatal("-+hidden:only opened the hidden population")
	}
	if got := negated.String(); got != "-+hidden:only" {
		t.Fatalf("negated hidden clause renders %q", got)
	}
}
