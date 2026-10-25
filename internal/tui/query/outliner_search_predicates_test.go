package query

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestCompileSearchDueSplitsBucketsFromExactDays(t *testing.T) {
	now := time.Now()
	buckets := map[string]DueDateFilter{
		"overdue": DueDateOverdue,
		"TODAY":   DueDateToday,
		" week ":  DueDateThisWeek,
		"none":    DueDateNone,
	}
	for value, want := range buckets {
		got := compileSearchDue(value, now)
		if !got.hasBucket || got.hasDay || got.hasHourWindow || got.bucket != want {
			t.Errorf("compileSearchDue(%q) = %+v, want bucket %v", value, got, want)
		}
	}

	day := now.AddDate(0, 0, 4)
	got := compileSearchDue(day.Format(util.DateFormat), now)
	if !got.hasDay || got.hasBucket || got.hasHourWindow || util.DaysBetween(got.day, day) != 0 {
		t.Fatalf("compileSearchDue(exact day) = %+v, want the calendar day", got)
	}

	if unresolvable := compileSearchDue("sarah", now); unresolvable.hasBucket || unresolvable.hasDay || unresolvable.hasHourWindow {
		t.Fatalf("compileSearchDue(sarah) = %+v, want no predicate", unresolvable)
	}
}

func TestCompileSearchDueHourWindow(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	got := compileSearchDue("+4h", now)
	if !got.hasHourWindow || got.hasBucket || got.hasDay {
		t.Fatalf("compileSearchDue(+4h) = %+v, want an hour window", got)
	}
	if want := now.Add(4 * time.Hour); !got.window.Equal(want) {
		t.Errorf("compileSearchDue(+4h).window = %v, want %v", got.window, want)
	}

	if upper := compileSearchDue("+12H", now); !upper.hasHourWindow || !upper.window.Equal(now.Add(12*time.Hour)) {
		t.Errorf("compileSearchDue(+12H) = %+v, want a case-insensitive hour window", upper)
	}

	// A minute/day offset or a bucket must not be read as an hour window.
	if minutes := compileSearchDue("+3m", now); minutes.hasHourWindow {
		t.Errorf("compileSearchDue(+3m) = %+v, want no hour window", minutes)
	}
	if today := compileSearchDue("today", now); today.hasHourWindow || !today.hasBucket {
		t.Errorf("compileSearchDue(today) = %+v, want the today bucket unchanged", today)
	}
}

func TestSearchPredicatesAdmitsDueHourWindow(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	predicates := &SearchPredicates{
		now: now,
		due: compileSearchDue("+4h", now),
	}
	project := &models.Project{ID: "alpha", Slug: "alpha"}

	overdue := now.Add(-3 * time.Hour)
	soon := now.Add(2 * time.Hour)
	later := now.Add(6 * time.Hour)
	dateOnlyToday := time.Date(2026, 8, 25, 0, 0, 0, 0, time.Local)

	cases := map[string]struct {
		task *models.Task
		want bool
	}{
		"overdue instant":   {&models.Task{ID: "a", WorkflowState: models.StateBacklog, DueDate: &overdue}, true},
		"due within window": {&models.Task{ID: "b", WorkflowState: models.StateBacklog, DueDate: &soon}, true},
		"date-only today":   {&models.Task{ID: "c", WorkflowState: models.StateBacklog, DueDate: &dateOnlyToday}, true},
		"beyond window":     {&models.Task{ID: "d", WorkflowState: models.StateBacklog, DueDate: &later}, false},
		"no due date":       {&models.Task{ID: "e", WorkflowState: models.StateBacklog}, false},
	}
	for name, tc := range cases {
		if got := predicates.Admits(tc.task, project); got != tc.want {
			t.Errorf("%s: admits = %v, want %v", name, got, tc.want)
		}
	}
}

