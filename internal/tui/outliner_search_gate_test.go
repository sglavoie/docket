package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// searchGateProjects builds one two-project scope whose task order is the source
// order a sigil-only query must preserve.
func searchGateProjects() (*models.Project, *models.Project) {
	today := time.Now()
	later := today.AddDate(0, 0, 3)

	pinned := primaryRowsTask("pinned", models.StateBacklog)
	pinned.Title = "Pin candidate"
	pinned.Pinned = true
	pinned.Tags = []string{"backend"}
	priorityOne := primaryRowsTask("priority-one", models.StateBacklog)
	priorityOne.Title = "Ship auth"
	priorityOne.Priority = models.PriorityHigh
	priorityOne.DueDate = &today
	contexted := primaryRowsTask("contexted", models.StateBacklog)
	contexted.Title = "Client work"
	contexted.Context = "client-acme"
	contexted.DueDate = &later
	plannedToday := primaryRowsTask("planned-today", models.StateBacklog)
	plannedToday.Title = "Planned today"
	plannedNext := primaryRowsTask("planned-next", models.StateBacklog)
	plannedNext.Title = "Planned next"
	finished := primaryRowsTask("finished", models.StateDone)
	finished.Title = "Old auth work"
	alpha := primaryRowsProject("alpha", pinned, priorityOne, contexted, plannedToday, plannedNext, finished)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta auth"
	betaTask.Tags = []string{"backend"}
	beta := primaryRowsProject("beta", betaTask)

	return alpha, beta
}

func searchGateOutliner() *OutlinerModel {
	alpha, beta := searchGateProjects()
	outliner := scopeTestOutliner(alpha, beta)
	outliner.runtime.Planning().SetToday([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-today"},
	})
	outliner.runtime.Planning().SetNext([]config.ProjectTaskLocator{
		{ProjectID: alpha.ID, TaskID: "planned-next"},
	})
	return outliner
}

func TestSearchSigilsGateTheProjectionAndAndTogether(t *testing.T) {
	exactDay := time.Now().AddDate(0, 0, 3).Format(util.DateFormat)
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{"pinned", "*pin", []string{"pinned"}},
		{"unpinned", "*unpin", []string{"priority-one", "contexted", "planned-today", "planned-next", "beta-task"}},
		{"priority equality", "!1", []string{"priority-one"}},
		{"tag prefix", "#back", []string{"pinned", "beta-task"}},
		{"excluded tag", "-#backend", []string{"priority-one", "contexted", "planned-today", "planned-next"}},
		{"context prefix", "##client", []string{"contexted"}},
		{"due bucket", "@today", []string{"priority-one"}},
		{"missing due date", "@none", []string{"pinned", "planned-today", "planned-next", "beta-task"}},
		{"exact due day", "@" + exactDay, []string{"contexted"}},
		{"terminal destination", "$done", []string{"finished"}},
		{"planning today", "$today", []string{"planned-today"}},
		{"planning next", "$next", []string{"planned-next"}},
		{"backlog includes planning members", "$backlog", []string{
			"pinned", "priority-one", "contexted", "planned-today", "planned-next", "beta-task",
		}},
		{"project prefix", "%bet", []string{"beta-task"}},
		{"predicate and free text", "#backend auth", []string{"beta-task"}},
		{"two predicates", "!1 @today", []string{"priority-one"}},
	}
	// The gate decides membership and nothing else, so the assertions read it
	// through the flat ranked list, whose order is the one the query itself
	// produces. The same gate narrows the mode's own lanes to the same set, in
	// whatever order those lanes already had.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outliner := searchGateOutliner()
			searchRankedList(outliner, tt.query)
			if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, tt.want) {
				t.Fatalf("search %q = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestSearchTerminalTasksNeedAnExplicitTerminalDestination(t *testing.T) {
	outliner := searchGateOutliner()
	searchRankedList(outliner, "auth")
	if got := searchVisibleTaskIDs(outliner); slices.Contains(got, "finished") {
		t.Fatalf("plain text search = %v, want the done task excluded", got)
	}
	searchRankedList(outliner, "$done auth")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"finished"}) {
		t.Fatalf("$done search = %v, want the done task revealed", got)
	}
}

