package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/syncexchange"
)

// syncCmd is the parent for device sync maintenance: enrolling this desktop's
// workspace, joining one another device enrolled, a manual exchange, and the
// status and diagnostics a person needs when something is waiting. The
// Outliner shows the same status in its strip; nothing here is needed for
// day-to-day use beyond enrolling once.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Enroll, exchange, and diagnose device sync",
	Long: `Sync one workspace between this desktop and other devices through an
exchange folder, typically in iCloud Drive.

Sync is opt-in. 'tt sync enroll' keeps the current workspace exactly as it is,
snapshots it, and builds a new local working copy outside iCloud that becomes
the enrolled workspace; only immutable history is written to the exchange
folder. A workspace that was never enrolled keeps working as it always has.

Enrolled workspaces use catalog schema ` + fmt.Sprint(config.SyncEnrolledConfigVersion) + `, which a tt older than sync refuses
to open rather than write around the history.`,
	Example: `  tt sync enroll --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange --working-copy ~/tt-synced
  tt sync status
  tt sync exchange
  tt sync doctor`,
	Args: noArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

var (
	syncExchangeFolder string
	syncWorkingCopy    string
	syncSnapshotDir    string
	syncForce          bool
	syncJSON           bool
)

var syncEnrollCmd = &cobra.Command{
	Use:   "enroll",
	Short: "Enroll this workspace in sync through a local working copy",
	Long: `Enroll the current workspace in device sync.

Enrollment never converts the workspace in place. It:

  1. snapshots the current workspace, verified file by file, under
     ~/.tt/enrollments/ (or --snapshot-dir);
  2. builds a new local working copy at --working-copy, which must be outside
     iCloud Drive, and selects it as this machine's workspace;
  3. records the working copy's initial history and publishes it to the
     exchange folder.

The original workspace is left untouched. Re-run the same command after an
interruption to finish; nothing is copied or published twice.`,
	Example: `  tt sync enroll --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange --working-copy ~/tt-synced`,
	Args:    noArgs,
	RunE:    runSyncEnroll,
}

var syncJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Build this desktop's working copy of a workspace another device enrolled",
	Long: `Join a workspace another device enrolled, from the history in its exchange
folder alone. The new working copy at --working-copy must be outside iCloud
Drive; it becomes this machine's workspace.`,
	Example: `  tt sync join --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange --working-copy ~/tt-synced`,
	Args:    noArgs,
	RunE:    runSyncJoin,
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what is committed, outstanding, held, and in conflict",
	Example: `  tt sync status
  tt sync status --json`,
	Args: noArgs,
	RunE: runSyncStatus,
}

var syncExchangeCmd = &cobra.Command{
	Use:   "exchange",
	Short: "Exchange history with the exchange folder now",
	Long: `Run one exchange pass now: publish this device's committed history, then
import what other devices published. Content that iCloud has not downloaded
yet is reported as unavailable and fetched on a later pass; nothing is ever
deleted because a file is missing.

--exchange points this device at a different exchange folder from now on.`,
	Example: `  tt sync exchange
  tt sync exchange --exchange ~/Library/Mobile\ Documents/com~apple~CloudDocs/tt-exchange`,
	Args: noArgs,
	RunE: runSyncExchange,
}

var syncDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose sync: placement, exchange folder, history, holds, and upgrades",
	Example: `  tt sync doctor
  tt sync doctor --json`,
	Args: noArgs,
	RunE: runSyncDoctor,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncEnrollCmd, syncJoinCmd, syncStatusCmd, syncExchangeCmd, syncDoctorCmd)

	syncEnrollCmd.Flags().StringVar(&syncExchangeFolder, "exchange", "", "exchange folder shared by the devices, typically in iCloud Drive (required)")
	syncEnrollCmd.Flags().StringVar(&syncWorkingCopy, "working-copy", "", "new local directory for the enrolled working copy, outside iCloud (required)")
	syncEnrollCmd.Flags().StringVar(&syncSnapshotDir, "snapshot-dir", "", "where to keep the snapshot of the original (default ~/.tt/enrollments)")
	syncEnrollCmd.Flags().BoolVarP(&syncForce, "force", "f", false, "skip the confirmation prompt only")
	syncJoinCmd.Flags().StringVar(&syncExchangeFolder, "exchange", "", "the exchange folder the enrolling device published to (required)")
	syncJoinCmd.Flags().StringVar(&syncWorkingCopy, "working-copy", "", "new local directory for this device's working copy (required)")
	syncExchangeCmd.Flags().StringVar(&syncExchangeFolder, "exchange", "", "use, and remember, this exchange folder")
	// --folder is the flag's earlier name, kept so existing scripts still run.
	syncExchangeCmd.Flags().StringVar(&syncExchangeFolder, "folder", "", "use, and remember, this exchange folder")
	_ = syncExchangeCmd.Flags().MarkHidden("folder")
	syncExchangeCmd.MarkFlagsMutuallyExclusive("exchange", "folder")
	_ = syncExchangeCmd.MarkFlagDirname("exchange")
	for _, command := range []*cobra.Command{syncStatusCmd, syncExchangeCmd, syncDoctorCmd} {
		command.Flags().BoolVar(&syncJSON, "json", false, "print the result as JSON")
	}
	for _, command := range []*cobra.Command{syncEnrollCmd, syncJoinCmd} {
		_ = command.MarkFlagRequired("exchange")
		_ = command.MarkFlagRequired("working-copy")
		_ = command.MarkFlagDirname("exchange")
		_ = command.MarkFlagDirname("working-copy")
	}
}

func syncAppVersion() string {
	version, _, _ := resolveBuildInfo()
	return version
}

func runSyncEnroll(_ *cobra.Command, _ []string) error {
	if err := requireExistingWorkspace(); err != nil {
		return err
	}
	original := config.WorkspaceRootFromConfigPath(config.NewManager(resolveConfigPath()).GetConfigPath())
	fmt.Printf("Enroll workspace in sync:\n  original:     %s (kept unchanged)\n  working copy: %s\n  exchange:     %s\n",
		config.DisplayPath(absolutePath(original)), config.DisplayPath(absolutePath(syncWorkingCopy)), config.DisplayPath(absolutePath(syncExchangeFolder)))
	switch confirmPrompt("Proceed? [y/N]: ", syncForce) {
	case confirmNoAnswer:
		return errors.New("enrollment not confirmed: no answer was read; re-run with --force to skip the prompt")
	case confirmDeclined:
		fmt.Println("Enrollment canceled; nothing changed.")
		return nil
	case confirmAccepted:
	}
	// Enrolling is a current-schema operation: open the original first, as a
	// launch would, so a pending migration or crashed commit is finished before
	// the snapshot is taken.
	if _, err := openDesktopRuntime(); err != nil {
		return fmt.Errorf("prepare the workspace for enrollment: %w", err)
	}
	result, err := engine.Enroll(engine.EnrollConfig{
		Original:     original,
		WorkingCopy:  syncWorkingCopy,
		Exchange:     syncExchangeFolder,
		SnapshotRoot: syncSnapshotDir,
		Machine:      engine.ProcessMachine(),
		AppVersion:   syncAppVersion(),
	})
	if result != nil && result.Enrollment != nil {
		enrollment := result.Enrollment
		if enrollment.Resumed {
			fmt.Println("Resumed an interrupted enrollment.")
		}
		if enrollment.Snapshot != "" {
			fmt.Printf("  snapshot:     %s\n", config.DisplayPath(enrollment.Snapshot))
		}
		fmt.Printf("  working copy: %s (now this machine's workspace; %s copied from %s)\n",
			config.DisplayPath(enrollment.WorkingCopy), Pluralize(enrollment.FilesCopied, "file", "files"), config.DisplayPath(enrollment.Original))
		fmt.Printf("  workspace:    %s\n", enrollment.WorkspaceID)
	}
	if result != nil && result.Genesis != "" {
		fmt.Printf("  history:      genesis %s\n", shortTxn(result.Genesis))
	}
	if err != nil {
		return fmt.Errorf("%w\nRe-run the same command to finish the enrollment", err)
	}
	fmt.Println(successMsg("Workspace enrolled; published %s to the exchange folder",
		Pluralize(len(result.Exchange.Published), "transaction", "transactions")))
	return nil
}