func TestSearchTagPrefixReadsNormalizedTagsOnly(t *testing.T) {
	task := &models.Task{Tags: []string{"Backend", "source:GitHub"}, Context: "Client-Acme"}
	for _, prefix := range []string{"back", "backend", "source-git"} {
		if !taskCarriesTagPrefix(task, prefix) {
			t.Errorf("taskCarriesTagPrefix(%q) = false, want a normalized prefix match", prefix)
		}
	}
	if !taskCarriesTagPrefix(task, normalizeSearchTagPrefix("source:GitHub")) {
		t.Error("a namespaced prefix did not reach the kebab-normalized stored tag")
	}
	for _, prefix := range []string{"end", "client", "sourcegit"} {
		if taskCarriesTagPrefix(task, prefix) {
			t.Errorf("taskCarriesTagPrefix(%q) = true, want tags only and prefixes only", prefix)
		}
	}

	predicates := &SearchPredicates{contexts: []string{"client"}}
	if !predicates.admitsContext(task) {
		t.Error("context prefix did not match the normalized task context")
	}
	predicates.contexts = []string{"acme"}
	if predicates.admitsContext(task) {
		t.Error("context matched a non-prefix substring")
	}
}

func TestSearchPredicatesAreNilSafeAndOpen(t *testing.T) {
	var predicates *SearchPredicates
	task := &models.Task{ID: "task", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	if predicates.Invalid() || predicates.AllowsTerminal() || predicates.GuidanceText() != "" {
		t.Fatal("a nil gate reported a state")
	}
	if !predicates.Admits(task, project) || !predicates.AdmitsProject(project) {
		t.Fatal("a nil gate rejected a task")
	}
}

func TestSearchPriorityGateIsEqualityRatherThanThreshold(t *testing.T) {
	predicates := &SearchPredicates{priorities: []int{models.PriorityMedium}, now: time.Now()}
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	for priority, want := range map[int]bool{
		models.PriorityHigh:   false,
		models.PriorityMedium: true,
		models.PriorityLow:    false,
		models.PriorityUnset:  false,
	} {
		task := &models.Task{ID: "task", WorkflowState: models.StateBacklog, Priority: priority}
		if got := predicates.Admits(task, project); got != want {
			t.Errorf("priority %d admitted = %v, want %v", priority, got, want)
		}
	}
}

// TestSearchPriorityThresholdWidensOnlyWhereTheSuffixIsTyped holds the split the
// threshold was added for: !N is still equality, and only !N+ opens the more
// urgent priorities, which are the smaller numbers.
func TestSearchPriorityThresholdWidensOnlyWhereTheSuffixIsTyped(t *testing.T) {
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	tests := []struct {
		query string
		want  map[int]bool
	}{
		{"!2", map[int]bool{models.PriorityHigh: false, models.PriorityMedium: true, models.PriorityLow: false}},
		{"!2+", map[int]bool{models.PriorityHigh: true, models.PriorityMedium: true, models.PriorityLow: false}},
		{"-!2+", map[int]bool{models.PriorityHigh: false, models.PriorityMedium: false, models.PriorityLow: true}},
		{"-!1", map[int]bool{models.PriorityHigh: false, models.PriorityMedium: true, models.PriorityLow: true}},
	}
	for _, tt := range tests {
		_, gate := CompileSearchQuery(tt.query, []*models.Project{project}, nil, models.Query{})
		if gate.Invalid() {
			t.Fatalf("%s produced guidance %q", tt.query, gate.GuidanceText())
		}
		for priority, want := range tt.want {
			task := &models.Task{ID: "task", WorkflowState: models.StateBacklog, Priority: priority}
			if got := gate.Admits(task, project); got != want {
				t.Errorf("%s admitted priority %d = %v, want %v", tt.query, priority, got, want)
			}
		}
	}
}

// TestSearchNegationsSubtractWhatTheirTwinsSelect walks the negation set through
// the compiled gate. Each one must remove exactly the population its positive
// twin would have kept, and leave an unrelated task alone: a negation narrows,
// it never re-scopes.
func TestSearchNegationsSubtractWhatTheirTwinsSelect(t *testing.T) {
	now := time.Now()
	due := now
	marked := &models.Task{
		ID: "marked", ShortID: 1, Title: "Marked",
		WorkflowState: models.StateDoing, Priority: models.PriorityHigh,
		Tags: []string{"backend"}, Context: "client", Pinned: true, DueDate: &due,
	}
	plain := &models.Task{ID: "plain", ShortID: 2, Title: "Plain", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "alpha", Slug: "alpha", Name: "Alpha", Tasks: []*models.Task{marked, plain}}
	for _, query := range []string{"-!1", "-#backend", "-##client", "-@today", "-$doing", "-*pin"} {
		_, gate := CompileSearchQuery(query, []*models.Project{project}, nil, models.Query{})
		if gate.Invalid() {
			t.Fatalf("%s produced guidance %q", query, gate.GuidanceText())
		}
		if gate.Admits(marked, project) {
			t.Errorf("%s admitted the task it names", query)
		}
		if !gate.Admits(plain, project) {
			t.Errorf("%s excluded a task it says nothing about", query)
		}
	}

	// The project negation is the one axis that answers about the project rather
	// than the task, so it is checked where it applies.
	_, scoped := CompileSearchQuery("-%alpha", []*models.Project{project}, nil, models.Query{})
	if scoped.Invalid() {
		t.Fatalf("-%%alpha produced guidance %q", scoped.GuidanceText())
	}
	if scoped.AdmitsProject(project) {
		t.Error("-%alpha admitted the project it names")
	}
}

// Two positive project scopes union: each project named by either prefix is
// admitted and one named by neither is not.
func TestSearchRepeatedProjectScopeIsAUnion(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Slug: "alpha", Name: "Alpha"}
	beta := &models.Project{ID: "beta", Slug: "beta", Name: "Beta"}
	gamma := &models.Project{ID: "gamma", Slug: "gamma", Name: "Gamma"}
	projects := []*models.Project{alpha, beta, gamma}
	_, gate := CompileSearchQuery("%alpha %beta", projects, nil, models.Query{})
	if gate.Invalid() {
		t.Fatalf("%%alpha %%beta produced guidance %q", gate.GuidanceText())
	}
	if !gate.AdmitsProject(alpha) || !gate.AdmitsProject(beta) {
		t.Fatal("a project named by the union was refused")
	}
	if gate.AdmitsProject(gamma) {
		t.Fatal("a project named by neither scope was admitted")
	}
}

