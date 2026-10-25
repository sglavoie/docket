package storage

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

// These tests pin the live task format itself rather than any one feature of
// it: the exact bytes it renders, the revision those bytes hash to, the
// escaping that keeps prose out of the block grammar, and the completeness of
// the record/model crossing in current_task_conversion.go. Together they are
// what makes an additive field a reviewable change: adding one moves the golden
// bytes and the revision deliberately, and failing to carry it across the
// crossing fails the round trip below without anyone having to remember to
// assert it.

// currentSchemaFixtureInstant is the base timestamp the fixture is built from.
// It is UTC so the rendered bytes do not depend on the machine's zone; only the
// due date is deliberately local, because that is the one stored value the
// reader resolves against a location.
var currentSchemaFixtureInstant = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

// currentSchemaFixtureProject returns a two-task project whose second task
// carries a non-zero value in every persisted models.Task field. The first task
// exists only to be a parent, since a parent reference must resolve inside the
// same document.
//
// It is a crossing fixture rather than a lifecycle-valid task: its job is to
// prove that every stored field survives model→record→bytes→record→model, so it
// sets values the lifecycle would never mint together. A terminal task carrying
// a check-in schedule is the case in point — ApplyStateTransitionAt clears one
// the moment a task finishes, and auditMergedDocument is where the combination
// is refused, because a concurrent merge is the only thing that can compose it.
func currentSchemaFixtureProject() *models.Project {
	base := currentSchemaFixtureInstant
	parentID := "task-parent"
	completedAt := base.Add(3 * time.Hour)
	pinnedAt := base.Add(90 * time.Minute)
	deferredUntil := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 8, 14, 9, 30, 0, 0, time.Local)
	lastReviewedAt := base.Add(45 * time.Minute)
	// A check-in is a calendar date, so it is built the way the model builds one:
	// no zone has ever lost 2026-08-25, so the error cannot fire here, and reading
	// the date back through the live path has to land on this exact instant.
	nextCheckIn, _ := util.StartOfCalendarDate(2026, 8, 25, time.Local)

	parent := &models.Task{
		ID: parentID, ShortID: 1, Title: "Parent",
		WorkflowState: models.StateBacklog, Priority: 0,
		CreatedAt: base, UpdatedAt: base, StateChangedAt: base,
	}
	child := &models.Task{
		ID:              "task-child",
		ShortID:         2,
		Title:           "Every persisted field",
		Description:     "first line\nsecond line",
		WorkflowState:   models.StateDone,
		Priority:        2,
		ParentID:        &parentID,
		ChecklistOrigin: &models.ChecklistOrigin{ProjectID: "project-alpha", TaskID: parentID, Index: 3},
		Tags:            []string{"alpha", "beta"},
		Context:         "deep-work",
		DeferCount:      3,
		CreatedAt:       base,
		UpdatedAt:       base.Add(2 * time.Hour),
		StateChangedAt:  base.Add(time.Hour),
		CompletedAt:     &completedAt,
		DueDate:         &dueDate,
		Pinned:          true,
		PinnedAt:        &pinnedAt,
		Review:          true,
		Hidden:          true,
		ResumeNote:      "waiting on the staging deploy",
		Comments:        []*models.Comment{{Text: "a stored comment", CreatedAt: base.Add(30 * time.Minute)}},
		Attachments: []models.Attachment{
			{Filename: "notes.md", OriginalPath: "/tmp/notes.md", AddedAt: base},
		},
		DeferredUntil:     &deferredUntil,
		Checklist:         []models.ChecklistItem{{Text: "step one", Done: true}},
		Dependencies:      []models.TaskReference{{ProjectID: "project-beta", TaskID: "task-9"}},
		NextCheckIn:       &nextCheckIn,
		CheckInRepeatDays: 7,
		LastReviewedAt:    &lastReviewedAt,
	}
	return &models.Project{Name: "Fixture", Tasks: []*models.Task{parent, child}}
}

