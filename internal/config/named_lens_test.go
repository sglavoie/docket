package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestValidateNamedLensID(t *testing.T) {
	cases := map[string]bool{
		"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01":   true,
		"":                                       false,
		"not-a-uuid":                             false,
		"9F8B8E42-7C3A-4D61-8F2A-1B0C9D7E5A01":   false, // uppercase is not canonical
		"{9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01}": false,
		"urn:uuid:9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01":   false,
		"9f8b8e427c3a4d618f2a1b0c9d7e5a01":                false, // unhyphenated
		"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01-9f8b8e4200": false,
	}
	for id, wantValid := range cases {
		err := ValidateNamedLensID(id)
		if wantValid && err != nil {
			t.Errorf("ValidateNamedLensID(%q) = %v, want nil", id, err)
		}
		if !wantValid && err == nil {
			t.Errorf("ValidateNamedLensID(%q) = nil, want error", id)
		}
	}
}

func TestNormalizeNamedLensName(t *testing.T) {
	got, err := NormalizeNamedLensName("  Review Queue  ")
	if err != nil {
		t.Fatalf("NormalizeNamedLensName: %v", err)
	}
	if got != "Review Queue" {
		t.Errorf("NormalizeNamedLensName trimmed = %q, want %q", got, "Review Queue")
	}

	for _, raw := range []string{"", "   ", "line\nbreak", "carriage\rreturn", "bell\a", strings.Repeat("x", MaxNamedLensNameRunes+1)} {
		if _, err := NormalizeNamedLensName(raw); err == nil {
			t.Errorf("NormalizeNamedLensName(%q) = nil error, want rejection", raw)
		}
	}
	if _, err := NormalizeNamedLensName(strings.Repeat("é", MaxNamedLensNameRunes)); err != nil {
		t.Errorf("a %d-rune multibyte name should fit: %v", MaxNamedLensNameRunes, err)
	}
}

func TestNamedLensValidateRejectsMalformedTokens(t *testing.T) {
	cases := []struct {
		name  string
		mutas func(*NamedLens)
	}{
		{"mode", func(l *NamedLens) { l.Mode = "everything" }},
		{"empty mode", func(l *NamedLens) { l.Mode = "" }},
		{"sort field", func(l *NamedLens) { l.Sort.Field = "color" }},
		{"retired status sort field", func(l *NamedLens) { l.Sort.Field = "status" }},
		{"overlong query", func(l *NamedLens) { l.Query = strings.Repeat("x", MaxNamedLensQueryRunes+1) }},
		{"query newline", func(l *NamedLens) { l.Query = "#one\n#two" }},
		{"query control character", func(l *NamedLens) { l.Query = "#one\a" }},
		{"untrimmed name", func(l *NamedLens) { l.Name = " Review Queue" }},
		{"id", func(l *NamedLens) { l.ID = "lens-1" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lens := validLens()
			tc.mutas(&lens)
			if err := lens.Validate(); err == nil {
				t.Fatalf("%s should be rejected", tc.name)
			}
		})
	}
}

func TestNamedLensValidateAcceptsCanonicalDefinition(t *testing.T) {
	if err := validLens().Validate(); err != nil {
		t.Fatalf("canonical lens should validate: %v", err)
	}
}

func TestNamedLensValidateAcceptsOmittedTokenDefaults(t *testing.T) {
	// omitempty writes "" for every axis left at its default, so loading one
	// back must read as the default rather than as a malformed token.
	lens := NamedLens{
		ID:   "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a02",
		Name: "Bare",
		Mode: LensModeToday,
	}
	if err := lens.Validate(); err != nil {
		t.Fatalf("bare lens should validate: %v", err)
	}
}

func TestNamedLensValidateAcceptsCanonicalUnusedTags(t *testing.T) {
	// A canonical tag no task carries is valid; it simply matches nothing.
	lens := validLens()
	lens.Query = "#nobody-uses-this ##nobody-is-here +#nor-this-one"
	if err := lens.Validate(); err != nil {
		t.Fatalf("canonical unused tags should validate: %v", err)
	}
}

