package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/ttenv"
	"github.com/sglavoie/docket/internal/tui"
)

var noColor bool

// noAdvisories belongs to the root command alone: it configures the interactive
// TUI, and no maintenance subcommand renders a status strip.
var noAdvisories bool

// notify belongs to the root command alone too: only the interactive TUI runs
// long enough to see work arrive.
var notify bool

// controlSocket belongs to the root command alone as well: it is the running
// TUI that answers a host on it.
var controlSocket string

var (
	openLogging  = logging.Init
	closeLogging = logging.Close
)

var dataDir string

var (
	// Build information, populated via ldflags
	Version   = "dev"
	GitCommit = ""
	BuildTime = ""
)

// devVersion is the compiled-in placeholder that means no ldflags were passed,
// and moduleDevVersion is what the toolchain records for a module built outside
// a released version. Neither is worth reporting when build info can do better.
const (
	devVersion       = "dev"
	moduleDevVersion = "(devel)"
)

// readBuildInfo is the debug.ReadBuildInfo seam the version tests replace.
var readBuildInfo = debug.ReadBuildInfo

// resolveBuildInfo reports the version, commit, and build time to display.
// The ldflags the justfile passes always win, so `just build` output is
// unchanged; when they are absent — which is every `go install` — the module
// build info the toolchain embeds supplies the module version and, for a build
// from a source checkout, the VCS revision and timestamp.
func resolveBuildInfo() (version, commit, buildTime string) {
	version, commit, buildTime = Version, GitCommit, BuildTime
	if version != devVersion {
		return version, commit, buildTime
	}
	info, ok := readBuildInfo()
	if !ok {
		return version, commit, buildTime
	}
	if moduleVersion := info.Main.Version; moduleVersion != "" && moduleVersion != moduleDevVersion {
		version = moduleVersion
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if commit == "" {
				commit = shortCommit(setting.Value)
			}
		case "vcs.time":
			if buildTime == "" {
				buildTime = setting.Value
			}
		}
	}
	return version, commit, buildTime
}

// shortCommitLength is how much of a revision the version line prints: what
// `git rev-parse --short` gives the justfile and GoReleaser's ShortCommit, so
// every install channel reports a commit the same way.
const shortCommitLength = 7

func shortCommit(revision string) string {
	if len(revision) > shortCommitLength {
		return revision[:shortCommitLength]
	}
	return revision
}

var rootCmd = &cobra.Command{
	Use:   "docket",
	Short: "An Outliner-first terminal task manager",
	Long: fmt.Sprintf(`Docket is an Outliner-first terminal task manager.

Run docket without a command to open the interactive Outliner. The command-line
surface is intentionally limited to workspace maintenance, diagnostics,
conflict recovery, project export, and shell completion.

Docket reads schema-%d catalogs and schema-%d task Markdown, and upgrades an
older workspace to them when it opens it. Upgrading is one-way, so run 'docket
workspace backup' before a version jump. 'docket doctor' checks integrity and
never upgrades: it refuses a workspace still owed an upgrade.`, config.CurrentConfigVersion, storage.CurrentTaskSchemaVersion),
	Example: `  docket
  docket doctor
  docket conflict list
  docket workspace location
  docket project export --all`,
	// No subcommand defines its own PersistentPreRunE: cobra runs only the
	// nearest one, so a child's would shadow this hook and skip logging.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		// Cobra checks required flags and flag groups only after this hook,
		// and raises those refusals bare. Checking them here first gives them
		// the same --help pointer every other command-line refusal carries.
		if err := cmd.ValidateRequiredFlags(); err != nil {
			return usageRefusal(cmd, err)
		}
		if err := cmd.ValidateFlagGroups(); err != nil {
			return usageRefusal(cmd, err)
		}
		if commandLogs(cmd) {
			// Before logging opens and before the active-workspace pointer is
			// read, both of which live under the machine-local root it moves.
			if err := config.AdoptLegacyRoots(); err != nil {
				return err
			}
			openLogging()
		}
		return nil
	},
	Args: noArgs,
	RunE: runRoot,
}

// Execute runs the root command and returns any error. The caller is
// responsible for printing the error and terminating the process on failure.
func Execute() error {
	return executeCommand(rootCmd)
}

