package models

import (
	"fmt"
	"slices"
)

// QueryMode is the projection axis. The tokens are the durable spellings the
// TUI mode catalog and the persisted lens already use; they live here because
// the query is the one value every carrier writes into, and a mode a lens cannot
// spell is a mode a query cannot own.
type QueryMode string

// Canonical mode tokens. QueryModeUnset means the query does not own the axis.
const (
	QueryModeUnset   QueryMode = ""
	QueryModeAll     QueryMode = "all"
	QueryModeInbox   QueryMode = "inbox"
	QueryModeToday   QueryMode = "today"
	QueryModeNext    QueryMode = "next"
	QueryModeBacklog QueryMode = "backlog"
	QueryModeDue     QueryMode = "due"
	QueryModeRecent  QueryMode = "recent"
)

var allQueryModes = []QueryMode{
	QueryModeAll,
	QueryModeInbox,
	QueryModeToday,
	QueryModeNext,
	QueryModeBacklog,
	QueryModeDue,
	QueryModeRecent,
}

// AllQueryModes returns the seven catalog modes in presentation order.
func AllQueryModes() []QueryMode {
	return slices.Clone(allQueryModes)
}

// Validate accepts the seven catalog modes and the unset axis.
func (m QueryMode) Validate() error {
	if m == QueryModeUnset || slices.Contains(allQueryModes, m) {
		return nil
	}
	return fmt.Errorf("unknown query mode %q", m)
}

// QuerySortField is the manual sort axis field. The tokens match the persisted
// lens spellings.
type QuerySortField string

// Canonical sort field tokens. QuerySortDefault is the mode's own order.
const (
	QuerySortDefault  QuerySortField = ""
	QuerySortPriority QuerySortField = "priority"
	QuerySortDue      QuerySortField = "due"
	QuerySortCreated  QuerySortField = "created"
	QuerySortStatus   QuerySortField = "status"
	QuerySortUpdated  QuerySortField = "updated"
	QuerySortTitle    QuerySortField = "title"
)

var allQuerySortFields = []QuerySortField{
	QuerySortPriority,
	QuerySortDue,
	QuerySortCreated,
	QuerySortStatus,
	QuerySortUpdated,
	QuerySortTitle,
}

// AllQuerySortFields returns every nameable sort field.
func AllQuerySortFields() []QuerySortField {
	return slices.Clone(allQuerySortFields)
}

// QuerySort is the manual order axis. Its zero value is the mode's own order,
// which is why Ascending carries no meaning until Field is set.
type QuerySort struct {
	Field     QuerySortField
	Ascending bool
}

// Validate accepts every nameable sort field and the default order.
func (s QuerySort) Validate() error {
	if s.Field == QuerySortDefault {
		return nil
	}
	if slices.Contains(allQuerySortFields, s.Field) {
		return nil
	}
	return fmt.Errorf("unknown query sort field %q", s.Field)
}
