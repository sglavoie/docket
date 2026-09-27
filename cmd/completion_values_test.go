package cmd

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

func completionChoices(values []cobra.Completion) []string {
	var out []string
	for _, value := range values {
		choice, _, _ := strings.Cut(value, "\t")
		out = append(out, choice)
	}
	return out
}

func TestCompleteProjectSlugsListsEveryCatalogedProject(t *testing.T) {
	newExportFixtureWorkspace(t)

	values, directive := completeProjectSlugs(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	got := completionChoices(values)
	for _, want := range []string{"alpha", "old", config.SystemUnfiledSlug} {
		if !slices.Contains(got, want) {
			t.Errorf("slugs %v lack %q", got, want)
		}
	}
	if !slices.Contains(values, cobra.CompletionWithDesc("old", "Old Work")) {
		t.Errorf("archived slug lacks its name description: %v", values)
	}

	filtered, _ := completeProjectSlugs(nil, nil, "al")
	if got := completionChoices(filtered); !slices.Equal(got, []string{"alpha"}) {
		t.Errorf("prefix al = %v, want [alpha]", got)
	}
	if extra, _ := completeProjectSlugs(nil, []string{"alpha"}, ""); len(extra) != 0 {
		t.Errorf("second argument completed %v, want nothing", extra)
	}
}

func TestCompleteProjectSlugsWritesNothingForAMissingWorkspace(t *testing.T) {
	root := t.TempDir() + "/absent"
	setCommandDataDir(t, root)
	t.Setenv("HOME", t.TempDir())

	values, directive := completeProjectSlugs(nil, nil, "")
	if len(values) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completion = %v, %v; want nothing and NoFileComp", values, directive)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("completion created the workspace: %v", err)
	}
}

func TestCompleteProjectSlugsQuietOnUnreadableCatalog(t *testing.T) {
	root := t.TempDir()
	setCommandDataDir(t, root)
	t.Setenv("HOME", t.TempDir())
	if err := os.MkdirAll(config.InternalDir(root), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.WorkspaceCatalogPath(root), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	values, directive := completeProjectSlugs(nil, nil, "")
	if len(values) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completion = %v, %v; want nothing and NoFileComp", values, directive)
	}
}

func TestCompleteProjectSlugsLeavesACorruptCatalogAndItsBackupAlone(t *testing.T) {
	newExportFixtureWorkspace(t)
	catalogPath := config.WorkspaceCatalogPath(completionDataDir())
	healthy, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	backupPath := catalogPath + ".bak"
	if err := os.WriteFile(backupPath, healthy, 0o600); err != nil {
		t.Fatal(err)
	}
	broken := []byte("{broken catalog")
	if err := os.WriteFile(catalogPath, broken, 0o600); err != nil {
		t.Fatal(err)
	}

	values, directive := completeProjectSlugs(nil, nil, "")
	if len(values) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completion = %v, %v; want nothing and NoFileComp", values, directive)
	}
	if got, _ := os.ReadFile(catalogPath); !bytes.Equal(got, broken) {
		t.Errorf("completion rewrote the corrupt catalog: %q", got)
	}
	if got, _ := os.ReadFile(backupPath); !bytes.Equal(got, healthy) {
		t.Errorf("completion changed the backup: %q", got)
	}
}

func TestCompleteConflictIDsOffersShortIDsWithProjectNames(t *testing.T) {
	store, project := setupConflictTest(t)
	id := recordCLIConflict(t, store, project)

	values, directive := completeConflictIDs(nil, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp", directive)
	}
	want := cobra.CompletionWithDesc(shortConflictID(id), project.Name)
	if !slices.Equal(values, []cobra.Completion{want}) {
		t.Errorf("conflict completion = %v, want [%q]", values, want)
	}
	if miss, _ := completeConflictIDs(nil, nil, "zzzz"); len(miss) != 0 {
		t.Errorf("non-matching prefix completed %v", miss)
	}
	if extra, _ := completeConflictIDs(nil, []string{id}, ""); len(extra) != 0 {
		t.Errorf("second argument completed %v, want nothing", extra)
	}
}

func TestCompleteConflictIDsQuietWithoutConflicts(t *testing.T) {
	setCommandDataDir(t, t.TempDir())
	t.Setenv("HOME", t.TempDir())
	values, directive := completeConflictIDs(nil, nil, "")
	if len(values) != 0 || directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("completion = %v, %v; want nothing and NoFileComp", values, directive)
	}
}

func TestCompletionFunctionsAreWired(t *testing.T) {
	for _, command := range []*cobra.Command{projectExportCmd, conflictShowCmd, conflictResolveCmd} {
		if command.ValidArgsFunction == nil {
			t.Errorf("%s has no ValidArgsFunction", command.CommandPath())
		}
	}
}

func TestCompleteProjectSlugsOffersNothingBesideAll(t *testing.T) {
	newExportFixtureWorkspace(t)
	resetProjectExportFlags(t)
	if err := projectExportCmd.Flags().Set("all", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = projectExportCmd.Flags().Set("all", "false") })

	if values, _ := completeProjectSlugs(projectExportCmd, nil, ""); len(values) != 0 {
		t.Fatalf("--all still completed slugs: %v", values)
	}
}