func runSyncJoin(_ *cobra.Command, _ []string) error {
	rt, report, err := engine.Join(engine.JoinConfig{
		DataDir:    syncWorkingCopy,
		Exchange:   syncExchangeFolder,
		Machine:    engine.ProcessMachine(),
		AppVersion: syncAppVersion(),
	})
	if err != nil {
		if report != nil {
			printFolderReport(report)
		}
		return err
	}
	fmt.Println(successMsg("Joined; %s is now this machine's workspace (%s imported)",
		config.DisplayPath(rt.DataDir()), Pluralize(report.Imported, "transaction", "transactions")))
	return nil
}

// openSyncRuntime opens the selected workspace for a sync command, refusing to
// create one.
func openSyncRuntime() (*engine.Runtime, error) {
	if err := requireExistingWorkspace(); err != nil {
		return nil, err
	}
	return openDesktopRuntime()
}

func runSyncStatus(_ *cobra.Command, _ []string) error {
	rt, err := openSyncRuntime()
	if err != nil {
		return err
	}
	status, err := rt.SyncStatus()
	if err != nil {
		return err
	}
	if syncJSON {
		return printJSON(status)
	}
	printSyncStatus(rt.DataDir(), status)
	return nil
}

const notEnrolledHint = "This workspace is not enrolled in sync. Run 'tt sync enroll --exchange DIR --working-copy DIR' to enroll it."

func printSyncStatus(dataDir string, status *engine.SyncStatus) {
	if !status.Enrolled {
		fmt.Println(notEnrolledHint)
		return
	}
	fmt.Printf("Workspace %s (enrolled)\n", status.WorkspaceID)
	fmt.Printf("  working copy:  %s\n", config.DisplayPath(dataDir))
	fmt.Printf("  device:        %s\n", status.DeviceID)
	if status.Enrollment != nil {
		fmt.Printf("  exchange:      %s\n", config.DisplayPath(status.Enrollment.ExchangeFolder))
	}
	fmt.Printf("  local:         %s committed on this device\n", Pluralize(int(status.LocalTransactions), "transaction", "transactions"))
	fmt.Printf("  to publish:    %d\n", status.OutstandingUp)
	fmt.Printf("  to apply:      %d\n", status.OutstandingDown)
	switch {
	case status.LastSuccess.IsZero():
		fmt.Println("  last exchange: never succeeded")
	default:
		fmt.Printf("  last exchange: %s\n", status.LastSuccess.Local().Format("2006-01-02 15:04:05"))
	}
	if last := status.LastExchange; last != nil && !last.Succeeded && len(last.Errors) > 0 {
		fmt.Printf("  last attempt:  failed at %s: %s\n", last.At.Local().Format("2006-01-02 15:04:05"), last.Errors[0])
	}
	if last := status.LastExchange; last != nil && last.Succeeded && len(last.Errors) > 0 {
		fmt.Printf("  exchange errors: %s\n", strings.Join(last.Errors, "; "))
	}
	if last := status.LastExchange; last != nil && len(last.Unavailable) > 0 {
		fmt.Printf("  unavailable:   %s not downloaded yet\n", Pluralize(len(last.Unavailable), "file", "files"))
	}
	for _, held := range status.Held {
		fmt.Printf("  held:          %s from %s seq %d — %s\n", shortTxn(held.TxnID), held.DeviceID, held.DeviceSeq, held.Upgrade)
	}
	for _, upgrade := range status.Upgrades {
		fmt.Printf("  upgrade:       %s\n", upgrade)
	}
	fmt.Printf("  conflicts:     %d open\n", len(status.Conflicts))
	for _, conflict := range status.Conflicts {
		fmt.Printf("    %s  %s (%d versions)\n", shortTxn(conflict.ID), conflict.Path, len(conflict.Candidates))
	}
}

