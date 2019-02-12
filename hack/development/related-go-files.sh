#!/bin/bash
# This command outputs a list of go files on which the given go file depends on
# Example: related-go-files.sh <base package name> <go file>

set -e

go list -f '{{ join .Deps "\n" }}' $2 | grep 'pkg' | grep -v 'vendor' | sed -e "s|^$1/||" | xargs -I % find % -name "*.go" -maxdepth 1 -type f 2>/dev/null
