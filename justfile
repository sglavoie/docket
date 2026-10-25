# docket - CLI task manager

binary_name := "docket"
alias_name := "dkt"
install_dir := env("HOME") / ".local/bin"

# Build information
version := env("VERSION", "dev")
git_commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_time := `date -u '+%Y-%m-%dT%H:%M:%SZ'`
ldflags := "-ldflags \"-X github.com/sglavoie/docket/cmd.Version=" + version + " -X github.com/sglavoie/docket/cmd.GitCommit=" + git_commit + " -X github.com/sglavoie/docket/cmd.BuildTime=" + build_time + "\""

# Show available recipes
help:
    @just --list

# Build the binary in the current directory
build:
    @echo "Building {{ binary_name }}..."
    go build {{ ldflags }} -o {{ binary_name }} .
    @echo "Built {{ binary_name }}"

# Install the binary to ~/.local/bin
install: build
    @echo "Installing {{ binary_name }} to {{ install_dir }}..."
    @mkdir -p {{ install_dir }}
    rm -f {{ install_dir }}/{{ binary_name }}
    cp {{ binary_name }} {{ install_dir }}/{{ binary_name }}
    ln -sfn {{ binary_name }} {{ install_dir }}/{{ alias_name }}
    @echo "Installed to {{ install_dir }}/{{ binary_name }} (linked as {{ alias_name }})"

# Alias for install
docket: install

# Run tests (with race detector; internal/tui split across processes)
test:
    @echo "Running tests..."
    ./scripts/test_race.sh

# Run tests outside internal/tui without the race detector (fast inner loop).
# -short skips the seeded sync fault histories, which run for minutes; the
# 20m deadline matches test_race.sh's for the sync suites that remain.
test-fast:
    go test -short -timeout=20m $(go list ./... | grep -v /internal/tui)

# Run one package's tests with the race detector, e.g. `just test-pkg ./cmd`
test-pkg PKG:
    GORACE="atexit_sleep_ms=0" go test -race {{ PKG }}

# Measure search latency and enforce allocation budgets; optionally --strict-time
search-bench *ARGS:
    ./scripts/check_search_budget.sh {{ ARGS }}

# Run tests with coverage report and enforce the coverage thresholds
coverage:
    @echo "Running tests with coverage..."
    ./scripts/test_race.sh coverage.out
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"
    ./scripts/check_coverage.sh

# Format Go code (goimports when installed, else go fmt)
fmt:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Formatting code..."
    if command -v goimports &>/dev/null; then
        goimports -w -local github.com/sglavoie/docket .
    else
        echo "goimports not found; falling back to go fmt"
        go fmt ./...
    fi

# Clean up go.mod and go.sum
tidy:
    go mod tidy

# Run go vet
vet:
    go vet ./...

# Run go fix
fix:
    go fix ./...

# Run formatting and vetting
lint: fmt vet

# Run golangci-lint (requires golangci-lint installed)
lint-ci:
    #!/usr/bin/env bash
    set -euo pipefail
    if ! command -v golangci-lint &>/dev/null; then
        echo "Error: golangci-lint not found. Install it from https://golangci-lint.run/welcome/install/"
        exit 1
    fi
    echo "Running golangci-lint..."
    golangci-lint run

# Run the whole gate GitHub CI runs split across checks, race-test shards, and coverage.
ci:
    ./scripts/ci.sh

# Replay every committed TUI scenario with one command.
tui-scenarios:
    ./scripts/run_tui_scenarios.sh

# Re-record the README animation (docs/images/demo.svg); needs uv
readme-media:
    ./scripts/readme_media.sh

# Fail when the committed README animation no longer matches the app
readme-media-check:
    ./scripts/readme_media.sh --check

# Run the two-machine hazard gate (outside `just ci`; DOCKET_OLD_BINARY_REF defaults to CI's schema-19 build)
cross-machine:
    DOCKET_OLD_BINARY_REF="${DOCKET_OLD_BINARY_REF:-08c0cb08fec10154e6d2c9455e32d4c4ffa5f72d}" ./scripts/cross_machine_gate.sh

# Build and vendor the pinned GhosttyKit.xcframework for the macOS app (outside `just ci`)
mac-ghosttykit:
    ./macos/scripts/build-ghosttykit.sh

# Build macos/build/tt.app (Debug, or Release bundling `just build`'s docket; outside `just ci`)
mac-app configuration="Debug":
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "{{ configuration }}" = Release ]; then
        just build
        export DOCKET_BINARY="$PWD/{{ binary_name }}"
    fi
    ./macos/scripts/build-app.sh {{ configuration }}

