#!/bin/sh
set -eu

if ! command -v trvl >/dev/null 2>&1; then
  if command -v brew >/dev/null 2>&1; then
    brew install MikkoParkkola/tap/trvl
  else
    echo "trvl binary not found and Homebrew is unavailable." >&2
    echo "Install trvl from https://github.com/MikkoParkkola/trvl, then restart Claude Code." >&2
    exit 127
  fi
fi

exec trvl mcp