// persistedTaskJSON is the persisted half of a task. models.Task tags its
// computed fields json:"-", so encoding is exactly the set the storage layer is
// responsible for carrying, and comparing two encodings compares that set
// without a hand-maintained field list that could fall behind the struct.
func persistedTaskJSON(t *testing.T, task *models.Task) string {
	t.Helper()
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task %s: %v", task.ID, err)
	}
	return string(data)
}

// TestCurrentTaskFixtureSetsEveryPersistedField is the tripwire that keeps the
// round trip below honest. A field left at its zero value would round trip
// trivially, so a new persisted field fails here until the fixture carries a
// real value for it.
//
// There is deliberately no exemption list. One existed while a newer record
// codec sat beside the live one without being selected; the
// change that advanced the live schema deleted it, because a named exemption
// that outlives its reason is a round trip quietly proving less than it looks
// like it does.
func TestCurrentTaskFixtureSetsEveryPersistedField(t *testing.T) {
	fixture := currentSchemaFixtureProject().Tasks[1]
	value := reflect.ValueOf(*fixture)
	structType := value.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if strings.Split(field.Tag.Get("json"), ",")[0] == "-" {
			continue
		}
		if value.Field(i).IsZero() {
			t.Errorf(
				"fixture leaves persisted field %s at its zero value; a round trip over it proves nothing",
				field.Name,
			)
		}
	}
}

