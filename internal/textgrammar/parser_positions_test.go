package textgrammar

import (
	"strings"
	"testing"
)

func TestTokenPositionsReferToOriginalInput(t *testing.T) {
	input := "  Réparer 日本 ;ignore @fake #tag; &'~/a b.txt' #auth !1 @tomorow %missing ~42"
	parsed := Parse(input, SurfaceCapture)
	for _, spelling := range []string{";ignore", "&'~/a", "#auth", "!1", "@tomorow", "%missing", "~42"} {
		want := strings.Index(input, spelling)
		found := false
		for _, token := range parsed.Tokens {
			if token.Start == want {
				found = true
				if !strings.HasPrefix(input[token.Start:], token.Form) {
					t.Fatalf("token %+v does not point to its sigil", token)
				}
			}
		}
		if !found {
			t.Errorf("no token starts at %q (%d): %+v", spelling, want, parsed.Tokens)
		}
	}
	if parsed.Text != "Réparer 日本" || len(parsed.Tokens) != 7 {
		t.Fatalf("token positions changed parsing: %+v", parsed)
	}
}

func TestTokenEndCoversTheWholeSpelling(t *testing.T) {
	input := "Ship it ;two words; #ux @next week !2 %web"
	want := map[Field]string{
		FieldComment:  ";two words;",
		FieldTag:      "#ux",
		FieldDue:      "@next week",
		FieldPriority: "!2",
		FieldProject:  "%web",
	}
	parsed := Parse(input, SurfaceCapture)
	for _, token := range parsed.Tokens {
		spelled, ok := want[token.Field]
		if !ok {
			continue
		}
		if got := input[token.Start:token.End]; got != spelled {
			t.Errorf("%s token spans %q, want %q", token.Field, got, spelled)
		}
		delete(want, token.Field)
	}
	if len(want) != 0 {
		t.Fatalf("tokens not found: %v", want)
	}
}