func executeCommand(command *cobra.Command) error {
	// Logging opens lazily in the root PersistentPreRunE, so importing this
	// package, as a test binary does, never creates a log file beneath the real
	// $HOME. Close is a no-op when nothing opened.
	defer closeLogging()
	// Nothing prints the whole usage block on a refusal. A refusal cobra raises
	// before the command runs — a stray or missing argument, an unknown flag —
	// goes through usageRefusal, which silences usage and ends the error with a
	// pointer to --help; PersistentPreRunE silences usage for every error that
	// originates inside RunE.
	command.SilenceErrors = true
	return command.Execute()
}

// commandLogs reports whether cmd may write a log, and so whether the pre-run
// hook adopts the legacy roots and opens one. Help, shell completion, and root
// --version never log, so they must not touch ~/.docket. --help needs no case:
// cobra answers it before any pre-run hook runs.
func commandLogs(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "help", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd, "completion":
		return false
	}
	if parent := cmd.Parent(); parent != nil && parent.Name() == "completion" {
		return false
	}
	if !cmd.HasParent() {
		if showVersion, err := cmd.Flags().GetBool("version"); err == nil && showVersion {
			return false
		}
	}
	return true
}

func init() {
	cobra.OnInitialize(initNoColor)
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	// Every subcommand inherits the root's flag error function.
	rootCmd.SetFlagErrorFunc(usageRefusal)

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", fmt.Sprintf("workspace data directory (env %s; default: %s)", ttenv.DataDirEnvVar, defaultDataDirDisplay))
	_ = rootCmd.MarkPersistentFlagDirname("data-dir")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.Flags().BoolVar(&noAdvisories, "no-advisories", false, "hide ambient advisory rows in the TUI (conflict count, load failures)")
	rootCmd.Flags().BoolVar(&notify, "notify", false, "send a desktop notification (OSC 777) when a check-in arrives, a timed deadline passes, or a deferral ends")
	rootCmd.Flags().StringVar(&controlSocket, "control", "", "serve a control socket at this path for a host app such as Docket.app")
	_ = rootCmd.MarkFlagFilename("control")
	rootCmd.Flags().Bool("version", false, "print the version and exit")
}

func initNoColor() {
	if noColor || os.Getenv("NO_COLOR") != "" {
		if err := os.Setenv("NO_COLOR", "1"); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set NO_COLOR: %v\n", err)
		}
	}
}

// GetDataDir returns the explicitly selected workspace data directory: the
// --data-dir flag, or the DOCKET_DATA_DIR environment variable when the flag is
// unset. It is empty when neither is provided, so callers can fall back to the
// active-workspace pointer or the legacy default.
func GetDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	return os.Getenv(ttenv.DataDirEnvVar)
}

// resolveConfigPath derives the config path the manager should use, honoring the
// documented workspace precedence: an explicit --data-dir/DOCKET_DATA_DIR or the
// machine-local active-workspace pointer selects the data dir and its config
// path; otherwise an empty string lets NewManager resolve the legacy default.
func resolveConfigPath() string {
	dd := GetDataDir()
	if dd == "" {
		if ptr, ok := config.ReadActiveWorkspacePointer(); ok {
			dd = ptr
		}
	}
	if dd != "" {
		return config.ConfigPathForDataDir(filepath.Clean(dd))
	}
	return ""
}

func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		printVersion()
		return nil
	}

	runtime, err := openDesktopRuntime()
	if err != nil {
		return err
	}

	version, _, _ := resolveBuildInfo()
	tui.AppVersion = version
	return tui.RunApp(runtime, tui.RunOptions{
		DisableColor:   noColor,
		HideAdvisories: noAdvisories,
		Notify:         notify,
		ControlSocket:  controlSocket,
		VersionLine:    versionLine(),
	})
}

func printVersion() {
	fmt.Println(versionLine())
}

func versionLine() string {
	version, commit, buildTime := resolveBuildInfo()
	if commit == "" && buildTime == "" {
		return fmt.Sprintf("docket version %s", version)
	}
	return fmt.Sprintf("docket version %s (commit: %s, built: %s)", version, commit, buildTime)
}
