package textgrammar

import (
	"reflect"
	"testing"
)

func TestCaptureDestinationNamesAreExactAndOrdered(t *testing.T) {
	want := []string{"inbox", "backlog", "today", "next", "doing", "waiting", "paused", "someday", "done", "canceled"}
	definition, ok := DefinitionFor(SurfaceCapture, FieldDestination)
	if !ok {
		t.Fatal("capture destination definition is missing")
	}
	if got := definition.Values; !reflect.DeepEqual(got, want) {
		t.Fatalf("capture destination values = %v, want %v", got, want)
	}
}

func TestRegistryDefinitionsCoverCanonicalSurfaces(t *testing.T) {
	tests := []struct {
		name    string
		surface Surface
		want    map[Field][]string
	}{
		{
			name: "capture", surface: SurfaceCapture,
			want: map[Field][]string{
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldContext: {"##context"}, FieldDue: {"@date"},
				FieldDefer:       {"@@defer"},
				FieldDestination: {"$destination"}, FieldPinned: {"*pin"},
				FieldReview:  {"=review"},
				FieldProject: {"%project"}, FieldParent: {">parent"},
				FieldChild: {"<child"}, FieldAttachment: {"&link"}, FieldComment: {";comment;"},
				FieldDependency: {"~task"},
			},
		},
		{
			name: "edit", surface: SurfaceEdit,
			want: map[Field][]string{
				FieldPriority: {"!1-3", "!0"}, FieldTag: {"#tag"}, FieldRemoveTag: {"-#tag"},
				FieldContext: {"##context", "##0"},
				FieldDue:     {"@date", "@0"},
				FieldDefer:   {"@@defer", "@@0"}, FieldDestination: {"$destination"},
				FieldPinned:  {"*pin", "*unpin"},
				FieldReview:  {"=review", "=unreview"},
				FieldProject: {"%project"}, FieldParent: {">parent", ">0"},
				FieldChild:      {"<child"},
				FieldDependency: {"~task", "~0"}, FieldRemoveDependency: {"-~task"},
			},
		},
		{
			name: "outline", surface: SurfaceOutline,
			want: map[Field][]string{
				FieldPriority: {"!1-3"}, FieldTag: {"#tag"}, FieldContext: {"##context"}, FieldDue: {"@date"},
				FieldDestination: {"$destination"}, FieldPinned: {"*pin"},
				FieldReview:     {"=review"},
				FieldDependency: {"~task", "~0"}, FieldRemoveDependency: {"-~task"},
			},
		},
		{
			name: "search", surface: SurfaceSearch,
			want: map[Field][]string{
				FieldPriority: {"!1-3", "!0"}, FieldRemovePriority: {"-!1-3"},
				FieldTag: {"#tag"}, FieldRemoveTag: {"-#tag"},
				FieldContext: {"##context"}, FieldRemoveContext: {"-##context"},
				FieldDue: {"@date"}, FieldRemoveDue: {"-@date"},
				FieldDefer: {"@@defer"}, FieldRemoveDefer: {"-@@defer"},
				FieldDestination: {"$destination"}, FieldRemoveDestination: {"-$destination"},
				FieldPinned: {"*pin", "*unpin"}, FieldRemovePinned: {"-*pin"},
				FieldReview: {"=review", "=unreview"}, FieldRemoveReview: {"-=review"},
				FieldProject: {"%project"}, FieldRemoveProject: {"-%project"},
				FieldDependency: {"~task"}, FieldRemoveDependency: {"-~task"},
				FieldDerived: {"?derived"}, FieldRemoveDerived: {"-?derived"},
				FieldAttachment: {"&any", "&none"}, FieldRemoveAttachment: {"-&any"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs := Definitions(tt.surface)
			if len(defs) != len(tt.want) {
				t.Fatalf("definitions = %d, want %d: %+v", len(defs), len(tt.want), defs)
			}
			for _, def := range defs {
				forms, ok := tt.want[def.Field]
				if !ok {
					t.Errorf("unexpected field %s", def.Field)
					continue
				}
				if !reflect.DeepEqual(def.Forms, forms) {
					t.Errorf("%s forms = %v, want %v", def.Field, def.Forms, forms)
				}
				if def.Trigger == "" || def.Description == "" {
					t.Errorf("%s is missing trigger/help metadata: %+v", def.Field, def)
				}
			}
		})
	}
}

func TestParseRecognizesOnlyRegistryFormsForEachSurface(t *testing.T) {
	const input = "Task !1 #add -#old @tomorrow $next *pin %core >1 <2"
	tests := []struct {
		name     string
		surface  Surface
		wantText string
		fields   []Field
	}{
		{"capture", SurfaceCapture, "Task -#old", []Field{FieldPriority, FieldTag, FieldDue, FieldDestination, FieldPinned, FieldProject, FieldParent, FieldChild}},
		{"edit", SurfaceEdit, "Task", []Field{FieldPriority, FieldTag, FieldRemoveTag, FieldDue, FieldDestination, FieldPinned, FieldProject, FieldParent, FieldChild}},
		{"outline", SurfaceOutline, "Task -#old %core >1 <2", []Field{FieldPriority, FieldTag, FieldDue, FieldDestination, FieldPinned}},
		{"search", SurfaceSearch, "Task >1 <2", []Field{FieldPriority, FieldTag, FieldRemoveTag, FieldDue, FieldDestination, FieldPinned, FieldProject}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := Parse(input, tt.surface)
			if parsed.Text != tt.wantText {
				t.Fatalf("Text = %q, want %q", parsed.Text, tt.wantText)
			}
			for _, field := range tt.fields {
				if len(parsed.TokensFor(field)) == 0 {
					t.Errorf("missing %s token in %+v", field, parsed.Tokens)
				}
			}
		})
	}
}

