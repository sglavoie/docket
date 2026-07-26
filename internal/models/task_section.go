package models

// SetSection moves the task to the workflow state a GTD section name maps to.
// Sections are retired as a stored concept; the name is inverted through
// SectionForState onto a WorkflowState. Built-in GTD section names (Inbox, Next,
// Waiting, Someday, Review, Standby, Completed) and the empty string (the
// default active/ready bucket) are accepted; any other name is ignored, matching
// the retired rule that custom grouping lives in tags. The completed bucket maps
// to done. UpdatedAt is bumped only when the state actually changes.
func (t *Task) SetSection(name string) {
	if !IsGTDSection(name) && name != "" {
		return
	}
	newState := StateForSection(name)
	if t.State() == newState {
		return
	}
	_ = t.ApplyStateTransition(newState)
}

// StateForSection inverts SectionForState: it maps a GTD section name to the
// canonical WorkflowState that section occupies. The empty name and any name
// SectionForState never emits resolve to StateReady (the default active bucket).
// The Completed section maps to StateDone (the terminal representative). This
// compatibility bridge lets legacy presentation adapters translate before they
// call state-native application services.
func StateForSection(name string) WorkflowState {
	switch name {
	case SectionInbox:
		return StateInbox
	case SectionNext:
		return StateNext
	case SectionWaiting:
		return StateWaiting
	case SectionStandby:
		return StatePaused
	case SectionReview:
		return StateReview
	case SectionSomeday:
		return StateSomeday
	case SectionCompleted:
		return StateDone
	default:
		return StateReady
	}
}

// InSection reports whether the task belongs to the given section bucket.
// Membership is derived from the task's workflow state (see SectionForState),
// not from the raw legacy Section field: a task whose state resolves elsewhere
// (e.g. an in-progress task still carrying the Next section) belongs to the
// bucket its state occupies. The empty name is the default active bucket
// (ready and in-progress work); both terminal states bucket under Completed.
func (t *Task) InSection(name string) bool {
	return SectionForState(t.State()) == name
}
