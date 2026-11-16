install_dir := env("HOME") / ".local/bin"

build:
    go build -o bin/docket .

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
    test -z "$(gofmt -l .)"
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
