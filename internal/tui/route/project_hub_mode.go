package route

// ProjectHubMode identifies the population exposed by the Project Hub. All is
// the zero value so existing route and message callers retain the full Hub.
//
// It lives here rather than beside the Hub because ManageParams carries it and
// canonicalRouteParams range-checks it, so the route layer would otherwise
// name a type the surface three layers above it declares.
type ProjectHubMode uint8

const (
	ProjectHubModeAll ProjectHubMode = iota
	ProjectHubModePinned
)
