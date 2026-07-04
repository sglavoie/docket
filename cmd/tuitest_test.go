package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui"
)

// captureTUITestOutput runs fn with os.Stdout/os.Stderr redirected to pipes and
// returns what each captured plus fn's error.
func captureTUITestOutput(t *testing.T, fn func() error) (stdout, stderr string, runErr error) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	// Drain both pipes concurrently while fn runs: a large observation dump can
	// exceed the OS pipe buffer (~64KB), and if nothing reads the pipe until fn
	// returns the writing side blocks forever (deadlock).
	var outBuf, errBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(&outBuf, stdoutR) }()
	go func() { defer wg.Done(); _, _ = io.Copy(&errBuf, stderrR) }()

	os.Stdout, os.Stderr = stdoutW, stderrW
	runErr = fn()
	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	wg.Wait()
	return outBuf.String(), errBuf.String(), runErr
}

// resetTUITestRunFlags restores the run command's flags to their declared
// defaults with Changed cleared, so each test starts from a known state and can
// opt into individual flags via Flags().Set.
func resetTUITestRunFlags(t *testing.T) {
	t.Helper()
	fs := tuiTestRunCmd.Flags()
	defaults := map[string]string{
		"fixture":        tui.FixtureRich,
		"size":           "100x30",
		"scenario":       "",
		"json":           "false",
		"ansi":           "false",
		"keep-workspace": "false",
		"drain-timeout":  "0s",
		"observations":   "false",
	}
	for name, val := range defaults {
		if err := fs.Set(name, val); err != nil {
			t.Fatalf("reset flag %q: %v", name, err)
		}
	}
	fs.VisitAll(func(f *pflag.Flag) { f.Changed = false })
	viper.Set("config", "")
	t.Cleanup(func() { viper.Set("config", "") })
}

// useFileSystemStorage forces real on-disk storage so fixture builders and the
// copied-config path round-trip through tasks.md files.
func useFileSystemStorage(t *testing.T) {
	t.Helper()
	old := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	t.Cleanup(func() { storage.DefaultRepository = old })
}

func writeScenario(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write scenario: %v", err)
	}
	return path
}

func exitCode(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ec *ExitCodeError
	if errors.As(err, &ec) {
		return ec.Code
	}
	t.Fatalf("error is not an *ExitCodeError: %v", err)
	return -1
}

