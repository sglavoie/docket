package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
)

var (
	syncResolveVersion int
	syncResolveCurrent bool
	syncResolveFile    string
	syncDiagnosticsOut string
)

var syncConflictsCmd = &cobra.Command{
	Use:   "conflicts [ID]",
	Short: "List open sync conflicts, or show one with every version",
	Long: `List the sync conflicts this device's history leaves open, or show one: the
version both sides started from, each competing version with the device and
change that wrote it, what the workspace holds meanwhile, and the other
conflicts the same change left. Every version is kept until a resolution;
nothing is chosen by timestamp.

ID is a conflict id or any unique prefix of it.`,
	Example: `  tt sync conflicts
  tt sync conflicts 3f2a9c
  tt sync conflicts --json`,
	Args: atMostOneArg("conflict ID"),
	// Deriving the open conflicts replays the whole sync history, too costly
	// for a Tab press, so completion offers nothing rather than file names.
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE:              runSyncConflicts,
}

var syncResolveCmd = &cobra.Command{
	Use:   "resolve ID",
	Short: "Resolve a sync conflict by keeping a version or writing a replacement",
	Long: `Resolve one open sync conflict as a new change that syncs to every device.

--version N keeps competing version N as 'tt sync conflicts ID' numbers them.
When the change that wrote it also left conflicts in other files, their
versions from that same change are kept too, in one resolution, so a change
that spanned files is never resolved half way.

--current keeps what the workspace holds now. --file writes replacement text
for tasks or a note; it is checked the way tt reads that file.

The conflict must still be the one you read: if another device resolved it or
an edit changed it since, nothing is written and the refusal says so. Re-read
it and choose again; every version is still kept.

This is not 'tt conflict resolve', which settles a local merge conflict
between this machine's edits and the file on disk with --use local|remote|file.
A sync conflict instead holds numbered versions from several devices, so it is
resolved by naming one.`,
	Example: `  tt sync resolve 3f2a9c --version 2
  tt sync resolve 3f2a9c --current
  tt sync resolve 3f2a9c --file merged-tasks.md`,
	Args:              syncResolveArgs,
	ValidArgsFunction: cobra.NoFileCompletions,
	RunE:              runSyncResolve,
}

var syncDiagnosticsCmd = &cobra.Command{
	Use:   "diagnostics",
	Short: "Export sync diagnostics: status, last exchange, outbox, and conflicts",
	Long: `Write a JSON document describing this device's sync: its status and last
exchange (including any transport error), the transactions waiting to
publish, and each open conflict's files, devices, and digests. It names files
and devices but holds no task or note content. Nothing is changed.`,
	Example: `  tt sync diagnostics
  tt sync diagnostics --file ~/Desktop/tt-sync-diagnostics.json`,
	Args: noArgs,
	RunE: runSyncDiagnostics,
}

func init() {
	syncCmd.AddCommand(syncConflictsCmd, syncResolveCmd, syncDiagnosticsCmd)
	syncConflictsCmd.Flags().BoolVar(&syncJSON, "json", false, "print the result as JSON")
	syncResolveCmd.Flags().BoolVar(&syncJSON, "json", false, "print the result as JSON")
	syncResolveCmd.Flags().IntVar(&syncResolveVersion, "version", 0, "keep competing version N (numbered from 1)")
	syncResolveCmd.Flags().BoolVar(&syncResolveCurrent, "current", false, "keep what the workspace holds now")
	syncResolveCmd.Flags().StringVar(&syncResolveFile, "file", "", "write this file's text as the resolution")
	syncDiagnosticsCmd.Flags().StringVarP(&syncDiagnosticsOut, "file", "o", "", "write to this file instead of standard output")
	// --output is the flag's earlier name, kept so existing scripts still run;
	// --file is the spelling project export uses for the same thing.
	syncDiagnosticsCmd.Flags().StringVar(&syncDiagnosticsOut, "output", "", "write to this file instead of standard output")
	_ = syncDiagnosticsCmd.Flags().MarkHidden("output")
	syncDiagnosticsCmd.MarkFlagsMutuallyExclusive("file", "output")
}