// TestSearchDerivedPredicatesAnswerComputedQuestions covers the ? set end to end
// through the same gate a query runs on. The two reference predicates are
// structural on purpose: one reads the task's own outgoing list and the other
// the incoming census, and neither looks at the other end's state.
func TestSearchDerivedPredicatesAnswerComputedQuestions(t *testing.T) {
	now := time.Now()
	old := now.AddDate(0, 0, -200)
	completedLongAgo := now.AddDate(0, 0, -120)
	later := now.Add(48 * time.Hour)

	stale := &models.Task{ID: "stale", ShortID: 1, Title: "Stale", WorkflowState: models.StateBacklog, UpdatedAt: old}
	child := &models.Task{ID: "child", ShortID: 2, Title: "Child", WorkflowState: models.StateBacklog, UpdatedAt: now}
	parent := &models.Task{
		ID: "parent", ShortID: 3, Title: "Parent", WorkflowState: models.StateBacklog,
		UpdatedAt: now, Children: []*models.Task{child},
	}
	deferred := &models.Task{
		ID: "deferred", ShortID: 4, Title: "Deferred", WorkflowState: models.StateBacklog,
		UpdatedAt: now, DeferredUntil: &later,
	}
	finished := &models.Task{
		ID: "finished", ShortID: 5, Title: "Finished", WorkflowState: models.StateDone,
		UpdatedAt: completedLongAgo, CompletedAt: &completedLongAgo,
	}
	target := &models.Task{ID: "target", ShortID: 6, Title: "Target", WorkflowState: models.StateBacklog, UpdatedAt: now}
	referrer := &models.Task{
		ID: "referrer", ShortID: 7, Title: "Referrer", WorkflowState: models.StateBacklog, UpdatedAt: now,
		Dependencies: []models.TaskReference{{ProjectID: "alpha", TaskID: "target"}},
	}
	project := &models.Project{
		ID: "alpha", Slug: "alpha", Name: "Alpha",
		Tasks: []*models.Task{stale, parent, child, deferred, finished, target, referrer},
	}

	cases := []struct {
		query string
		want  *models.Task
	}{
		{"?stale", stale},
		{"?deferred", deferred},
		{"?parent", parent},
		{"?completed:90d", finished},
		{"?blocked", referrer},
		{"?blocking", target},
	}
	for _, tc := range cases {
		_, gate := CompileSearchQuery(tc.query, []*models.Project{project}, nil, models.Query{})
		if gate.Invalid() {
			t.Fatalf("%s produced guidance %q", tc.query, gate.GuidanceText())
		}
		for _, task := range project.Tasks {
			want := task == tc.want
			if got := gate.Admits(task, project); got != want {
				t.Errorf("%s admitted %s = %v, want %v", tc.query, task.ID, got, want)
			}
		}
	}

	// Leaf is the complement of parent over the same census, so it is checked as
	// a set rather than as a single row.
	_, leaves := CompileSearchQuery("?leaf", []*models.Project{project}, nil, models.Query{})
	if leaves.Admits(parent, project) {
		t.Error("?leaf admitted a task with an open child")
	}
	if !leaves.Admits(child, project) || !leaves.Admits(stale, project) {
		t.Error("?leaf excluded a task with no open child")
	}

	// The completion question opens the terminal population; its negation asks
	// about live work and deliberately does not.
	_, notCompleted := CompileSearchQuery("-?completed", []*models.Project{project}, nil, models.Query{})
	if notCompleted.AllowsTerminal() {
		t.Error("-?completed opened the terminal population")
	}
	if notCompleted.Admits(finished, project) {
		t.Error("-?completed admitted a completed task")
	}
}

