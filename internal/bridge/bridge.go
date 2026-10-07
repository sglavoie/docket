package bridge

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"runtime/debug"
	"sync"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/logging"
)

// Options are what a host injects beyond the request stream. The mobile
// binding passes none; tests pin the clock and identity generator here, which
// is the contract's "same configuration struct on the Go side".
type Options struct {
	Clock app.Clock
	IDs   app.IDGenerator
}

// Bridge serves one host. It holds at most one open workspace and answers one
// request at a time, so a host may call it from any thread.
type Bridge struct {
	mu        sync.Mutex
	options   Options
	workspace *openWorkspace
}

// openWorkspace is the runtime a workspace.open produced and the replay
// memory of its committed requests.
type openWorkspace struct {
	runtime  *engine.Runtime
	id       string
	deviceID string
	// runtimeDir is the device's machine-local directory, the one place an
	// attachment's staged bytes may be handed over from.
	runtimeDir string
	replay     *replayCache
}

// New returns a bridge with no workspace open.
func New(options Options) *Bridge {
	return &Bridge{options: options}
}

// op is one operation's handler and the envelope rules it is served under.
type op struct {
	// mutates marks a command: its expect is checked, its success replayed.
	mutates bool
	// opens marks workspace.open, the one op that names no workspace.
	opens bool
	run   func(b *Bridge, request Request) (any, error)
}

// operations are the ops this engine build serves.
var operations = map[string]op{
	"workspace.open":        {opens: true, run: (*Bridge).workspaceOpen},
	"workspace.join":        {opens: true, run: (*Bridge).workspaceJoin},
	"workspace.status":      {run: (*Bridge).workspaceStatus},
	"workspace.close":       {run: (*Bridge).workspaceClose},
	"query.projects":        {run: (*Bridge).queryProjects},
	"query.tasks":           {run: (*Bridge).queryTasks},
	"query.task":            {run: (*Bridge).queryTask},
	"query.lenses":          {run: (*Bridge).queryLenses},
	"query.capture_preview": {run: (*Bridge).queryCapturePreview},
	"query.project":         {run: (*Bridge).queryProject},
	"command.capture":       {mutates: true, run: (*Bridge).commandCapture},
	"command.edit":          {mutates: true, run: (*Bridge).commandEdit},
	"command.transition":    {mutates: true, run: (*Bridge).commandTransition},
	"command.plan":          {mutates: true, run: (*Bridge).commandPlan},
	"command.checklist":     {mutates: true, run: (*Bridge).commandChecklist},
	"command.check_in":      {mutates: true, run: (*Bridge).commandCheckIn},
	"command.hierarchy":     {mutates: true, run: (*Bridge).commandHierarchy},
	"command.bulk":          {mutates: true, run: (*Bridge).commandBulk},
	"command.project":       {mutates: true, run: (*Bridge).commandProject},
	"query.notes":           {run: (*Bridge).queryNotes},
	"query.note":            {run: (*Bridge).queryNote},
	"command.comment":       {mutates: true, run: (*Bridge).commandComment},
	"command.note":          {mutates: true, run: (*Bridge).commandNote},
	"command.attachment":    {mutates: true, run: (*Bridge).commandAttachment},
	"sync.status":           {run: (*Bridge).syncStatus},
	"sync.export_pending":   {run: (*Bridge).syncExportPending},
	"sync.import":           {mutates: true, run: (*Bridge).syncImport},
	"sync.acknowledge":      {mutates: true, run: (*Bridge).syncAcknowledge},
}

// reservedOperations are part of bridge version 1 but not yet served. They
// answer unknown_op with details.reserved, so a client can tell an op this
// build lacks from one the protocol never had.
var reservedOperations = map[string]bool{
	"command.undo": true, "command.redo": true,
	"conflict.list": true, "conflict.show": true, "conflict.resolve": true,
}

// Call answers one JSON request with one JSON response. It never panics and
// never returns an empty string.
func (b *Bridge) Call(raw string) (response string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var request Request
	defer func() {
		if recovered := recover(); recovered != nil {
			logging.Warn("bridge: panic serving %s (request %s): %v\n%s", request.Op, request.RequestID, recovered, debug.Stack())
			response = encodeResponse(request, nil, newError(CodeInternal, "the engine failed while serving %s", request.Op))
		}
	}()
	result, err := b.serve(raw, &request)
	return encodeResponse(request, result, err)
}

