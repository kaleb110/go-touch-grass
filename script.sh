#!/bin/bash
# Build script for go-touch-grass.
# After building, run ./go-touch-grass install to set up the systemd service.
set -e

echo "Building go-touch-grass..."
go build -o go-touch-grass main.go

echo "Done. Next steps:"
echo "  ./go-touch-grass install        # install binary + systemd service"
echo "  ./go-touch-grass tracker        # show today's usage dashboard"
echo "  ./go-touch-grass --help         # full help"
