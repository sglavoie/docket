package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// This file defines the file-based scenario format agents and CI drive the TUI
// with. A scenario is validated strictly before any action runs (unknown action
// types, malformed sizes, invalid keys, and empty scenarios all fail up front),
// then executed through the driver into a stable structured result shape. The
// hidden `tt tui-test run` CLI (session 5) wires these helpers to stdin/stdout.

const (
	// defaultScenarioFixture backs a scenario that omits `fixture`.
	defaultScenarioFixture = FixtureRich
	// defaultScenarioWidth / defaultScenarioHeight back a scenario that omits
	// `size`. 100x30 is a comfortable review size that renders every view.
	defaultScenarioWidth  = 100
	defaultScenarioHeight = 30
)

// uitestAssertKinds is the set of assertion kinds runAssert understands. It is
// the single source of truth for scenario validation; keep it in sync with the
// switch in runAssert.
var uitestAssertKinds = map[string]bool{
	"contains":         true,
	"not_contains":     true,
	"count_contains":   true,
	"view":             true,
	"no_layout_issues": true,
	"status_contains":  true,
}

// UITestScenarioResult is the structured outcome of running a scenario. It is
// the JSON contract returned by `tt tui-test run`.
type UITestScenarioResult struct {
	// OK is true when every action ran without failing.
	OK bool `json:"ok"`
	// Scenario echoes the scenario name (may be empty).
	Scenario string `json:"scenario"`
	// ActionsRun counts the actions that ran successfully before the result was
	// produced (all of them on success; the count before the failure on error).
	ActionsRun int `json:"actions_run"`
	// FinalObservation is the screen state after the last successful action, or
	// the failure screen when an action failed. Nil only when zero actions ran.
	FinalObservation *UITestObservation `json:"final_observation,omitempty"`
	// Observations is the per-action observation list, included only when the
	// caller requests it (large payload).
	Observations []UITestObservation `json:"observations,omitempty"`
	// Error describes the failing action when OK is false.
	Error *UITestActionError `json:"error,omitempty"`
}

// RunScenarioOptions tunes scenario execution and result verbosity. The zero
// value is usable: empty defaults fall back to the package defaults.
type RunScenarioOptions struct {
	// DefaultFixture backs a scenario that omits `fixture`.
	DefaultFixture string
	// DefaultWidth / DefaultHeight back a scenario that omits `size`.
	DefaultWidth  int
	DefaultHeight int
	// IncludeObservations attaches the per-action observation list to the result.
	IncludeObservations bool
	// IncludeANSI attaches the raw styled screen to each observation.
	IncludeANSI bool
	// DrainTimeout overrides the per-command drain budget when positive. The CLI
	// wires this from --drain-timeout; zero keeps the driver default.
	DrainTimeout time.Duration
}

func (o RunScenarioOptions) fixtureOr() string {
	if strings.TrimSpace(o.DefaultFixture) != "" {
		return o.DefaultFixture
	}
	return defaultScenarioFixture
}

func (o RunScenarioOptions) sizeOr() (int, int) {
	w, h := o.DefaultWidth, o.DefaultHeight
	if w <= 0 {
		w = defaultScenarioWidth
	}
	if h <= 0 {
		h = defaultScenarioHeight
	}
	return w, h
}

// ParseScenario decodes a scenario from JSON bytes. Decoding is strict: unknown
// fields are rejected so field-name typos surface immediately. It does not run
// full semantic validation — call Validate for that.
func ParseScenario(data []byte) (UITestScenario, error) {
	var sc UITestScenario
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		return UITestScenario{}, fmt.Errorf("failed to parse scenario JSON: %w", err)
	}
	if dec.More() {
		return UITestScenario{}, fmt.Errorf("scenario JSON has trailing content after the first object")
	}
	return sc, nil
}

// LoadScenario reads and parses a scenario JSON file.
func LoadScenario(path string) (UITestScenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UITestScenario{}, fmt.Errorf("failed to read scenario file %q: %w", path, err)
	}
	return ParseScenario(data)
}

// Validate checks the scenario is well-formed before any action runs. It fails
// on empty scenarios, malformed sizes, unknown action types, invalid keys, and
// unknown assertion kinds.
func (sc UITestScenario) Validate() error {
	if len(sc.Actions) == 0 {
		return errors.New("scenario has no actions")
	}
	if strings.TrimSpace(sc.Size) != "" {
		if _, _, err := parseSize(sc.Size); err != nil {
			return err
		}
	}
	for i, a := range sc.Actions {
		if err := a.validate(); err != nil {
			return fmt.Errorf("action %d (%s): %w", i, a.Type, err)
		}
	}
	return nil
}

