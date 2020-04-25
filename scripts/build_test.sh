#!/bin/bash -e

# Ted: contact me when you make any changes

SRC_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "$SRC_DIR/env.sh"

which golangci-lint || (curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.25.0)
golangci-lint run

go test -race -coverprofile=coverage.out -covermode=atomic ./...