func shortTxn(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func runSyncExchange(_ *cobra.Command, _ []string) error {
	rt, err := openSyncRuntime()
	if err != nil {
		return err
	}
	if !rt.State().Catalog().Enrolled() {
		return fmt.Errorf("%s", notEnrolledHint)
	}
	if syncExchangeFolder != "" {
		if setErr := config.SetEnrollmentExchangeFolder(rt.DataDir(), syncExchangeFolder); setErr != nil {
			return setErr
		}
	}
	report, err := rt.SyncExchange(engine.SyncExchangeOptions{AppVersion: syncAppVersion()})
	if syncJSON && report != nil {
		if printErr := printJSON(report); printErr != nil {
			return printErr
		}
		return err
	}
	if report != nil {
		printFolderReport(report)
	}
	return err
}

func printFolderReport(report *syncexchange.FolderReport) {
	fmt.Printf("Exchange with %s\n", config.DisplayPath(report.Folder))
	fmt.Printf("  published:   %d\n", len(report.Published))
	fmt.Printf("  imported:    %d\n", report.Imported)
	fmt.Printf("  to publish:  %d\n", report.OutstandingUp)
	fmt.Printf("  to apply:    %d\n", report.OutstandingDown)
	if len(report.Unavailable) > 0 {
		fmt.Printf("  unavailable: %s not downloaded yet\n", Pluralize(len(report.Unavailable), "file", "files"))
	}
	if report.External != nil {
		for _, held := range report.External.Held {
			fmt.Printf("  held edit:   %s — %s\n", held.Path, held.Reason)
		}
	}
	for _, held := range report.Held {
		fmt.Printf("  held:        %s from %s — %s\n", shortTxn(held.TxnID), held.DeviceID, held.Upgrade)
	}
	for _, message := range report.Errors {
		fmt.Printf("  error:       %s\n", message)
	}
}

// syncFinding is one sync doctor line: a note is how things are, an issue
// is something to act on.
type syncFinding struct {
	Check  string `json:"check"`
	Issue  bool   `json:"issue"`
	Detail string `json:"detail"`
}

func runSyncDoctor(_ *cobra.Command, _ []string) error {
	rt, err := openSyncRuntime()
	if err != nil {
		return err
	}
	findings, err := diagnoseSync(rt)
	if err != nil {
		return err
	}
	issues := 0
	for _, finding := range findings {
		if finding.Issue {
			issues++
		}
	}
	if syncJSON {
		if err := printJSON(findings); err != nil {
			return err
		}
	} else {
		for _, finding := range findings {
			mark := "ok "
			if finding.Issue {
				mark = "!! "
			}
			fmt.Printf("%s%-12s %s\n", mark, finding.Check, finding.Detail)
		}
	}
	if issues > 0 {
		return fmt.Errorf("sync doctor found %s", Pluralize(issues, "issue", "issues"))
	}
	return nil
}

// diagnoseSync runs the sync checks in the order a person should read them:
// whether this is an enrolled working copy, where it lives, whether its
// exchange folder is reachable and its own, its history, then what waits.
func diagnoseSync(rt *engine.Runtime) ([]syncFinding, error) {
	status, err := rt.SyncStatus()
	if err != nil {
		return nil, err
	}
	note := func(check, format string, args ...any) syncFinding {
		return syncFinding{Check: check, Detail: fmt.Sprintf(format, args...)}
	}
	issue := func(check, format string, args ...any) syncFinding {
		return syncFinding{Check: check, Issue: true, Detail: fmt.Sprintf(format, args...)}
	}
	catalog := rt.State().Catalog()
	if !status.Enrolled {
		return []syncFinding{note("enrollment", "not enrolled; catalog schema %d, the current schema for a workspace that does not sync", catalog.SchemaVersion)}, nil
	}
	findings := []syncFinding{note("enrollment", "enrolled %s; catalog schema %d", catalog.Sync.EnrolledAt.Local().Format("2006-01-02"), catalog.SchemaVersion)}
	if config.InICloudDrive(rt.DataDir(), "") {
		findings = append(findings, issue("placement", "the working copy %s is inside iCloud Drive; enroll again into a local folder", rt.DataDir()))
	} else {
		findings = append(findings, note("placement", "the working copy is local: %s", config.DisplayPath(rt.DataDir())))
	}
	record := status.Enrollment
	switch {
	case record == nil || record.ExchangeFolder == "":
		findings = append(findings, issue("exchange", "no exchange folder is configured; run 'tt sync exchange --exchange DIR'"))
	default:
		findings = append(findings, diagnoseExchangeFolder(record.ExchangeFolder, status.WorkspaceID))
	}
	if record != nil && record.Snapshot != "" {
		if _, statErr := os.Stat(filepath.Join(record.Snapshot, "snapshot.json")); statErr != nil {
			findings = append(findings, issue("snapshot", "the enrollment snapshot %s is missing", record.Snapshot))
		} else {
			findings = append(findings, note("snapshot", "the original is preserved at %s", config.DisplayPath(record.Snapshot)))
		}
	}
	if status.Genesis {
		findings = append(findings, note("history", "%s recorded on device %s, %d to publish",
			Pluralize(int(status.LocalTransactions), "transaction", "transactions"), status.DeviceID, status.OutstandingUp))
	} else {
		findings = append(findings, issue("history", "the initial history is not recorded; run 'tt sync exchange'"))
	}
	if last := status.LastExchange; last != nil {
		switch {
		case !last.Succeeded:
			findings = append(findings, issue("last pass", "failed at %s: %s", last.At.Local().Format(time.RFC3339), strings.Join(last.Errors, "; ")))
		case len(last.Errors) > 0:
			findings = append(findings, issue("last pass", "completed with errors: %s", strings.Join(last.Errors, "; ")))
		case status.LastSuccess.IsZero():
		default:
			findings = append(findings, note("last pass", "succeeded at %s", status.LastSuccess.Local().Format(time.RFC3339)))
		}
		if last.External != nil {
			for _, held := range last.External.Held {
				findings = append(findings, issue("held edit", "%s is not published: %s", held.Path, held.Reason))
			}
		}
		if len(last.Unavailable) > 0 {
			findings = append(findings, note("downloads", "%s not downloaded yet", Pluralize(len(last.Unavailable), "exchange file", "exchange files")))
		}
	} else {
		findings = append(findings, issue("last pass", "this device has never exchanged; run 'tt sync exchange'"))
	}
	for _, held := range status.Held {
		findings = append(findings, issue("held", "%s from %s seq %d waits: %s", shortTxn(held.TxnID), held.DeviceID, held.DeviceSeq, held.Reason))
	}
	for _, upgrade := range status.Upgrades {
		findings = append(findings, issue("upgrade", "%s", upgrade))
	}
	if len(status.Conflicts) > 0 {
		findings = append(findings, issue("conflicts", "%s open; browse them with gc in the Outliner",
			Pluralize(len(status.Conflicts), "sync conflict", "sync conflicts")))
	} else {
		findings = append(findings, note("conflicts", "none open"))
	}
	return findings, nil
}

func diagnoseExchangeFolder(folder, workspaceID string) syncFinding {
	id, err := syncexchange.ReadExchangeIdentity(folder)
	switch {
	case err != nil:
		return syncFinding{Check: "exchange", Issue: true, Detail: err.Error()}
	case id != workspaceID:
		return syncFinding{Check: "exchange", Issue: true, Detail: fmt.Sprintf("%s belongs to workspace %s", folder, id)}
	}
	return syncFinding{Check: "exchange", Detail: "reachable and this workspace's: " + config.DisplayPath(folder)}
}
