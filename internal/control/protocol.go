// Package control is the inbound half of a hosted tt session: a Unix socket a
// host such as tt.app connects to, so it can ask the running TUI to act and hear
// what the TUI reports, instead of reading the window title back and typing
// keys into the terminal.
//
// The package owns the wire and the connections alone. It never touches the
// workspace: every request is handed to the TUI's own Update through the
// Dispatch function, so a mutation it asks for runs through the same executor,
// transaction, and undo ledger a keystroke would, and a request arriving while
// the user is mid-interaction is refused by the same guards.
//
// The protocol is newline-delimited JSON in both directions. A host writes
// Request lines; tt writes one Reply per request, carrying the request's id,
// and Event lines whenever it has something to report. Every connection hears
// every event, and a new connection hears the latest pulse at once.
package control

// The operations a Request may name.
const (
	// OpReveal shows one task in the Outliner, seating the cursor on it.
	OpReveal = "reveal"
	// OpAcknowledge reviews one task, as R does on its row.
	OpAcknowledge = "acknowledge"
	// OpDefer defers one task by Minutes from now.
	OpDefer = "defer"
	// OpCapture captures Text through the composer's grammar.
	OpCapture = "capture"
	// OpComplete asks what the composer would suggest for Text, read as the
	// draft up to the caret. It reads and never writes, so tt answers it even
	// while the user is mid-interaction.
	OpComplete = "complete"
	// OpPreview describes the full capture draft without writing anything.
	OpPreview = "preview"
)

// The events tt reports.
const (
	// EventPulse carries the workspace pulse whenever it changes.
	EventPulse = "pulse"
	// EventArrivals carries the tasks that crossed into needing attention since
	// the last periodic tick.
	EventArrivals = "arrivals"
)

// The reasons an Arrival names.
const (
	ArrivalCheckIn  = "check-in"
	ArrivalDeadline = "deadline"
	ArrivalWake     = "wake"
)

// Request is one line a host writes. Project and Task are the stored project
// and task IDs a previous Event named, never a slug or a ShortID, so a rename
// or a renumbering between the event and the request cannot retarget it.
type Request struct {
	ID      int    `json:"id"`
	Op      string `json:"op"`
	Project string `json:"project,omitempty"`
	Task    string `json:"task,omitempty"`
	Text    string `json:"text,omitempty"`
	Minutes int    `json:"minutes,omitempty"`
}

// Reply answers the Request whose ID it carries. Message is what tt said: the
// outcome on success, the refusal otherwise. Completion answers OpComplete
// alone, and is absent when the caret stands on no sigil the grammar completes.
// Preview answers OpPreview with the full draft's human-readable route and date.
type Reply struct {
	ID         int         `json:"reply"`
	OK         bool        `json:"ok"`
	Message    string      `json:"message,omitempty"`
	Completion *Completion `json:"completion,omitempty"`
	Preview    string      `json:"preview,omitempty"`
}

// Completion is what the composer would offer for the token ending at the
// caret. Token is that token as the host sent it, sigil included, so the host
// replaces exactly the text it ends with and never counts bytes against its
// own string encoding. Exact says the token already stands as typed — a value
// on offer, or a tag or context the grammar accepts as written — so Enter
// submits rather than accepting a suggestion, as it does in the composer.
type Completion struct {
	Token string           `json:"token"`
	Items []CompletionItem `json:"items"`
	Exact bool             `json:"exact,omitempty"`
}

// CompletionItem is one suggestion: Label is the plain text a list shows, and
// Insert is the whole replacement for Token, sigil and trailing space
// included, exactly what accepting it in the composer writes.
type CompletionItem struct {
	Label  string `json:"label"`
	Insert string `json:"insert"`
}

// Pulse is the workspace pulse: actionable inbox, Today, and overdue counts,
// and the current local day's done count.
type Pulse struct {
	Inbox   int `json:"inbox"`
	Today   int `json:"today"`
	Overdue int `json:"overdue"`
	Done    int `json:"done"`
}

// Arrival is one task that crossed into needing attention, and why.
type Arrival struct {
	Kind    string `json:"kind"`
	Project string `json:"project"`
	Task    string `json:"task"`
	Title   string `json:"title"`
	// Reviewable is whether acknowledging it is allowed now, so a host offers
	// that action only where tt would accept it.
	Reviewable bool `json:"reviewable,omitempty"`
}

// Event is one line tt writes unprompted. Title and Body word an arrivals
// event as one notification, the way --notify would.
type Event struct {
	Event    string    `json:"event"`
	Pulse    *Pulse    `json:"pulse,omitempty"`
	Arrivals []Arrival `json:"arrivals,omitempty"`
	Title    string    `json:"title,omitempty"`
	Body     string    `json:"body,omitempty"`
}
