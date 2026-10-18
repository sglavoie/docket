package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
)

func newDoctorCommand(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	command := &cobra.Command{}
	command.Flags().Bool("fix", false, "")
	command.Flags().Bool("json", false, "")
	if err := command.Flags().Parse(args); err != nil {
		t.Fatalf("parse doctor flags %v: %v", args, err)
	}
	return command
}

// plantStaleTodayEntry commits a Today entry naming a task Unfiled does not
// own, the fixable skew two machines most often leave behind.
func plantStaleTodayEntry(t *testing.T, workspace *CLIWorkspace) {
	t.Helper()
	unfiled := workspace.Runtime.SystemUnfiledProject()
	planning := &config.PlanningState{Today: []config.ProjectTaskLocator{{ProjectID: unfiled.ID, TaskID: "task-gone"}}}
	if err := workspace.Runtime.CommitPlanning(planning); err != nil {
		t.Fatalf("plant stale Today entry: %v", err)
	}
}

func TestRunDoctorFixRepairsThenReportsTheCleanState(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)

	stdout, stderr, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	if err != nil {
		t.Fatalf("doctor --fix: %v\n%s", err, stdout)
	}
	if stderr != "" {
		t.Fatalf("doctor --fix stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Fixed: pruned 1 planning entry from Today and Next") ||
		strings.Contains(stdout, "Planning membership issues") ||
		!strings.Contains(stdout, "All checks passed") {
		t.Fatalf("doctor --fix did not report the post-fix state:\n%s", stdout)
	}
	if strings.Contains(stdout, "Re-run") {
		t.Fatalf("doctor --fix still asks for a re-run:\n%s", stdout)
	}

	stdout, _, err = captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t), nil)
	})
	if err != nil {
		t.Fatalf("second doctor run: %v\n%s", err, stdout)
	}
}

func TestRunDoctorWithoutFixExitsTwoForAFixableIssue(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)

	stdout, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t), nil)
	})
	var issues *doctorIssuesError
	if !errors.As(err, &issues) || issues.ExitCode() != 2 {
		t.Fatalf("doctor error = %v, want an exit-2 issues error", err)
	}
	if !strings.Contains(stdout, "Run 'tt doctor --fix'") {
		t.Fatalf("doctor did not point at --fix:\n%s", stdout)
	}
}

func TestRunDoctorUnfixableIssueExitsTwo(t *testing.T) {
	newDoctorFixtureWorkspace(t, func(document string, alpha *models.Task) string {
		stamp := alpha.CreatedAt.Format(time.RFC3339Nano)
		return strings.Replace(document, "state: backlog,", "state: backlog, completed: "+stamp+",", 1)
	})

	stdout, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	var issues *doctorIssuesError
	if !errors.As(err, &issues) {
		t.Fatalf("doctor --fix error = %v, want doctorIssuesError\n%s", err, stdout)
	}
	if issues.ExitCode() != 2 || issues.count != 1 || err.Error() != "found 1 issue" {
		t.Fatalf("issues error = %+v (%q), want exit 2 for one issue", issues, err.Error())
	}
	if !strings.Contains(stdout, "Invalid completion timestamps: 1") ||
		!strings.Contains(stdout, "Resolve the reported integrity issues") {
		t.Fatalf("doctor did not report the unfixable issue:\n%s", stdout)
	}
}

func TestRunDoctorFixFailureIsNotAnIssuesVerdict(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)
	unfiledID := workspace.Runtime.SystemUnfiledProject().ID
	apply := doctorApplyFixes
	t.Cleanup(func() { doctorApplyFixes = apply })
	doctorApplyFixes = func(state *doctorState) ([]string, error) {
		// Another process rewrites planning after doctor read it.
		other, err := LoadCLIWorkspace()
		if err != nil {
			t.Fatalf("load concurrent workspace: %v", err)
		}
		concurrent := &config.PlanningState{Next: []config.ProjectTaskLocator{{ProjectID: unfiledID, TaskID: "task-elsewhere"}}}
		if err := other.Runtime.CommitPlanning(concurrent); err != nil {
			t.Fatalf("concurrent planning commit: %v", err)
		}
		return apply(state)
	}

	_, _, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix"), nil)
	})
	var issues *doctorIssuesError
	if err == nil || errors.As(err, &issues) {
		t.Fatalf("doctor --fix error = %v, want a fix failure that exits 1", err)
	}
	if !strings.Contains(err.Error(), "could not apply every repair") ||
		!strings.Contains(err.Error(), "repair planning membership") {
		t.Fatalf("doctor --fix error = %v, want it to name the failed planning repair", err)
	}
}

