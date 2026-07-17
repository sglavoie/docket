package tui

import (
	"testing"
)

func TestDetectKeywordTrigger(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantMode    int
		wantPos     int
		wantPartial string
	}{
		{
			name:        "empty input",
			input:       "",
			wantMode:    acModeNone,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "has: no partial",
			input:       "has:",
			wantMode:    acModeHas,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "has: with partial",
			input:       "has:ch",
			wantMode:    acModeHas,
			wantPos:     0,
			wantPartial: "ch",
		},
		{
			name:        "has: after text",
			input:       "foo has:ch",
			wantMode:    acModeHas,
			wantPos:     4,
			wantPartial: "ch",
		},
		{
			name:        "is: no partial",
			input:       "is:",
			wantMode:    acModeIs,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "is: with partial",
			input:       "is:pin",
			wantMode:    acModeIs,
			wantPos:     0,
			wantPartial: "pin",
		},
		{
			name:        "no: no partial",
			input:       "no:",
			wantMode:    acModeNo,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "no: with partial",
			input:       "no:due",
			wantMode:    acModeNo,
			wantPos:     0,
			wantPartial: "due",
		},
		{
			name:        "p: no partial",
			input:       "p:",
			wantMode:    acModeSearchProject,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "p: with partial",
			input:       "p:my",
			wantMode:    acModeSearchProject,
			wantPos:     0,
			wantPartial: "my",
		},
		{
			name:        "p: after other filters",
			input:       "#backend p:my",
			wantMode:    acModeSearchProject,
			wantPos:     9,
			wantPartial: "my",
		},
		{
			name:        "plain text, no trigger",
			input:       "some task",
			wantMode:    acModeNone,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "no trigger when word has no colon",
			input:       "haste",
			wantMode:    acModeNone,
			wantPos:     0,
			wantPartial: "",
		},
		{
			name:        "case-insensitive: HAS:",
			input:       "HAS:desc",
			wantMode:    acModeHas,
			wantPos:     0,
			wantPartial: "desc",
		},
		{
			name:        "is: with full property after text",
			input:       "fix is:pinned",
			wantMode:    acModeIs,
			wantPos:     4,
			wantPartial: "pinned",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, pos, partial := DetectKeywordTrigger(tt.input)
			if mode != tt.wantMode {
				t.Errorf("mode = %d, want %d", mode, tt.wantMode)
			}
			if pos != tt.wantPos {
				t.Errorf("pos = %d, want %d", pos, tt.wantPos)
			}
			if partial != tt.wantPartial {
				t.Errorf("partial = %q, want %q", partial, tt.wantPartial)
			}
		})
	}
}

func TestBuildKeywordAcceptedInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		keyword string
		mode    int
		trigPos int
		items   []ACSuggestion
		cursor  int
		wantOut string
	}{
		{
			name:    "accept has:comment from start",
			input:   "has:co",
			keyword: "has",
			mode:    acModeHas,
			trigPos: 0,
			items:   []ACSuggestion{{Display: "has:comment (...)", Value: "comment"}},
			cursor:  0,
			wantOut: "has:comment ",
		},
		{
			name:    "accept has:checklist after text",
			input:   "fix has:ch",
			keyword: "has",
			mode:    acModeHas,
			trigPos: 4,
			items:   []ACSuggestion{{Display: "has:checklist (...)", Value: "checklist"}},
			cursor:  0,
			wantOut: "fix has:checklist ",
		},
		{
			name:    "accept p:myproject",
			input:   "p:my",
			keyword: "p",
			mode:    acModeSearchProject,
			trigPos: 0,
			items:   []ACSuggestion{{Display: "p:myproject", Value: "myproject"}},
			cursor:  0,
			wantOut: "p:myproject ",
		},
		{
			name:    "accept is:pinned",
			input:   "#backend is:pin",
			keyword: "is",
			mode:    acModeIs,
			trigPos: 9,
			items:   []ACSuggestion{{Display: "is:pinned (...)", Value: "pinned"}},
			cursor:  0,
			wantOut: "#backend is:pinned ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := AutocompleteState{
				Mode:       tt.mode,
				TriggerPos: tt.trigPos,
				Items:      tt.items,
				Cursor:     tt.cursor,
			}
			got := ac.BuildKeywordAcceptedInput(tt.input, tt.keyword)
			if got != tt.wantOut {
				t.Errorf("BuildKeywordAcceptedInput = %q, want %q", got, tt.wantOut)
			}
		})
	}
}

func TestGetHasSuggestions(t *testing.T) {
	m := &GlobalSearchModel{}

	// Empty partial returns all
	all := m.getHasSuggestions("")
	if len(all) == 0 {
		t.Fatal("expected suggestions for empty partial")
	}

	// Partial "ch" matches "checklist" and "children"
	filtered := m.getHasSuggestions("ch")
	for _, s := range filtered {
		if s.Value != "checklist" && s.Value != "children" {
			t.Errorf("unexpected suggestion %q for partial 'ch'", s.Value)
		}
	}

	issue := m.getHasSuggestions("iss")
	if len(issue) != 1 || issue[0].Value != "issue" {
		t.Errorf("expected [issue] for partial 'iss', got %v", issue)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 suggestions for 'ch', got %d", len(filtered))
	}

	// Values are property names, display includes keyword prefix
	for _, s := range all {
		if len(s.Display) == 0 {
			t.Errorf("suggestion has empty Display")
		}
		if len(s.Value) == 0 {
			t.Errorf("suggestion has empty Value")
		}
	}
}

func TestGetIsSuggestions(t *testing.T) {
	m := &GlobalSearchModel{}

	all := m.getIsSuggestions("")
	if len(all) == 0 {
		t.Fatal("expected suggestions for empty partial")
	}
	for _, suggestion := range all {
		if suggestion.Value == "active" {
			t.Fatal("is:active should not be suggested")
		}
	}

	// "pin" should match "pinned"
	filtered := m.getIsSuggestions("pin")
	if len(filtered) != 1 || filtered[0].Value != "pinned" {
		t.Errorf("expected [pinned] for partial 'pin', got %v", filtered)
	}
}

func TestGetNoSuggestions(t *testing.T) {
	m := &GlobalSearchModel{}

	all := m.getNoSuggestions("")
	if len(all) == 0 {
		t.Fatal("expected suggestions for empty partial")
	}

	// "du" should match "due"
	filtered := m.getNoSuggestions("du")
	if len(filtered) != 1 || filtered[0].Value != "due" {
		t.Errorf("expected [due] for partial 'du', got %v", filtered)
	}

	issue := m.getNoSuggestions("iss")
	if len(issue) != 1 || issue[0].Value != "issue" {
		t.Errorf("expected [issue] for partial 'iss', got %v", issue)
	}
}

func TestSearchKeywordSuggestionsIncludeIssue(t *testing.T) {
	hasIssue := searchHasSuggestions("iss")
	if len(hasIssue) != 1 || hasIssue[0].Value != "issue" {
		t.Errorf("searchHasSuggestions(iss) = %v, want issue", hasIssue)
	}

	noIssue := searchNoSuggestions("iss")
	if len(noIssue) != 1 || noIssue[0].Value != "issue" {
		t.Errorf("searchNoSuggestions(iss) = %v, want issue", noIssue)
	}
}

func TestSearchIsSuggestionsExcludeActive(t *testing.T) {
	for _, suggestion := range searchIsSuggestions("") {
		if suggestion.Value == "active" {
			t.Fatal("is:active should not be suggested")
		}
	}
}
