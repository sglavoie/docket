package config

import "testing"

// TestReconcileCatalogArtifactsMergesAliasFieldWise treats the project alias
// like the name: one device's alias survives another's concurrent edit of a
// different field, two different aliases compete and keep the primary version
// whole, and a clear one side made is a change like any other.
func TestReconcileCatalogArtifactsMergesAliasFieldWise(t *testing.T) {
	base := reconcileCatalog(t, nil)
	aliased := reconcileCatalog(t, func(c *WorkspaceCatalog) { c.Projects[1].Alias = "IA" })
	renamed := reconcileCatalog(t, func(c *WorkspaceCatalog) { c.Projects[1].Name = "Alpha Prime" })
	merged, conflicts, err := ReconcileCatalogArtifacts(base, [][]byte{renamed, aliased})
	if err != nil {
		t.Fatal(err)
	}
	c := decodeReconciledCatalog(t, merged)
	if c.Projects[1].Alias != "IA" || c.Projects[1].Name != "Alpha Prime" {
		t.Fatalf("project = %+v, want the alias and the rename both kept", c.Projects[1])
	}
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v, want none", conflicts)
	}

	other := reconcileCatalog(t, func(c *WorkspaceCatalog) { c.Projects[1].Alias = "web" })
	merged, conflicts, err = ReconcileCatalogArtifacts(base, [][]byte{other, aliased})
	if err != nil {
		t.Fatal(err)
	}
	c = decodeReconciledCatalog(t, merged)
	if c.Projects[1].Alias != "web" {
		t.Fatalf("alias = %q, want the primary version's %q", c.Projects[1].Alias, "web")
	}
	if len(conflicts) != 1 || conflicts[0].Field != "alias" {
		t.Fatalf("conflicts = %+v, want one alias conflict", conflicts)
	}

	cleared := reconcileCatalog(t, func(c *WorkspaceCatalog) { c.Projects[1].Alias = "" })
	merged, _, err = ReconcileCatalogArtifacts(aliased, [][]byte{cleared, aliased})
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeReconciledCatalog(t, merged).Projects[1].Alias; got != "" {
		t.Fatalf("alias = %q, want the clear one side made kept", got)
	}
}
