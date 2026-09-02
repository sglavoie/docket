package textgrammar

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestAttachmentSigilLeavesOrdinaryProseLiteral pins the lexical half of the
// attachment rule. The & character occurs constantly in real titles, so anything
// the application layer could not attach must stay in the text rather than
// vanish from it.
func TestAttachmentSigilLeavesOrdinaryProseLiteral(t *testing.T) {
	for _, input := range []string{
		"R&D pipeline",
		"Q&A with legal",
		"Fix A & B",
		"&10:30 standup",
		"Escape &nbsp; in the footer",
		"Ship &",
		// Quoting widens the span a token may cover; it does not exempt the value
		// from the lexical test, so a quoted phrase naming nothing attachable is
		// still returned to the title with its quotes intact.
		`Ask &"why" before the demo`,
		"Read &'notes' first",
	} {
		parsed := Parse(input, SurfaceCapture)
		if got := parsed.TokensFor(FieldAttachment); len(got) != 0 {
			t.Errorf("%q produced attachment tokens %+v", input, got)
		}
		if parsed.Text != input {
			t.Errorf("%q text = %q, want it unchanged", input, parsed.Text)
		}
	}
}

// TestAttachmentSigilRecognizesEveryAttachableShape covers the three paths the
// application layer classifies into: URL, code reference, and file path.
func TestAttachmentSigilRecognizesEveryAttachableShape(t *testing.T) {
	parsed := Parse("Fix OAuth &https://example.com/i/412 &main.go:42 &~/notes/report.pdf", SurfaceCapture)
	var values []string
	for _, token := range parsed.TokensFor(FieldAttachment) {
		values = append(values, token.Value)
	}
	want := []string{"https://example.com/i/412", "main.go:42", "~/notes/report.pdf"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("attachment values = %v, want %v", values, want)
	}
	if parsed.Text != "Fix OAuth" {
		t.Fatalf("Text = %q, want Fix OAuth", parsed.Text)
	}
}

// TestAttachmentSigilSpansAPathWithSpaces pins the shapes a real path arrives
// in. A file manager copies "~/Library/Application Support/…" quoted or with
// backslash-escaped spaces, and a bare \S+ run stopped at the first space, which
// attached half the path and left the other half in the title. Each shape
// normalizes to the same plain path, so the application layer classifies a
// quoted paste exactly as it classifies a typed one.
func TestAttachmentSigilSpansAPathWithSpaces(t *testing.T) {
	const want = "/Users/me/Application Support/shot 2.png"
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"single quoted", `Fix it &'/Users/me/Application Support/shot 2.png'`},
		{"double quoted", `Fix it &"/Users/me/Application Support/shot 2.png"`},
		{"escaped spaces", `Fix it &/Users/me/Application\ Support/shot\ 2.png`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed := Parse(tc.input, SurfaceCapture)
			tokens := parsed.TokensFor(FieldAttachment)
			if len(tokens) != 1 || tokens[0].Value != want {
				t.Fatalf("attachment tokens = %+v, want exactly %q", tokens, want)
			}
			if parsed.Text != "Fix it" {
				t.Fatalf("Text = %q, want the whole span out of the title", parsed.Text)
			}
		})
	}
}

// TestQuotedAttachmentKeepsItsPositionAheadOfTag re-checks registry order over
// the widened span: a sigil inside a quoted path is part of the path, and only
// peeling the attachment first can keep it there.
func TestQuotedAttachmentKeepsItsPositionAheadOfTag(t *testing.T) {
	parsed := Parse(`Ship it &'/Users/me/Release #4 notes.md' #work`, SurfaceCapture)
	attachments := parsed.TokensFor(FieldAttachment)
	if len(attachments) != 1 || attachments[0].Value != "/Users/me/Release #4 notes.md" {
		t.Fatalf("attachment tokens = %+v, want the quoted path whole", attachments)
	}
	tags := parsed.TokensFor(FieldTag)
	if len(tags) != 1 || tags[0].Value != "work" {
		t.Fatalf("tag tokens = %+v, want only the tag written outside the quotes", tags)
	}
	if parsed.Text != "Ship it" {
		t.Fatalf("Text = %q, want Ship it", parsed.Text)
	}
}

