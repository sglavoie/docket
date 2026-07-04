package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/tui"
)

// This file wires the hidden `tt tui-test` command group: a repo-native surface
// for LLM-driven review of the Bubble Tea TUI. It drives the AppModel through
// the internal/tui review harness against isolated fixture or copied-config
// data and returns normalized JSON observations.
//
// The command group is hidden so it never pollutes normal `tt --help` output or
// the generated CLI reference (scripts/gen-api-ref skips Hidden commands).
//
// Exit codes:
//   0  scenario ran and every action/assertion passed
//   1  a scenario action or assertion failed
//   2  invalid CLI input or an invalid/unparseable scenario

var (
	tuiTestFixture       string
	tuiTestSize          string
	tuiTestScenario      string
	tuiTestANSI          bool
	tuiTestKeepWorkspace bool
	tuiTestDrainTimeout  time.Duration
	tuiTestObservations  bool

	// session-mode flags use their own variables so they never share Changed
	// state with the run command's flags.
	tuiTestSessionFixture string
	tuiTestSessionSize    string
	tuiTestSessionANSI    bool
)

var tuiTestCmd = &cobra.Command{
	Use:    "tui-test",
	Short:  "LLM-driven TUI review harness (hidden)",
	Hidden: true,
	Long: `tui-test drives the interactive TUI programmatically for automated review.

It runs the app against isolated fixture data (or a copy of real config/data)
and returns normalized JSON observations. Intended for LLM-driven review and CI,
not day-to-day use.`,
}

var tuiTestFixturesCmd = &cobra.Command{
	Use:   "fixtures",
	Short: "List built-in review fixtures",
	Args:  cobra.NoArgs,
	RunE:  runTUITestFixtures,
}

var tuiTestRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a scenario file and print the result",
	Args:  cobra.NoArgs,
	RunE:  runTUITestRun,
}

var tuiTestSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Interactive NDJSON control loop over stdin/stdout",
	Long: `session reads one JSON request per line from stdin and writes one JSON
response per line to stdout, keeping a single driver alive so state persists
across requests.

Request:  {"id":"...","action":{...}}  or  {"id":"...","command":"observe|reset|quit"}
Response: {"id":"...","ok":true,"observation":{...}}  or  {"id":"...","ok":false,"error":{...}}

Stdout is pure NDJSON: logs, prompts, and human text go to stderr only.`,
	Args: cobra.NoArgs,
	RunE: runTUITestSession,
}

func init() {
	rootCmd.AddCommand(tuiTestCmd)
	tuiTestCmd.AddCommand(tuiTestFixturesCmd)
	tuiTestCmd.AddCommand(tuiTestRunCmd)
	tuiTestCmd.AddCommand(tuiTestSessionCmd)

	tuiTestFixturesCmd.Flags().Bool("json", false, "output result as JSON")

	tuiTestRunCmd.Flags().StringVar(&tuiTestFixture, "fixture", tui.FixtureRich, "built-in fixture to load (mutually exclusive with --config)")
	tuiTestRunCmd.Flags().StringVar(&tuiTestSize, "size", "100x30", "initial terminal size WxH (overrides the scenario's size)")
	tuiTestRunCmd.Flags().StringVar(&tuiTestScenario, "scenario", "", "path to the scenario JSON file (required)")
	tuiTestRunCmd.Flags().Bool("json", false, "output result as JSON")
	tuiTestRunCmd.Flags().BoolVar(&tuiTestANSI, "ansi", false, "include the raw styled (ANSI) screen in observations")
	tuiTestRunCmd.Flags().BoolVar(&tuiTestKeepWorkspace, "keep-workspace", false, "keep the temp workspace and report its path")
	tuiTestRunCmd.Flags().DurationVar(&tuiTestDrainTimeout, "drain-timeout", 0, "per-command async drain budget (0 uses the default)")
	tuiTestRunCmd.Flags().BoolVar(&tuiTestObservations, "observations", false, "include the per-action observation list in the result")

	tuiTestSessionCmd.Flags().StringVar(&tuiTestSessionFixture, "fixture", tui.FixtureRich, "built-in fixture to load (mutually exclusive with --config)")
	tuiTestSessionCmd.Flags().StringVar(&tuiTestSessionSize, "size", "100x30", "initial terminal size WxH")
	tuiTestSessionCmd.Flags().BoolVar(&tuiTestSessionANSI, "ansi", false, "include the raw styled (ANSI) screen in observations")
}

// invalidInput wraps err as an exit-code-2 (invalid CLI input / invalid
// scenario) failure.
func invalidInput(err error) error {
	return &ExitCodeError{Code: 2, Err: err}
}

func runTUITestFixtures(cmd *cobra.Command, _ []string) error {
	names := tui.BuiltinFixtureNames()

	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return printJSON(map[string]any{"fixtures": names})
	}
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}

