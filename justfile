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

run: build
    DOCKET_DB=./dev.db ./bin/docket
