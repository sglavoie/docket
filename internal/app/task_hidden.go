package app

// SetHidden hides or unhides every target as one checked transaction and one
// undo entry. The frozen qualified targets are re-resolved against the snapshot
// the commit writes, so a target that moved or vanished since the keypress is
// refused rather than guessed at.
//
// Hidden is an axis beside the pin and the review flag, not a lifecycle state,
// so no state is refusable: a terminal, deferred, or Waiting task hides exactly
// as an open one does. Each target is written through ApplyHidden, which leaves
// an already-correct member untouched and otherwise writes the flag and
// UpdatedAt alone. Only the targeted tasks store the flag; their descendants are
// hidden by inheritance at read time.
func (s *Service) SetHidden(locators []TaskLocator, hidden bool) (BulkResult, error) {
	action := "Unhide Tasks"
	if hidden {
		action = "Hide Tasks"
	}
	return s.bulkAttributeOverSnapshot(locators, action, "hide", func(_ Snapshot, _ []TaskRef) (func(TaskRef) error, error) {
		now := s.clock.Now()
		return func(ref TaskRef) error { ref.Task.ApplyHidden(hidden, now); return nil }, nil
	})
}

// ToggleHidden flips the hidden flag over the whole selection as one decision
// read from the commit snapshot, the rule the review toggle uses: every target
// is hidden unless all of them already store the flag, in which case every
// target is unhidden. It reads the stored flag, never inheritance, so unhiding
// a task under a hidden parent stores nothing new and the parent still hides it.
func (s *Service) ToggleHidden(locators []TaskLocator) (BulkResult, error) {
	return s.bulkAttributeOverSnapshot(locators, "Toggle Hidden", "hide", func(_ Snapshot, refs []TaskRef) (func(TaskRef) error, error) {
		hidden := false
		for _, ref := range refs {
			if !ref.Task.Hidden {
				hidden = true
				break
			}
		}
		now := s.clock.Now()
		return func(ref TaskRef) error { ref.Task.ApplyHidden(hidden, now); return nil }, nil
	})
}
