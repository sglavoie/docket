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