func runTUITestRun(cmd *cobra.Command, _ []string) error {
	if strings.TrimSpace(tuiTestScenario) == "" {
		return invalidInput(errors.New("--scenario is required"))
	}

	// --size always parses (its default "100x30" is valid); it backs scenarios
	// that omit a size and, when explicitly set, overrides the scenario size.
	defWidth, defHeight, err := tui.ParseSize(tuiTestSize)
	if err != nil {
		return invalidInput(err)
	}

	sc, err := tui.LoadScenario(tuiTestScenario)
	if err != nil {
		return invalidInput(err)
	}
	if cmd.Flags().Changed("size") {
		sc.Size = tuiTestSize
	}

	sourceConfig := GetConfigPath()
	useConfig := strings.TrimSpace(sourceConfig) != ""
	if useConfig && cmd.Flags().Changed("fixture") {
		return invalidInput(errors.New("--fixture and --config are mutually exclusive"))
	}

	opts := tui.RunScenarioOptions{
		DefaultFixture:      tuiTestFixture,
		DefaultWidth:        defWidth,
		DefaultHeight:       defHeight,
		IncludeObservations: tuiTestObservations,
		IncludeANSI:         tuiTestANSI,
		DrainTimeout:        tuiTestDrainTimeout,
	}

	var result tui.UITestScenarioResult
	var workspaceRoot string

	if useConfig {
		// Copied-config mode: copy the real config/data into a temp workspace and
		// drive the app there. The source is never mutated.
		if err := sc.Validate(); err != nil {
			return invalidInput(err)
		}
		width, height, err := sc.ResolveSize(defWidth, defHeight)
		if err != nil {
			return invalidInput(err)
		}
		driver, ws, err := tui.NewCopiedWorkspaceDriver(sourceConfig, width, height)
		if err != nil {
			return invalidInput(err)
		}
		defer func() {
			if !tuiTestKeepWorkspace {
				_ = ws.Cleanup()
			}
		}()
		workspaceRoot = ws.Root
		result = driver.RunScenarioResult(sc, opts)
	} else {
		// Fixture mode: build an isolated workspace from a built-in fixture.
		if cmd.Flags().Changed("fixture") {
			sc.Fixture = tuiTestFixture
		}
		res, ws, err := tui.RunScenarioFixture(sc, opts)
		if err != nil {
			return invalidInput(err)
		}
		defer func() {
			if !tuiTestKeepWorkspace {
				_ = ws.Cleanup()
			}
		}()
		workspaceRoot = ws.Root
		result = res
	}

	if tuiTestKeepWorkspace && workspaceRoot != "" {
		// Reported on stderr so stdout stays a clean result payload.
		fmt.Fprintf(cmd.ErrOrStderr(), "kept workspace: %s\n", workspaceRoot)
	}

	if err := printTUITestResult(cmd, result); err != nil {
		return err
	}

	if !result.OK {
		// The result was already printed; signal failure without a stderr line.
		return &ExitCodeError{Code: 1}
	}
	return nil
}

func printTUITestResult(cmd *cobra.Command, result tui.UITestScenarioResult) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return printJSON(result)
	}

	if result.OK {
		fmt.Printf("ok: scenario %q passed (%d actions)\n", result.Scenario, result.ActionsRun)
		return nil
	}
	fmt.Printf("FAIL: scenario %q failed after %d action(s)\n", result.Scenario, result.ActionsRun)
	if result.Error != nil {
		fmt.Printf("  %s\n", result.Error.Error())
	}
	return nil
}

// wsCleanup is the exported surface of the internal fixture/copied-config
// workspace. The concrete type is unexported in internal/tui, so the session
// loop holds it through this interface to rebuild the driver on `reset`.
type wsCleanup interface {
	Cleanup() error
}

// sessionRequest is one NDJSON line the interactive session mode reads. A
// request carries either a driver action or a session command, never both.
type sessionRequest struct {
	ID      string            `json:"id"`
	Command string            `json:"command,omitempty"`
	Action  *tui.UITestAction `json:"action,omitempty"`
}

// sessionResponse is the one-line NDJSON reply written for each request.
type sessionResponse struct {
	ID          string                 `json:"id"`
	OK          bool                   `json:"ok"`
	Observation *tui.UITestObservation `json:"observation,omitempty"`
	Error       *sessionError          `json:"error,omitempty"`
}

// sessionError is the structured failure detail attached to a non-ok response.
type sessionError struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// Session error kinds. These are stable strings a caller can branch on.
const (
	sessionErrBadRequest     = "bad_request"
	sessionErrUnknownCommand = "unknown_command"
	sessionErrResetFailed    = "reset_failed"
	sessionErrActionFailed   = "action_failed"
)

