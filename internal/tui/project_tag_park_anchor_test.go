package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/route"
)

func newSession4TaggedProject(t *testing.T, name string, tags ...string) *models.Project {
	t.Helper()
	project := newSession4Project(t, name)
	project.Tags = tags
	return project
}

// parkTagMembersThroughProjectTagVocabulary drives the F# tag row action the
// way the vocabulary sheet does: the tag's members become the bulk targets, and
// the park is answered at the shared confirmation. It returns the committed
// follow-up so the caller can inspect the anchor before the reload consumes it.
func parkTagMembersThroughProjectTagVocabulary(
	t *testing.T,
	fixture *session4LifecycleFixture,
	tag string,
) tea.Cmd {
	t.Helper()
	app := fixture.app
	members := app.projectTagMembers(tag)
	if len(members) == 0 {
		t.Fatalf("no available project carries #%s", tag)
	}
	targets := make([]operation.OperationTarget, 0, len(members))
	for _, member := range members {
		targets = append(targets, operation.OperationTargetFor(operation.ProjectEntity(member.ID)))
	}
	app.projectTagActionSubject = tag
	result := app.executeProjectTagVocabulary(
		operation.OperationInvocation{Descriptor: operation.OperationDescriptor{ID: operationProjectTagParkAll}},
		targets,
	)
	if result.Err != nil {
		t.Fatalf("Park all members: %v", result.Err)
	}
	msg, ok := result.Message.(ProjectBulkTagsMsg)
	if !ok {
		t.Fatalf("Park all members emitted %#v, want a bulk tag request", result.Message)
	}
	if cmd := app.requestProjectBulkTags(msg); cmd != nil {
		t.Fatalf("bulk park committed without confirmation: %#v", cmd())
	}
	handled, cmd := app.handleProjectMessages(projectParkConfirmedMsg{})
	if !handled {
		t.Fatal("park confirmation was not handled")
	}
	return cmd
}

func TestProjectTagParkAllMembersAnchorsTheOutlinerCursorOnASurvivor(t *testing.T) {
	first := newSession4TaggedProject(t, "Sonar", "hold")
	survivor := newSession4Project(t, "Radar")
	second := newSession4TaggedProject(t, "Keel", "hold")
	fixture := newSession4LifecycleFixture(t, first, survivor, second)

	cmd := parkTagMembersThroughProjectTagVocabulary(t, fixture, "hold")
	if got := fixture.cross.crossReloadHeaderProjectID; got != survivor.ID {
		t.Fatalf("bulk park anchor = %q, want the surviving header %q", got, survivor.ID)
	}
	runSession2AppCommand(t, fixture.app, cmd)

	if fixture.cross.cursor < 0 || fixture.cross.cursor >= len(fixture.cross.rows) {
		t.Fatalf("post-park cursor %d is outside %d rows", fixture.cross.cursor, len(fixture.cross.rows))
	}
	row := fixture.cross.rows[fixture.cross.cursor]
	if !row.IsProjectHeader || row.ProjectID != survivor.ID {
		t.Fatalf("post-park cursor row = %#v, want the surviving project header", row)
	}
}

// The Project Hub keeps parked rows visible, so a bulk park committed from it
// must leave the Outliner cursor exactly where the user left it.
func TestProjectBulkParkFromTheProjectHubCapturesNoOutlinerAnchor(t *testing.T) {
	member := newSession4TaggedProject(t, "Sonar", "hold")
	sibling := newSession4Project(t, "Radar")
	fixture := newSession4LifecycleFixture(t, member, sibling)
	hub := mustRoute(t, route.SecondaryTool, route.ManageParams{Module: route.ManageProjects})
	if err := fixture.app.navigation.Primary.PushChild(hub); err != nil {
		t.Fatalf("push the Project Hub route: %v", err)
	}

	if cmd := fixture.app.requestProjectBulkTags(ProjectBulkTagsMsg{
		ProjectIDs: []string{member.ID},
		Action:     ProjectBulkTagPark,
	}); cmd != nil {
		t.Fatalf("bulk park committed without confirmation: %#v", cmd())
	}
	if handled, _ := fixture.app.handleProjectMessages(projectParkConfirmedMsg{}); !handled {
		t.Fatal("park confirmation was not handled")
	}
	if got := fixture.cross.crossReloadHeaderProjectID; got != "" {
		t.Fatalf("hub bulk park captured anchor %q, want none", got)
	}
}

// A tag whose every visible member is parked leaves no sibling header, so the
// anchor is a departing project that no longer resolves and the cursor settles
// at the top of the projection instead of wherever the numeric fallback lands.
func TestProjectTagParkAllMembersWithNoSurvivorLandsAtTheTopOfTheProjection(t *testing.T) {
	first := newSession4TaggedProject(t, "Sonar", "hold")
	second := newSession4TaggedProject(t, "Keel", "hold")
	fixture := newSession4LifecycleFixture(t, first, second)
	fixture.cross.cursor = len(fixture.cross.rows) - 1

	cmd := parkTagMembersThroughProjectTagVocabulary(t, fixture, "hold")
	anchor := fixture.cross.crossReloadHeaderProjectID
	if anchor != first.ID && anchor != second.ID {
		t.Fatalf("bulk park anchor = %q, want a departing project as the section-header sentinel", anchor)
	}
	runSession2AppCommand(t, fixture.app, cmd)

	if fixture.cross.cursor != 0 {
		t.Fatalf("post-park cursor = %d, want the top of the projection", fixture.cross.cursor)
	}
	for _, row := range fixture.cross.rows {
		if row.IsProjectHeader && (row.ProjectID == first.ID || row.ProjectID == second.ID) {
			t.Fatalf("parked project %q kept a header row", row.ProjectID)
		}
	}
}
