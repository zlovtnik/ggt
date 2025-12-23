#!/usr/bin/env bash
set -euo pipefail

go test -race -coverprofile=coverage.out ./...