// TestAttachmentIsParsedBeforeTag pins the one place registry order carries
// meaning. The tag pattern stops at the &, so peeling tags first would leave a
// whitespace-preceded & that becomes a spurious attachment.
func TestAttachmentIsParsedBeforeTag(t *testing.T) {
	parsed := Parse("Task #work&http://x/y", SurfaceCapture)
	if got := parsed.TokensFor(FieldAttachment); len(got) != 0 {
		t.Fatalf("attachment tokens = %+v, want none; the & is glued to the tag", got)
	}
	tags := parsed.TokensFor(FieldTag)
	if len(tags) != 1 || tags[0].Value != "work" {
		t.Fatalf("tag tokens = %+v, want one work", tags)
	}
	if parsed.Text != "Task &http://x/y" {
		t.Fatalf("Text = %q, want the remainder to stay literal", parsed.Text)
	}
}

// TestCommentSigilLeavesOrdinaryProseLiteral pins the lexical half of the
// comment rule. A semicolon is ordinary punctuation, so only a paired,
// whitespace-preceded delimiter may consume anything.
func TestCommentSigilLeavesOrdinaryProseLiteral(t *testing.T) {
	for _, input := range []string{
		"Fix a; then b",
		"Ship ;unterminated",
		"Escape &nbsp; in the footer",
		"Ship ;",
		"Ship ; ;",
	} {
		parsed := Parse(input, SurfaceCapture)
		if got := parsed.TokensFor(FieldComment); len(got) != 0 {
			t.Errorf("%q produced comment tokens %+v", input, got)
		}
		if parsed.Text != input {
			t.Errorf("%q text = %q, want it unchanged", input, parsed.Text)
		}
	}
}

// TestCommentIsParsedBeforeEveryOtherSigil pins the reason the comment leads the
// registry: RE2 has no lookaround, so peeling any other field first would let it
// reach inside the comment's span and mint metadata out of comment prose.
func TestCommentIsParsedBeforeEveryOtherSigil(t *testing.T) {
	parsed := Parse("Fix login ;check the #auth flow;", SurfaceCapture)
	comments := parsed.TokensFor(FieldComment)
	if len(comments) != 1 || comments[0].Value != "check the #auth flow" {
		t.Fatalf("comment tokens = %+v, want one holding the whole span", comments)
	}
	if got := parsed.TokensFor(FieldTag); len(got) != 0 {
		t.Fatalf("tag tokens = %+v, want none; #auth is comment text", got)
	}
	if parsed.Text != "Fix login" {
		t.Fatalf("Text = %q, want Fix login", parsed.Text)
	}
}

// TestCommentSigilIsRepeatable keeps two comments on one line distinct rather
// than folding them into one span.
func TestCommentSigilIsRepeatable(t *testing.T) {
	parsed := Parse("Call Dana ;ask about SSO; ;then confirm the date;", SurfaceCapture)
	var values []string
	for _, token := range parsed.TokensFor(FieldComment) {
		values = append(values, token.Value)
	}
	want := []string{"ask about SSO", "then confirm the date"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("comment values = %v, want %v", values, want)
	}
	if parsed.Text != "Call Dana" {
		t.Fatalf("Text = %q, want Call Dana", parsed.Text)
	}
}

// TestSearchRefusesTheCommentSigilByName mirrors the attachment refusal: search
// is a fourth surface of the same registry, not a place a comment can be written.
func TestSearchRefusesTheCommentSigilByName(t *testing.T) {
	guidance := SearchGuidance(ParseSearch("dana ;ask about SSO;"))
	if !strings.Contains(guidance, ";comment; is not a search predicate") {
		t.Fatalf("guidance = %q, want the comment refusal", guidance)
	}
	if !strings.Contains(guidance, "Comments panel") {
		t.Fatalf("guidance = %q, want it to name where comments are added", guidance)
	}
	if got := SearchGuidance(ParseSearch("fix a; then b")); got != "" {
		t.Fatalf("guidance = %q, want an ordinary query to run", got)
	}
}

