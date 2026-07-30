package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

type quickCaptureParser struct {
	input       string
	project     *models.Project
	allProjects []*models.Project
	skipProject bool
	strict      bool
	result      *quickCaptureResult
	err         error
	errMsg      string
}

func newQuickCaptureParser(input string, project *models.Project, allProjects []*models.Project, skipProject, strict bool) *quickCaptureParser {
	return &quickCaptureParser{
		input:       input,
		project:     project,
		allProjects: allProjects,
		skipProject: skipProject,
		strict:      strict,
		result:      &quickCaptureResult{priority: 3},
	}
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
	p.result.title = textgrammar.NormalizeText(p.input)
	if p.strict && p.result.title == "" {
		p.err = fmt.Errorf("task title cannot be empty")
	}
}
