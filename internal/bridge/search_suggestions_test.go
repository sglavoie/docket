package bridge

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestBridgeSearchSuggestions(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	for _, tc := range []struct {
		name    string
		payload map[string]any
		tokens  []string
		search  string
	}{
		{"tag", map[string]any{"mode": "all", "search": "#fe"}, []string{"#feature"}, "#feature "},
		{"uppercase", map[string]any{"mode": "all", "search": "#FE"}, []string{"#feature"}, "#feature "},
		{"negated tag", map[string]any{"mode": "all", "search": "-#fe"}, []string{"-#feature"}, "-#feature "},
		{"context", map[string]any{"mode": "all", "search": "##la"}, []string{"##laptop"}, "##laptop "},
		{"negated context", map[string]any{"mode": "all", "search": "-##la"}, []string{"-##laptop"}, "-##laptop "},
		{"state", map[string]any{"mode": "all", "search": "$wai"}, []string{"$waiting"}, "$waiting "},
		{"deferred date", map[string]any{"mode": "all", "query": "?deferred", "search": "@@2026"}, []string{"@@2026-10-07"}, "@@2026-10-07 "},
		{"negated deferred date", map[string]any{"mode": "all", "query": "?deferred", "search": "-@@2026"}, []string{"-@@2026-10-07"}, "-@@2026-10-07 "},
		{"preserves text", map[string]any{"mode": "all", "search": "Ship  #fe"}, []string{"#feature"}, "Ship  #feature "},
		{"unicode separator", map[string]any{"mode": "all", "search": "Ship\u2003#fe"}, []string{"#feature"}, "Ship\u2003#feature "},
		{"lens", map[string]any{"lens": atlasReleaseLens, "search": "#re"}, []string{"#release"}, "#release "},
		{"mode boundary", map[string]any{"mode": "inbox", "search": "#fe"}, nil, ""},
		{"project boundary", map[string]any{"mode": "all", "scope": homeProject, "search": "#fe"}, nil, ""},
		{"filter boundary", map[string]any{"mode": "all", "query": "$waiting", "search": "#fe"}, nil, ""},
		{"prefix boundary", map[string]any{"mode": "all", "search": "$waiting #fe"}, nil, ""},
		{"already used", map[string]any{"mode": "all", "search": "#feature #fe"}, nil, ""},
		{"accepted", map[string]any{"mode": "all", "search": "#feature "}, nil, ""},
		{"exact", map[string]any{"mode": "all", "search": "#feature"}, nil, ""},
		{"no matches", map[string]any{"mode": "all", "search": "#unknown"}, nil, ""},
		{"plain text", map[string]any{"mode": "all", "search": "fe"}, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result searchSuggestionsResult
			h.ok(h.call("query.search_suggestions", tc.payload), &result)
			var tokens []string
			for _, suggestion := range result.Suggestions {
				tokens = append(tokens, suggestion.Token)
				if suggestion.Search != tc.search {
					t.Errorf("completed search = %q, want %q", suggestion.Search, tc.search)
				}
			}
			requireTitles(t, "suggestions", tokens, tc.tokens)
		})
	}
}

func TestBridgeSearchSuggestionsIncludeNewTagsOnce(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{"text": "Plan a launch #epic\nPlan another #epic"}), &captured)
	var suggestions searchSuggestionsResult
	h.ok(h.call("query.search_suggestions", map[string]any{"mode": "all", "search": "#ep"}), &suggestions)
	if len(suggestions.Suggestions) != 1 || suggestions.Suggestions[0].Token != "#epic" {
		t.Fatalf("new tag should complete once: %+v", suggestions)
	}
	requireTitles(t, "accepted completion", h.titles(map[string]any{"mode": "all", "search": suggestions.Suggestions[0].Search}), []string{"Plan a launch", "Plan another"})
}