// TestGuideOrderTeachesTheSameFieldsParseOrderRecognizes is the fence that makes
// splitting the two lists safe. Parse order can move a field for
// disambiguation; it may never change what the guide teaches.
func TestGuideOrderTeachesTheSameFieldsParseOrderRecognizes(t *testing.T) {
	for _, surface := range []Surface{SurfaceCapture, SurfaceEdit, SurfaceOutline, SurfaceSearch} {
		parseOrder := Definitions(surface)
		guide := GuideDefinitions(surface)
		if len(parseOrder) != len(guide) {
			t.Fatalf("surface %d guide = %d definitions, want %d", surface, len(guide), len(parseOrder))
		}
		parseFields := make([]Field, len(parseOrder))
		for i, def := range parseOrder {
			parseFields[i] = def.Field
		}
		guideFields := make([]Field, len(guide))
		for i, def := range guide {
			guideFields[i] = def.Field
		}
		slices.Sort(parseFields)
		slices.Sort(guideFields)
		if !slices.Equal(parseFields, guideFields) {
			t.Errorf("surface %d guide fields = %v, want %v", surface, guideFields, parseFields)
		}
	}

	// The attachment and comment sigils are the reason the split exists: parsed
	// first and second, taught last.
	capture := Definitions(SurfaceCapture)
	if capture[0].Field != FieldComment {
		t.Errorf("parse order position 0 = %s, want comment ahead of every other field", capture[0].Field)
	}
	if capture[1].Field != FieldAttachment {
		t.Errorf("parse order position 1 = %s, want attachment ahead of tag", capture[1].Field)
	}
	guide := GuideDefinitions(SurfaceCapture)
	if guide[len(guide)-2].Field != FieldAttachment {
		t.Errorf("guide order second-to-last = %s, want attachment", guide[len(guide)-2].Field)
	}
	if guide[len(guide)-1].Field != FieldComment {
		t.Errorf("guide order ends with %s, want comment", guide[len(guide)-1].Field)
	}
}

// TestSearchRefusesTheAttachmentSigilByName keeps the refusal specific: the
// sigil is named only when the value is one capture would have honored, so an
// ordinary query containing an ampersand still runs.
func TestSearchRefusesTheAttachmentSigilByName(t *testing.T) {
	refused := ParseSearch("spec &https://example.com/spec.pdf")
	guidance := SearchGuidance(refused)
	if !strings.Contains(guidance, "&link is not a search predicate") {
		t.Fatalf("guidance = %q, want the attachment refusal", guidance)
	}
	if !strings.Contains(guidance, "Attachments panel") {
		t.Fatalf("guidance = %q, want it to name where attachments are added", guidance)
	}
	if got := SearchGuidance(ParseSearch("escape &nbsp; in the footer")); got != "" {
		t.Fatalf("guidance = %q, want an ordinary query to run", got)
	}
}

func TestDueCapturesOptionalClockSuffix(t *testing.T) {
	timed := Parse("Ship it @friday 15:30", SurfaceCapture)
	dueTokens := timed.TokensFor(FieldDue)
	if len(dueTokens) != 1 || dueTokens[0].Value != "friday 15:30" {
		t.Fatalf("@friday 15:30 due tokens = %+v, want one value \"friday 15:30\"", dueTokens)
	}
	if timed.Text != "Ship it" {
		t.Errorf("timed due Text = %q, want \"Ship it\"", timed.Text)
	}

	prose := Parse("Ship it @friday 3 apples", SurfaceCapture)
	proseTokens := prose.TokensFor(FieldDue)
	if len(proseTokens) != 1 || proseTokens[0].Value != "friday" {
		t.Fatalf("@friday 3 apples due tokens = %+v, want one value \"friday\"", proseTokens)
	}
	if prose.Text != "Ship it 3 apples" {
		t.Errorf("prose due Text = %q, want \"Ship it 3 apples\"", prose.Text)
	}

	iso := Parse("Ship it @2026-08-25 09:00", SurfaceCapture)
	isoTokens := iso.TokensFor(FieldDue)
	if len(isoTokens) != 1 || isoTokens[0].Value != "2026-08-25 09:00" {
		t.Fatalf("@2026-08-25 09:00 due tokens = %+v, want one value \"2026-08-25 09:00\"", isoTokens)
	}
}
