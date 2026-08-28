package storage

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// An export is a document, not a view, so a reference has to survive as prose.
// When the target is loadable the label names it the way a person would; when it
// is not, the durable locator goes out unchanged rather than the line vanishing.
func TestWriteProjectMarkdownCleanNamesOutgoingDependencies(t *testing.T) {
	sibling := mustTask("sibling", 2, "Agree the schema")
	source := mustTask("source", 1, "Wire the importer")
	source.Dependencies = []models.TaskReference{
		{ProjectID: "proj-id", TaskID: "sibling"},
		{ProjectID: "proj-b", TaskID: "remote"},
		{ProjectID: "proj-b", TaskID: "gone"},
	}
	project := newProject("Alpha", source, sibling)

	remote := mustTask("remote", 7, "Publish the contract")
	beta, err := models.NewProject("proj-b", "Beta", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	beta.Slug = "beta"
	beta.Tasks = []*models.Task{remote}

	out := WriteProjectMarkdownClean(project, beta)
	if !strings.Contains(out, "**Dependencies:**") {
		t.Fatalf("export omitted the dependency section:\n%s", out)
	}
	for _, want := range []string{
		"- Alpha (#2) Agree the schema",
		"- Beta (#7) Publish the contract",
		"- proj-b:gone",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("export missing %q:\n%s", want, out)
		}
	}
}

// Dependents are computed from whatever happens to be loaded, so exporting them
// would freeze an artifact of the caller's population into a document. Only the
// task's own stored list is exported.
func TestWriteProjectMarkdownCleanOmitsComputedDependents(t *testing.T) {
	sibling := mustTask("sibling", 2, "Agree the schema")
	source := mustTask("source", 1, "Wire the importer")
	source.Dependencies = []models.TaskReference{{ProjectID: "proj-id", TaskID: "sibling"}}
	out := WriteProjectMarkdownClean(newProject("Alpha", source, sibling))

	if strings.Contains(strings.ToLower(out), "dependents") {
		t.Fatalf("export named computed dependents:\n%s", out)
	}
	if strings.Count(out, "**Dependencies:**") != 1 {
		t.Fatalf("dependency section count = %d, want exactly the referring task's:\n%s", strings.Count(out, "**Dependencies:**"), out)
	}
}

func TestWriteProjectMarkdownCleanOmitsTheSectionWithoutReferences(t *testing.T) {
	out := WriteProjectMarkdownClean(newProject("Alpha", mustTask("solo", 1, "Alone")))
	if strings.Contains(out, "**Dependencies:**") {
		t.Fatalf("a task with no references grew a dependency section:\n%s", out)
	}
}
