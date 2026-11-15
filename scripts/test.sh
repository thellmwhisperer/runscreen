#!/bin/sh
# The whole test suite.
set -eu
cd "$(dirname "$0")/.."
go test -count=1 ./...
