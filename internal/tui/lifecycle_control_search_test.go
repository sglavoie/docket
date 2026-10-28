package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/pulse"
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

// TestControlIndexListsWhatASigilFreeSearchCouldFind: open work is indexed
// with what a system search matches on, finished and hidden work is not, and
// the revision answers Unchanged until something indexed changes.
func TestControlIndexListsWhatASigilFreeSearchCouldFind(t *testing.T) {
	open := reviewSettledTask(t, 1, "Renew passport")
	open.Description = "Bring the old one"
	open.Tags = []string{"errand"}
	done := reviewSettledTask(t, 2, "Finished chore")
	done.WorkflowState = models.StateDone
	hidden := reviewSettledTask(t, 3, "Hidden plan")
	hidden.Hidden = true
	env := controlEnv(t, open, done, hidden)

	reply := env.request(t, control.Request{Op: control.OpIndex})
	if !reply.OK || reply.Index == nil || reply.Index.Unchanged || reply.Index.Revision == "" {
		t.Fatalf("reply = %+v", reply)
	}
	if len(reply.Index.Items) != 1 || reply.Index.Total != 1 {
		t.Fatalf("items = %+v, want the open task alone", reply.Index.Items)
	}
	item := reply.Index.Items[0]
	if item.Title != open.Title || item.Description != open.Description || len(item.Tags) != 1 || item.Tags[0] != "errand" {
		t.Fatalf("item = %+v", item)
	}

	again := env.request(t, control.Request{Op: control.OpIndex, Text: reply.Index.Revision})
	if !again.OK || !again.Index.Unchanged || len(again.Index.Items) != 0 || again.Index.Revision != reply.Index.Revision {
		t.Fatalf("an unchanged index answered %+v", again.Index)
	}
	if captured := env.request(t, control.Request{Op: control.OpCapture, Text: "Book the photo appointment %" + env.env.projects[0].Slug}); !captured.OK {
		t.Fatalf("capture refused: %+v", captured)
	}
	changed := env.request(t, control.Request{Op: control.OpIndex, Text: reply.Index.Revision})
	if changed.Index.Unchanged || changed.Index.Revision == reply.Index.Revision || changed.Index.Total != 2 {
		t.Fatalf("a committed capture left the index standing: %+v", changed.Index)
	}
}

func TestIndexDescriptionCutsAtARune(t *testing.T) {
	long := strings.Repeat("é", control.IndexDescriptionRunes+5)
	if got := indexDescription("  " + long); utf8.RuneCountInString(got) != control.IndexDescriptionRunes || !utf8.ValidString(got) {
		t.Fatalf("got %d runes", utf8.RuneCountInString(got))
	}
}

// focusEnv is controlEnv over two projects, each holding one overdue task,
// with a publisher recording what the host is told.
func focusEnv(t *testing.T) (*reviewAsyncEnv, *recordingPublisher) {
	t.Helper()
	yesterday := time.Now().AddDate(0, 0, -1)
	work := reviewSettledTask(t, 1, "Work report")
	work.DueDate = &yesterday
	home := reviewSettledTask(t, 1, "Home repair")
	home.DueDate = &yesterday
	env := newReviewAsyncEnv(t, []*models.Task{work}, []*models.Task{home})
	env.model.features = coordinator.NewFeatureCoordinators(env.env.svc)
	env.model.domain.runtime.Catalog().SystemUnfiledProjectID = env.env.projects[0].ID
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner)
	publisher := &recordingPublisher{}
	WithControlPublisher(publisher)(env.model)
	env.model.refreshWorkspacePulseAt(time.Now())
	return env, publisher
}

func lastPulseEvent(t *testing.T, publisher *recordingPublisher) control.Pulse {
	t.Helper()
	for index := len(publisher.events) - 1; index >= 0; index-- {
		if event := publisher.events[index]; event.Event == control.EventPulse && event.Pulse != nil {
			return *event.Pulse
		}
	}
	t.Fatal("no pulse was published")
	return control.Pulse{}
}

