#!/bin/bash
set -euo pipefail

echo 'Running tests...'
go test ./...

echo 'Building agentsandbox...'
go build ./cmd/agentsandbox

echo 'Done!'
