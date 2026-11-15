#!/bin/sh
# The lint: golangci-lint with .golangci.yml.
set -eu
cd "$(dirname "$0")/.."
golangci-lint run