func TestSearchProjectSigilNarrowsWithinFocusRatherThanEscapingIt(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	parent.Title = "Parent"
	child := primaryRowsTask("child", models.StateBacklog)
	child.Title = "Child"
	child.ParentID = &parent.ID
	outside := primaryRowsTask("outside", models.StateBacklog)
	outside.Title = "Outside"
	alpha := primaryRowsProject("alpha", parent, child, outside)

	betaTask := primaryRowsTask("beta-task", models.StateBacklog)
	betaTask.Title = "Beta"
	beta := primaryRowsProject("beta", betaTask)

	outliner := scopeTestOutliner(alpha, beta)
	outliner.pushFocusFrame(taskref.TaskIdentity{ProjectID: alpha.ID, TaskID: parent.ID}, 0, 0)

	searchRankedList(outliner, "%alpha")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{child.ID}) {
		t.Fatalf("%%alpha under Focus = %v, want the focused descendant only", got)
	}
	searchRankedList(outliner, "%beta")
	if got := searchVisibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("%%beta under Focus = %v, want no escape from the focused branch", got)
	}
}

func TestSearchProjectQualifierSurvivesButProjectSigilIsADifferentQuery(t *testing.T) {
	target := primaryRowsTask("target", models.StateBacklog)
	target.Title = "Exact qualifier target"
	target.ShortID = 7
	other := primaryRowsTask("other", models.StateBacklog)
	other.Title = "Tagged seven"
	other.Tags = []string{"7"}
	alpha := primaryRowsProject("alpha", target, other)
	outliner := scopeTestOutliner(alpha)

	searchRankedList(outliner, "alpha#7")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{target.ID}) {
		t.Fatalf("alpha#7 = %v, want the exact short-ID lookup", got)
	}
	searchRankedList(outliner, "%alpha #7")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{other.ID}) {
		t.Fatalf("%%alpha #7 = %v, want the tag gate", got)
	}
}

func TestSearchRefusesRecognizedAndWrongValuesWithRegistryGuidance(t *testing.T) {
	outliner := searchGateOutliner()
	for _, query := range []string{"!9", "$ready", "##0", "auth >4"} {
		outliner.SetSearchQuery(query)
		if len(outliner.rows) != 0 {
			t.Fatalf("search %q kept rows %v", query, searchVisibleTaskIDs(outliner))
		}
		guidance := outliner.searchGate.GuidanceText()
		if guidance == "" {
			t.Fatalf("search %q produced no guidance", query)
		}
		if got := outliner.outlinerEmptyMessage(); got != guidance+". Press Esc to clear." {
			t.Fatalf("empty message = %q, want the guidance sentence", got)
		}
		if got := outliner.classifyEmptyCause(); got != outlinerEmptySearchInvalid {
			t.Fatalf("empty cause = %v, want the invalid-search cause", got)
		}
	}
	searchRankedList(outliner, "!9")
	want := textgrammar.InvalidPriorityError("9").Error()
	if got := outliner.searchGate.GuidanceText(); got != want {
		t.Fatalf("!9 guidance = %q, want the registry sentence %q", got, want)
	}
}

func TestSearchCaseRelaxationRetryAdmitsTheSameGatedSet(t *testing.T) {
	outliner := searchGateOutliner()
	searchRankedList(outliner, "#backend Auth")
	if got := searchVisibleTaskIDs(outliner); !slices.Equal(got, []string{"beta-task"}) {
		t.Fatalf("relaxed gated search = %v, want the backend-tagged match", got)
	}
	if !outliner.searchCaseRelaxed {
		t.Fatal("the retry did not report a relaxed pass")
	}
	if outliner.searchRemainder != "Auth" {
		t.Fatalf("remainder = %q, want the sigil lifted out", outliner.searchRemainder)
	}
}