func runTUITestSession(cmd *cobra.Command, _ []string) error {
	width, height, err := tui.ParseSize(tuiTestSessionSize)
	if err != nil {
		return invalidInput(err)
	}

	sourceConfig := GetConfigPath()
	useConfig := strings.TrimSpace(sourceConfig) != ""
	if useConfig && cmd.Flags().Changed("fixture") {
		return invalidInput(errors.New("--fixture and --config are mutually exclusive"))
	}

	// buildDriver constructs a fresh driver+workspace from the same fixture or
	// copied config and size. It backs both the initial build and `reset`.
	buildDriver := func() (*tui.UITestDriver, wsCleanup, error) {
		if useConfig {
			driver, ws, err := tui.NewCopiedWorkspaceDriver(sourceConfig, width, height)
			if err != nil {
				return nil, nil, err
			}
			return driver, ws, nil
		}
		driver, ws, err := tui.NewFixtureDriver(tuiTestSessionFixture, width, height)
		if err != nil {
			return nil, nil, err
		}
		return driver, ws, nil
	}

	driver, ws, err := buildDriver()
	if err != nil {
		return invalidInput(err)
	}
	defer func() { _ = ws.Cleanup() }()

	out := cmd.OutOrStdout()
	observeOpts := tui.ObserveOptions{IncludeANSI: tuiTestSessionANSI}

	scanner := bufio.NewScanner(cmd.InOrStdin())
	// Allow large request lines (a resize burst or long text action) without
	// tripping bufio.Scanner's default 64 KiB token limit.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		req, perr := parseSessionRequest(line)
		if perr != nil {
			writeSessionResponse(out, sessionResponse{
				ID:    req.ID,
				OK:    false,
				Error: &sessionError{Kind: sessionErrBadRequest, Reason: perr.Error()},
			})
			continue
		}

		hasCommand := strings.TrimSpace(req.Command) != ""
		if hasCommand && req.Action != nil {
			writeSessionResponse(out, sessionResponse{
				ID:    req.ID,
				OK:    false,
				Error: &sessionError{Kind: sessionErrBadRequest, Reason: "request has both an action and a command"},
			})
			continue
		}

		if hasCommand {
			switch req.Command {
			case "observe":
				obs := driver.Observe(observeOpts)
				writeSessionResponse(out, sessionResponse{ID: req.ID, OK: true, Observation: &obs})
			case "reset":
				newDriver, newWS, berr := buildDriver()
				if berr != nil {
					// Keep the existing driver alive; the reset simply failed.
					writeSessionResponse(out, sessionResponse{
						ID:    req.ID,
						OK:    false,
						Error: &sessionError{Kind: sessionErrResetFailed, Reason: berr.Error()},
					})
					continue
				}
				_ = ws.Cleanup()
				driver, ws = newDriver, newWS
				obs := driver.Observe(observeOpts)
				writeSessionResponse(out, sessionResponse{ID: req.ID, OK: true, Observation: &obs})
			case "quit":
				obs := driver.Observe(observeOpts)
				writeSessionResponse(out, sessionResponse{ID: req.ID, OK: true, Observation: &obs})
				return nil
			default:
				writeSessionResponse(out, sessionResponse{
					ID:    req.ID,
					OK:    false,
					Error: &sessionError{Kind: sessionErrUnknownCommand, Reason: fmt.Sprintf("unknown command %q", req.Command)},
				})
			}
			continue
		}

		if req.Action == nil {
			writeSessionResponse(out, sessionResponse{
				ID:    req.ID,
				OK:    false,
				Error: &sessionError{Kind: sessionErrBadRequest, Reason: "request has neither an action nor a command"},
			})
			continue
		}

		obs, aerr := driver.ApplyAction(*req.Action, observeOpts)
		if aerr != nil {
			// The observation is the failure screen; return it alongside the error
			// so the caller still sees where the action left the app.
			writeSessionResponse(out, sessionResponse{
				ID:          req.ID,
				OK:          false,
				Observation: &obs,
				Error:       &sessionError{Kind: sessionErrActionFailed, Reason: aerr.Error()},
			})
			continue
		}
		writeSessionResponse(out, sessionResponse{ID: req.ID, OK: true, Observation: &obs})
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read session input: %w", err)
	}
	return nil
}

// parseSessionRequest strictly decodes one NDJSON request line. Unknown fields
// are rejected so field-name typos surface immediately, matching ParseScenario.
func parseSessionRequest(line []byte) (sessionRequest, error) {
	var req sessionRequest
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, err
	}
	if dec.More() {
		return req, errors.New("request line has trailing content after the first object")
	}
	return req, nil
}

// writeSessionResponse marshals resp as a single compact JSON line. Stdout stays
// pure NDJSON, so a marshal failure degrades to a minimal machine-readable line
// rather than human text.
func writeSessionResponse(w io.Writer, resp sessionResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		data = []byte(`{"ok":false,"error":{"kind":"internal","reason":"failed to marshal response"}}`)
	}
	fmt.Fprintf(w, "%s\n", data)
}
