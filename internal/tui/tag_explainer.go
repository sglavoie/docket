package tui

import "strings"

// tagExplainer is a width-fitted explainer block plus the words each of its
// lines could not show. The overlay prints Body; whatever footer is still
// visible behind it prints Continuation, which is why the fit breaks on a word
// boundary instead of mid-word. Continuations is a slice because an explainer
// may render one line per registry entry.
type tagExplainer struct {
	Body          string
	Continuations []string
}

// Continuation joins the unshown remainders on the separator the status bar's
// existing wrapping already understands.
func (e tagExplainer) Continuation() string {
	return strings.Join(e.Continuations, footerHelpSeparator)
}

// joinTagExplainerContinuations combines every non-empty remainder from each
// explainer in the same order their blocks are rendered.
func joinTagExplainerContinuations(explainers ...tagExplainer) string {
	combined := tagExplainer{}
	for _, explainer := range explainers {
		for _, continuation := range explainer.Continuations {
			if continuation != "" {
				combined.Continuations = append(combined.Continuations, continuation)
			}
		}
	}
	return combined.Continuation()
}
