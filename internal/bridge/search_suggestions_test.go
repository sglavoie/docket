package bridge

import "testing"

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