// TestSearchReviewDerivedUsesTheSharedAttentionPredicate covers the typed-query
// carrier independently from the canonical facet a saved lens restores. Its
// positive form opens deferred work because a stored review flag remains active
// there; the negation narrows only the ordinary live population.
func TestSearchReviewDerivedUsesTheSharedAttentionPredicate(t *testing.T) {
	now := util.StartOfDay(time.Now())
	future := now.AddDate(0, 0, 3)
	deferredUntil := now.AddDate(0, 0, 5)
	build := func(id string, mutate func(*models.Task)) *models.Task {
		task := &models.Task{ID: id, WorkflowState: models.StateBacklog, UpdatedAt: now}
		mutate(task)
		return task
	}
	tasks := []*models.Task{
		build("plain", func(*models.Task) {}),
		build("flagged", func(task *models.Task) { task.Review = true }),
		build("due", func(task *models.Task) { task.NextCheckIn = &now }),
		build("future", func(task *models.Task) { task.NextCheckIn = &future }),
		build("stale", func(task *models.Task) {
			task.UpdatedAt = now.AddDate(0, 0, -models.StaleTaskThresholdDays)
		}),
		build("waiting", func(task *models.Task) { task.WorkflowState = models.StateWaiting }),
		build("deferred-due", func(task *models.Task) {
			task.NextCheckIn = &now
			task.DeferredUntil = &deferredUntil
		}),
		build("deferred-flagged", func(task *models.Task) {
			task.Review = true
			task.DeferredUntil = &deferredUntil
		}),
		build("terminal-flagged", func(task *models.Task) {
			task.Review = true
			task.WorkflowState = models.StateDone
		}),
	}
	project := &models.Project{ID: "alpha", Slug: "alpha", Tasks: tasks}

	for _, spelling := range []string{"?review", "-?review"} {
		_, gate := CompileSearchQuery(spelling, []*models.Project{project}, nil, models.Query{})
		if gate.Invalid() {
			t.Fatalf("%s produced guidance %q", spelling, gate.GuidanceText())
		}
		negative := spelling[0] == '-'
		for _, task := range tasks {
			want := models.TaskNeedsAttention(task, gate.now)
			if negative {
				want = !want
			}
			if got := gate.Admits(task, project); got != want {
				t.Errorf("%s admitted %s = %t, want %t", spelling, task.ID, got, want)
			}
		}
		if got, want := gate.AllowsDeferred(), !negative; got != want {
			t.Errorf("%s AllowsDeferred = %t, want %t", spelling, got, want)
		}
		if gate.AllowsTerminal() {
			t.Errorf("%s opened terminal work the attention predicate refuses", spelling)
		}
	}
}

