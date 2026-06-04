#!/bin/bash
set -e

echo "building and moving the binary..."

go build -o go-touch-grass main.go
mkdir -p ~/.local/bin/
mv go-touch-grass ~/.local/bin/