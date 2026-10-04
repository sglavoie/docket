package grammar

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// syntaxGuideTitle is the neutral heading shared by both capture hosts (Quick
// Capture and the Linked Task Creator). It is intentionally host-agnostic so
// the same grammar reference reads correctly wherever it appears.
const syntaxGuideTitle = "Capture Syntax"

// SyntaxGuideEntry documents one canonical sigil family: the accepted form, a
// short description, display examples, and a set of full capture inputs that
// must parse successfully. The display fields (forms/desc/examples) may carry
// presentation sugar such as arrows or "(repeatable)" hints; the validation
// inputs are kept separate so parser-backed tests exercise real, parseable
// capture strings rather than the rendered sugar.
type SyntaxGuideEntry struct {
	Forms      string
	desc       string
	examples   string
	validation []string
}

// SyntaxGuideGroup clusters related capture sigils under a heading so the
// reference scans as intent groups rather than a flat sigil soup.
type SyntaxGuideGroup struct {
	name    string
	Entries []SyntaxGuideEntry
}

// SyntaxGuideGroups returns the grouped sigil reference in teach order, which is
// why it reads GuideDefinitions rather than the parse-order projection: the
// attachment sigil has to be peeled before #tag and would otherwise lead the
// guide. Project Routing is only included when %project capture is enabled, so
// the Linked Task Creator (which skips project syntax) never advertises it.
func (q *QuickCaptureInput) SyntaxGuideGroups() []SyntaxGuideGroup {
	var groupOrder []string
	byGroup := make(map[string][]SyntaxGuideEntry)
	for _, def := range textgrammar.GuideDefinitions(textgrammar.SurfaceCapture) {
		if def.Legacy || (q.skipProjectSyntax && def.Field == textgrammar.FieldProject) {
			continue
		}
		if _, exists := byGroup[def.Group]; !exists {
			groupOrder = append(groupOrder, def.Group)
		}
		entry := SyntaxGuideEntry{
			Forms:    strings.Join(def.Forms, " · "),
			desc:     def.Description,
			examples: strings.Join(def.Examples, "  "),
		}
		for _, example := range def.Examples {
			entry.validation = append(entry.validation, "Task "+example)
		}
		byGroup[def.Group] = append(byGroup[def.Group], entry)
	}

	groups := make([]SyntaxGuideGroup, 0, len(groupOrder))
	for _, name := range groupOrder {
		if entries := byGroup[name]; len(entries) > 0 {
			groups = append(groups, SyntaxGuideGroup{name: name, Entries: entries})
		}
	}
	return groups
}

// Layout constants for the syntax guide's measured responsive rendering.
const (
	syntaxEntryIndent = 2 // Leading indent for an entry's first field.
	syntaxFieldIndent = 4 // Indent for wrapped/stacked continuation fields.
	syntaxColumnGap   = 2 // Gap between aligned columns.
)

