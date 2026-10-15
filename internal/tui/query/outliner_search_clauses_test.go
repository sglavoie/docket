package query

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSearchClausesFollowTypedOrder(t *testing.T) {
	remainder, clauses := SearchClauses("bank #home @overdue ##desk %atlas")
	if remainder != "bank" {
		t.Fatalf("remainder = %q, want bank", remainder)
	}
	var spellings []string
	var vocabulary []bool
	for _, clause := range clauses {
		spellings = append(spellings, clause.Spelling)
		vocabulary = append(vocabulary, clause.Vocabulary())
	}
	if want := []string{"#home", "@overdue", "##desk", "%atlas"}; !slices.Equal(spellings, want) {
		t.Fatalf("spellings = %v, want typed order %v", spellings, want)
	}
	if want := []bool{true, false, true, true}; !slices.Equal(vocabulary, want) {
		t.Fatalf("vocabulary = %v, want %v", vocabulary, want)
	}
	if !clauses[3].NamesProject() || clauses[0].NamesProject() {
		t.Fatalf("only %%atlas names a project: %+v", clauses)
	}

	if _, invalid := SearchClauses("bank $ready"); invalid != nil {
		t.Fatalf("an invalid query names clauses: %+v", invalid)
	}
}

func TestCompileSearchQueryWithoutDropsOneClause(t *testing.T) {
	tagged := &models.Task{ID: "tagged", Title: "bank", Tags: []string{"home"}, WorkflowState: models.StateBacklog}
	plain := &models.Task{ID: "plain", Title: "bank", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "p", Slug: "p", Tasks: []*models.Task{tagged, plain}}
	projects := []*models.Project{project}

	text := "bank #home !1"
	_, clauses := SearchClauses(text)
	_, full := CompileSearchQuery(text, projects, nil, models.Query{})
	if full.Admits(tagged, project) {
		t.Fatal("the full gate admits a task with no priority")
	}
	rest, withoutPriority := CompileSearchQueryWithout(text, projects, nil, models.Query{}, clauses[1])
	if rest != "bank" {
		t.Fatalf("remainder = %q, want the free text unchanged", rest)
	}
	if !withoutPriority.Admits(tagged, project) || withoutPriority.Admits(plain, project) {
		t.Fatal("dropping !1 must keep #home narrowing")
	}

	alone := CompileSearchClauseAlone(text, projects, nil, clauses[0])
	if !alone.Admits(tagged, project) || alone.Admits(plain, project) {
		t.Fatal("#home alone admits exactly the tagged task")
	}
	done := &models.Task{ID: "done", Tags: []string{"home"}, WorkflowState: models.StateDone}
	if !alone.AdmitsPopulation(done) {
		t.Fatal("a clause asked alone opens every population")
	}
}
