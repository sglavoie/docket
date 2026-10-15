package capturetext

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// Defaults are what a capture host supplies beyond the typed lines: the source
// project's composer defaults, which a line's own token always overrides.
type Defaults struct {
	// Priority applies to a line that names no priority; zero is none.
	Priority int
	// Tags are merged ahead of each line's own tags.
	Tags []string
	// ParentID nests a line that names no parent.
	ParentID *string
	// Due applies to a line that names no due date.
	Due *time.Time
	// Destination is the root default for a line that names none; empty is
	// Inbox. A child line always defaults to Backlog.
	Destination app.CaptureDestination
	// SkipProject leaves %project literal, for a host bound to one project.
	SkipProject bool
	// Now is the instant a due date is measured against for the Today route;
	// the zero value reads the wall clock.
	Now time.Time
}

// Requests reads text exactly as the compact composer does on submit: every
// nonblank line through the capture grammar and the defaults, each refusal
// reported against its own line. It never returns requests beside a refusal,
// so a batch is all or nothing before the application layer sees it.
func Requests(text string, source *models.Project, allProjects []*models.Project, defaults Defaults) ([]app.CaptureRequest, []LineError) {
	if source == nil {
		return nil, []LineError{{Line: 1, Message: "project is unavailable"}}
	}
	lines := strings.Split(text, "\n")
	reqs := make([]app.CaptureRequest, 0, len(lines))
	var lineErrors []LineError
	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		req, problem := lineRequest(idx+1, raw, line, source, allProjects, defaults)
		if problem != nil {
			lineErrors = append(lineErrors, *problem)
			continue
		}
		reqs = append(reqs, req)
	}
	if len(reqs) == 0 && len(lineErrors) == 0 {
		lineErrors = []LineError{{Line: 1, Message: "no tasks to create"}}
	}
	if len(lineErrors) > 0 {
		return nil, lineErrors
	}
	return reqs, nil
}

func lineRequest(number int, raw, line string, source *models.Project, allProjects []*models.Project, defaults Defaults) (app.CaptureRequest, *LineError) {
	refuse := func(message string) (app.CaptureRequest, *LineError) {
		return app.CaptureRequest{}, &LineError{Line: number, Message: message}
	}
	parsed, err := ParseQuickCapture(line, source, allProjects, defaults.SkipProject)
	if err != nil {
		problem := LineError{Line: number, Message: err.Error()}
		var tokenError *CaptureTokenError
		if errors.As(err, &tokenError) {
			leadingBytes := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
			problem.Column = utf8.RuneCountInString(raw[:leadingBytes]) + tokenError.Column
			problem.UnknownProject = tokenError.UnknownProject
		}
		return app.CaptureRequest{}, &problem
	}
	if err := models.ValidateTaskTitle(parsed.Title); err != nil {
		return refuse(err.Error())
	}
	target := source
	if parsed.TargetProjectID != nil {
		target = projectByID(*parsed.TargetProjectID, source, allProjects)
	}
	if target == nil {
		return refuse("target project is unavailable")
	}

	priority := parsed.Priority
	if defaults.Priority != 0 && !parsed.PrioritySet {
		priority = defaults.Priority
	}
	parentID := parsed.ParentID
	if parentID == nil {
		parentID = cloneString(defaults.ParentID)
	}
	due := parsed.DueDate
	if due == nil {
		due = cloneTime(defaults.Due)
	}
	destination := parsed.Destination
	if !parsed.DestinationSet {
		destination = defaultDestination(defaults.Destination, parentID)
	}
	if DueIsToday(due, defaults.Now) {
		destination = app.CaptureDestinationToday
	}
	destination = ComposerDeferredDestination(parsed, destination)
	if parentID != nil {
		if err := validateParent(target, *parentID); err != nil {
			return refuse(err.Error())
		}
	}
	return app.CaptureRequest{
		SourceLine:   number,
		ProjectID:    target.ID,
		Title:        parsed.Title,
		Priority:     priority,
		Tags:         mergeTags(defaults.Tags, parsed.Tags),
		Context:      parsed.Context,
		Due:          due,
		DeferSet:     parsed.DeferSet,
		Defer:        parsed.DeferUntil,
		ParentID:     parentID,
		ChildIDs:     parsed.ChildIDs(),
		Destination:  destination,
		Prepend:      true,
		Pinned:       parsed.Pinned,
		Review:       parsed.Review,
		Attachments:  parsed.Attachments,
		Comments:     parsed.Comments,
		Dependencies: parsed.DependencyLocators(),
	}, nil
}

// DueIsToday compares calendar fields rather than instants: a due date is a
// local calendar day, and a timed one keeps its day whatever the clock says.
// The instant is read in the due date's own zone; a zero now reads the wall
// clock.
func DueIsToday(due *time.Time, now time.Time) bool {
	if due == nil {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	dueYear, dueMonth, dueDay := due.Date()
	nowYear, nowMonth, nowDay := now.In(due.Location()).Date()
	return dueYear == nowYear && dueMonth == nowMonth && dueDay == nowDay
}

func defaultDestination(root app.CaptureDestination, parentID *string) app.CaptureDestination {
	if parentID != nil {
		return app.CaptureDestinationBacklog
	}
	if root != "" {
		return root
	}
	return app.CaptureDestinationInbox
}

// validateParent mirrors the application-layer child placement rule so a line
// nesting too deep is refused against its own line rather than the batch.
func validateParent(project *models.Project, parentID string) error {
	parent := project.GetTaskByID(parentID)
	if parent == nil {
		return fmt.Errorf("parent not found: %s", parentID)
	}
	if parent.Level+1 > models.MaxNestingDepth {
		return fmt.Errorf("nesting under #%d would exceed maximum depth of %d levels", parent.ShortID, models.MaxNestingDepth)
	}
	return nil
}

func mergeTags(shared, line []string) []string {
	result := make([]string, 0, len(shared)+len(line))
	seen := make(map[string]bool, len(shared)+len(line))
	for _, tag := range append(append([]string(nil), shared...), line...) {
		key := strings.ToLower(strings.TrimSpace(tag))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, tag)
	}
	return result
}

func projectByID(id string, source *models.Project, allProjects []*models.Project) *models.Project {
	if source != nil && source.ID == id {
		return source
	}
	return ProjectByStableID(id, allProjects)
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cp := *value
	return &cp
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cp := *value
	return &cp
}