// Validation bounds the query and refuses a control character in it; it
// deliberately does not parse it. A query the vocabulary cannot read keeps the
// ordinary invalid-query guidance at projection time instead of making a whole
// workspace file unloadable.
func TestNamedLensValidateDoesNotParseTheQuery(t *testing.T) {
	for _, query := range []string{
		"unparseable-prose",
		"@+3h",
		"$today",
		strings.Repeat("x", MaxNamedLensQueryRunes),
	} {
		lens := validLens()
		lens.Query = query
		if err := lens.Validate(); err != nil {
			t.Errorf("query %q should validate: %v", query, err)
		}
	}
}

// The context facet is two independent halves: a vocabulary and the gap the
// vocabulary cannot name. Either alone is a legal saved facet, so neither may
// require the other to be present.
func TestNamedLensValidateAcceptsEitherHalfOfTheContextFacet(t *testing.T) {
	gapOnly := validLens()
	gapOnly.Query = "##0"
	if err := gapOnly.Validate(); err != nil {
		t.Fatalf("the context gap alone should validate: %v", err)
	}

	namesOnly := validLens()
	namesOnly.Query = "##deep ##errand"
	if err := namesOnly.Validate(); err != nil {
		t.Fatalf("context names alone should validate: %v", err)
	}
}

// A saved query must survive the machine payload unchanged. omitempty drops it
// when the lens narrows nothing, and an absent query is what a record naming no
// narrowing carries, so it must decode as "no query" rather than as a malformed
// one.
func TestNamedLensQueryJSONRoundTripAndOmission(t *testing.T) {
	lens := validLens()
	data, err := json.Marshal(lens)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"query":"`+validLensQuery+`"`) {
		t.Fatalf("query missing from payload: %s", data)
	}
	var decoded NamedLens
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Query != validLensQuery {
		t.Fatalf("query round trip = %q, want %q", decoded.Query, validLensQuery)
	}

	absent := validLens()
	absent.Query = ""
	bare, err := json.Marshal(absent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(bare), "query") {
		t.Fatalf("an absent query should be omitted entirely: %s", bare)
	}
	var reread NamedLens
	if err := json.Unmarshal(bare, &reread); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reread.Query != "" {
		t.Fatalf("omitted query decoded as %q", reread.Query)
	}
	if err := reread.Validate(); err != nil {
		t.Fatalf("a record naming no narrowing must still validate: %v", err)
	}
}

// A schema-16 record is lifted rather than refused: the two lens homes upgrade
// at different times, so a machine-local list an older build wrote must still
// load. Every carrier facet becomes its canonical clause, and the saved review
// tag becomes the stored review predicate.
func TestNamedLensUnmarshalLiftsSchema16Carriers(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "review tag becomes the review predicate",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"tags":["review"]}}`,
			want:    "=review",
		},
		{
			name:    "review search term becomes the review predicate",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","query":"#review","tasks":{"stale":true}}`,
			// The lifted term joins the clause half, so canonical field order puts
			// review ahead of stale rather than leaving it where it was typed.
			want: "=review ?stale",
		},
		{
			name:    "negated review search term keeps its polarity",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","query":"-#review","tasks":{}}`,
			want:    "-=review",
		},
		{
			name:    "stored review flag becomes the review predicate",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"review":true}}`,
			want:    "=review",
		},
		{
			name: "every carrier family lifts in canonical order",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all",` +
				`"tasks":{"states":["waiting"],"states_set":true,"priority":2,"tags":["deep-work"],` +
				`"contexts":["errand"],"no_context":true,"due_date":"week","completion_age":"30d",` +
				`"completion_age_set":true,"hierarchy":"root","stale":true,"deferred":true,"pinned":true},` +
				`"projects":{"tags":["client"],"untagged":true,"parked":"include","archived":"only"}}`,
			want: validLensQuery,
		},
		{
			name: "prose the canonical vocabulary cannot spell follows the clauses",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all",` +
				`"query":"needle %alpha","tasks":{"pinned":true}}`,
			want: "*pin needle %alpha",
		},
		{
			name:    "the retired status sort field becomes the default",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","sort":{"field":"status"}}`,
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var decoded NamedLens
			if err := json.Unmarshal([]byte(tc.payload), &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded.Query != tc.want {
				t.Fatalf("lifted query = %q, want %q", decoded.Query, tc.want)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatalf("a lifted record must validate: %v", err)
			}
			// The carriers are gone from the live shape, so a re-encode drops them
			// and the record self-heals the next time this machine writes it.
			if data := mustJSON(t, decoded); strings.Contains(data, `"tasks"`) ||
				strings.Contains(data, `"projects"`) {
				t.Fatalf("a lifted record re-encoded a carrier: %s", data)
			}
		})
	}

	// The retired sort token is rewritten rather than refused, and only there:
	// an unknown one is still malformed.
	var status NamedLens
	if err := json.Unmarshal([]byte(
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","sort":{"field":"status"}}`,
	), &status); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if status.Sort.Field != LensSortDefault {
		t.Fatalf("status sort field = %q, want %q", status.Sort.Field, LensSortDefault)
	}
}

