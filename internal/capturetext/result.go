package capturetext

import (
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
)

// CapturedChildRef is one resolved <child reference. The identifying fields are
// copied out of the task the reference named rather than kept as a pointer, so a
// parse result can be previewed and mapped into a request without holding a
// handle on a view-owned task.
type CapturedChildRef struct {
	Id       string
	ShortID  int
	Title    string
	Terminal bool
}

// QuickCaptureResult holds parsed metadata from quick capture input.
type QuickCaptureResult struct {
	Title             string
	Priority          int
	PrioritySet       bool
	Tags              []string
	Context           string
	DueDate           *time.Time
	DeferUntil        *time.Time
	DeferSet          bool
	ParentID          *string
	Children          []CapturedChildRef
	Destination       app.CaptureDestination
	DestinationSet    bool
	Pinned            bool
	Review            bool
	DependencyRefs    []CapturedDependencyRef
	Attachments       []string
	Comments          []string
	TargetProjectID   *string         // Target project ID for cross-project capture
	TargetProjectName string          // Target project name (for preview display)
	TargetProject     *models.Project // Target project (for the preview's project color)
}

// ChildIDs projects the resolved children into the stable IDs a CaptureRequest
// carries, in reference order.
func (r *QuickCaptureResult) ChildIDs() []string {
	if len(r.Children) == 0 {
		return nil
	}
	ids := make([]string, len(r.Children))
	for i, child := range r.Children {
		ids[i] = child.Id
	}
	return ids
}

// DependencyLocators projects the resolved references into the locators a
// CaptureRequest carries, in reference order.
func (r *QuickCaptureResult) DependencyLocators() []app.TaskLocator {
	return DependencyLocatorsOf(r.DependencyRefs)
}
