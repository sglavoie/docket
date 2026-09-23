package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
			name: "prose the canonical vocabulary cannot spell follows the clauses, and %slug is a clause",
			payload: `{"id":"9f8b8e42-7c3a-4d61-8f2a-1b0c9d7e5a01","name":"X","mode":"all",` +
				`"query":"needle %alpha","tasks":{"pinned":true}}`,
			want: "%alpha *pin needle",
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
