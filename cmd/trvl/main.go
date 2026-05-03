// Package main is the entrypoint for the trvl CLI.
package main

import (
	"context"
	"os"

	"github.com/MikkoParkkola/trvl/internal/selfupdate"
)

func main() {
	// Fire-and-forget daily update check. Returns immediately; the
	// goroutine writes a one-line stderr notice on the NEXT invocation
	// once the cache is warm. Skipped automatically for dev builds and
	// CI environments. Bounded to 6 s so trvl's actual exit isn't
	// noticeably delayed even if GitHub is slow.
	ctx, cancel := context.WithCancel(context.Background())
	selfupdate.CheckInBackground(ctx, Version, os.Stderr)
	defer cancel()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
