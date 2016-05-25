#!/bin/bash
set -e

for GOOS in darwin linux; do
    for GOARCH in 386 amd64; do
        export GOOS
        export GOARCH

        DIR="git-annex-remote-b2.$GOOS-$GOARCH"
        echo "Creating ${DIR}.tar.gz"
        rm -rf "$DIR"
        mkdir "$DIR"

        go build -o "$DIR/git-annex-remote-b2"
        cp README.md LICENSE "$DIR/"

        rm -f "$DIR".tar.gz
        tar -czf "$DIR".tar.gz "$DIR"
        rm -rf "$DIR"
    done
done