// & reads presence alone in search, and -& is its twin inverted, so the four
// spellings split the population into exactly two halves.
func TestSearchAttachmentPresenceSplitsThePopulation(t *testing.T) {
	attached := &models.Task{
		ID: "attached", ShortID: 1, Title: "Attached", WorkflowState: models.StateBacklog,
		Attachments: []models.Attachment{{Filename: "brief.md", AddedAt: time.Now()}},
	}
	bare := &models.Task{ID: "bare", ShortID: 2, Title: "Bare", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "alpha", Slug: "alpha", Name: "Alpha", Tasks: []*models.Task{attached, bare}}
	for query, wantAttached := range map[string]bool{"&any": true, "-&none": true, "&none": false, "-&any": false, "&ANY": true} {
		_, gate := CompileSearchQuery(query, []*models.Project{project}, nil, models.Query{})
		if gate.Invalid() {
			t.Fatalf("%s produced guidance %q", query, gate.GuidanceText())
		}
		if gate.Admits(attached, project) != wantAttached || gate.Admits(bare, project) == wantAttached {
			t.Errorf("%s admitted attached=%t bare=%t, want attached=%t alone",
				query, gate.Admits(attached, project), gate.Admits(bare, project), wantAttached)
		}
	}
	_, contradiction := CompileSearchQuery("&any &none", []*models.Project{project}, nil, models.Query{})
	if contradiction.Admits(attached, project) || contradiction.Admits(bare, project) {
		t.Error("&any &none admitted a task; a contradiction admits nothing")
	}
}

// TestSearchDueOffsetsMatchTheirPersistedQueryReading holds the typed `/`
// search and the persisted query a lens or the bridge reads to one meaning for
// every relative due value, so a lens saved from a search admits what it showed.
func TestSearchDueOffsetsMatchTheirPersistedQueryReading(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.Local)
	project := &models.Project{ID: "alpha", Slug: "alpha"}
	at := func(days, hours int) *time.Time {
		instant := time.Date(2026, 8, 25+days, 0, 0, 0, 0, time.Local).Add(time.Duration(hours) * time.Hour)
		return &instant
	}
	dues := []*time.Time{nil, at(-3, 0), at(0, 0), at(0, 9), at(0, 14), at(0, 20), at(1, 0), at(2, 0),
		at(3, 0), at(3, 18), at(4, 0), at(13, 0), at(14, 0), at(15, 0)}
	for _, value := range []string{"+4h", "+12h", "+1d", "+3d", "+2w"} {
		persisted, remainder := models.PartitionQueryString("@" + value)
		if remainder != "" || len(persisted.Clauses) != 1 {
			t.Fatalf("@%s persisted as %+v with remainder %q, want one clause", value, persisted, remainder)
		}
		search := &SearchPredicates{now: now, due: compileSearchDue(value, now)}
		for _, due := range dues {
			task := &models.Task{ID: "t", WorkflowState: models.StateBacklog, DueDate: due}
			if typed, saved := search.Admits(task, project), persisted.Admits(task, project, now); typed != saved {
				t.Errorf("@%s due %v: search admits %t, persisted query %t", value, due, typed, saved)
			}
		}
	}
}