func TestDoctorJSONPrintsOnlyAParseableReport(t *testing.T) {
	root, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)
	home := os.Getenv("HOME")

	// Anything logged while doctor runs, as BuildTaskTree's warnings are, must
	// reach the log file the root pre-run opened and never the standard logger.
	var leaked bytes.Buffer
	log.SetOutput(&leaked)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	runE := doctorCmd.RunE
	doctorCmd.RunE = func(cmd *cobra.Command, args []string) error {
		logging.Warn("doctor json probe")
		return runE(cmd, args)
	}

	rootCmd.SetArgs([]string{"doctor", "--json", "--data-dir", root})
	t.Cleanup(func() {
		doctorCmd.RunE = runE
		rootCmd.SetArgs(nil)
		_ = doctorCmd.Flags().Set("json", "false")
	})
	stdout, stderr, err := captureDoctorOutput(t, func() error {
		return executeCommand(rootCmd)
	})

	var issues *doctorIssuesError
	if !errors.As(err, &issues) || issues.ExitCode() != 2 || err.Error() != "" {
		t.Fatalf("doctor --json error = %v, want a silent exit-2 issues error", err)
	}
	if stderr != "" || leaked.Len() != 0 {
		t.Fatalf("stderr = %q, standard logger = %q; want both empty", stderr, leaked.String())
	}
	var report doctorReport
	if decodeErr := json.Unmarshal([]byte(stdout), &report); decodeErr != nil {
		t.Fatalf("stdout is not one JSON report: %v\n%s", decodeErr, stdout)
	}
	if report.Schema != doctorReportSchema || report.TotalIssues != 1 ||
		len(report.Buckets["planning_membership"]) != 1 ||
		report.Buckets["duplicate_short_ids"] == nil ||
		report.Notes["workspace_status"] == nil ||
		report.Fixed == nil {
		t.Fatalf("report = %+v", report)
	}

	logs, globErr := filepath.Glob(filepath.Join(home, ".tt", "logs", "tt-*.log"))
	if globErr != nil || len(logs) != 1 {
		t.Fatalf("log files = %v (%v), want exactly one", logs, globErr)
	}
	content, readErr := os.ReadFile(logs[0])
	if readErr != nil || !strings.Contains(string(content), "doctor json probe") {
		t.Fatalf("log file = %q (%v), want the probe warning", content, readErr)
	}
}

func TestDoctorJSONFixCarriesFixedLinesInTheReport(t *testing.T) {
	_, workspace := newDoctorFixtureWorkspace(t)
	plantStaleTodayEntry(t, workspace)

	stdout, stderr, err := captureDoctorOutput(t, func() error {
		return runDoctor(newDoctorCommand(t, "--fix", "--json"), nil)
	})
	if err != nil || stderr != "" {
		t.Fatalf("doctor --fix --json = %v, stderr %q", err, stderr)
	}
	var report doctorReport
	if decodeErr := json.Unmarshal([]byte(stdout), &report); decodeErr != nil {
		t.Fatalf("stdout is not one JSON report: %v\n%s", decodeErr, stdout)
	}
	if report.TotalIssues != 0 || len(report.Fixed) != 1 ||
		report.Fixed[0] != "pruned 1 planning entry from Today and Next" {
		t.Fatalf("report = %+v", report)
	}
}

func captureDoctorOutput(t *testing.T, fn func() error) (stdout string, stderr string, runErr error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	runErr = fn()
	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)

	return stdoutBuf.String(), stderrBuf.String(), runErr
}