func TestBridgeSearchSuggestionsAcrossSigils(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	for _, tc := range []struct {
		input string
		token string
	}{
		{"%AT", "%atlas-launch"},
		{"-%ho", "-%home-renovation"},
		{"$", "$waiting"},
		{"-$WA", "-$waiting"},
		{"!", "!0"},
		{"!2", "!2+"},
		{"-!2", "-!2+"},
		{"@no", "@none"},
		{"-@no", "-@none"},
		{"@+", "@+4h"},
		{"@2026-10-0", "@2026-10-09"},
		{"@@", "@@any"},
		{"@@no", "@@none"},
		{"-@@a", "-@@any"},
		{"*p", "*pin"},
		{"*u", "*unpin"},
		{"-*p", "-*pin"},
		{"=r", "=review"},
		{"=u", "=unreview"},
		{"-=r", "-=review"},
		{"?le", "?leaf"},
		{"-?le", "-?leaf"},
		{"&", "&any"},
		{"&n", "&none"},
		{"-&a", "-&any"},
		{"~", "~" + homeProject + ":" + paintTask},
		{"-~", "-~" + homeProject + ":" + paintTask},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var result searchSuggestionsResult
			h.ok(h.call("query.search_suggestions", map[string]any{"mode": "all", "search": tc.input}), &result)
			found := false
			for _, suggestion := range result.Suggestions {
				if suggestion.Token == tc.token {
					found = true
				}
				clauses, refusal := compileSearch(suggestion.Search)
				if refusal != nil || len(clauses) != 1 || clauses[0].Field == models.QueryFieldText {
					t.Fatalf("suggestion %q must compile to a filter, got %+v, %v", suggestion.Search, clauses, refusal)
				}
				var tasks tasksResult
				h.ok(h.call("query.tasks", map[string]any{"mode": "all", "search": suggestion.Search}), &tasks)
			}
			if !found {
				t.Fatalf("%q missing from %+v", tc.token, result.Suggestions)
			}
		})
	}
}

func TestBridgeProjectSuggestionsRespectListAndPrefix(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	for _, tc := range []struct {
		payload map[string]any
		want    []string
	}{
		{map[string]any{"mode": "all", "search": "%"}, []string{"%atlas-launch", "%home-renovation", "%unfiled"}},
		{map[string]any{"mode": "all", "scope": homeProject, "search": "%"}, []string{"%home-renovation"}},
		{map[string]any{"lens": atlasReleaseLens, "search": "%"}, []string{"%atlas-launch"}},
		{map[string]any{"mode": "all", "search": "Ship  %AT"}, []string{"%atlas-launch"}},
		{map[string]any{"mode": "all", "search": "%atlas-launch %at"}, nil},
		{map[string]any{"mode": "all", "search": "%atlas-launch "}, nil},
		{map[string]any{"mode": "all", "search": "%atlas-launch"}, nil},
		{map[string]any{"mode": "all", "search": "%missing"}, nil},
	} {
		var result searchSuggestionsResult
		h.ok(h.call("query.search_suggestions", tc.payload), &result)
		var got []string
		for _, suggestion := range result.Suggestions {
			got = append(got, suggestion.Token)
			if tc.payload["search"] == "Ship  %AT" && suggestion.Search != "Ship  %atlas-launch " {
				t.Fatalf("prefix lost: %+v", suggestion)
			}
		}
		requireTitles(t, "project suggestions", got, tc.want)
	}
}

func TestBridgeSearchPresenceCompletionsFilterResults(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	for _, tc := range []struct {
		search string
		want   bool
	}{
		{"&any", true}, {"&none", false}, {"-&any", false}, {"-&none", true},
		{"@none", false}, {"-@none", true},
		{"*unpin", false}, {"@@none", true}, {"-@@none", false},
		{"@@any", false}, {"-@@any", true}, {"=unreview", true},
	} {
		got := h.titles(map[string]any{"mode": "all", "search": "Ship " + tc.search})
		if slices.Contains(got, "Ship beta build") != tc.want {
			t.Errorf("Ship %s = %v, want match %v", tc.search, got, tc.want)
		}
	}
}