// validate checks a single action for well-formedness. Feature support that
// arrives in later sessions (command execution) is validated only for shape.
func (a UITestAction) validate() error {
	switch a.Type {
	case UITestActionKey:
		if _, err := parseUITestKey(a.Key); err != nil {
			return err
		}
	case UITestActionText:
		if a.Text == "" {
			return errors.New("text action has empty text")
		}
	case UITestActionResize:
		if a.Width <= 0 || a.Height <= 0 {
			return fmt.Errorf("resize needs positive width and height, got %dx%d", a.Width, a.Height)
		}
	case UITestActionWait:
		if _, err := a.waitDuration(); err != nil {
			return err
		}
	case UITestActionRestart:
		// Nothing to validate.
	case UITestActionSwitchMachine:
		// The machine name is optional, but a name of only whitespace is an
		// authoring slip that would silently read as "no name" and land on a fresh
		// home instead of the machine the scenario meant.
		if a.Machine != "" && strings.TrimSpace(a.Machine) == "" {
			return errors.New("switch_machine machine name is blank")
		}
	case UITestActionCommand:
		if strings.TrimSpace(a.Command) == "" {
			return errors.New("command action has empty command")
		}
	case UITestActionAssert:
		if !uitestAssertKinds[a.Assert] {
			return fmt.Errorf("unknown or missing assertion kind %q", a.Assert)
		}
		if a.Assert == "count_contains" {
			if a.Value == "" {
				return errors.New("count_contains assertion has empty value")
			}
			if a.Count <= 0 {
				return fmt.Errorf("count_contains assertion needs a positive count, got %d", a.Count)
			}
		}
	default:
		return fmt.Errorf("unknown action type %q", a.Type)
	}
	return nil
}

// waitDuration returns how far a wait action advances the virtual clock. An
// omitted duration keeps the short default every existing scenario relies on, and
// a non-positive one is an authoring slip rather than a way to spend no time:
// a wait that advances nothing is a no-op the author did not intend.
func (a UITestAction) waitDuration() (time.Duration, error) {
	if strings.TrimSpace(a.Duration) == "" {
		return defaultWaitTimeout, nil
	}
	span, err := time.ParseDuration(a.Duration)
	if err != nil {
		return 0, fmt.Errorf("wait duration %q: %w", a.Duration, err)
	}
	if span <= 0 {
		return 0, fmt.Errorf("wait duration %q must be positive", a.Duration)
	}
	return span, nil
}

// ResolveSize returns the scenario's initial terminal size, falling back to the
// provided defaults when `size` is empty.
func (sc UITestScenario) ResolveSize(defWidth, defHeight int) (int, int, error) {
	if strings.TrimSpace(sc.Size) == "" {
		return defWidth, defHeight, nil
	}
	return parseSize(sc.Size)
}

// resolveFixture returns the scenario's fixture, falling back to def.
func (sc UITestScenario) resolveFixture(def string) string {
	if strings.TrimSpace(sc.Fixture) != "" {
		return sc.Fixture
	}
	return def
}

// parseSize parses a "WxH" terminal size (e.g. "100x30"). Both dimensions must
// be positive integers.
func parseSize(s string) (int, int, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("malformed size %q (want WxH, e.g. 100x30)", s)
	}
	w, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("malformed size %q: width must be a positive integer", s)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("malformed size %q: height must be a positive integer", s)
	}
	return w, h, nil
}

// RunScenarioResult drives an already-built driver through the scenario and
// packages the outcome into a UITestScenarioResult. It assumes the scenario has
// already been validated. Both fixture and copied-config callers reuse it, so
// it never constructs or owns a workspace.
func (d *UITestDriver) RunScenarioResult(sc UITestScenario, opts RunScenarioOptions) UITestScenarioResult {
	if opts.DrainTimeout > 0 {
		d.SetDrainTimeout(opts.DrainTimeout)
	}
	observations, err := d.RunScenario(sc, ObserveOptions{IncludeANSI: opts.IncludeANSI})

	result := UITestScenarioResult{
		OK:         err == nil,
		Scenario:   sc.Name,
		ActionsRun: len(observations),
	}
	if opts.IncludeObservations {
		result.Observations = observations
	}

	var actErr *UITestActionError
	if errors.As(err, &actErr) {
		result.Error = actErr
		obs := actErr.Observation
		result.FinalObservation = &obs
	} else if len(observations) > 0 {
		final := observations[len(observations)-1]
		result.FinalObservation = &final
	}
	return result
}

// RunScenarioFixture validates the scenario, resolves its fixture and size
// (falling back to opts/package defaults), builds an isolated fixture-backed
// driver, runs the scenario, and returns the structured result plus the owning
// workspace. The caller must Cleanup the returned workspace when it is non-nil.
// Validation and construction failures return a nil workspace and an error.
func RunScenarioFixture(sc UITestScenario, opts RunScenarioOptions) (UITestScenarioResult, *uitestWorkspace, error) {
	if err := sc.Validate(); err != nil {
		return UITestScenarioResult{}, nil, err
	}

	defWidth, defHeight := opts.sizeOr()
	width, height, err := sc.ResolveSize(defWidth, defHeight)
	if err != nil {
		return UITestScenarioResult{}, nil, err
	}

	driver, ws, err := NewFixtureDriver(sc.resolveFixture(opts.fixtureOr()), width, height)
	if err != nil {
		return UITestScenarioResult{}, nil, err
	}

	return driver.RunScenarioResult(sc, opts), ws, nil
}