// An unmintable carrier value is refused rather than dropped: a lens whose
// narrowing this build cannot express must fail loudly instead of silently
// selecting different rows.
func TestNamedLensUnmarshalRefusesUnmintableCarrierValues(t *testing.T) {
	for _, payload := range []string{
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"states":["ready"],"states_set":true}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"priority":4}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"due_date":"fortnight"}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"hierarchy":"sibling"}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"completion_age":"14d","completion_age_set":true}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","tasks":{"tags":["Deep-Work"]}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","projects":{"parked":"maybe"}}`,
		`{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","projects":{"archived":"maybe"}}`,
	} {
		var decoded NamedLens
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			t.Errorf("payload %s should be refused", payload)
		}
	}
}

func TestNamedLensListValidateRejectsDuplicates(t *testing.T) {
	base := validLens()

	dupID := base
	dupID.Name = "Other Name"
	if err := (NamedLensList{base, dupID}).Validate(); err == nil {
		t.Error("duplicate lens id should be rejected")
	}

	dupName := base
	dupName.ID = "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a03"
	dupName.Name = strings.ToUpper(base.Name)
	if err := (NamedLensList{base, dupName}).Validate(); err == nil {
		t.Error("case-insensitively duplicate lens name should be rejected")
	}

	distinct := base
	distinct.ID = "9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a04"
	distinct.Name = "Something Else"
	if err := (NamedLensList{base, distinct}).Validate(); err != nil {
		t.Errorf("distinct lenses should validate: %v", err)
	}
}

func TestMachineStateValidateRejectsInvalidLens(t *testing.T) {
	m := NewMachineState()
	broken := validLens()
	broken.Mode = "everything"
	m.NamedLenses = append(m.NamedLenses, broken)
	if err := m.Validate(); err == nil {
		t.Fatal("machine state carrying a malformed lens should not validate")
	}
}

