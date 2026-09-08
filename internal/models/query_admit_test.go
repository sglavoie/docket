package models

import (
	"testing"
	"time"
)

func queryTestDay(day int) time.Time {
	return time.Date(2026, 8, day, 0, 0, 0, 0, time.Local)
}

func queryTestInstant(day, hour int) time.Time {
	return time.Date(2026, 8, day, hour, 0, 0, 0, time.Local)
}

func TestQueryAdmitsRefusesAnIncompleteTriple(t *testing.T) {
	now := queryTestNow()
	query := Query{Clauses: []QueryClause{positiveClause(t, QueryFieldPinned, QueryOperatorPresent, "")}}
	if query.Admits(nil, queryTestProject(), now) {
		t.Fatal("a nil task was admitted")
	}
	task := queryTestTask()
	task.Pinned = true
	if query.Admits(task, nil, now) {
		t.Fatal("a task read outside a project was admitted")
	}
	if !query.Admits(task, queryTestProject(), time.Time{}) {
		t.Fatal("a zero instant refused a clause that does not read the clock")
	}
}

func TestQueryAdmitsEverythingWhenItCarriesNoClause(t *testing.T) {
	if !(Query{}).Admits(queryTestTask(), queryTestProject(), queryTestNow()) {
		t.Fatal("the zero query narrowed something")
	}
	scoped := Query{Mode: QueryModeToday, Scope: "project-alpha", Sort: QuerySort{Field: QuerySortTitle}}
	if !scoped.Admits(queryTestTask(), queryTestProject(), queryTestNow()) {
		t.Fatal("a query owning only the three axes narrowed a row")
	}
}

func TestQueryAdmitsRefusesAClauseNamingAnUnknownField(t *testing.T) {
	query := Query{Clauses: []QueryClause{{Field: "nonexistent", Operator: QueryOperatorIs, Value: "x"}}}
	if query.Admits(queryTestTask(), queryTestProject(), queryTestNow()) {
		t.Fatal("a clause no evaluator can answer admitted a row")
	}
}

func TestQueryAdmitsFreeTextOverTheWholeTaskSideCorpus(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	build := func(mutate func(*Task)) *Task {
		task := queryTestTask()
		mutate(task)
		return task
	}
	cases := []struct {
		name   string
		value  string
		task   *Task
		admits bool
	}{
		{"title", "LOGIN", queryTestTask(), true},
		{"description", "oauth", build(func(task *Task) { task.Description = "Uses OAuth" }), true},
		{"resume note", "callback", build(func(task *Task) {
			task.ResumeNote = "stopped inside the callback handler"
		}), true},
		{"context", "erra", build(func(task *Task) { task.Context = "errand" }), true},
		{"tag", "spike", build(func(task *Task) { task.Tags = []string{"spike"} }), true},
		{"project name", "alph", queryTestTask(), true},
		{"project slug", "alpha", queryTestTask(), true},
		{"checklist", "token", build(func(task *Task) {
			task.Checklist = []ChecklistItem{{Text: "Refresh the token"}}
		}), true},
		{"comment", "regression", build(func(task *Task) {
			task.Comments = []*Comment{nil, {Text: "Found a regression"}}
		}), true},
		{"attachment filename", "diagram.png", build(func(task *Task) {
			task.Attachments = []Attachment{{Filename: "diagram.png"}}
		}), true},
		{"attachment path", "/tmp/notes", build(func(task *Task) {
			task.Attachments = []Attachment{{OriginalPath: "/tmp/notes.md"}}
		}), true},
		{"absent", "kubernetes", queryTestTask(), false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			query := Query{Clauses: []QueryClause{
				positiveClause(t, QueryFieldText, QueryOperatorContains, testCase.value),
			}}
			if got := query.Admits(testCase.task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryAdmitsADependencyByDurableLocator(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	task := queryTestTask()
	task.Dependencies = []TaskReference{
		{ProjectID: "project-beta", TaskID: "task-7"},
		{ProjectID: "project-alpha", TaskID: "task-2"},
	}
	named := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldDependency, QueryOperatorIs, "project-beta:task-7"),
	}}
	if !named.Admits(task, project, now) {
		t.Fatal("a task depending on the named locator was refused")
	}
	other := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldDependency, QueryOperatorIs, "project-beta:task-9"),
	}}
	if other.Admits(task, project, now) {
		t.Fatal("a locator the task does not carry was admitted")
	}
}

