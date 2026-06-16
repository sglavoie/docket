package tui

import (
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

var (
	captureTypeAliasRe     = regexp.MustCompile(`(?i)(?:^|\s)type:(epic|story|task|bug|idea|note)`)
	captureTypeRe          = regexp.MustCompile(`(?i)(?:^|\s)\^(epic|story|task|bug|idea|note)`)
	capturePriorityAliasRe = regexp.MustCompile(`(?i)(?:^|\s)(?:pri|priority):(critical|urgent|high|normal|medium|lowest|low|cr|ur|hi|no|med|me|lo|c|u|h|n|m|l|[1-5])`)
	capturePriorityRe      = regexp.MustCompile(`(?i)(?:^|\s)!(critical|urgent|high|normal|medium|lowest|low|cr|ur|hi|no|med|me|lo|c|u|h|n|m|l|[1-5])`)
	captureTagAliasRe      = regexp.MustCompile(`(?i)(?:^|\s)tag:([\w][\w.-]*)`)
	captureTagRe           = regexp.MustCompile(`(?:^|\s)#([\w][\w.-]*)`)
	captureDueDateRe       = regexp.MustCompile(`(?i)(?:^|\s)@(next\s+\w+|in\s+\d+\s+\w+|\S+)`)
	captureDueAliasRe      = regexp.MustCompile(`(?i)(?:^|\s)due:(next\s+\w+|in\s+\d+\s+\w+|\S+)`)
	captureRecurAliasRe    = regexp.MustCompile(`(?i)(?:^|\s)recur:(\S+)`)
	captureRecurRe         = regexp.MustCompile(`(?:^|\s)~(\S+)`)
	captureProjectAliasRe  = regexp.MustCompile(`(?i)(?:^|\s)(?:p|proj):(\S+)`)
	captureProjectRe       = regexp.MustCompile(`(?:^|\s)%(\S+)`)
	captureParentAliasRe   = regexp.MustCompile(`(?i)(?:^|\s)parent:(\S+)`)
	captureParentRe        = regexp.MustCompile(`(?:^|\s)>(\S+)`)
	captureSectionAliasRe  = regexp.MustCompile(`(?i)(?:^|\s)(?:sec|section):(\S+)`)
	captureSectionRe       = regexp.MustCompile(`(?:^|\s)/(\S+)`)
	captureStatusAliasRe   = regexp.MustCompile(`(?i)(?:^|\s)status:(\w+)`)
	captureStatusRe        = regexp.MustCompile(`(?:^|\s)\$(\w+)`)
	captureWhitespaceRe    = regexp.MustCompile(`\s+`)
)

func (p *quickCaptureParser) parseTaskTypeAlias() bool {
	return p.parseTaskTypeWith(captureTypeAliasRe, "type:")
}

func (p *quickCaptureParser) parseTaskType() bool {
	return p.parseTaskTypeWith(captureTypeRe, "")
}

func (p *quickCaptureParser) parseTaskTypeWith(re *regexp.Regexp, alias string) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	taskType, ok := models.ParseTaskTypeInsensitive(match[1])
	if !ok && p.failf("invalid task type: %s", match[1]) {
		return false
	}
	if ok {
		p.result.taskType = taskType
		if alias != "" {
			p.addAliasUsed(alias)
		}
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parsePriorityAlias() bool {
	return p.parsePriorityWith(capturePriorityAliasRe, "pri:")
}

func (p *quickCaptureParser) parsePriority() bool {
	return p.parsePriorityWith(capturePriorityRe, "")
}

func (p *quickCaptureParser) parsePriorityWith(re *regexp.Regexp, alias string) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	p.result.priority = parsePriorityToken(match[1])
	if alias != "" {
		p.addAliasUsed(alias)
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseTagAlias() bool {
	matches := captureTagAliasRe.FindAllStringSubmatch(p.input, -1)
	if len(matches) == 0 {
		return true
	}

	for _, match := range matches {
		if len(match) > 1 {
			p.result.tags = append(p.result.tags, match[1])
		}
	}
	p.addAliasUsed("tag:")
	p.input = captureTagAliasRe.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseTags() bool {
	for _, match := range captureTagRe.FindAllStringSubmatch(p.input, -1) {
		if len(match) > 1 {
			p.result.tags = append(p.result.tags, match[1])
		}
	}
	p.input = captureTagRe.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseDueDate() bool {
	return p.parseDateWith(captureDueDateRe, ParseDate)
}

func (p *quickCaptureParser) parseDueDateAlias() bool {
	if p.result.dueDate != nil {
		p.input = captureDueAliasRe.ReplaceAllString(p.input, " ")
		return true
	}
	return p.parseDateWith(captureDueAliasRe, ParseFlexibleDate)
}

func (p *quickCaptureParser) parseDateWith(re *regexp.Regexp, parse func(string) (time.Time, error)) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	date, err := parse(match[1])
	if err != nil {
		if p.failf("invalid date: %s", match[1]) {
			return false
		}
	} else {
		p.result.dueDate = &date
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseRecurrenceAlias() bool {
	return p.parseRecurrenceWith(captureRecurAliasRe, "recur:")
}

func (p *quickCaptureParser) parseRecurrence() bool {
	return p.parseRecurrenceWith(captureRecurRe, "")
}

func (p *quickCaptureParser) parseRecurrenceWith(re *regexp.Regexp, alias string) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	pattern := models.ParseRecurrence(match[1])
	if pattern == nil {
		if p.failf("invalid recurrence: %s", match[1]) {
			return false
		}
	} else {
		p.result.recurrence = pattern
		if alias != "" {
			p.addAliasUsed(alias)
		}
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseProjectAlias() bool {
	if p.skipProject {
		return true
	}
	return p.parseProjectWith(captureProjectAliasRe, "p:")
}

func (p *quickCaptureParser) parseProject() bool {
	if p.skipProject {
		return true
	}
	return p.parseProjectWith(captureProjectRe, "")
}

func (p *quickCaptureParser) parseProjectWith(re *regexp.Regexp, alias string) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	ref := match[1]
	targetProject := resolveProject(ref, p.allProjects)
	if targetProject == nil {
		if p.failf("project not found: '%s' (use %%project-slug or name)", ref) {
			return false
		}
	} else {
		p.result.targetProjectID = &targetProject.ID
		p.result.targetProjectName = targetProject.Name
		p.project = targetProject
		if alias != "" {
			p.addAliasUsed(alias)
		}
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseParentAlias() bool {
	return p.parseParentWith(captureParentAliasRe, "parent:")
}

func (p *quickCaptureParser) parseParent() bool {
	return p.parseParentWith(captureParentRe, "")
}

func (p *quickCaptureParser) parseParentWith(re *regexp.Regexp, alias string) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	ref := match[1]
	parent := resolveParent(ref, p.project)
	if parent == nil {
		if p.failf("parent not found: '%s' (use >short-id or title prefix)", ref) {
			return false
		}
	} else if !p.result.taskType.CanBeChildOf(parent.Type) {
		if p.failf("%s cannot be child of %s", p.result.taskType.HeadingPrefix(), parent.Type.HeadingPrefix()) {
			return false
		}
	} else {
		p.result.parentID = &parent.ID
		if alias != "" {
			p.addAliasUsed(alias)
		}
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func (p *quickCaptureParser) parseSectionAlias() bool {
	return p.parseSectionWith(captureSectionAliasRe, "sec:")
}

func (p *quickCaptureParser) parseSection() bool {
	return p.parseSectionWith(captureSectionRe, "")
}

func (p *quickCaptureParser) parseSectionWith(re *regexp.Regexp, alias string) bool {
	loc := re.FindStringSubmatchIndex(p.input)
	if loc == nil {
		return true
	}

	token := p.input[loc[2]:loc[3]]
	section, ok := resolveSection(token, p.project)
	if !ok {
		if p.failf("section not found: '%s' (use section name or /inbox /next /someday)", token) {
			return false
		}
	} else {
		p.result.section = section
		if alias != "" {
			p.addAliasUsed(alias)
		}
	}
	p.input = p.input[:loc[0]] + " " + p.input[loc[1]:]
	return true
}

func (p *quickCaptureParser) parseStatusAlias() bool {
	return p.parseStatusWith(
		captureStatusAliasRe,
		"status:",
		"invalid status: status:%s (use start, blocked, done, canceled, review, standby, hold, maybe)",
		"invalid status: status:%s",
	)
}

func (p *quickCaptureParser) parseStatus() bool {
	return p.parseStatusWith(
		captureStatusRe,
		"",
		"invalid status: $%s (use $start, $blocked, $done, $canceled, $review, $standby, $hold, $maybe)",
		"invalid status: $%s",
	)
}

func (p *quickCaptureParser) parseStatusWith(re *regexp.Regexp, alias, strictErr, previewErr string) bool {
	match := re.FindStringSubmatch(p.input)
	if len(match) <= 1 {
		return true
	}

	status, activate, ok := parseCaptureStatus(match[1])
	if !ok {
		format := strictErr
		if !p.strict {
			format = previewErr
		}
		if p.failf(format, match[1]) {
			return false
		}
	} else {
		p.result.status = status
		p.result.activate = activate
		if alias != "" {
			p.addAliasUsed(alias)
		}
	}
	p.input = re.ReplaceAllString(p.input, " ")
	return true
}

func parseCaptureStatus(token string) (models.TaskStatus, bool, bool) {
	switch strings.ToLower(token) {
	case "start":
		return models.StatusInProgress, true, true
	case "blocked":
		return models.StatusBlocked, false, true
	case "done":
		return models.StatusDone, false, true
	case "canceled":
		return models.StatusCanceled, false, true
	case "review":
		return models.StatusReview, false, true
	case "standby", "hold":
		return models.StatusStandby, false, true
	case "maybe":
		return models.StatusMaybe, false, true
	default:
		return "", false, false
	}
}

func resolveSection(sectionName string, project *models.Project) (string, bool) {
	for _, section := range project.GetSections() {
		if models.IsGTDSection(section) && strings.EqualFold(section, sectionName) {
			return section, true
		}
	}
	return "", false
}

func normalizeCaptureTitle(input string) string {
	return captureWhitespaceRe.ReplaceAllString(strings.TrimSpace(input), " ")
}
