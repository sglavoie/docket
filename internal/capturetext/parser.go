package capturetext

import (
	"fmt"
	"unicode/utf8"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

type quickCaptureParser struct {
	input       string
	source      string
	project     *models.Project
	allProjects []*models.Project
	skipProject bool
	strict      bool
	surface     textgrammar.Surface
	result      *QuickCaptureResult
	err         error
	errMsg      string
}

func newQuickCaptureParser(input string, project *models.Project, allProjects []*models.Project, skipProject, strict bool) *quickCaptureParser {
	return &quickCaptureParser{
		input:       input,
		source:      input,
		project:     project,
		allProjects: allProjects,
		skipProject: skipProject,
		strict:      strict,
		result:      &QuickCaptureResult{Priority: models.PriorityUnset},
	}
}

// CaptureTokenError keeps the parser's refusal and its original rune column
// together, so capture can navigate without trying to parse an error message.
type CaptureTokenError struct {
	Column  int // zero-based; includes leading whitespace in the supplied input
	Message string
	// UnknownProject is the %project value the refusal names when no available
	// project holds it, and empty for every other refusal.
	UnknownProject string
}

func (e *CaptureTokenError) Error() string { return e.Message }

func (p *quickCaptureParser) failTokenf(token textgrammar.Token, format string, args ...any) bool {
	stop := p.failf(format, args...)
	if stop {
		p.err = &CaptureTokenError{
			Column:  utf8.RuneCountInString(p.source[:token.Start]),
			Message: p.err.Error(),
		}
	}
	return stop
}

func (p *quickCaptureParser) failf(format string, args ...any) bool {
	msg := fmt.Sprintf(format, args...)
	if p.strict {
		p.err = fmt.Errorf("%s", msg)
		return true
	}
	if p.errMsg == "" {
		p.errMsg = msg
	}
	return false
}

func (p *quickCaptureParser) finalizeTitle() {
	p.result.Title = textgrammar.NormalizeText(p.input)
	if p.strict && p.result.Title == "" {
		p.err = fmt.Errorf("task title cannot be empty")
	}
}
