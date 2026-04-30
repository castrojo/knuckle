package main

import (
"context"
"flag"
"fmt"
"log/slog"
"os"

"github.com/castrojo/knuckle/internal/bakery"
"github.com/castrojo/knuckle/internal/install"
"github.com/castrojo/knuckle/internal/probe"
"github.com/castrojo/knuckle/internal/runner"
"github.com/castrojo/knuckle/internal/tui"
"github.com/castrojo/knuckle/internal/wizard"
)

var (
version = "dev"
)

func main() {
var (
dryRun  bool
logFile string
channel string
showVer bool
)

flag.BoolVar(&dryRun, "dry-run", false, "simulate installation without writing to disk")
flag.StringVar(&logFile, "log-file", "/tmp/knuckle.log", "path to log file")
flag.StringVar(&channel, "channel", "stable", "Flatcar release channel (stable, beta, alpha, edge)")
flag.BoolVar(&showVer, "version", false, "print version and exit")
flag.Parse()

if showVer {
fmt.Printf("knuckle %s\n", version)
os.Exit(0)
}

// Validate channel flag
validChannels := map[string]bool{"stable": true, "beta": true, "alpha": true, "edge": true}
if !validChannels[channel] {
fmt.Fprintf(os.Stderr, "Error: invalid channel %q (must be stable, beta, alpha, or edge)\n", channel)
os.Exit(1)
}

// Set up logging to file (never stdout — Bubble Tea owns stdout)
logWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
if err != nil {
fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
os.Exit(1)
}
defer func() { _ = logWriter.Close() }()

logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
Level: slog.LevelDebug,
}))

logger.Info("knuckle starting", "version", version, "dry-run", dryRun, "channel", channel)

// Set up runner (dry-run or real)
var cmdRunner runner.Runner
if dryRun {
cmdRunner = runner.NewDryRunner(logger)
} else {
cmdRunner = runner.NewRealRunner(logger)
}

// Prober always uses real runner — it only reads system state
realRunner := runner.NewRealRunner(logger)
prober := probe.NewSystemProber(realRunner)
bakeryClient := bakery.NewHTTPClient()
installer := install.NewFlatcarInstaller(cmdRunner, logger)

// Create wizard
w := wizard.New(prober, bakeryClient, installer)
w.State.Config.Channel = channel
w.State.Config.DryRun = dryRun

// Probe hardware before starting TUI
ctx := context.Background()
if err := w.ProbeHardware(ctx); err != nil {
logger.Warn("hardware probe failed", "error", err)
// Non-fatal — user can still proceed with manual input
}

// Fetch sysext catalog (non-fatal if it fails)
if err := w.FetchSysexts(ctx); err != nil {
logger.Warn("sysext catalog fetch failed", "error", err)
}

// Fetch channel version info (non-fatal if it fails)
if err := w.FetchChannels(ctx); err != nil {
logger.Warn("channel info fetch failed", "error", err)
}

// Run the TUI
if err := tui.Run(w); err != nil {
logger.Error("TUI error", "error", err)
fmt.Fprintf(os.Stderr, "Error: %v\n", err)
os.Exit(1)
}

logger.Info("knuckle finished")
}
