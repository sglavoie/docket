package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/ttenv"
	"github.com/sglavoie/tt/internal/tui"
)

var noColor bool
var closeLogging = logging.Close

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
				commit = setting.Value
			}
		case "vcs.time":
			if buildTime == "" {
				buildTime = setting.Value
			}
		}
	}
	return version, commit, buildTime
}

var rootCmd = &cobra.Command{
	Use:   "tt",
	Short: "An Outliner-first terminal task manager",
	Long: `tt is an Outliner-first terminal task manager.

Run tt without a command to open the interactive Outliner. The command-line
surface is intentionally limited to workspace maintenance, diagnostics,
conflict recovery, project export, and shell completion.

Startup validates schema-16 catalogs and schema-14 task Markdown before loading,
and migrates any supported older workspace forward through the whole chain of
immutable snapshot-first boundaries. Migration is one-way, so keep a copy of
your workspace before a version jump. Run 'tt doctor' for read-only integrity
and migration-snapshot diagnostics.

Examples:
  tt
  tt doctor
  tt conflict list
  tt workspace location
  tt project export`,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cmd.SilenceUsage = true
		return nil
	},
	Args: cobra.NoArgs,
	RunE: runRoot,
}

// Execute runs the root command and returns any error. The caller is
// responsible for printing the error and terminating the process on failure.
func Execute() error {
	return executeCommand(rootCmd)
}

func executeCommand(command *cobra.Command) error {
	defer closeLogging()
	// SilenceUsage is enabled in PersistentPreRunE so usage is still printed
	// when cobra itself rejects args (wrong count, unknown flags, etc.) but
	// suppressed for errors that originate inside RunE.
	command.SilenceErrors = true
	return command.Execute()
}

func init() {
	cobra.OnInitialize(initNoColor)
	logging.Init()
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", fmt.Sprintf("workspace data directory (env %s; default: %s)", ttenv.DataDirEnvVar, defaultDataDirDisplay))
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable color output")
	rootCmd.Flags().Bool("version", false, "show version information")
}

func initNoColor() {
	if noColor || os.Getenv("NO_COLOR") != "" {
		if err := os.Setenv("NO_COLOR", "1"); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set NO_COLOR: %v\n", err)
		}
	}
}

// GetDataDir returns the explicitly selected workspace data directory: the
// --data-dir flag, or the TT_DATA_DIR environment variable when the flag is
// unset. It is empty when neither is provided, so callers can fall back to the
// active-workspace pointer or the legacy default.
func GetDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	return os.Getenv(ttenv.DataDirEnvVar)
}

// resolveConfigPath derives the config path the manager should use, honoring the
// documented workspace precedence: an explicit --data-dir/TT_DATA_DIR or the
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

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	version, _, _ := resolveBuildInfo()
	tui.AppVersion = version
	return tui.RunApp(workspace.Runtime, workspace.Manager, tui.RunOptions{DisableColor: noColor})
}

func printVersion() {
	version, commit, buildTime := resolveBuildInfo()
	if commit == "" && buildTime == "" {
		fmt.Printf("tt version %s\n", version)
		return
	}
	fmt.Printf("tt version %s (commit: %s, built: %s)\n", version, commit, buildTime)
}