# Run the tt.app XCTest suite with xcodebuild test (outside `just ci`)
mac-test:
    ./macos/scripts/test-app.sh

# Build the Release tt.app and install it to /Applications (outside `just ci`)
mac-install: (mac-app "Release")
    ./macos/scripts/install-app.sh

# Bind the Go engine into the iOS XCFramework, device and simulator slices (outside `just ci`)
ios-bridge:
    ./ios/scripts/build-bridge.sh

# Build the iOS bridge and generate ios/TTMobile.xcodeproj (outside `just ci`)
ios-project:
    ./ios/scripts/generate-project.sh

# Build tt.app for the simulator; result bundle and diagnostics in ios/.cache/results/build (outside `just ci`)
ios-build:
    ./ios/scripts/app.sh build

# Build, sign, install, and launch on your paired iPhone, preserving its data
[positional-arguments]
ios-install team="" device="":
    #!/usr/bin/env bash
    set -euo pipefail
    export DOCKET_IOS_DEVELOPMENT_TEAM="${1:-${DOCKET_IOS_DEVELOPMENT_TEAM:-}}"
    export DOCKET_IOS_DEVICE="${2:-${DOCKET_IOS_DEVICE:-}}"
    exec ./ios/scripts/install-device.sh --launch

# Run Swift suites: unit, bridge, app (hosted simulator tests), sync, or all; summary in ios/.cache/results/<suite> (outside `just ci`)
ios-test suite="all":
    ./ios/scripts/test-suites.sh {{ suite }} "${DOCKET_IOS_RESULTS:-ios/.cache/results}/{{ suite }}"

# Unattended simulator qualification. start detaches from SSH; status/resume use the same output directory.
ios-qualify command="run" output="ios/.cache/qualification":
    ./ios/scripts/qualify-simulator.sh {{ command }} {{ quote(output) }}

# Run the simulator UI suite, or one class, on the default or small simulator; result bundle, summary, and screenshots in ios/.cache/results/ui-<device> (outside `just ci`)
ios-ui-test only="" device="default":
    #!/usr/bin/env bash
    set -euo pipefail
    max_width=""
    if [ "{{ device }}" = small ]; then max_width=375; fi
    DOCKET_UITEST_ONLY="{{ only }}" DOCKET_IOS_DEVICE="{{ device }}" DOCKET_UITEST_MAX_WIDTH="$max_width" \
        ./ios/scripts/test-ui.sh "${DOCKET_IOS_RESULTS:-ios/.cache/results}/ui-{{ device }}"

# Launch tt on a simulator over a fresh copy of a fixture (representative or empty), light or dark, at a text size (outside `just ci`)
ios-run fixture="representative" appearance="light" text_size="large" device="default":
    ./ios/scripts/app.sh run {{ fixture }} {{ appearance }} {{ text_size }} {{ device }}

# Reseed the running app's isolated workspace from its fixture and relaunch it
ios-reset:
    ./ios/scripts/app.sh reset

# Save a screenshot of the simulator as ios/.cache/run/screenshots/NAME.png
ios-screenshot name="screen":
    ./ios/scripts/app.sh screenshot {{ name }}

# Print the running app's semantic state and save it with its recent log under ios/.cache/run/export
ios-export:
    ./ios/scripts/app.sh export

# Stop the app ios-run launched
ios-stop:
    ./ios/scripts/app.sh stop

# Run read-only code quality checks (gofmt, goimports, vet, golangci-lint, deadcode)
check:
    @echo "Running code quality checks..."
    ./scripts/check.sh

# Find dead code in the codebase
dead:
    @echo "Finding dead code..."
    ./scripts/dead_code.sh

# Build release binaries for darwin/linux × amd64/arm64
release:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p dist
    for os in darwin linux; do
        for arch in amd64 arm64; do
            out="dist/{{ binary_name }}-${os}-${arch}"
            echo "Building ${out}..."
            GOOS=${os} GOARCH=${arch} go build {{ ldflags }} -o "${out}" .
        done
    done
    (cd dist && shasum -a 256 {{ binary_name }}-* > SHA256SUMS)
    echo "Release binaries and SHA256SUMS written to dist/"

# Remove build artifacts
clean:
    @echo "Cleaning..."
    rm -f {{ binary_name }} coverage.out coverage.html
    rm -rf dist
    go clean
    @echo "Clean complete"

# Install git hooks (points core.hooksPath at scripts/hooks)
install-hooks:
    git config core.hooksPath scripts/hooks
    @echo "Git hooks installed from scripts/hooks"
