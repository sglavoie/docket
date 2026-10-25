// Package grammar is the presentation half of the capture and edit sigils:
// the parsers behind quick capture, the inline row editor and the outline
// buffer, the autocomplete state machine they share, and the preview and
// syntax-guide renderings that show a draft back to the user. internal/textgrammar
// stays the canonical definition of what a sigil is; this package is what turns
// a typed line into a validated request against a workspace snapshot.
//
// It sits at L2 and imports only chrome, theme, textfit and taskref from
// internal/tui. In particular it never imports query: the search surface is the
// fourth consumer of the same registry, so it reads this package rather than
// the other way round.
package capturetext

import (
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// ParseQuickCapture extracts metadata from the input string.
// Canonical token forms come from textgrammar.SurfaceCapture.
// When skipProject is true, %project syntax is ignored (treated as literal text).
func ParseQuickCapture(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*QuickCaptureResult, error) {
	return ParseQuickCaptureSurface(input, project, allProjects, skipProject, textgrammar.SurfaceCapture)
}

func ParseQuickCaptureSurface(input string, project *models.Project, allProjects []*models.Project, skipProject bool, surface textgrammar.Surface) (*QuickCaptureResult, error) {
	parser := newQuickCaptureParser(input, project, allProjects, skipProject, true)
	parser.parse(surface)
	if parser.err != nil {
		return nil, parser.err
	}
	return parser.result, nil
}

// ParseQuickCaptureForPreview parses input for live preview without failing early.
// Returns partial results and any validation error string.
func ParseQuickCaptureForPreview(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*QuickCaptureResult, string) {
	parser := newQuickCaptureParser(input, project, allProjects, skipProject, false)
	parser.parse(textgrammar.SurfaceCapture)
	return parser.result, parser.errMsg
}

func (p *quickCaptureParser) parse(surface textgrammar.Surface) {
	p.surface = surface
	p.applySharedTokens(textgrammar.Parse(p.input, surface))
	if p.err != nil {
		return
	}
	p.finalizeTitle()
}
