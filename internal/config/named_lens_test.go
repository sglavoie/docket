package config

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// validLens returns a definition that passes validation, so each test can make
// exactly one thing wrong. Its query names every facet family the canonical
// vocabulary carries, which is what the schema-16 carriers used to spell in
// struct fields.
func validLens() NamedLens {
	return NamedLens{
		ID:    "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01",
		Name:  "Review Queue",
		Mode:  LensModeAll,
		Query: validLensQuery,
		Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
	}
}

// validLensQuery is that fixture's narrowing, already in canonical order: a
// priority threshold, a tag, a context and the context gap, a due bucket, a
// state, the pin, the stale and deferred facets, a completion window, the
// root position spelled as a negated child clause, and both population axes.
const validLensQuery = "!2+ #deep-work ##errand ##0 @week $waiting *pin " +
	"?stale ?deferred ?completed:30d -?child +#client +#0 +parked +archived:only"

func TestNewMachineStateSeedsStarterLenses(t *testing.T) {
	m := NewMachineState()
	if err := m.Validate(); err != nil {
		t.Fatalf("seeded machine state should validate: %v", err)
	}
	names := make([]string, len(m.NamedLenses))
	for i, lens := range m.NamedLenses {
		names[i] = lens.Name
	}
	want := []string{"Waiting", "High Priority", "Stale"}
	if len(names) != len(want) {
		t.Fatalf("starter lens names = %v, want %v", names, want)
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("starter lens names = %v, want %v", names, want)
		}
	}

	waiting := m.NamedLensByID(lensWaitingStarterID)
	if waiting == nil {
		t.Fatal("Waiting starter missing")
	}
	if waiting.Mode != LensModeAll || waiting.Query != "$"+string(models.StateWaiting) {
		t.Fatalf("Waiting starter = %+v, want All mode with the waiting state", waiting)
	}
	if waiting.Sort.Field != LensSortDefault || !waiting.Sort.Ascending {
		t.Fatalf("Waiting starter sort = %+v, want default ascending", waiting.Sort)
	}

	high := m.NamedLensByID(lensHighPriorityStartID)
	if high == nil {
		t.Fatal("High Priority starter missing")
	}
	if high.Mode != LensModeAll || high.Query != fmt.Sprintf("!%d+", models.PriorityHigh) {
		t.Fatalf("High Priority starter = %+v, want All mode at the High threshold", high)
	}
	if high.Sort.Field != LensSortPriority || !high.Sort.Ascending {
		t.Fatalf("High Priority starter sort = %+v, want priority ascending", high.Sort)
	}

	stale := m.NamedLensByID(lensStaleStarterID)
	if stale == nil {
		t.Fatal("Stale starter missing")
	}
	if stale.Mode != LensModeAll || stale.Query != "?stale" {
		t.Fatalf("Stale starter = %+v, want All mode with the stale facet", stale)
	}
	if stale.Sort.Field != LensSortUpdated || !stale.Sort.Ascending {
		t.Fatalf("Stale starter sort = %+v, want updated ascending", stale.Sort)
	}
}

func TestStarterLensIDsAreCanonicalUUIDs(t *testing.T) {
	for _, lens := range StarterNamedLenses() {
		if err := ValidateNamedLensID(lens.ID); err != nil {
			t.Errorf("starter %q id: %v", lens.Name, err)
		}
	}
}

