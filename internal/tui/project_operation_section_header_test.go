package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/operation"
)

// A section header names a lane, not a project. projectOperationApplicable ends
// in a scope fallback, so in a single-project scope that row would otherwise
// hand a project key the sole scoped project it never named — `*` silently
// pinning it, `#` silently opening its tag picker. These cases pin the gate that
// closes it, and the shapes below are the two the context builders actually
// produce: AppModel leaves ProjectID empty on a section header while
// OutlinerModel fills it from the scoped project, and the gate has to refuse
// both or the footer advertises a key dispatch declines.
func TestProjectOperationsRefuseASectionHeaderCursor(t *testing.T) {
	scoped := func(entity operation.EntityContext) operation.CommandContext {
		ctx := simplifiedOperationContext()
		ctx.Entity = entity
		ctx.CurrentEntity = operation.CommandContextCurrentEntity(entity)
		ctx.ScopeProjectIDs = []string{"only-project"}
		return ctx
	}

	for name, ctx := range map[string]operation.CommandContext{
		"AppModel shape, ProjectID unfilled": scoped(operation.EntityContext{
			HasSectionHeaderCursor: true,
		}),
		"OutlinerModel shape, ProjectID from the scoped project": scoped(operation.EntityContext{
			ProjectID:              "only-project",
			HasSectionHeaderCursor: true,
		}),
	} {
		if projectOperationApplicable(ctx) {
			t.Errorf("%s: a section header cursor admits a project operation", name)
		}
	}

	// The positive control: the same single-project scope on a project header,
	// which is the row that does name a project.
	header := scoped(operation.EntityContext{ProjectID: "only-project", HasProjectHeaderCursor: true})
	if !projectOperationApplicable(header) {
		t.Error("a project header cursor no longer admits a project operation")
	}
}

// TestSectionHeaderClaimsNeitherArmOfASharedKey walks the shared keys through
// the trie rather than the predicate, in the shape the leak actually lives in: a
// section header in a single-project Outliner, whose context builder fills
// ProjectID from that project so the Archived guard has something to read. The
// typed identity is then a project, which makes the task arm decline, so without
// the gate the project arm answers a row that named no project. Both arms
// declining is what the row looked like before `*` was shared, and an unbound
// root key is silent.
func TestSectionHeaderClaimsNeitherArmOfASharedKey(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)

	section := simplifiedOperationContext()
	section.Entity = operation.EntityContext{ProjectID: "only-project", HasSectionHeaderCursor: true}
	section.CurrentEntity = operation.CommandContextCurrentEntity(section.Entity)
	section.ScopeProjectIDs = []string{"only-project"}
	for _, key := range []string{"*", "#"} {
		if binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, key, section); ok {
			t.Errorf("%s on a section header resolves %s", key, binding.OperationID)
		}
	}

	// The positive control, one row away: the header that does name a project
	// answers with the project arm, under the label the footer teaches.
	header := simplifiedOperationContext()
	header.Entity = operation.EntityContext{ProjectID: "only-project", HasProjectHeaderCursor: true}
	header.CurrentEntity = operation.CommandContextCurrentEntity(header.Entity)
	header.ScopeProjectIDs = []string{"only-project"}
	binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, "*", header)
	if !ok || binding.OperationID != operationScopePinProject {
		t.Fatalf("* on a project header = %s available=%t, want %s", binding.OperationID, ok, operationScopePinProject)
	}
	if binding.FooterLabel != "project pin" {
		t.Errorf("project header * footer label = %q, want %q", binding.FooterLabel, "project pin")
	}
}