// TestControlFocusNarrowsWhatTheHostIsTold: a Focus on one project narrows the
// pulse, agenda, and arrivals the host hears, leaves the session's own pulse
// and searches whole, and clearing it restores the workspace's.
func TestControlFocusNarrowsWhatTheHostIsTold(t *testing.T) {
	env, publisher := focusEnv(t)
	work, home := env.env.projects[0], env.env.projects[1]
	env.model.publishControlPulse()
	if got := lastPulseEvent(t, publisher); got.Overdue != 2 {
		t.Fatalf("unfocused overdue = %d, want 2", got.Overdue)
	}

	listed := env.request(t, control.Request{Op: control.OpProjects})
	if !listed.OK || listed.Projects == nil || len(listed.Projects.Items) != 2 {
		t.Fatalf("projects = %+v", listed)
	}

	if reply := env.request(t, control.Request{Op: control.OpFocus, Projects: []string{work.ID, "gone"}}); !reply.OK {
		t.Fatalf("focus refused: %+v", reply)
	}
	env.model.publishControlPulse()
	env.model.publishControlAgenda()
	if got := lastPulseEvent(t, publisher); got.Overdue != 1 {
		t.Fatalf("focused overdue = %d, want 1", got.Overdue)
	}
	agenda := publisher.events[len(publisher.events)-1].Agenda
	if agenda == nil || len(agenda.Items) != 1 || agenda.Items[0].Project != work.ID {
		t.Fatalf("focused agenda = %+v", agenda)
	}
	if env.model.workspacePulse.Overdue != 2 {
		t.Fatalf("the session's own pulse narrowed to %d", env.model.workspacePulse.Overdue)
	}
	if found := env.request(t, control.Request{Op: control.OpSearch, Text: "repair"}); len(searchTitles(found)) != 1 {
		t.Fatalf("a focused search left out %q: %+v", "Home repair", found)
	}

	before := len(publisher.events)
	env.model.publishControlArrivals([]pulse.Arrival{
		{Kind: pulse.ArrivalDeadline, ProjectID: work.ID, TaskID: "a", Title: "Work"},
		{Kind: pulse.ArrivalDeadline, ProjectID: home.ID, TaskID: "b", Title: "Home"},
	})
	if arrivals := publisher.events[before].Arrivals; len(arrivals) != 1 || arrivals[0].Project != work.ID {
		t.Fatalf("focused arrivals = %+v", arrivals)
	}
	before = len(publisher.events)
	env.model.publishControlArrivals([]pulse.Arrival{{Kind: pulse.ArrivalDeadline, ProjectID: home.ID, TaskID: "b", Title: "Home"}})
	if len(publisher.events) != before {
		t.Fatal("an arrival outside the Focus was published")
	}

	if reply := env.request(t, control.Request{Op: control.OpFocus}); !reply.OK {
		t.Fatalf("clearing refused: %+v", reply)
	}
	env.model.publishControlPulse()
	if got := lastPulseEvent(t, publisher); got.Overdue != 2 {
		t.Fatalf("cleared overdue = %d, want 2", got.Overdue)
	}
}

// TestControlFocusOnNoKnownProjectNarrowsNothing fails open: a Focus whose
// projects are all gone is refused rather than silencing everything.
func TestControlFocusOnNoKnownProjectNarrowsNothing(t *testing.T) {
	env, publisher := focusEnv(t)
	if reply := env.request(t, control.Request{Op: control.OpFocus, Projects: []string{"gone"}}); reply.OK {
		t.Fatalf("a Focus on no known project answered %+v", reply)
	}
	env.model.publishControlPulse()
	if got := lastPulseEvent(t, publisher); got.Overdue != 2 {
		t.Fatalf("overdue = %d, want the whole workspace's 2", got.Overdue)
	}
}