// TestBuiltinNamedLensTemplatesExactCatalog pins every field of every template.
// These definitions are durable product identities, not defaults: a changed ID
// orphans an added lens and a changed facet silently redefines a saved view, so
// each is asserted exactly rather than spot-checked.
func TestBuiltinNamedLensTemplatesExactCatalog(t *testing.T) {
	want := []NamedLens{
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c01",
			Name:  "Waiting",
			Mode:  LensModeAll,
			Query: "$waiting",
			Sort:  LensSort{Field: LensSortDefault, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c02",
			Name:  "High Priority",
			Mode:  LensModeAll,
			Query: "!1+",
			Sort:  LensSort{Field: LensSortPriority, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c03",
			Name:  "Stale",
			Mode:  LensModeAll,
			Query: "?stale",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c04",
			Name:  "Overdue",
			Mode:  LensModeDue,
			Query: "@overdue",
			Sort:  LensSort{Field: LensSortDefault, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c05",
			Name:  "Active Work",
			Mode:  LensModeAll,
			Query: "$doing",
			Sort:  LensSort{Field: LensSortPriority, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c06",
			Name:  "Review Queue",
			Mode:  LensModeAll,
			Query: "=review",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c07",
			Name:  "Deferred",
			Mode:  LensModeAll,
			Query: "?deferred",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c08",
			Name:  "Cleanup 90d",
			Mode:  LensModeAll,
			Query: "$canceled $done ?completed:90d",
			Sort:  LensSort{Field: LensSortUpdated, Ascending: true},
		},
		{
			ID:    "6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6c09",
			Name:  "Parked Work",
			Mode:  LensModeAll,
			Query: "+parked:only",
			Sort:  LensSort{Field: LensSortDefault, Ascending: true},
		},
	}

	got := BuiltinNamedLensTemplates()
	if len(got) != len(want) {
		t.Fatalf("template count = %d, want %d", len(got), len(want))
	}
	for i, wantLens := range want {
		t.Run(wantLens.Name, func(t *testing.T) {
			gotJSON, wantJSON := mustJSON(t, got[i]), mustJSON(t, wantLens)
			if gotJSON != wantJSON {
				t.Fatalf("template %d:\n got %s\nwant %s", i, gotJSON, wantJSON)
			}
		})
	}
}

// A built-in query is a durable string, so it must already be exactly what
// models.Query.String renders: a template whose literal is merely equivalent to
// the canonical spelling would compare unequal to the same lens saved from the
// view it describes.
func TestBuiltinNamedLensTemplateQueriesAreCanonical(t *testing.T) {
	for _, lens := range BuiltinNamedLensTemplates() {
		parsed, err := models.ParseQuery(lens.Query)
		if err != nil {
			t.Errorf("template %q query %q does not parse: %v", lens.Name, lens.Query, err)
			continue
		}
		if rendered := parsed.String(); rendered != lens.Query {
			t.Errorf("template %q query = %q, want canonical %q", lens.Name, lens.Query, rendered)
		}
	}
}

func TestBuiltinNamedLensTemplatesValidateAsACollection(t *testing.T) {
	// Collection validation is what proves the frozen IDs stay canonical and
	// distinct and that no two template names fold together.
	if err := BuiltinNamedLensTemplates().Validate(); err != nil {
		t.Fatalf("built-in template catalog should validate: %v", err)
	}
	if err := StarterNamedLenses().Validate(); err != nil {
		t.Fatalf("starter catalog should validate: %v", err)
	}
}

func TestStarterNamedLensesArePrefixOfTemplates(t *testing.T) {
	templates := BuiltinNamedLensTemplates()
	starters := StarterNamedLenses()
	if len(starters) != 3 {
		t.Fatalf("starter count = %d, want 3; automatic seeding must not grow with the catalog", len(starters))
	}
	if got, want := mustJSON(t, starters), mustJSON(t, templates[:3]); got != want {
		t.Fatalf("starters diverged from the first three templates:\n got %s\nwant %s", got, want)
	}
}

func TestBuiltinNamedLensTemplatesDetachOnEveryCall(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func() NamedLensList
	}{
		{"templates", BuiltinNamedLensTemplates},
		{"starters", StarterNamedLenses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseline := mustJSON(t, tc.call())

			mutated := tc.call()
			mutated[0].Name = "Mutated"
			mutated[0].Query = "$mutated-state"
			if grown := append(mutated, validLens()); len(grown) != len(mutated)+1 {
				t.Fatalf("append length = %d, want %d", len(grown), len(mutated)+1)
			}

			if got := mustJSON(t, tc.call()); got != baseline {
				t.Fatalf("mutating a returned catalog leaked into a later call:\n got %s\nwant %s", got, baseline)
			}
		})
	}
}

// A template that shares a backing array with the catalog would let a starter
// append overwrite template four, which callers never observe directly.
func TestStarterNamedLensesAppendCannotReachRemainingTemplates(t *testing.T) {
	starters := StarterNamedLenses()
	starters = append(starters, validLens())
	if got := BuiltinNamedLensTemplates()[3].Name; got != "Overdue" {
		t.Fatalf("appending to the starters overwrote template 4: %q", got)
	}
	if len(starters) != 4 {
		t.Fatalf("starter length after append = %d, want 4", len(starters))
	}
}
