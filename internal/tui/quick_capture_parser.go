package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// parseQuickCapture extracts metadata from the input string.
// Canonical token forms come from textgrammar.SurfaceCapture.
// When skipProject is true, %project syntax is ignored (treated as literal text).
func parseQuickCapture(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*quickCaptureResult, error) {
	return parseQuickCaptureSurface(input, project, allProjects, skipProject, textgrammar.SurfaceCapture)
}

func parseQuickCaptureSurface(input string, project *models.Project, allProjects []*models.Project, skipProject bool, surface textgrammar.Surface) (*quickCaptureResult, error) {
	parser := newQuickCaptureParser(input, project, allProjects, skipProject, true)
	parser.parse(surface)
	if parser.err != nil {
		return nil, parser.err
	}
	return parser.result, nil
}

// parseQuickCaptureForPreview parses input for live preview without failing early.
// Returns partial results and any validation error string.
func parseQuickCaptureForPreview(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*quickCaptureResult, string) {
	parser := newQuickCaptureParser(input, project, allProjects, skipProject, false)
	parser.parse(textgrammar.SurfaceCapture)
	return parser.result, parser.errMsg
}

func (p *quickCaptureParser) parse(surface textgrammar.Surface) {
	p.applySharedTokens(textgrammar.Parse(p.input, surface))
	if p.err != nil {
		return
	}
	p.finalizeTitle()
}
