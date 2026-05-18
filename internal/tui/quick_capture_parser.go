package tui

import "github.com/sglavoie/tt/internal/models"

// parseQuickCapture extracts metadata from the input string.
// Syntax: "^bug Task title !1 #backend #urgent @tomorrow >3 %projectname"
// When skipProject is true, %project syntax is ignored (treated as literal text).
func parseQuickCapture(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*quickCaptureResult, error) {
	parser := newQuickCaptureParser(input, project, allProjects, skipProject, true)
	parser.parse()
	if parser.err != nil {
		return nil, parser.err
	}
	return parser.result, nil
}

// parseQuickCaptureForPreview parses input for live preview without failing early.
// Returns partial results and any validation error string.
func parseQuickCaptureForPreview(input string, project *models.Project, allProjects []*models.Project, skipProject bool) (*quickCaptureResult, string) {
	parser := newQuickCaptureParser(input, project, allProjects, skipProject, false)
	parser.parse()
	return parser.result, parser.errMsg
}

func (p *quickCaptureParser) parse() {
	for _, step := range []func() bool{
		p.parseTaskTypeAlias,
		p.parseTaskType,
		p.parsePriorityAlias,
		p.parsePriority,
		p.parseTagAlias,
		p.parseTags,
		p.parseDueDate,
		p.parseDueDateAlias,
		p.parseRecurrenceAlias,
		p.parseRecurrence,
		p.parseProjectAlias,
		p.parseProject,
		p.parseParentAlias,
		p.parseParent,
		p.parseSectionAlias,
		p.parseSection,
		p.parseStatusAlias,
		p.parseStatus,
	} {
		if !step() {
			return
		}
	}

	p.finalizeTitle()
}