func TestRegistryClearSemanticsAreSurfaceSpecific(t *testing.T) {
	for _, field := range []Field{FieldPriority, FieldDue, FieldParent} {
		capture, ok := DefinitionFor(SurfaceCapture, field)
		if !ok || len(capture.Clears) != 0 {
			t.Errorf("capture %s clears = %+v, want none", field, capture.Clears)
		}
		edit, ok := DefinitionFor(SurfaceEdit, field)
		if !ok || len(edit.Clears) != 1 || edit.Clears[0].Value != "0" {
			t.Errorf("edit %s clears = %+v, want one zero clear", field, edit.Clears)
		}
	}
	for _, field := range []Field{FieldDue, FieldContext} {
		search, ok := DefinitionFor(SurfaceSearch, field)
		if !ok || len(search.Clears) != 0 {
			t.Errorf("search %s clears = %+v, want none; a clear is a mutation", field, search.Clears)
		}
	}
	// Priority's clear is the one search reads, as a question: !0 matches the
	// tasks carrying no priority.
	search, ok := DefinitionFor(SurfaceSearch, FieldPriority)
	if !ok || len(search.Clears) != 1 || search.Clears[0].Value != "0" ||
		search.Clears[0].Description != "Match tasks with no priority" {
		t.Errorf("search priority clears = %+v, want the !0 question", search.Clears)
	}
	if _, ok := DefinitionFor(SurfaceSearch, FieldParent); ok {
		t.Error("search advertises parent; placement is not a search predicate")
	}
	if _, ok := DefinitionFor(SurfaceSearch, FieldChild); ok {
		t.Error("search advertises child; placement is not a search predicate")
	}
}

func TestSearchDefinitionsDescribePredicatesRatherThanMutations(t *testing.T) {
	for _, def := range Definitions(SurfaceSearch) {
		if def.Description == "" {
			t.Errorf("search %s has no description", def.Field)
			continue
		}
		// A search-only field has no mutation reading to differ from: it says its
		// one sentence once, and Definitions hands that sentence to every surface
		// it can reach. Only a field that also mutates must read differently here.
		mutation, ok := DefinitionFor(SurfaceEdit, def.Field)
		if !ok {
			continue
		}
		if def.Description == mutation.Description {
			t.Errorf("search %s description = %q, want predicate wording", def.Field, def.Description)
		}
	}
	pinned, ok := DefinitionFor(SurfaceSearch, FieldPinned)
	if !ok || len(pinned.Clears) != 1 || pinned.Clears[0].Description != "Match unpinned tasks" {
		t.Fatalf("search pin clears = %+v, want the unpin predicate wording", pinned.Clears)
	}
}

// TestSearchOnlyNegationsReachNoMutationSurface holds the line the uniform
// negation was built on: `-` means "take away" everywhere, but only the two
// fields that hold a stored set can take anything away from a task. The other
// eight are questions, and a question that reached edit would be a silent no-op
// on save.
func TestSearchOnlyNegationsReachNoMutationSurface(t *testing.T) {
	searchOnly := []Field{
		FieldRemovePriority, FieldRemoveContext, FieldRemoveDue, FieldRemoveDefer,
		FieldRemoveDestination, FieldRemovePinned, FieldRemoveReview, FieldRemoveProject,
		FieldDerived, FieldRemoveDerived,
	}
	for _, field := range searchOnly {
		for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceOutline} {
			if _, ok := DefinitionFor(surface, field); ok {
				t.Errorf("%s is advertised on mutation surface %d", field, surface)
			}
		}
		if _, ok := DefinitionFor(SurfaceSearch, field); !ok {
			t.Errorf("%s is missing from search", field)
		}
	}
}

// TestEveryNegationSubtractsItsTwinsVocabulary asserts that a negation resolves
// to its positive twin rather than owning a second, drift-prone copy of the
// values, presets, and examples that twin advertises.
func TestEveryNegationSubtractsItsTwinsVocabulary(t *testing.T) {
	for _, spec := range registry {
		if spec.negates == "" {
			continue
		}
		twin, ok := registrySpecFor(spec.negates)
		if !ok {
			t.Fatalf("%s negates unknown field %s", spec.Field, spec.negates)
		}
		if !reflect.DeepEqual(SearchPresetValues(spec.Field), SearchPresetValues(twin.Field)) {
			t.Errorf("%s presets = %v, want its twin's %v",
				spec.Field, SearchPresetValues(spec.Field), SearchPresetValues(twin.Field))
		}
		if !reflect.DeepEqual(SearchExampleValues(spec.Field), SearchExampleValues(twin.Field)) {
			t.Errorf("%s search examples = %v, want its twin's %v",
				spec.Field, SearchExampleValues(spec.Field), SearchExampleValues(twin.Field))
		}
		if len(spec.Values) != 0 && !reflect.DeepEqual(spec.Values, twin.Values) {
			t.Errorf("%s values = %v, want its twin's %v", spec.Field, spec.Values, twin.Values)
		}
	}
}