// TestCurrentTaskSchemaRoundTripCarriesEveryPersistedField exercises the whole
// live path -- model to record to bytes to record to model -- and compares the
// persisted field set on both ends.
func TestCurrentTaskSchemaRoundTripCarriesEveryPersistedField(t *testing.T) {
	project := currentSchemaFixtureProject()
	content, err := WriteProjectMarkdown(project)
	if err != nil {
		t.Fatalf("WriteProjectMarkdown: %v", err)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if parsed.ProjectName != project.Name {
		t.Fatalf("project name = %q, want %q", parsed.ProjectName, project.Name)
	}
	if len(parsed.Tasks) != len(project.Tasks) {
		t.Fatalf("parsed %d tasks, want %d", len(parsed.Tasks), len(project.Tasks))
	}
	for i, task := range parsed.Tasks {
		before := persistedTaskJSON(t, project.Tasks[i])
		after := persistedTaskJSON(t, task)
		if before != after {
			t.Errorf("task %d changed across a round trip\nbefore: %s\nafter:  %s", i, before, after)
		}
	}
	if !parsed.Tasks[1].DueHasTime() {
		t.Error("a timed due date came back as date-only, so due_time was dropped")
	}
}

// TestCurrentTaskSchemaGoldenBytes pins the exact rendering. It is the check an
// additive field must move deliberately: a change here that nobody intended is
// a format change nobody reviewed, and every stored revision in every workspace
// depends on these bytes.
func TestCurrentTaskSchemaGoldenBytes(t *testing.T) {
	project := currentSchemaFixtureProject()
	content, err := WriteProjectMarkdown(project)
	if err != nil {
		t.Fatalf("WriteProjectMarkdown: %v", err)
	}
	const golden = "# Project: Fixture\n" +
		"<!-- tt-schema: 20 -->\n" +
		"\n" +
		"## Task: Parent\n" +
		"<!-- id: task-parent, short_id: 1, state: backlog, priority: 0, created: 2026-08-01T12:00:00Z, updated: 2026-08-01T12:00:00Z, state_changed: 2026-08-01T12:00:00Z -->\n" +
		"\n" +
		"### Task: Every persisted field\n" +
		"<!-- id: task-child, short_id: 2, state: done, priority: 2, parent: task-parent, tags: alpha;beta, created: 2026-08-01T12:00:00Z, updated: 2026-08-01T14:00:00Z, state_changed: 2026-08-01T13:00:00Z, completed: 2026-08-01T15:00:00Z, due: 2026-08-14, deferred: 2026-08-20T09:00:00Z, pinned: true, pinned_at: 2026-08-01T13:30:00Z, attachments: file|notes.md|%2Ftmp%2Fnotes.md|0|2026-08-01T12:00:00Z, checklist: 1|step+one, context: deep-work, defer_count: 3, due_time: 09:30, depends: project-beta:task-9, review: true, check_in: 2026-08-25, check_in_repeat: 7, reviewed: 2026-08-01T12:45:00Z, hidden: true -->\n" +
		"<!-- checklist-origin: {\"project_id\":\"project-alpha\",\"task_id\":\"task-parent\",\"index\":3} -->\n" +
		"<!-- RESUME -->\n" +
		"waiting on the staging deploy\n" +
		"<!-- /RESUME -->\n" +
		"<!-- DESCRIPTION -->\n" +
		"first line\n" +
		"second line\n" +
		"<!-- /DESCRIPTION -->\n" +
		"<!-- COMMENTS -->\n" +
		"<!-- comment: 2026-08-01T12:30:00Z -->\n" +
		"a stored comment\n" +
		"<!-- /COMMENTS -->\n"
	if content != golden {
		t.Fatalf("live rendering changed\n got: %q\nwant: %q", content, golden)
	}
}

// TestCurrentTaskSchemaRevisionIsStableAcrossRoundTrips pins the revision to
// meaning rather than to bytes a caller happens to hold. A stale revision
// aborts a commit, so a revision that moved because a document was reloaded --
// not because anyone edited it -- would abort commits at random.
func TestCurrentTaskSchemaRevisionIsStableAcrossRoundTrips(t *testing.T) {
	project := currentSchemaFixtureProject()
	first := ToProjectDocument(project)
	content, err := first.WriteCurrent()
	if err != nil {
		t.Fatalf("WriteCurrent: %v", err)
	}
	second, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if second.Revision != first.Revision {
		t.Fatalf("revision moved on a round trip: %s then %s", first.Revision, second.Revision)
	}
	third, err := ParseProjectDocument("\uFEFF" + content + "\n\n")
	if err != nil {
		t.Fatalf("ParseProjectDocument with a BOM and trailing blank lines: %v", err)
	}
	if third.Revision != first.Revision {
		t.Fatalf("revision reads the caller's bytes rather than the canonical form: %s then %s", first.Revision, third.Revision)
	}
	edited, err := ParseProjectDocument(strings.Replace(content, "a stored comment", "an edited comment", 1))
	if err != nil {
		t.Fatalf("ParseProjectDocument after an edit: %v", err)
	}
	if edited.Revision == first.Revision {
		t.Fatal("a real content edit left the revision unchanged")
	}
}

// TestCurrentTaskSchemaEscapesDelimitersAndHeadings keeps prose out of the
// block grammar in both directions: text that spells a block delimiter or a
// document heading must survive verbatim, and must not be readable back as
// structure.
func TestCurrentTaskSchemaEscapesDelimitersAndHeadings(t *testing.T) {
	hostile := strings.Join([]string{
		"<!-- DESCRIPTION -->",
		"<!-- /DESCRIPTION -->",
		"<!-- COMMENTS -->",
		"## Task: not a heading",
		"# Project: not a project",
		"a & b --> c",
		`\## Task: already escaped`,
	}, "\n")

	base := currentSchemaFixtureInstant
	task := &models.Task{
		ID: "task-hostile", ShortID: 1, Title: "Hostile prose",
		Description:   hostile,
		WorkflowState: models.StateBacklog,
		CreatedAt:     base, UpdatedAt: base, StateChangedAt: base,
		Comments: []*models.Comment{{Text: hostile, CreatedAt: base}},
	}
	content, err := WriteProjectMarkdown(&models.Project{Name: "Escapes", Tasks: []*models.Task{task}})
	if err != nil {
		t.Fatalf("WriteProjectMarkdown: %v", err)
	}
	parsed, err := ParseProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseProjectDocument: %v", err)
	}
	if len(parsed.Tasks) != 1 {
		t.Fatalf("parsed %d tasks, want 1", len(parsed.Tasks))
	}
	if parsed.Tasks[0].Description != hostile {
		t.Errorf("description changed\n got: %q\nwant: %q", parsed.Tasks[0].Description, hostile)
	}
	if len(parsed.Tasks[0].Comments) != 1 {
		t.Fatalf("parsed %d comments, want 1", len(parsed.Tasks[0].Comments))
	}
	if parsed.Tasks[0].Comments[0].Text != hostile {
		t.Errorf("comment changed\n got: %q\nwant: %q", parsed.Tasks[0].Comments[0].Text, hostile)
	}
}