func TestMachineStateRemoveNamedLens(t *testing.T) {
	m := NewMachineState()
	if !m.RemoveNamedLens(lensStaleStarterID) {
		t.Fatal("removing a present lens should report a change")
	}
	if m.NamedLensByID(lensStaleStarterID) != nil {
		t.Fatal("removed lens is still reachable")
	}
	if m.RemoveNamedLens(lensStaleStarterID) {
		t.Fatal("removing an absent lens should report no change")
	}
	for _, lens := range StarterNamedLenses() {
		m.RemoveNamedLens(lens.ID)
	}
	if len(m.NamedLenses) != 0 {
		t.Fatalf("expected an empty list, got %+v", m.NamedLenses)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"named_lenses":[]`) {
		t.Fatalf("emptied lens list must persist as [], got %s", data)
	}
}

// Every axis is a value now, so a clone is independent by construction. The test
// stays because that is the guarantee callers rely on, not because a slice is
// being detached.
func TestNamedLensCloneCopiesEveryAxis(t *testing.T) {
	original := validLens()
	clone := original.Clone()
	clone.Name = "Mutated"
	clone.Query = "$done"
	clone.Scope = lensScopeProjectID
	clone.Sort.Field = LensSortTitle
	if original.Name != "Review Queue" || original.Query != validLensQuery ||
		original.Scope != "" || original.Sort.Field != LensSortUpdated {
		t.Fatalf("mutating a clone reached the original: %+v", original)
	}
	// Reading the clone back is what proves the writes above landed somewhere. A
	// clone that silently discarded them would satisfy the assertion on original.
	if clone.Name != "Mutated" || clone.Query != "$done" ||
		clone.Scope != lensScopeProjectID || clone.Sort.Field != LensSortTitle {
		t.Fatalf("the clone did not take the mutations: %+v", clone)
	}
}

func TestNamedLensListCloneDetachesFromSnapshot(t *testing.T) {
	snapshot := NewMachineState()
	clone := snapshot.NamedLenses.Clone()
	clone[0].Name = "Renamed"
	clone[0].Query = "$done"
	clone = append(clone, validLens())
	if len(clone) != 4 {
		t.Fatalf("clone length after append = %d, want 4", len(clone))
	}

	if snapshot.NamedLenses[0].Name != "Waiting" {
		t.Error("clone aliases the lens name")
	}
	if snapshot.NamedLenses[0].Query != "$"+string(models.StateWaiting) {
		t.Error("clone aliases the query axis")
	}
	if len(snapshot.NamedLenses) != 3 {
		t.Errorf("appending to the clone changed the snapshot: %d lenses", len(snapshot.NamedLenses))
	}
	if NamedLensList(nil).Clone() != nil {
		t.Error("cloning a nil list should stay nil")
	}
}

func TestMachineStateLensJSONOmissionSemantics(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr bool
		wantLen int
	}{
		// Three, literally: the catalog has nine templates and an omitted key
		// must still seed only the three starters.
		{name: "omitted seeds exactly three starters", payload: `{"theme":"light"}`, wantLen: 3},
		{name: "explicit empty stays empty", payload: `{"theme":"light","named_lenses":[]}`, wantLen: 0},
		{name: "null is invalid", payload: `{"theme":"light","named_lenses":null}`, wantErr: true},
		{name: "unknown nested field is invalid", payload: `{"theme":"light","named_lenses":[{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all","nope":1}]}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := NewMachineState()
			err := json.Unmarshal([]byte(tc.payload), out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("payload %s should fail to decode", tc.payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("decode %s: %v", tc.payload, err)
			}
			if len(out.NamedLenses) != tc.wantLen {
				t.Fatalf("lens count = %d, want %d", len(out.NamedLenses), tc.wantLen)
			}
		})
	}
}

func TestMachineStateLensJSONRoundTrip(t *testing.T) {
	m := NewMachineState()
	m.NamedLenses = append(m.NamedLenses, validLens())
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	reloaded := NewMachineState()
	if err := json.Unmarshal(data, reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(reloaded.NamedLenses) != len(m.NamedLenses) {
		t.Fatalf("lens count = %d, want %d", len(reloaded.NamedLenses), len(m.NamedLenses))
	}
	got, want := reloaded.NamedLenses[len(reloaded.NamedLenses)-1], validLens()
	if gotJSON, wantJSON := mustJSON(t, got), mustJSON(t, want); gotJSON != wantJSON {
		t.Fatalf("round trip lost detail:\n got %s\nwant %s", gotJSON, wantJSON)
	}
	if err := reloaded.Validate(); err != nil {
		t.Fatalf("round-tripped machine state should validate: %v", err)
	}
}

func TestNilNamedLensListMarshalsAsEmptyArray(t *testing.T) {
	// A MachineState assembled without the constructor must never persist null,
	// which loading would reject.
	data, err := json.Marshal(&MachineState{Theme: "dark"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"named_lenses":[]`) {
		t.Fatalf("nil lens list should render as [], got %s", data)
	}
}

func TestNamedLensesStayOutOfSyncedDomains(t *testing.T) {
	// Lens definitions own their home, machine-local or workspace-side: neither
	// the catalog nor the planning domain may carry them.
	catalog := NewWorkspaceCatalog()
	catalog.WorkspaceID = testWorkspaceID
	planning := NewPlanningState()
	planning.AddToToday("t1", "p1")
	for _, payload := range []any{catalog, planning} {
		data := mustJSON(t, payload)
		if strings.Contains(data, "named_lenses") || strings.Contains(data, "lens") {
			t.Fatalf("synced domain leaked lens data: %s", data)
		}
	}
}

func TestDomainFileStoreMachineLensRoundTrip(t *testing.T) {
	s := newIsolatedStore(t)

	machine := NewMachineState()
	machine.Theme = "light"
	machine.NamedLenses = append(machine.NamedLenses, validLens())
	if err := s.SaveMachine(machine); err != nil {
		t.Fatalf("SaveMachine: %v", err)
	}

	reloaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if len(reloaded.NamedLenses) != 4 {
		t.Fatalf("lens count = %d, want 4", len(reloaded.NamedLenses))
	}
	if got, want := mustJSON(t, reloaded.NamedLenses), mustJSON(t, machine.NamedLenses); got != want {
		t.Fatalf("persisted lenses changed:\n got %s\nwant %s", got, want)
	}

	// Deleting every lens is durable: reload must not reseed the starters.
	reloaded.NamedLenses = NamedLensList{}
	if err = s.SaveMachine(reloaded); err != nil {
		t.Fatalf("SaveMachine emptied: %v", err)
	}
	emptied, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine emptied: %v", err)
	}
	if len(emptied.NamedLenses) != 0 {
		t.Fatalf("emptied lens list reseeded: %+v", emptied.NamedLenses)
	}
}

func TestDomainFileStoreMachineLensLoadSemantics(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr bool
		wantLen int
	}{
		{name: "legacy payload without the key seeds exactly three starters", payload: `{"theme":"light"}`, wantLen: 3},
		{name: "explicit empty stays empty", payload: `{"theme":"light","named_lenses":[]}`, wantLen: 0},
		{name: "null is rejected", payload: `{"theme":"light","named_lenses":null}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newIsolatedStore(t)
			writeRawMachinePayload(t, tc.payload)

			loaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("payload %s should fail to load", tc.payload)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadMachine: %v", err)
			}
			if len(loaded.NamedLenses) != tc.wantLen {
				t.Fatalf("lens count = %d, want %d", len(loaded.NamedLenses), tc.wantLen)
			}
			if err := loaded.Validate(); err != nil {
				t.Fatalf("loaded machine state should validate: %v", err)
			}
		})
	}
}

// A malformed token survives decoding — the JSON shape is legal — and is caught
// by validation, which is what every persisted write runs.
func TestMachineLensMalformedTokenFailsValidationNotDecoding(t *testing.T) {
	s := newIsolatedStore(t)
	writeRawMachinePayload(t, `{"theme":"light","named_lenses":[{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"everything"}]}`)

	loaded, err := NewDomainFileStore(s.DataDir()).LoadMachine()
	if err != nil {
		t.Fatalf("LoadMachine: %v", err)
	}
	if err := loaded.Validate(); err == nil {
		t.Fatal("a malformed mode token should fail validation")
	}
	if err := s.SaveMachine(loaded); err == nil {
		t.Fatal("a malformed mode token should never be persisted")
	}
}

func TestSaveMachineRejectsInvalidLens(t *testing.T) {
	s := newIsolatedStore(t)
	machine := NewMachineState()
	broken := validLens()
	broken.Query = strings.Repeat("x", MaxNamedLensQueryRunes+1)
	machine.NamedLenses = append(machine.NamedLenses, broken)
	if err := s.SaveMachine(machine); err == nil {
		t.Fatal("saving a machine state with a malformed lens should fail")
	}
}

// writeRawMachinePayload replaces machine.json with a hand-authored payload
// wrapped in the ordinary domain envelope.
func writeRawMachinePayload(t *testing.T, payload string) {
	t.Helper()
	data, err := marshalDomainEnvelope(domainMachine, testWorkspaceID, json.RawMessage(payload))
	if err != nil {
		t.Fatalf("marshalDomainEnvelope: %v", err)
	}
	path, err := MachineStatePath(testWorkspaceID)
	if err != nil {
		t.Fatalf("MachineStatePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatalf("write machine.json: %v", err)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

// lensScopeProjectID is shaped exactly like a real project ID, which
// internal/app mints with uuid.New().String(); that is why a lens scope is held
// to the same canonical spelling a lens identity is.
const lensScopeProjectID = "3c2a1f60-4e5d-4b8a-9c7e-2d1f0a3b4c5d"

func TestNamedLensScopeAcceptsEmptyOrCanonicalUUIDOnly(t *testing.T) {
	unscoped := validLens()
	if unscoped.Scope != "" {
		t.Fatalf("the shared fixture should be unscoped, got %q", unscoped.Scope)
	}
	if err := unscoped.Validate(); err != nil {
		t.Fatalf("an unscoped lens must validate: %v", err)
	}

	scoped := validLens()
	scoped.Scope = lensScopeProjectID
	if err := scoped.Validate(); err != nil {
		t.Fatalf("a canonically scoped lens must validate: %v", err)
	}

	// The rejection set is exactly ValidateNamedLensID's, minus the empty
	// string: both go through validateCanonicalUUID, so one spelling of an
	// identity is the rule on both fields.
	for _, malformed := range []string{
		"not-a-uuid",
		strings.ToUpper(lensScopeProjectID),
		"{" + lensScopeProjectID + "}",
		"urn:uuid:" + lensScopeProjectID,
		strings.ReplaceAll(lensScopeProjectID, "-", ""),
	} {
		broken := validLens()
		broken.Scope = malformed
		if err := broken.Validate(); err == nil {
			t.Errorf("lens scope %q should be rejected", malformed)
		}
		if err := ValidateNamedLensID(malformed); err == nil {
			t.Errorf("lens id %q should be rejected by the same helper", malformed)
		}
	}

	// The one place the two fields part company: an identity may not be empty,
	// a scope must be allowed to be.
	if err := ValidateNamedLensID(""); err == nil {
		t.Error("an empty lens id should be rejected")
	}
}

func TestNamedLensScopeJSONOmissionAndRoundTrip(t *testing.T) {
	bare, err := json.Marshal(validLens())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(bare), "scope") {
		t.Fatalf("an unscoped lens must omit the key entirely so older machine files stay byte-identical: %s", bare)
	}
	var reread NamedLens
	if err = json.Unmarshal(bare, &reread); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reread.Scope != "" {
		t.Fatalf("an omitted scope decoded as %q", reread.Scope)
	}
	if err = reread.Validate(); err != nil {
		t.Fatalf("a record predating the scope axis must still validate: %v", err)
	}

	scoped := validLens()
	scoped.Scope = lensScopeProjectID
	data, err := json.Marshal(scoped)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"scope":"`+lensScopeProjectID+`"`) {
		t.Fatalf("scope missing from payload: %s", data)
	}
	var decoded NamedLens
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Scope != lensScopeProjectID {
		t.Fatalf("scope round trip = %q, want %q", decoded.Scope, lensScopeProjectID)
	}
	if clone := scoped.Clone(); clone.Scope != lensScopeProjectID {
		t.Fatalf("Clone dropped the scope: %q", clone.Scope)
	}
}

func TestBuiltinAndStarterLensesStayUnscoped(t *testing.T) {
	// A starter lens is seeded on a fresh install, where no project ID this
	// machine could name exists yet, so every catalog entry must be globally
	// meaningful.
	for _, lens := range BuiltinNamedLensTemplates() {
		if lens.Scope != "" {
			t.Errorf("template %q carries scope %q; the catalog must stay unscoped", lens.Name, lens.Scope)
		}
	}
	for _, lens := range StarterNamedLenses() {
		if lens.Scope != "" {
			t.Errorf("starter %q carries scope %q", lens.Name, lens.Scope)
		}
	}
	for _, lens := range NewMachineState().NamedLenses {
		if lens.Scope != "" {
			t.Errorf("seeded lens %q carries scope %q", lens.Name, lens.Scope)
		}
	}
}

func TestMachineStateActiveNamedLensResolvesOnlyALiveID(t *testing.T) {
	m := NewMachineState()
	if m.ActiveLensID != "" {
		t.Fatalf("a fresh machine state should carry no active lens, got %q", m.ActiveLensID)
	}
	if m.ActiveNamedLens() != nil {
		t.Fatal("no active id must resolve to nil")
	}

	m.ActiveLensID = lensStaleStarterID
	active := m.ActiveNamedLens()
	if active == nil || active.ID != lensStaleStarterID {
		t.Fatalf("active lens = %+v, want the Stale starter", active)
	}

	// A dangling pointer reads as "no lens" rather than as an error, which is
	// what keeps a hand-edited machine file usable.
	m.ActiveLensID = lensScopeProjectID
	if got := m.ActiveNamedLens(); got != nil {
		t.Fatalf("a dangling active id must resolve to nil, got %+v", got)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("a dangling active id must never refuse the machine file: %v", err)
	}

	var nilState *MachineState
	if nilState.ActiveNamedLens() != nil {
		t.Fatal("a nil machine state must resolve to nil")
	}
}

func TestMachineStateActiveLensValidationIsShapeOnly(t *testing.T) {
	m := NewMachineState()
	m.ActiveLensID = strings.ToUpper(lensStaleStarterID)
	if err := m.Validate(); err == nil {
		t.Fatal("a non-canonical active lens id should be rejected on shape")
	}
	m.ActiveLensID = lensStaleStarterID
	if err := m.Validate(); err != nil {
		t.Fatalf("a canonical active lens id should validate: %v", err)
	}
}

func TestMachineStateActiveLensJSONOmissionAndRoundTrip(t *testing.T) {
	m := NewMachineState()
	bare, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(bare), "active_lens_id") {
		t.Fatalf("no active lens must omit the key entirely: %s", bare)
	}

	m.ActiveLensID = lensStaleStarterID
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"active_lens_id":"`+lensStaleStarterID+`"`) {
		t.Fatalf("active lens id missing from payload: %s", data)
	}
	reloaded := NewMachineState()
	if err := json.Unmarshal(data, reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reloaded.ActiveLensID != lensStaleStarterID {
		t.Fatalf("active lens id round trip = %q, want %q", reloaded.ActiveLensID, lensStaleStarterID)
	}

	// A payload naming a lens the list does not carry still decodes and still
	// validates; only the resolution comes back empty.
	dangling := NewMachineState()
	if err := json.Unmarshal([]byte(`{"theme":"dark","named_lenses":[],"active_lens_id":"`+lensStaleStarterID+`"}`), dangling); err != nil {
		t.Fatalf("a dangling active lens payload must decode: %v", err)
	}
	if err := dangling.Validate(); err != nil {
		t.Fatalf("a dangling active lens payload must validate: %v", err)
	}
	if dangling.ActiveNamedLens() != nil {
		t.Fatal("a dangling active lens payload must resolve to nil")
	}
}

func TestRemoveNamedLensClearsTheActivePointerItMatches(t *testing.T) {
	m := NewMachineState()
	m.ActiveLensID = lensStaleStarterID
	if !m.RemoveNamedLens(lensHighPriorityStartID) {
		t.Fatal("removing a present lens should report a change")
	}
	if m.ActiveLensID != lensStaleStarterID {
		t.Fatalf("removing another lens must leave the pointer alone, got %q", m.ActiveLensID)
	}
	if !m.RemoveNamedLens(lensStaleStarterID) {
		t.Fatal("removing the live lens should report a change")
	}
	if m.ActiveLensID != "" {
		t.Fatalf("deleting the live lens must clear the pointer, got %q", m.ActiveLensID)
	}
	if m.ActiveNamedLens() != nil {
		t.Fatal("the cleared pointer must resolve to nil")
	}
}