func TestTUITestFixturesJSON(t *testing.T) {
	if err := tuiTestFixturesCmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("set json: %v", err)
	}
	t.Cleanup(func() { _ = tuiTestFixturesCmd.Flags().Set("json", "false") })

	stdout, _, err := captureTUITestOutput(t, func() error {
		return runTUITestFixtures(tuiTestFixturesCmd, nil)
	})
	if err != nil {
		t.Fatalf("fixtures --json: %v", err)
	}

	var payload struct {
		Fixtures []string `json:"fixtures"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal fixtures JSON: %v\noutput: %s", err, stdout)
	}
	want := tui.BuiltinFixtureNames()
	if strings.Join(payload.Fixtures, ",") != strings.Join(want, ",") {
		t.Fatalf("fixtures = %v, want %v", payload.Fixtures, want)
	}
}

func TestTUITestFixturesPlain(t *testing.T) {
	_ = tuiTestFixturesCmd.Flags().Set("json", "false")

	stdout, _, err := captureTUITestOutput(t, func() error {
		return runTUITestFixtures(tuiTestFixturesCmd, nil)
	})
	if err != nil {
		t.Fatalf("fixtures: %v", err)
	}
	got := strings.Fields(strings.TrimSpace(stdout))
	want := tui.BuiltinFixtureNames()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("plain fixtures = %v, want %v", got, want)
	}
}

func TestTUITestRunFixtureSuccess(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestRunFlags(t)

	scenario := writeScenario(t, `{
      "name": "rich smoke",
      "actions": [
        {"type": "assert", "assert": "contains", "value": "Rich Project"}
      ]
    }`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("json", "true")

	stdout, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s", code, stdout)
	}

	var result tui.UITestScenarioResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v\noutput: %s", err, stdout)
	}
	if !result.OK {
		t.Fatalf("result.OK = false, want true: %+v", result)
	}
	if result.ActionsRun != 1 {
		t.Fatalf("actions_run = %d, want 1", result.ActionsRun)
	}
}

func TestTUITestRunScenarioFailureExitsOne(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestRunFlags(t)

	scenario := writeScenario(t, `{
      "name": "missing token",
      "actions": [
        {"type": "assert", "assert": "contains", "value": "Definitely Not On Screen"}
      ]
    }`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("json", "true")

	stdout, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 1 {
		t.Fatalf("exit code = %d, want 1\nstdout: %s", code, stdout)
	}

	var result tui.UITestScenarioResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v\noutput: %s", err, stdout)
	}
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	if result.Error == nil {
		t.Fatalf("result.Error is nil, want a failing action")
	}
}

func TestTUITestRunMissingScenarioExitsTwo(t *testing.T) {
	resetTUITestRunFlags(t)
	// No --scenario set.
	_, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
}

func TestTUITestRunBadSizeExitsTwo(t *testing.T) {
	resetTUITestRunFlags(t)
	scenario := writeScenario(t, `{"actions":[{"type":"wait"}]}`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("size", "not-a-size")

	_, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
}

func TestTUITestRunInvalidScenarioExitsTwo(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestRunFlags(t)
	// Empty actions fail validation before execution.
	scenario := writeScenario(t, `{"name":"empty","actions":[]}`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)

	_, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
}

func TestTUITestRunUnknownFixtureExitsTwo(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestRunFlags(t)
	scenario := writeScenario(t, `{"actions":[{"type":"wait"}]}`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("fixture", "does-not-exist")

	_, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
}

func TestTUITestRunFixtureAndConfigMutuallyExclusive(t *testing.T) {
	resetTUITestRunFlags(t)
	scenario := writeScenario(t, `{"actions":[{"type":"wait"}]}`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("fixture", "rich") // explicitly changed
	viper.Set("config", "/some/config.json")

	_, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutual-exclusion message", err)
	}
}

func TestTUITestRunConfigModeCopiesData(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestRunFlags(t)

	sourceConfig := buildSourceConfig(t, "Copied Project", "Copied task title")
	viper.Set("config", sourceConfig)

	scenario := writeScenario(t, `{
      "name": "config smoke",
      "actions": [
        {"type": "assert", "assert": "contains", "value": "Copied Project"}
      ]
    }`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("json", "true")

	stdout, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s", code, stdout)
	}

	var result tui.UITestScenarioResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v\noutput: %s", err, stdout)
	}
	if !result.OK {
		t.Fatalf("result.OK = false, want true: %+v", result)
	}

	// The source config and its tasks.md must be untouched by the copy path.
	if _, statErr := os.Stat(sourceConfig); statErr != nil {
		t.Fatalf("source config disappeared: %v", statErr)
	}
}

func TestTUITestRunObservationsIncludesList(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestRunFlags(t)

	scenario := writeScenario(t, `{
      "name": "observations",
      "actions": [
        {"type": "assert", "assert": "contains", "value": "Rich Project"},
        {"type": "wait"}
      ]
    }`)
	fs := tuiTestRunCmd.Flags()
	_ = fs.Set("scenario", scenario)
	_ = fs.Set("json", "true")
	_ = fs.Set("observations", "true")
	_ = fs.Set("ansi", "true")

	stdout, _, err := captureTUITestOutput(t, func() error {
		return runTUITestRun(tuiTestRunCmd, nil)
	})
	if code := exitCode(t, err); code != 0 {
		t.Fatalf("exit code = %d, want 0\nstdout: %s", code, stdout)
	}

	var result tui.UITestScenarioResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v\noutput: %s", err, stdout)
	}
	if len(result.Observations) != result.ActionsRun {
		t.Fatalf("observations = %d, want %d", len(result.Observations), result.ActionsRun)
	}
	if len(result.Observations) == 0 || result.Observations[0].ANSIScreen == "" {
		t.Fatalf("expected ANSI screen in first observation with --ansi")
	}
}

func TestTUITestRunHiddenFromReference(t *testing.T) {
	if !tuiTestCmd.Hidden {
		t.Fatal("tui-test command group must be Hidden so it stays out of help and the CLI reference")
	}
}

// resetTUITestSessionFlags restores the session command's flags to their
// declared defaults with Changed cleared, and clears any inherited --config.
func resetTUITestSessionFlags(t *testing.T) {
	t.Helper()
	fs := tuiTestSessionCmd.Flags()
	defaults := map[string]string{
		"fixture": tui.FixtureRich,
		"size":    "100x30",
		"ansi":    "false",
	}
	for name, val := range defaults {
		if err := fs.Set(name, val); err != nil {
			t.Fatalf("reset flag %q: %v", name, err)
		}
	}
	fs.VisitAll(func(f *pflag.Flag) { f.Changed = false })
	viper.Set("config", "")
	t.Cleanup(func() { viper.Set("config", "") })
}

// runSession drives the session command with the given NDJSON input and returns
// the decoded response lines. It sets stdin/stdout on the command so the loop is
// exercised end to end without redirecting the process streams.
func runSession(t *testing.T, input string) []sessionResponse {
	t.Helper()
	var out bytes.Buffer
	tuiTestSessionCmd.SetIn(strings.NewReader(input))
	tuiTestSessionCmd.SetOut(&out)
	tuiTestSessionCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		tuiTestSessionCmd.SetIn(nil)
		tuiTestSessionCmd.SetOut(nil)
		tuiTestSessionCmd.SetErr(nil)
	})

	if err := runTUITestSession(tuiTestSessionCmd, nil); err != nil {
		t.Fatalf("session returned error: %v\noutput: %s", err, out.String())
	}
	return decodeSessionResponses(t, out.String())
}

// decodeSessionResponses parses NDJSON response lines, asserting each is a single
// well-formed JSON object (no human text, no pretty-printing).
func decodeSessionResponses(t *testing.T, output string) []sessionResponse {
	t.Helper()
	var responses []sessionResponse
	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var resp sessionResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response line is not valid JSON: %v\nline: %q", err, line)
		}
		responses = append(responses, resp)
	}
	return responses
}

func TestTUITestSessionActionsObserveQuit(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestSessionFlags(t)

	// Two actions, an explicit observe, then quit — the integration flow.
	input := strings.Join([]string{
		`{"id":"a1","action":{"type":"assert","assert":"contains","value":"Rich Project"}}`,
		`{"id":"a2","action":{"type":"resize","width":120,"height":40}}`,
		`{"id":"o1","command":"observe"}`,
		`{"id":"q1","command":"quit"}`,
	}, "\n") + "\n"

	responses := runSession(t, input)
	if len(responses) != 4 {
		t.Fatalf("got %d responses, want 4: %+v", len(responses), responses)
	}

	ids := []string{"a1", "a2", "o1", "q1"}
	for i, want := range ids {
		if responses[i].ID != want {
			t.Fatalf("response[%d].ID = %q, want %q", i, responses[i].ID, want)
		}
		if !responses[i].OK {
			t.Fatalf("response[%d] (%s) not ok: %+v", i, want, responses[i])
		}
		if responses[i].Observation == nil {
			t.Fatalf("response[%d] (%s) missing observation", i, want)
		}
	}

	// State persisted across requests: the resize is still reflected at observe.
	if got := responses[2].Observation.Width; got != 120 {
		t.Fatalf("observe width = %d, want 120 (resize should persist)", got)
	}
	if got := responses[2].Observation.Height; got != 40 {
		t.Fatalf("observe height = %d, want 40 (resize should persist)", got)
	}
}

func TestTUITestSessionCommandAction(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestSessionFlags(t)

	// A driver command action invokes a registered TUI command through the
	// driver — here the global "help" command opens the Help view — without
	// going through the command-palette UI. This mirrors the run-mode command
	// action and confirms session mode exercises it with no extra CLI plumbing.
	input := strings.Join([]string{
		`{"id":"h","action":{"type":"command","command":"help"}}`,
		`{"id":"q1","command":"quit"}`,
	}, "\n") + "\n"

	responses := runSession(t, input)
	if len(responses) != 2 {
		t.Fatalf("got %d responses, want 2: %+v", len(responses), responses)
	}
	if responses[0].ID != "h" {
		t.Fatalf("response[0].ID = %q, want h", responses[0].ID)
	}
	if !responses[0].OK {
		t.Fatalf("command action not ok: %+v", responses[0])
	}
	if responses[0].Observation == nil {
		t.Fatalf("command action missing observation")
	}
	if got := responses[0].Observation.CurrentView; got != "Help" {
		t.Fatalf("current_view = %q, want Help (command action should open Help)", got)
	}
}

func TestTUITestSessionStopsAfterQuit(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestSessionFlags(t)

	// A request after quit must not be processed.
	input := strings.Join([]string{
		`{"id":"q1","command":"quit"}`,
		`{"id":"after","command":"observe"}`,
	}, "\n") + "\n"

	responses := runSession(t, input)
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1 (loop must stop at quit): %+v", len(responses), responses)
	}
	if responses[0].ID != "q1" {
		t.Fatalf("response ID = %q, want q1", responses[0].ID)
	}
}

func TestTUITestSessionResetRestoresState(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestSessionFlags(t)

	input := strings.Join([]string{
		`{"id":"r1","action":{"type":"resize","width":80,"height":24}}`,
		`{"id":"reset1","command":"reset"}`,
		`{"id":"quit","command":"quit"}`,
	}, "\n") + "\n"

	responses := runSession(t, input)
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3: %+v", len(responses), responses)
	}
	if !responses[1].OK || responses[1].Observation == nil {
		t.Fatalf("reset response not ok with observation: %+v", responses[1])
	}
	// After reset the driver is rebuilt at the original 100x30 size, discarding
	// the earlier 80x24 resize.
	if got := responses[1].Observation.Width; got != 100 {
		t.Fatalf("post-reset width = %d, want 100 (rebuilt at original size)", got)
	}
}

func TestTUITestSessionBadRequests(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestSessionFlags(t)

	input := strings.Join([]string{
		`not json`,
		`{"id":"amb","command":"observe","action":{"type":"wait"}}`,
		`{"id":"nada"}`,
		`{"id":"huh","command":"frobnicate"}`,
		`{"id":"badkey","action":{"type":"key","key":"definitely-not-a-key"}}`,
	}, "\n") + "\n"

	responses := runSession(t, input)
	if len(responses) != 5 {
		t.Fatalf("got %d responses, want 5: %+v", len(responses), responses)
	}
	wantKinds := []string{
		sessionErrBadRequest,     // unparseable
		sessionErrBadRequest,     // both action and command
		sessionErrBadRequest,     // neither
		sessionErrUnknownCommand, // unknown command
		sessionErrActionFailed,   // invalid key
	}
	for i, kind := range wantKinds {
		if responses[i].OK {
			t.Fatalf("response[%d] unexpectedly ok: %+v", i, responses[i])
		}
		if responses[i].Error == nil || responses[i].Error.Kind != kind {
			t.Fatalf("response[%d] error kind = %+v, want %q", i, responses[i].Error, kind)
		}
	}
	// The invalid-key action failure still carries the failure-screen observation.
	if responses[4].Observation == nil {
		t.Fatalf("action_failed response should include the failure observation")
	}
	// Echoed IDs survive even on parse-level failures when present.
	if responses[3].ID != "huh" {
		t.Fatalf("unknown-command response ID = %q, want huh", responses[3].ID)
	}
}

func TestTUITestSessionFixtureAndConfigMutuallyExclusive(t *testing.T) {
	resetTUITestSessionFlags(t)
	_ = tuiTestSessionCmd.Flags().Set("fixture", "rich") // explicitly changed
	viper.Set("config", "/some/config.json")

	tuiTestSessionCmd.SetIn(strings.NewReader(""))
	tuiTestSessionCmd.SetOut(io.Discard)
	tuiTestSessionCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		tuiTestSessionCmd.SetIn(nil)
		tuiTestSessionCmd.SetOut(nil)
		tuiTestSessionCmd.SetErr(nil)
	})

	err := runTUITestSession(tuiTestSessionCmd, nil)
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v, want mutual-exclusion message", err)
	}
}

func TestTUITestSessionBadSizeExitsTwo(t *testing.T) {
	resetTUITestSessionFlags(t)
	_ = tuiTestSessionCmd.Flags().Set("size", "not-a-size")

	tuiTestSessionCmd.SetIn(strings.NewReader(""))
	tuiTestSessionCmd.SetOut(io.Discard)
	tuiTestSessionCmd.SetErr(io.Discard)
	t.Cleanup(func() {
		tuiTestSessionCmd.SetIn(nil)
		tuiTestSessionCmd.SetOut(nil)
		tuiTestSessionCmd.SetErr(nil)
	})

	err := runTUITestSession(tuiTestSessionCmd, nil)
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (err=%v)", code, err)
	}
}

func TestTUITestSessionConfigModeCopiesData(t *testing.T) {
	useFileSystemStorage(t)
	resetTUITestSessionFlags(t)

	sourceConfig := buildSourceConfig(t, "Session Copied Project", "Session copied task")
	viper.Set("config", sourceConfig)

	input := strings.Join([]string{
		`{"id":"c1","action":{"type":"assert","assert":"contains","value":"Session Copied Project"}}`,
		`{"id":"q","command":"quit"}`,
	}, "\n") + "\n"

	responses := runSession(t, input)
	if len(responses) != 2 {
		t.Fatalf("got %d responses, want 2: %+v", len(responses), responses)
	}
	if !responses[0].OK {
		t.Fatalf("config-mode assert failed: %+v", responses[0])
	}
	// The source config must be untouched by the copy path.
	if _, statErr := os.Stat(sourceConfig); statErr != nil {
		t.Fatalf("source config disappeared: %v", statErr)
	}
}

// buildSourceConfig writes a real config file plus data dir with one project and
// task, returning the config path suitable for the copied-config CLI path.
func buildSourceConfig(t *testing.T, projectName, taskTitle string) string {
	t.Helper()
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")

	cfg := config.NewConfig()
	cfg.DataDir = dataDir
	cfg.FzfEnabled = false
	cfg.GlowEnabled = false
	cfg.Projects = []*models.Project{}
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("ensure data dir: %v", err)
	}

	project, err := models.NewProject(uuid.New().String(), projectName, "")
	if err != nil {
		t.Fatalf("new project: %v", err)
	}
	project.Path = cfg.GetProjectPath(project.Slug)
	project.EnsureDefaultSections()
	task, err := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, taskTitle)
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
	project.BuildTaskTree()

	cfg.Projects = append(cfg.Projects, project)
	cfg.CurrentProject = project.ID
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	configPath := filepath.Join(root, "tt.json")
	if err := config.NewManager(configPath).Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	return configPath
}