func (b *Bridge) serve(raw string, request *Request) (any, error) {
	if err := decodeStrict([]byte(raw), request); err != nil {
		// Echo whatever identity a lenient read can recover, so a client can still
		// correlate the refusal.
		var echo struct {
			RequestID string `json:"request_id"`
		}
		_ = json.Unmarshal([]byte(raw), &echo)
		*request = Request{RequestID: echo.RequestID}
		return nil, newError(CodeInvalidRequest, "request: %v", err)
	}
	switch {
	case request.BridgeVersion < 1:
		return nil, newError(CodeInvalidRequest, "bridge_version is required")
	case request.BridgeVersion > Version:
		return nil, newError(CodeUnsupportedVersion, "bridge_version %d is newer than this engine's %d", request.BridgeVersion, Version).
			with("supported", Version)
	case request.RequestID == "":
		return nil, newError(CodeInvalidRequest, "request_id is required")
	case request.Op == "":
		return nil, newError(CodeInvalidRequest, "op is required")
	}
	handler, ok := operations[request.Op]
	if !ok {
		refusal := newError(CodeUnknownOp, "%q is not an operation of bridge version %d", request.Op, Version)
		if reservedOperations[request.Op] {
			refusal = newError(CodeUnknownOp, "%q is part of bridge version %d but this engine build does not serve it yet", request.Op, Version).
				with("reserved", true)
		}
		return nil, refusal
	}
	if !handler.opens {
		if b.workspace == nil {
			return nil, newError(CodeNotOpen, "no workspace is open")
		}
		if request.Workspace == "" {
			return nil, newError(CodeInvalidRequest, "workspace is required")
		}
		if request.Workspace != b.workspace.id {
			return nil, newError(CodeWorkspaceMismatch, "workspace %s is not the open workspace", request.Workspace).
				with("open_workspace", b.workspace.id)
		}
	} else if request.Workspace != "" {
		return nil, newError(CodeInvalidRequest, "workspace.open names no workspace")
	}
	if request.Expect != nil && !handler.mutates {
		return nil, newError(CodeInvalidRequest, "%s takes no expect", request.Op)
	}
	if !handler.mutates {
		result, err := handler.run(b, *request)
		return result, classify(err)
	}
	return b.serveMutation(handler, *request)
}

// serveMutation replays a request id it already committed, then checks the
// preconditions and runs the command.
func (b *Bridge) serveMutation(handler op, request Request) (any, error) {
	fingerprint := requestFingerprint(request)
	workspace := b.workspace
	if previous, seen := workspace.replay.lookup(request.RequestID); seen {
		if previous.fingerprint != fingerprint {
			return nil, newError(CodeInvalidRequest, "request_id %s was already used for a different request", request.RequestID)
		}
		return previous.result, nil
	}
	if err := checkExpect(workspace.runtime, request.Expect); err != nil {
		return nil, classify(err)
	}
	result, err := handler.run(b, request)
	if err != nil {
		return nil, classify(err)
	}
	workspace.replay.remember(request.RequestID, fingerprint, result)
	return result, nil
}

func requestFingerprint(request Request) [sha256.Size]byte {
	canonical, _ := json.Marshal(struct {
		Op      string          `json:"op"`
		Expect  *Expect         `json:"expect"`
		Payload json.RawMessage `json:"payload"`
	}{request.Op, request.Expect, compactJSON(request.Payload)})
	return sha256.Sum256(canonical)
}

// compactJSON drops insignificant whitespace so a replay is recognized however
// the client spaced it.
func compactJSON(raw json.RawMessage) json.RawMessage {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return raw
	}
	compact, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return compact
}

func encodeResponse(request Request, result any, err error) string {
	response := Response{BridgeVersion: Version, RequestID: request.RequestID}
	if err != nil {
		var failure *Error
		if !errors.As(classify(err), &failure) {
			failure = newError(CodeInternal, "%s", err)
		}
		response.Error = failure
	} else {
		response.OK = true
		if result == nil {
			result = struct{}{}
		}
		response.Result = result
	}
	data, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		fallback, _ := json.Marshal(Response{
			BridgeVersion: Version,
			RequestID:     request.RequestID,
			Error:         newError(CodeInternal, "the engine could not encode its response: %v", marshalErr),
		})
		return string(fallback)
	}
	return string(data)
}

// replayLimit bounds how many committed request ids a session remembers.
const replayLimit = 256

type replayEntry struct {
	fingerprint [sha256.Size]byte
	result      any
}

// replayCache remembers committed requests by id, oldest evicted first.
type replayCache struct {
	entries map[string]replayEntry
	order   []string
}

func newReplayCache() *replayCache {
	return &replayCache{entries: make(map[string]replayEntry)}
}

func (c *replayCache) lookup(id string) (replayEntry, bool) {
	entry, ok := c.entries[id]
	return entry, ok
}

func (c *replayCache) remember(id string, fingerprint [sha256.Size]byte, result any) {
	if len(c.order) == replayLimit {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[id] = replayEntry{fingerprint: fingerprint, result: result}
	c.order = append(c.order, id)
}