// RenderSyntaxGuide returns the grouped, measured, wrap-aware canonical sigil
// reference shown when the user toggles the gated syntax help. Shared by Quick
// Capture and the linked-task creator so both surfaces stay in lockstep.
//
// Each entry is laid out with a measured responsive strategy, tried in order:
//  1. Aligned row: forms, description, and examples share one line, with forms
//     and descriptions padded to consistent columns.
//  2. If the aligned row does not fit, keep forms and description on the first
//     row and wrap the examples underneath.
//  3. If forms and description cannot coexist on one row, stack every field.
//
// All measurement uses ANSI-aware visual width, so the guide never overflows
// q.width at narrow widths regardless of styling.
// Group offsets come from the same layout that wraps the guide's text.
func (q *QuickCaptureInput) RenderSyntaxGuide() (string, map[string]int) {
	offsets := make(map[string]int)
	groups := q.SyntaxGuideGroups()

	// Compute aligned column widths across every entry so aligned rows line up
	// their forms and descriptions into consistent columns.
	formsCol, descCol := 0, 0
	for _, g := range groups {
		for _, e := range g.Entries {
			formsCol = max(formsCol, lipgloss.Width(e.Forms))
			descCol = max(descCol, lipgloss.Width(e.desc))
		}
	}

	var lines []string

	// appendWrappedField wraps a single field to q.width at the given indent.
	appendWrappedField := func(indent int, style lipgloss.Style, text string) {
		pad := strings.Repeat(" ", indent)
		avail := q.Width - indent
		if q.Width <= 0 || avail <= 0 {
			lines = append(lines, pad+style.Render(text))
			return
		}
		for _, w := range textfit.WrapStyled(text, avail) {
			lines = append(lines, pad+style.Render(w))
		}
	}

	// padStyled pads an already-styled field with trailing spaces so its rendered
	// visual width reaches col. Trailing padding is unstyled, which is width-safe.
	padStyled := func(styled string, plainWidth, col int) string {
		if plainWidth >= col {
			return styled
		}
		return styled + strings.Repeat(" ", col-plainWidth)
	}

	appendEntry := func(e SyntaxGuideEntry) {
		formsW := lipgloss.Width(e.Forms)
		descW := lipgloss.Width(e.desc)
		examplesW := lipgloss.Width(e.examples)

		styledForms := theme.MatchHighlightStyle.Render(e.Forms)
		styledDesc := theme.MutedStyle.Render(e.desc)
		styledExamples := theme.MutedStyle.Render(e.examples)

		indentStr := strings.Repeat(" ", syntaxEntryIndent)
		gapStr := strings.Repeat(" ", syntaxColumnGap)
		formsDescRow := indentStr +
			padStyled(styledForms, formsW, formsCol) + gapStr +
			padStyled(styledDesc, descW, descCol)
		fdWidth := syntaxEntryIndent + formsCol + syntaxColumnGap + descCol

		switch {
		case q.Width <= 0:
			// No width budget: fall back to a simple stacked form.
			appendWrappedField(syntaxEntryIndent, theme.MatchHighlightStyle, e.Forms)
			appendWrappedField(syntaxFieldIndent, theme.MutedStyle, e.desc)
			if examplesW > 0 {
				appendWrappedField(syntaxFieldIndent, theme.MutedStyle, e.examples)
			}
		case examplesW > 0 && fdWidth+syntaxColumnGap+examplesW <= q.Width:
			// Mode 1: aligned single row.
			lines = append(lines, formsDescRow+gapStr+styledExamples)
		case fdWidth <= q.Width:
			// Mode 2: forms + description on the first row, examples wrapped below.
			lines = append(lines, formsDescRow)
			if examplesW > 0 {
				appendWrappedField(syntaxFieldIndent, theme.MutedStyle, e.examples)
			}
		default:
			// Mode 3: forms and description cannot coexist; stack every field.
			appendWrappedField(syntaxEntryIndent, theme.MatchHighlightStyle, e.Forms)
			appendWrappedField(syntaxFieldIndent, theme.MutedStyle, e.desc)
			if examplesW > 0 {
				appendWrappedField(syntaxFieldIndent, theme.MutedStyle, e.examples)
			}
		}
	}

	lines = append(lines, theme.MutedStyle.Render(syntaxGuideTitle))
	for _, g := range groups {
		lines = append(lines, "")
		offsets[g.name] = len(lines)
		appendWrappedField(0, theme.SectionHeaderStyle, g.name)
		for _, e := range g.Entries {
			appendEntry(e)
		}
	}
	return strings.Join(lines, "\n") + "\n", offsets
}

// IsAutocompleteActive returns whether autocomplete is currently showing.
func (q *QuickCaptureInput) IsAutocompleteActive() bool {
	return q.AC.IsActive()
}

// syntaxGroupAtCaret uses the grammar's group metadata rather than another
// sigil-to-section table. A boundary after a token still refers to that token.
func (q *QuickCaptureInput) syntaxGroupAtCaret() string {
	prefix := strings.TrimRight(q.Input.Value()[:q.cursorByteOffset()], " \t")
	mode, _, _ := DetectTrigger(prefix, q.Triggers)
	if mode == ACModeNone {
		return ""
	}
	for _, def := range textgrammar.GuideDefinitions(textgrammar.SurfaceCapture) {
		if !def.Legacy && AutocompleteModeForField(def.Field) == mode {
			return def.Group
		}
	}
	return ""
}

func (q *QuickCaptureInput) refreshSyntaxGuide() {
	content, offsets := q.RenderSyntaxGuide()
	q.SyntaxVP.SetContent(content)
	q.syntaxAnchorOffset = offsets[q.syntaxAnchor]
	q.positionSyntaxAnchor()
}

func (q *QuickCaptureInput) positionSyntaxAnchor() {
	if q.syntaxAnchor != "" {
		q.SyntaxVP.SetYOffset(q.syntaxAnchorOffset)
	}
	q.clampSyntaxOffset()
}
