install_dir := env("HOME") / ".local/bin"
binary_name := "bin/docket"

# `docket --version` names the commit and the build time, so Docket.app's
# Settings can tell two builds apart.
build:
    go build -ldflags "-X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev) -X main.Commit=$(git rev-parse --short HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o {{ binary_name }} .

# Install the binary to ~/.local/bin (linked as dkt)
install: build
    mkdir -p {{ install_dir }}
    rm -f {{ install_dir }}/docket
    cp bin/docket {{ install_dir }}/docket
    ln -sfn docket {{ install_dir }}/dkt
    @echo "Installed to {{ install_dir }}/docket (linked as dkt)"

test:
    go test ./...

check:
    test -z "$(gofmt -l main.go internal tools)"
    go vet ./...
    go test ./...

# Open ./dev.db, seeding it with a heavy outline the first time
run: build
    test -e dev.db || go run ./tools/seed -db dev.db
    DOCKET_DB=./dev.db ./bin/docket

# Replace ./dev.db with a fresh heavy outline (just seed 20000 80: tasks, projects)
seed tasks="6000" projects="48":
    rm -f dev.db dev.db-wal dev.db-shm
    go run ./tools/seed -db dev.db -tasks {{ tasks }} -projects {{ projects }}

# Build and vendor the pinned GhosttyKit.xcframework for the macOS app
mac-ghosttykit:
    ./macos/scripts/build-ghosttykit.sh

# Build macos/build/Docket.app (Debug, or Release bundling `just build`'s docket)
mac-app configuration="Debug":
    #!/usr/bin/env bash
    set -euo pipefail
    if [ "{{ configuration }}" = Release ]; then
        just build
        export DOCKET_BINARY="$PWD/{{ binary_name }}"
    fi
    ./macos/scripts/build-app.sh {{ configuration }}

# Run the Docket.app XCTest suite with xcodebuild test
mac-test:
    ./macos/scripts/test-app.sh

# Build the Release Docket.app and install it to /Applications
mac-install: (mac-app "Release")
    ./macos/scripts/install-app.sh

# Tag the next release from v2 and follow it (patch, minor, major, or vX.Y.Z)
release version *flags:
    ./scripts/release.sh {{ version }} {{ flags }}
