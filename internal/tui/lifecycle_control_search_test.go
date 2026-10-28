package tui

import (
	"strconv"
	"testing"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func searchTitles(reply control.Reply) []string {
	if reply.Search == nil {
		return nil
	}
	titles := make([]string, 0, len(reply.Search.Items))
	for _, item := range reply.Search.Items {
		titles = append(titles, item.Title)
	}
	return titles
}

// TestControlSearchRanksTheWorkspaceAsSlashDoes: a title hit outranks a
// description hit, each item carries the stored IDs a reveal names, and a
// description hit says which field ranked it.
func TestControlSearchRanksTheWorkspaceAsSlashDoes(t *testing.T) {
	described := reviewSettledTask(t, 1, "Quarterly numbers")
	described.Description = "send the invoice to finance"
	titled := reviewSettledTask(t, 2, "Invoice the client")
	other := reviewSettledTask(t, 3, "Water the plants")
	env := controlEnv(t, described, titled, other)
	project := env.env.projects[0]

	reply := env.request(t, control.Request{Op: control.OpSearch, Text: "invoice"})
	if !reply.OK || reply.Search == nil {
		t.Fatalf("reply = %+v", reply)
	}
	if got := searchTitles(reply); len(got) != 2 || got[0] != titled.Title || got[1] != described.Title {
		t.Fatalf("titles = %q, want the title hit before the description hit", got)
	}
	if reply.Search.Total != 2 {
		t.Fatalf("total = %d, want 2", reply.Search.Total)
	}
	first, second := reply.Search.Items[0], reply.Search.Items[1]
	if first.Project != project.ID || first.Task != titled.ID || first.ProjectName != project.Name {
		t.Fatalf("first = %+v, want the stored IDs of %q", first, titled.Title)
	}
	if first.Ref != project.Slug+"#"+strconv.Itoa(titled.ShortID) || first.State != string(models.StateBacklog) {
		t.Fatalf("first = %+v", first)
	}
	if first.Match != "" || second.Match != "description" {
		t.Fatalf("match = %q, %q, want title unlabeled and description named", first.Match, second.Match)
	}
}

// TestControlSearchReadsSigilsAndRefusesWhatTheGateCannotHonor: the query is
// the slash grammar, so a tag narrows before scoring and a recognized value the
// grammar cannot honor is the refusal.
func TestControlSearchReadsSigilsAndRefusesWhatTheGateCannotHonor(t *testing.T) {
	tagged := reviewSettledTask(t, 1, "Fix login")
	tagged.Tags = []string{"bug"}
	plain := reviewSettledTask(t, 2, "Fix layout")
	env := controlEnv(t, tagged, plain)

	reply := env.request(t, control.Request{Op: control.OpSearch, Text: "fix #bug"})
	if got := searchTitles(reply); !reply.OK || len(got) != 1 || got[0] != tagged.Title {
		t.Fatalf("reply = %+v, titles %q", reply, got)
	}
	if reply := env.request(t, control.Request{Op: control.OpSearch, Text: "fix $ready"}); reply.OK || reply.Message == "" {
		t.Fatalf("an unhonorable value answered %+v", reply)
	}
	if reply := env.request(t, control.Request{Op: control.OpSearch, Text: "   "}); !reply.OK || reply.Search == nil || len(reply.Search.Items) != 0 {
		t.Fatalf("an empty query answered %+v", reply)
	}
}

// TestControlSearchBoundsItemsAndCountsTheRest: Limit bounds the items while
// Total counts every match.
func TestControlSearchBoundsItemsAndCountsTheRest(t *testing.T) {
	tasks := make([]*models.Task, 0, 5)
	for index := 1; index <= 5; index++ {
		tasks = append(tasks, reviewSettledTask(t, index, "Review chapter "+strconv.Itoa(index)))
	}
	env := controlEnv(t, tasks...)

	reply := env.request(t, control.Request{Op: control.OpSearch, Text: "chapter", Limit: 2})
	if !reply.OK || len(reply.Search.Items) != 2 || reply.Search.Total != 5 {
		t.Fatalf("reply = %+v", reply.Search)
	}
}

// TestControlSearchLeavesHiddenWorkOut: hidden work answers only the clause
// that opens it, as it does in the Outliner.
func TestControlSearchLeavesHiddenWorkOut(t *testing.T) {
	hidden := reviewSettledTask(t, 1, "Secret plan")
	hidden.Hidden = true
	env := controlEnv(t, hidden, reviewSettledTask(t, 2, "Public plan"))

	reply := env.request(t, control.Request{Op: control.OpSearch, Text: "plan"})
	if got := searchTitles(reply); len(got) != 1 || got[0] != "Public plan" {
		t.Fatalf("titles = %q, want hidden work left out", got)
	}
}

// TestControlSearchIgnoresTheViewAndAnswersMidInteraction: the answer is the
// workspace's, not the view's, and reading it disturbs nothing the user has
// open, so an open child view does not refuse it.
func TestControlSearchIgnoresTheViewAndAnswersMidInteraction(t *testing.T) {
	doing := reviewSettledTask(t, 1, "Draft the talk")
	doing.WorkflowState = models.StateDoing
	backlog := reviewSettledTask(t, 2, "Draft the abstract")
	env := controlEnv(t, doing, backlog)
	out := env.model.crossOutliner
	facets, _ := models.PartitionQueryString("$doing")
	out.SetFilterQuery(facets)
	cursor, filter := out.Cursor, out.FilterQuery.String()
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)

	reply := env.request(t, control.Request{Op: control.OpSearch, Text: "draft"})
	if got := searchTitles(reply); !reply.OK || len(got) != 2 {
		t.Fatalf("reply = %+v, titles %q, want both despite the view's $doing facet", reply, got)
	}
	if out.Cursor != cursor || out.FilterQuery.String() != filter || out.SearchQuery() != "" {
		t.Fatal("a host search moved the view")
	}
}