func TestQueryAdmitsPriorityThroughItsThreeReadings(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	cases := []struct {
		name     string
		operator QueryOperator
		value    string
		priority int
		admits   bool
	}{
		{"exactly medium matches medium", QueryOperatorIs, "2", PriorityMedium, true},
		{"exactly medium refuses high", QueryOperatorIs, "2", PriorityHigh, false},
		{"medium or more urgent admits high", QueryOperatorAtLeast, "2", PriorityHigh, true},
		{"medium or more urgent admits medium", QueryOperatorAtLeast, "2", PriorityMedium, true},
		{"medium or more urgent refuses low", QueryOperatorAtLeast, "2", PriorityLow, false},
		{"medium or more urgent refuses unset", QueryOperatorAtLeast, "2", PriorityUnset, false},
		{"unset matches an unset priority", QueryOperatorNone, "", PriorityUnset, true},
		{"unset matches a corrupt priority", QueryOperatorNone, "", 47, true},
		{"unset refuses a set priority", QueryOperatorNone, "", PriorityLow, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			task.Priority = testCase.priority
			query := Query{Clauses: []QueryClause{
				positiveClause(t, QueryFieldPriority, testCase.operator, testCase.value),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryAdmitsTagAndContextExactlyOrByPrefix(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	cases := []struct {
		name     string
		field    QueryField
		operator QueryOperator
		value    string
		tags     []string
		context  string
		admits   bool
	}{
		{"exact tag matches", QueryFieldTag, QueryOperatorIs, "auth", []string{"auth"}, "", true},
		{"exact tag refuses a longer tag", QueryFieldTag, QueryOperatorIs, "auth", []string{"authz"}, "", false},
		{"tag prefix matches a longer tag", QueryFieldTag, QueryOperatorPrefix, "auth", []string{"ui", "authz"}, "", true},
		{"tag prefix refuses an unrelated tag", QueryFieldTag, QueryOperatorPrefix, "auth", []string{"ui"}, "", false},
		{"exact context matches", QueryFieldContext, QueryOperatorIs, "home", nil, "home", true},
		{"context prefix matches", QueryFieldContext, QueryOperatorPrefix, "ho", nil, "home", true},
		{"context prefix refuses an empty context", QueryFieldContext, QueryOperatorPrefix, "ho", nil, "", false},
		{"context none matches an unset context", QueryFieldContext, QueryOperatorNone, "", nil, "", true},
		{"context none refuses a set context", QueryFieldContext, QueryOperatorNone, "", nil, "home", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			task.Tags = testCase.tags
			task.Context = testCase.context
			query := Query{Clauses: []QueryClause{
				positiveClause(t, testCase.field, testCase.operator, testCase.value),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryAdmitsDueDatesByBucketWindowAndExactDay(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	cases := []struct {
		name     string
		operator QueryOperator
		value    string
		due      *time.Time
		terminal bool
		admits   bool
	}{
		{"overdue admits yesterday", QueryOperatorIs, QueryDueOverdue, ptr(queryTestDay(28)), false, true},
		{"overdue refuses a date-only today", QueryOperatorIs, QueryDueOverdue, ptr(queryTestDay(29)), false, false},
		{"overdue admits a passed clock time today", QueryOperatorIs, QueryDueOverdue, ptr(queryTestInstant(29, 9)), false, true},
		{"overdue refuses a future clock time today", QueryOperatorIs, QueryDueOverdue, ptr(queryTestInstant(29, 11)), false, false},
		{"overdue refuses terminal work", QueryOperatorIs, QueryDueOverdue, ptr(queryTestDay(28)), true, false},
		{"today admits today", QueryOperatorIs, QueryDueToday, ptr(queryTestDay(29)), false, true},
		{"today refuses tomorrow", QueryOperatorIs, QueryDueToday, ptr(queryTestDay(30)), false, false},
		{"week admits the sixth day out", QueryOperatorIs, QueryDueWeek, ptr(queryTestDay(31)), false, true},
		{"week refuses the seventh day out", QueryOperatorIs, QueryDueWeek, ptr(queryTestDay(29 + 7)), false, false},
		{"week refuses the past", QueryOperatorIs, QueryDueWeek, ptr(queryTestDay(28)), false, false},
		{"hour window admits inside it", QueryOperatorIs, "+6h", ptr(queryTestInstant(29, 14)), false, true},
		{"hour window refuses beyond it", QueryOperatorIs, "+6h", ptr(queryTestInstant(29, 20)), false, false},
		{"hour window refuses the past", QueryOperatorIs, "+6h", ptr(queryTestInstant(29, 9)), false, false},
		{"day window admits its last day", QueryOperatorIs, "+3d", ptr(queryTestDay(30)), false, true},
		{"day window refuses beyond it", QueryOperatorIs, "+3d", ptr(queryTestDay(29 + 4)), false, false},
		{"week window spans seven days each", QueryOperatorIs, "+2w", ptr(queryTestDay(29 + 14)), false, true},
		{"exact day matches", QueryOperatorIs, "2026-08-31", ptr(queryTestInstant(31, 18)), false, true},
		{"exact day refuses another day", QueryOperatorIs, "2026-08-31", ptr(queryTestDay(30)), false, false},
		{"none matches an undated task", QueryOperatorNone, "", nil, false, true},
		{"none refuses a dated task", QueryOperatorNone, "", ptr(queryTestDay(30)), false, false},
		{"a bucket refuses an undated task", QueryOperatorIs, QueryDueToday, nil, false, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			task.DueDate = testCase.due
			if testCase.terminal {
				task.WorkflowState = StateDone
			}
			query := Query{Clauses: []QueryClause{
				positiveClause(t, QueryFieldDue, testCase.operator, testCase.value),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryAdmitsADeferralByItsWakeDay(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldDefer, QueryOperatorIs, "2026-09-01"),
	}}
	waking := queryTestTask()
	waking.DeferredUntil = ptr(time.Date(2026, 9, 1, 8, 30, 0, 0, time.Local))
	if !query.Admits(waking, project, now) {
		t.Fatal("a task waking on the named day was refused")
	}
	other := queryTestTask()
	other.DeferredUntil = ptr(queryTestDay(30))
	if query.Admits(other, project, now) {
		t.Fatal("a task waking on another day was admitted")
	}
	if query.Admits(queryTestTask(), project, now) {
		t.Fatal("a task with no deferral was admitted by a defer clause")
	}
}

func TestQueryAdmitsALifecycleState(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	query := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldState, QueryOperatorIs, string(StateWaiting)),
	}}
	waiting := queryTestTask()
	waiting.WorkflowState = StateWaiting
	if !query.Admits(waiting, project, now) {
		t.Fatal("a waiting task was refused by a waiting clause")
	}
	if query.Admits(queryTestTask(), project, now) {
		t.Fatal("a doing task was admitted by a waiting clause")
	}
}

func TestQueryAdmitsTheDerivedTaskFlags(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	parentID := "task-parent"
	openChild := &Task{ID: "task-child", WorkflowState: StateBacklog}
	doneChild := &Task{ID: "task-child", WorkflowState: StateDone}

	cases := []struct {
		name   string
		field  QueryField
		mutate func(*Task)
		admits bool
	}{
		{"pinned matches a pinned task", QueryFieldPinned, func(task *Task) { task.Pinned = true }, true},
		{"pinned refuses an unpinned task", QueryFieldPinned, func(*Task) {}, false},
		{"stale matches untouched open work", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -StaleTaskThresholdDays)
		}, true},
		{"stale refuses work touched inside the window", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -(StaleTaskThresholdDays - 1))
		}, false},
		{"stale refuses terminal work", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -365)
			task.WorkflowState = StateDone
		}, false},
		{"stale refuses deferred work", QueryFieldStale, func(task *Task) {
			task.UpdatedAt = now.AddDate(0, 0, -365)
			task.DeferredUntil = ptr(now.AddDate(0, 0, 3))
		}, false},
		{"deferred matches an unexpired deferral", QueryFieldDeferred, func(task *Task) {
			task.DeferredUntil = ptr(now.Add(time.Hour))
		}, true},
		{"deferred refuses an expired deferral", QueryFieldDeferred, func(task *Task) {
			task.DeferredUntil = ptr(now.Add(-time.Hour))
		}, false},
		{"parent matches an open direct child", QueryFieldParent, func(task *Task) {
			task.Children = []*Task{openChild}
		}, true},
		{"parent refuses a wholly finished subtree", QueryFieldParent, func(task *Task) {
			task.Children = []*Task{nil, doneChild}
		}, false},
		{"leaf matches no open child", QueryFieldLeaf, func(task *Task) {
			task.Children = []*Task{doneChild}
		}, true},
		{"leaf refuses an open child", QueryFieldLeaf, func(task *Task) {
			task.Children = []*Task{openChild}
		}, false},
		{"child matches a parented task", QueryFieldChild, func(task *Task) { task.ParentID = &parentID }, true},
		{"child refuses a root task", QueryFieldChild, func(*Task) {}, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			testCase.mutate(task)
			query := Query{Clauses: []QueryClause{
				positiveClause(t, testCase.field, QueryOperatorPresent, ""),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryAdmitsCompletionOnlyWithAUsableTimestamp(t *testing.T) {
	now := queryTestNow()
	project := queryTestProject()
	cases := []struct {
		name        string
		operator    QueryOperator
		value       string
		completedAt *time.Time
		admits      bool
	}{
		{"present matches any completion", QueryOperatorPresent, "", ptr(now.AddDate(0, 0, -1)), true},
		{"present refuses a missing timestamp", QueryOperatorPresent, "", nil, false},
		{"present refuses a zero timestamp", QueryOperatorPresent, "", ptr(time.Time{}), false},
		{"age matches at the threshold", QueryOperatorAtLeast, "30d", ptr(now.AddDate(0, 0, -30)), true},
		{"age refuses inside the threshold", QueryOperatorAtLeast, "30d", ptr(now.AddDate(0, 0, -29)), false},
		{"age refuses a missing timestamp", QueryOperatorAtLeast, "1d", nil, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			task := queryTestTask()
			task.WorkflowState = StateDone
			task.CompletedAt = testCase.completedAt
			query := Query{Clauses: []QueryClause{
				positiveClause(t, QueryFieldCompleted, testCase.operator, testCase.value),
			}}
			if got := query.Admits(task, project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryAdmitsProjectTagsFromTheOwningProject(t *testing.T) {
	now := queryTestNow()
	task := queryTestTask()
	tagged := &Project{ID: "project-alpha", Slug: "alpha", Name: "Alpha", Tags: []string{"infra"}}
	untagged := queryTestProject()

	named := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldProjectTag, QueryOperatorIs, "infra"),
	}}
	if !named.Admits(task, tagged, now) {
		t.Fatal("a task in a tagged project was refused")
	}
	if named.Admits(task, untagged, now) {
		t.Fatal("a task in an untagged project was admitted by a named project tag")
	}

	none := Query{Clauses: []QueryClause{
		positiveClause(t, QueryFieldProjectTag, QueryOperatorNone, ""),
	}}
	if !none.Admits(task, untagged, now) {
		t.Fatal("a task in an untagged project was refused by the untagged reading")
	}
	if none.Admits(task, tagged, now) {
		t.Fatal("a task in a tagged project was admitted by the untagged reading")
	}
}

// The population clauses are the one place a clause value is not purely a
// narrowing: include carries the reload obligation and narrows nothing, while
// only both loads and isolates.
func TestQueryAdmitsPopulationsWhereOnlyIsolatesAndIncludeDoesNot(t *testing.T) {
	now := queryTestNow()
	task := queryTestTask()
	ordinary := queryTestProject()
	parked := &Project{ID: "project-parked", Slug: "parked", Name: "Parked", Tags: []string{ParkedProjectTag}}
	archived := &Project{ID: "project-archived", Slug: "archived", Name: "Archived", Archived: true}

	cases := []struct {
		name    string
		field   QueryField
		value   string
		project *Project
		admits  bool
	}{
		{"parked include admits an ordinary project", QueryFieldParked, QueryPopulationInclude, ordinary, true},
		{"parked include admits a parked project", QueryFieldParked, QueryPopulationInclude, parked, true},
		{"parked only admits a parked project", QueryFieldParked, QueryPopulationOnly, parked, true},
		{"parked only refuses an ordinary project", QueryFieldParked, QueryPopulationOnly, ordinary, false},
		{"archived include admits an ordinary project", QueryFieldArchived, QueryPopulationInclude, ordinary, true},
		{"archived only admits an archived project", QueryFieldArchived, QueryPopulationOnly, archived, true},
		{"archived only refuses a live project", QueryFieldArchived, QueryPopulationOnly, ordinary, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			clause := positiveClause(t, testCase.field, QueryOperatorIs, testCase.value)
			if !clause.RequiresReload {
				t.Fatal("a population clause did not carry the reload obligation")
			}
			query := Query{Clauses: []QueryClause{clause}}
			if got := query.Admits(task, testCase.project, now); got != testCase.admits {
				t.Fatalf("Admits = %t, want %t", got, testCase.admits)
			}
		})
	}
}

func TestQueryTaskIsStaleReadsTheTouchClockAlone(t *testing.T) {
	now := queryTestNow()
	if QueryTaskIsStale(nil, now) {
		t.Fatal("a nil task reported stale")
	}
	task := queryTestTask()
	task.UpdatedAt = now.AddDate(0, 0, -StaleTaskThresholdDays)
	task.StateChangedAt = now
	if !QueryTaskIsStale(task, now) {
		t.Fatal("an untouched task was not stale even though its state clock is recent")
	}
	task.UpdatedAt = now
	task.StateChangedAt = now.AddDate(0, 0, -365)
	if QueryTaskIsStale(task, now) {
		t.Fatal("a recently touched task was stale because of an old state clock")
	}
}

func ptr[T any](value T) *T {
	return &value
}
