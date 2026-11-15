build:
    go build -o bin/docket .

test:
    go test ./...

check:
    test -z "$(gofmt -l .)"
    go vet ./...
    go test ./...

run: build
    DOCKET_DB=./dev.db ./bin/docket
