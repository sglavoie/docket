package bridge

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/textgrammar"
)

func TestBridgeCaptureSuggestionsEverySigil(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	wanted := map[string]string{
		"!": "!1", "#": "#release", "##": "##laptop", "@": "@tomorrow", "@@": "@@+2h",
		"$": "$today", "*": "*pin", "=": "=review", "%": "%atlas-launch",
		">": ">1", "<": "<1", "~": "~home-renovation:1", "&": "&https://", ";": ";comment;",
	}
	before := h.revisions()
	for _, def := range textgrammar.Definitions(textgrammar.SurfaceCapture) {
		t.Run(def.Trigger, func(t *testing.T) {
			token, exists := wanted[def.Trigger]
			if !exists {
				t.Fatalf("capture sigil %s has no completion coverage", def.Trigger)
			}
			text := "New task " + def.Trigger
			if def.Trigger != "%" {
				text = "New task %atlas-launch " + def.Trigger
			}
			var result captureSuggestionsResult
			h.ok(h.call("query.capture_suggestions", captureSuggestionsPayload{Text: text, Caret: utf16Length(text)}), &result)
			for _, suggestion := range result.Suggestions {
				if suggestion.Token == token {
					if suggestion.Label == "" || suggestion.Caret > utf16Length(suggestion.Text) {
						t.Fatalf("invalid suggestion: %+v", suggestion)
					}
					return
				}
			}
			t.Fatalf("missing %s: %+v", token, result)
		})
	}
	if after := h.revisions(); after != before {
		t.Fatal("suggestions changed workspace revisions")
	}
}

func TestBridgeCaptureSuggestionsPreserveDraftAndScope(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	for _, tc := range []struct {
		name, marked, token, want string
	}{
		{"unicode middle", "Buy 🥛 #re|lease then call\nOther #feature", "#release", "Buy 🥛 #release then call\nOther #feature"},
		{"unicode space", "Buy\u00a0#re|", "#release", "Buy\u00a0#release "},
		{"newline", "Buy #re|\nOther task", "#release", "Buy #release \nOther task"},
		{"longest trigger", "Buy ##la|", "##laptop", "Buy ##laptop "},
		{"multiword date", "Buy @next f|riday #release", "@next friday", "Buy @next friday #release"},
		{"project after caret", "Buy >Sh| %atlas-launch", ">1", "Buy >1 %atlas-launch"},
		{"line scope", "First %home-renovation\nSecond <Sh| %atlas-launch", "<1", "First %home-renovation\nSecond <1 %atlas-launch"},
		{"cross project", "Buy ~Ship|", "~atlas-launch:1", "Buy ~atlas-launch:1 "},
		{"comment close", "Buy ;ask about #auth|", ";ask about #auth;", "Buy ;ask about #auth; "},
		{"attachment", "Buy &htt|", "&https://", "Buy &https://"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prefix, suffix, _ := strings.Cut(tc.marked, "|")
			var result captureSuggestionsResult
			h.ok(h.call("query.capture_suggestions", captureSuggestionsPayload{Text: prefix + suffix, Caret: utf16Length(prefix)}), &result)
			for _, suggestion := range result.Suggestions {
				if suggestion.Token == tc.token {
					if suggestion.Text != tc.want {
						t.Fatalf("text = %q, want %q", suggestion.Text, tc.want)
					}
					return
				}
			}
			t.Fatalf("missing %s: %+v", tc.token, result)
		})
	}
}

func TestBridgeCaptureSuggestionsSuppressInvalidOrDuplicateTokens(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	for _, marked := range []string{
		"Buy milk|", "Buy email@example|", "Buy #release |", "Buy @| @tomorrow", "Buy ##| ##laptop",
		"Buy ;literal #re|;", "Buy &'path with #re| inside'", "Buy %absent >|", "Buy *un|", "Buy =un|",
		"Buy &path\\ with\\ #re|", "Buy &\"path \\\"with #re| inside\"",
		"Buy ;comment;#re|", "Buy &'path'#re|",
		"Buy ^|", "Buy -#|", "Buy -~|",
	} {
		t.Run(marked, func(t *testing.T) {
			prefix, suffix, _ := strings.Cut(marked, "|")
			var result captureSuggestionsResult
			h.ok(h.call("query.capture_suggestions", captureSuggestionsPayload{Text: prefix + suffix, Caret: utf16Length(prefix)}), &result)
			if len(result.Suggestions) != 0 {
				t.Fatalf("unexpected suggestions: %+v", result)
			}
		})
	}
	for _, caret := range []int{-1, 1, 3} {
		h.rejects(h.call("query.capture_suggestions", captureSuggestionsPayload{Text: "🥛", Caret: caret}), CodeInvalidRequest)
	}
	for _, marked := range []string{"New #release #|", "New %atlas-launch <Ship <|", "New ~atlas-launch:Ship ~|"} {
		prefix := strings.TrimSuffix(marked, "|")
		var result captureSuggestionsResult
		h.ok(h.call("query.capture_suggestions", captureSuggestionsPayload{Text: prefix, Caret: utf16Length(prefix)}), &result)
		for _, suggestion := range result.Suggestions {
			if suggestion.Token == "#release" || suggestion.Token == "<1" || suggestion.Token == "~atlas-launch:1" {
				t.Fatalf("duplicate suggestion for %q: %+v", marked, suggestion)
			}
		}
	}
}