// TestCurrentTaskDocumentRefusesMalformedBodies keeps the live reader as strict
// as the frozen ones it shares a body grammar with. Each case is a body the
// writer can never produce, so accepting one would mean guessing at a file
// something else wrote.
func TestCurrentTaskDocumentRefusesMalformedBodies(t *testing.T) {
	// The header comes from the live writer rather than a literal, so this case
	// keeps testing the current schema after the marker line moves.
	empty, err := WriteProjectMarkdown(&models.Project{Name: "Strict"})
	if err != nil {
		t.Fatalf("WriteProjectMarkdown: %v", err)
	}
	header := empty + "## Task: Strict\n"
	metadata := "<!-- id: task-1, short_id: 1, state: backlog, priority: 0, " +
		"created: 2026-08-01T12:00:00Z, updated: 2026-08-01T12:00:00Z, " +
		"state_changed: 2026-08-01T12:00:00Z -->\n"
	for name, body := range map[string]string{
		"duplicate description": "<!-- DESCRIPTION -->\none\n<!-- /DESCRIPTION -->\n<!-- DESCRIPTION -->\ntwo\n<!-- /DESCRIPTION -->\n",
		"unterminated block":    "<!-- DESCRIPTION -->\ntext\n",
		"duplicate comments":    "<!-- COMMENTS -->\n<!-- comment: 2026-08-01T12:00:00Z -->\none\n<!-- /COMMENTS -->\n<!-- COMMENTS -->\n<!-- comment: 2026-08-01T12:00:00Z -->\ntwo\n<!-- /COMMENTS -->\n",
		"unknown block":         "<!-- SUMMARY -->\nlater\n<!-- /SUMMARY -->\n",
		"retired decisions":     "<!-- DECISIONS -->\n<!-- decision: 2026-08-01T12:00:00Z -->\nno\n<!-- /DECISIONS -->\n",
		"loose prose":           "just some text\n",
		"duplicate resume":      "<!-- RESUME -->\none\n<!-- /RESUME -->\n<!-- RESUME -->\ntwo\n<!-- /RESUME -->\n",
		"unterminated resume":   "<!-- RESUME -->\nlater\n",
		// The note is one line by contract and an absent one is written as no
		// block, so neither shape is something the canonical writer can produce.
		"multi-line resume": "<!-- RESUME -->\none\ntwo\n<!-- /RESUME -->\n",
		"empty resume":      "<!-- RESUME -->\n<!-- /RESUME -->\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseProjectDocument(header + metadata + body); err == nil {
				t.Fatalf("the live reader accepted a malformed body:\n%s", body)
			}
		})
	}
}

// TestWriteCurrentRefusesAnotherSchemaVersion keeps WriteCurrent honest about
// its name: a document carrying any other version is refused rather than
// rendered at the current one, which is what stops a migration's intermediate
// document from being written back as if it were live.
func TestWriteCurrentRefusesAnotherSchemaVersion(t *testing.T) {
	document := ToProjectDocument(currentSchemaFixtureProject())
	document.SchemaVersion = CurrentTaskSchemaVersion - 1
	if _, err := document.WriteCurrent(); err == nil {
		t.Fatal("WriteCurrent rendered a document that is not at the current schema")
	}
}
