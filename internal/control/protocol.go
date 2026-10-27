// Package control is the inbound half of a hosted Docket session: a Unix socket a
// host such as Docket.app connects to, so it can ask the running TUI to act and hear
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
// Request lines; Docket writes one Reply per request, carrying the request's id,
// and Event lines whenever it has something to report. Every connection hears
// every event, and a new connection hears the latest pulse, agenda, and schedule
// at once.
//
// OpQuit is how a host ends the session it hosts: Docket quits as its quit key
// would, or refuses while content is unsaved or a save is pending. A host treats the connection
// ending as the OK it may never read, and falls back to a hangup for a Docket that
// answers "unknown operation".
package control

// The operations a Request may name.
const (
	// OpVersion reads the running process's build identity, never the executable
	// currently installed at its path. It remains available during editing.
	OpVersion = "version"
	// OpReveal shows one task in the Outliner, seating the cursor on it.
	OpReveal = "reveal"
	// OpAcknowledge reviews one task, as R does on its row.
	OpAcknowledge = "acknowledge"
	// OpDefer defers one task by Minutes from now.
	OpDefer = "defer"
	// OpDone marks one task Done, as wd does on its row. It refuses a task
	// already Done or Canceled. OpComplete is not this: it is the composer's
	// completion.
	OpDone = "done"
	// OpCapture captures Text through the composer's grammar.
	OpCapture = "capture"
	// OpCaptureOnce requires a durable CaptureID; retrying it returns the
	// original committed outcome. Older engines refuse this operation rather
	// than silently accepting a request without duplicate protection.
	OpCaptureOnce    = "capture_once"
	OpCaptureContext = "capture_context"
	// OpComplete asks what the composer would suggest for Text, read as the
	// draft up to the caret. It reads and never writes, so Docket answers it even
	// while the user is mid-interaction.
	OpComplete = "complete"
	// OpPreview describes the full capture draft without writing anything.
	OpPreview = "preview"
	// OpQuit ends the session the way the quit key does, saving view and
	// filter state, and is refused while content is unsaved or a save is pending.
	// Clean searches and pickers may quit without confirmation. The OK reply may
	// not outlive the exit, so a host reads the connection closing as the
	// same answer.
	OpQuit = "quit"
	// OpAppearance applies Text, "dark" or "light", as this session's theme.
	// It is presentation alone: Docket answers it even while the user is
	// mid-interaction, writes no machine state, and records no ledger entry,
	// so a Docket launched from a terminal keeps its own saved theme.
	OpAppearance = "appearance"
	// OpThemePalette applies a Ghostty palette sent by the host to the running
	// TUI. Empty Text restores the saved machine theme. Like OpAppearance it
	// changes presentation only, even while an editor is open.
	OpThemePalette = "theme_palette"
	// OpView switches the Outliner to the mode Text names, "inbox", "today", or
	// "due", exactly as the mode jump does, or, for "done", to All with its
	// filters replaced by "$canceled $done ?completed". It is refused where the
	// mode jump is: while the user is mid-interaction or a search projection is
	// open.
	OpView = "view"
	// OpUndo undoes the transaction Text names, as u would, only while it is
	// still the newest undo entry, so a host never takes back something newer
	// the user did in Docket. It is refused while the user is mid-interaction.
	OpUndo = "undo"
	// OpEdit acts on the focused draft or the existing workspace undo/search operation.
	OpEdit = "edit"
)

// The events Docket reports.
const (
	// EventPulse carries the workspace pulse whenever it changes.
	EventPulse   = "pulse"
	EventEditing = "editing"
	// EventArrivals carries the tasks that crossed into needing attention since
	// the last periodic tick.
	EventArrivals = "arrivals"
	// EventAgenda names the tasks behind the pulse's Today and Overdue counts
	// whenever they change, so a host can list them beside the counts.
	EventAgenda = "agenda"
	// EventSchedule names the arrivals still to come, earliest first, whenever
	// they change, so a host can book each one with the system ahead of time
	// and the reminder appears on time even when no Docket is running then.
	EventSchedule = "schedule"
)