// openEnrolledRuntime opens the workspace and refuses one that is not enrolled.
func openEnrolledRuntime() (*engine.Runtime, error) {
	rt, err := openSyncRuntime()
	if err != nil {
		return nil, err
	}
	if !rt.State().Catalog().Enrolled() {
		return nil, fmt.Errorf("%s", notEnrolledHint)
	}
	return rt, nil
}

// findSyncConflict resolves an id or unique prefix among the open conflicts.
func findSyncConflict(views []engine.ConflictView, id string) (engine.ConflictView, error) {
	id = strings.TrimPrefix(strings.TrimSpace(id), "sha256:")
	var found []engine.ConflictView
	for _, view := range views {
		if id != "" && strings.HasPrefix(strings.TrimPrefix(view.ID, "sha256:"), id) {
			found = append(found, view)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return engine.ConflictView{}, fmt.Errorf("no open sync conflict %s; run 'tt sync conflicts' to list them", id)
	}
	return engine.ConflictView{}, fmt.Errorf("%s names %d open conflicts; give more of the id", id, len(found))
}

func runSyncConflicts(_ *cobra.Command, args []string) error {
	rt, err := openEnrolledRuntime()
	if err != nil {
		return err
	}
	views, err := rt.SyncConflicts(nil)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		if syncJSON {
			return printJSON(struct {
				Conflicts []engine.ConflictView `json:"conflicts"`
			}{views})
		}
		if len(views) == 0 {
			fmt.Println("No open sync conflicts.")
			return nil
		}
		for _, view := range views {
			fmt.Printf("%s  %s  (%s, %d versions)\n", shortTxn(view.ID), view.Label, view.Path, len(view.Versions))
		}
		fmt.Println("\nShow one with 'tt sync conflicts ID'; resolve it with 'tt sync resolve ID'.")
		return nil
	}
	view, err := findSyncConflict(views, args[0])
	if err != nil {
		return err
	}
	if syncJSON {
		return printJSON(view)
	}
	printSyncConflict(view)
	return nil
}

func printSyncConflict(view engine.ConflictView) {
	fmt.Printf("Sync conflict %s\n", shortTxn(view.ID))
	fmt.Printf("  %s\n  file: %s\n", view.Label, view.Path)
	if len(view.Details) > 0 {
		fmt.Println("\nWhat differs:")
		for _, detail := range view.Details {
			subject := strings.TrimSpace(strings.Join([]string{detail.Subject, detail.Field}, " "))
			if subject != "" {
				subject += ": "
			}
			fmt.Printf("  %s%s\n", subject, detail.Detail)
		}
	}
	printConflictContent("\nStarted from", view.Base)
	for _, version := range view.Versions {
		holding := ""
		if version.Current {
			holding = " — the workspace holds this one meanwhile"
		}
		fmt.Printf("\nVersion %d%s\n", version.Index+1, holding)
		for _, writer := range version.Writers {
			fmt.Printf("  %s on %s at %s\n", writer.Change, conflictDevice(writer), writer.CreatedAt)
		}
		for _, also := range version.AlsoResolves {
			fmt.Printf("  keeping it also keeps this change's version of %s (%s)\n", also.Label, shortTxn(also.ID))
		}
		printConflictBody(version.ConflictContent)
	}
	fmt.Printf("\nResolve with 'tt sync resolve %s --version N', '--current', or '--file PATH'.\n", shortTxn(view.ID))
}

func conflictDevice(writer engine.ConflictWriter) string {
	name := writer.DeviceID
	if writer.DeviceName != "" {
		name = writer.DeviceName + " (" + writer.DeviceID + ")"
	}
	if writer.ThisDevice {
		name += ", this device"
	}
	return name
}

