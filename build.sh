#!/bin/sh
flags="-X main.Commit=$(git rev-parse HEAD) -X main.Version=$(git describe --tags)"
go build -ldflags="$flags" .