// The kinds an AgendaItem names.
const (
	AgendaOverdue = "overdue"
	AgendaToday   = "today"
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
	CaptureID string `json:"capture_id,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	ID        int    `json:"id"`
	Op        string `json:"op"`
	Project   string `json:"project,omitempty"`
	Task      string `json:"task,omitempty"`
	Text      string `json:"text,omitempty"`
	Minutes   int    `json:"minutes,omitempty"`
}

// Reply answers the Request whose ID it carries. Message is what Docket said: the
// outcome on success, the refusal otherwise. Completion answers OpComplete
// alone, and is absent when the caret stands on no sigil the grammar completes.
// Preview answers OpPreview with the full draft's human-readable route and date.
// Transaction answers a successful OpCapture with the committed transaction's
// ID, the one OpUndo names.
type Reply struct {
	Version     string         `json:"version,omitempty"`
	Workspace   string         `json:"workspace,omitempty"`
	ID          int            `json:"reply"`
	OK          bool           `json:"ok"`
	Message     string         `json:"message,omitempty"`
	Completion  *Completion    `json:"completion,omitempty"`
	Preview     string         `json:"preview,omitempty"`
	Transaction string         `json:"transaction,omitempty"`
	Selection   *TextSelection `json:"selection,omitempty"`
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
	// that action only where Docket would accept it.
	Reviewable bool `json:"reviewable,omitempty"`
}

// Agenda is the tasks the pulse counts as Today or Overdue: overdue first,
// oldest deadline leading, then Today by priority. Items holds at most a
// bounded number of them; Total is how many there are.
type Agenda struct {
	Items []AgendaItem `json:"items"`
	Total int          `json:"total"`
}

// AgendaItem is one agenda task. Project and Task are its stored IDs, as an
// Arrival's are. Due is RFC 3339 and absent without a deadline; DueHasTime says
// it names an instant rather than a whole day. Priority is 1–3, or absent.
type AgendaItem struct {
	Kind        string `json:"kind"`
	Project     string `json:"project"`
	Task        string `json:"task"`
	Title       string `json:"title"`
	ProjectName string `json:"project_name"`
	Priority    int    `json:"priority,omitempty"`
	Due         string `json:"due,omitempty"`
	DueHasTime  bool   `json:"due_has_time,omitempty"`
	// Reviewable is as an Arrival's is.
	Reviewable bool `json:"reviewable,omitempty"`
}

// Schedule is the arrivals EventArrivals will report later, earliest first and
// at most a bounded number of them, as the workspace stands now. A newer
// schedule replaces the previous one whole: an arrival it no longer names will
// not happen.
type Schedule struct {
	Items []ScheduledArrival `json:"items"`
}

// ScheduledArrival is one Arrival that has not happened yet. At is the RFC
// 3339 instant it will; Reviewable is read at that instant.
type ScheduledArrival struct {
	Arrival
	At string `json:"at"`
}

// Event is one line Docket writes unprompted. Title and Body word an arrivals
// event as one notification, the way --notify would.
type Event struct {
	Event    string    `json:"event"`
	Editing  *Editing  `json:"editing,omitempty"`
	Pulse    *Pulse    `json:"pulse,omitempty"`
	Arrivals []Arrival `json:"arrivals,omitempty"`
	Agenda   *Agenda   `json:"agenda,omitempty"`
	Schedule *Schedule `json:"schedule,omitempty"`
	Title    string    `json:"title,omitempty"`
	Body     string    `json:"body,omitempty"`
}

// TextSelection addresses a refused capture in UTF-16 code units, the native
// text field's indexing. It refers to the exact submitted draft.
type TextSelection struct {
	Start  int `json:"start"`
	Length int `json:"length"`
}

// Editing describes native menu availability; blank labels mean unavailable.
type Editing struct {
	Undo   string `json:"undo"`
	Redo   string `json:"redo"`
	Search bool   `json:"search"`
}