func printConflictContent(heading string, content *engine.ConflictContent) {
	if content == nil {
		fmt.Printf("%s: nothing (the file did not exist)\n", heading)
		return
	}
	fmt.Printf("%s:\n", heading)
	printConflictBody(*content)
}

func printConflictBody(content engine.ConflictContent) {
	switch {
	case content.Deleted:
		fmt.Println("  (deleted)")
	case !content.Available:
		fmt.Println("  (not on this device)")
	case content.Text == nil:
		fmt.Printf("  (%d bytes, not text)\n", content.Size)
	default:
		for _, line := range strings.Split(strings.TrimRight(*content.Text, "\n"), "\n") {
			fmt.Println("  │ " + line)
		}
	}
}

// syncResolveChoiceFlags are the three ways to resolve a sync conflict, of
// which exactly one must be given.
var syncResolveChoiceFlags = []string{"version", "current", "file"}

// validateSyncResolveChoice refuses anything but exactly one choice, spelling
// the flags as typed; cobra's own flag-group errors drop the dashes and the
// --help pointer.
func validateSyncResolveChoice(cmd *cobra.Command) error {
	var set []string
	for _, name := range syncResolveChoiceFlags {
		if cmd.Flags().Changed(name) {
			set = append(set, "--"+name)
		}
	}
	switch len(set) {
	case 0:
		return usageRefusal(cmd, fmt.Errorf("choose a resolution: --version N, --current, or --file PATH"))
	case 1:
		return nil
	}
	return usageRefusal(cmd, fmt.Errorf("%s cannot be combined; choose one", strings.Join(set, " and ")))
}

// syncResolveArgs checks the conflict ID and the choice before anything is
// opened; cobra has parsed the flags by the time it validates arguments.
func syncResolveArgs(cmd *cobra.Command, args []string) error {
	if err := exactlyOneArg("conflict ID")(cmd, args); err != nil {
		return err
	}
	return validateSyncResolveChoice(cmd)
}

func runSyncResolve(_ *cobra.Command, args []string) error {
	rt, err := openEnrolledRuntime()
	if err != nil {
		return err
	}
	views, err := rt.SyncConflicts(nil)
	if err != nil {
		return err
	}
	view, err := findSyncConflict(views, args[0])
	if err != nil {
		return err
	}
	request := engine.ResolveRequest{ConflictID: view.ID}
	switch {
	case syncResolveCurrent:
		request.Choice = engine.ResolveCurrent
	case syncResolveFile != "":
		data, readErr := os.ReadFile(filepath.Clean(syncResolveFile))
		if readErr != nil {
			return readErr
		}
		request.Choice, request.Text = engine.ResolveReplace, string(data)
	default:
		if syncResolveVersion < 1 || syncResolveVersion > len(view.Versions) {
			return fmt.Errorf("conflict %s has versions 1 to %d", shortTxn(view.ID), len(view.Versions))
		}
		request.Choice, request.Version = engine.ResolveVersion, syncResolveVersion-1
	}
	result, err := rt.ResolveSyncConflict(request)
	if err != nil {
		return err
	}
	if syncJSON {
		return printJSON(result)
	}
	for _, resolved := range result.Resolved {
		fmt.Printf("Resolved %s  %s\n", shortTxn(resolved.ID), resolved.Label)
	}
	fmt.Printf("%s still open. The resolution syncs at the next exchange.\n", Pluralize(result.Remaining, "conflict", "conflicts"))
	return nil
}

func runSyncDiagnostics(_ *cobra.Command, _ []string) error {
	rt, err := openEnrolledRuntime()
	if err != nil {
		return err
	}
	report, err := rt.SyncDiagnostics(nil)
	if err != nil {
		return err
	}
	if syncDiagnosticsOut == "" {
		return printJSON(report)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(syncDiagnosticsOut, append(data, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Printf("Wrote sync diagnostics to %s\n", config.DisplayPath(syncDiagnosticsOut))
	return nil
}
