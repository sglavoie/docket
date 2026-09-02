package config

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestMachineStateMenuBarVisibleJSONOmission(t *testing.T) {
	// machine.json decodes with DisallowUnknownFields, so the only way a machine
	// that has never shown the bar stays byte-identical to one built before the
	// field existed is for the hidden state to write no key at all.
	bare, err := json.Marshal(NewMachineState())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(bare), "menu_bar_visible") {
		t.Fatalf("a hidden menu bar must omit the key entirely: %s", bare)
	}

	// A load decodes into the seeded value, so "omitted means the default" is a
	// claim about NewMachineState as much as about the tag: the constructor must
	// leave this field alone or an older file would reopen with the bar shown.
	reloaded := NewMachineState()
	if err := json.Unmarshal([]byte(`{"theme":"light"}`), reloaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if reloaded.MenuBarVisible {
		t.Fatal("an omitted menu_bar_visible key must decode as hidden")
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
