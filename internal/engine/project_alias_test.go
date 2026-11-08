package engine

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
)

// TestEditProjectSetsKeepsAndClearsTheAlias drives the alias through the one
// project-edit use case: a set value is trimmed and committed with the rest of
// the edit, a nil Alias keeps what is stored so a surface that never edits it
// cannot erase it, a pointer to "" clears it, and an invalid value is refused as
// input before anything is written.
func TestEditProjectSetsKeepsAndClearsTheAlias(t *testing.T) {
	env := newHeadlessEnvironment(t)
	runtime := openHeadless(t, env.config(true))
	created, err := runtime.CreateProject("Infrastructure Automation")
	if err != nil {
		t.Fatal(err)
	}
	id := created.Project.ID
	stored := func() string {
		t.Helper()
		// The catalog on disk, not the runtime's copy, is what another device
		// and the next launch read.
		data, readErr := os.ReadFile(config.WorkspaceCatalogPath(env.dataDir))
		if readErr != nil {
			t.Fatal(readErr)
		}
		var envelope struct {
			Data config.WorkspaceCatalog `json:"data"`
		}
		if decodeErr := json.Unmarshal(data, &envelope); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		for _, project := range envelope.Data.Projects {
			if project.ID == id {
				return project.Alias
			}
		}
		t.Fatalf("project %s is not in the catalog", id)
		return ""
	}
	alias := func(value string) *string { return &value }

	saved, err := runtime.EditProject(id, ProjectEdit{Name: "Infrastructure Automation", Alias: alias("  IA ")})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Project.Alias != "IA" || stored() != "IA" {
		t.Fatalf("alias = %q in the result, %q stored; want %q", saved.Project.Alias, stored(), "IA")
	}

	if _, err = runtime.EditProject(id, ProjectEdit{Name: "Infrastructure Automation", Description: "Terraform and friends"}); err != nil {
		t.Fatal(err)
	}
	if got := stored(); got != "IA" {
		t.Fatalf("an edit naming no alias changed it to %q", got)
	}

	_, err = runtime.EditProject(id, ProjectEdit{Name: "Infrastructure Automation", Alias: alias("two words")})
	var invalid *app.InvalidInputError
	if !errors.As(err, &invalid) || invalid.Field != "project alias" {
		t.Fatalf("EditProject = %v, want an invalid project alias", err)
	}
	if got := stored(); got != "IA" {
		t.Fatalf("a refused edit changed the alias to %q", got)
	}

	if _, err = runtime.EditProject(id, ProjectEdit{Name: "Infrastructure Automation", Alias: alias("")}); err != nil {
		t.Fatal(err)
	}
	if got := stored(); got != "" {
		t.Fatalf("alias = %q, want it cleared", got)
	}
}
