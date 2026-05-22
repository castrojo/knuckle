package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/projectbluefin/knuckle/internal/bakery"
	"github.com/projectbluefin/knuckle/internal/headless"
	"github.com/projectbluefin/knuckle/internal/install"
	"github.com/projectbluefin/knuckle/internal/probe"
	"github.com/projectbluefin/knuckle/internal/runner"
	"github.com/projectbluefin/knuckle/internal/tui"
	"github.com/projectbluefin/knuckle/internal/validate"
	"github.com/projectbluefin/knuckle/internal/wizard"
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

	var (
		configFile   string
		headlessMode bool
	)
	flag.BoolVar(&dryRun, "dry-run", false, "simulate installation without writing to disk")
	flag.StringVar(&logFile, "log-file", "/tmp/knuckle.log", "path to log file")
	flag.StringVar(&channel, "channel", "stable", "Flatcar release channel (stable, beta, alpha, lts, edge)")
	flag.BoolVar(&showVer, "version", false, "print version and exit")
	flag.StringVar(&configFile, "config", "", "path to JSON config file for headless install")
	flag.BoolVar(&headlessMode, "headless", false, "run without TUI (requires --config)")
	var flatcarVersion string
	flag.StringVar(&flatcarVersion, "flatcar-version", "", "pin to specific Flatcar version (e.g. 3510.2.8)")
	flag.Parse()

	if showVer {
		fmt.Printf("knuckle %s\n", version)
		os.Exit(0)
	}

	// Handle headless mode early
	if headlessMode || configFile != "" {
		if configFile == "" {
			fmt.Fprintf(os.Stderr, "Error: --headless requires --config <file>\n")
			os.Exit(1)
		}
		runHeadless(configFile, dryRun, logFile)
		return
	}

	// Validate channel flag
	if err := validate.Channel(channel); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Set up logging to file (never stdout — Bubble Tea owns stdout)
	logWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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
	w.State.Config.Version = flatcarVersion

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

	// Run the TUI — wire reboot through the runner so dry-run/spy work correctly
	var rebootFn func(context.Context) error
	if !w.State.Config.DryRun {
		rebootFn = func(ctx context.Context) error {
			_, err := cmdRunner.Run(ctx, "systemctl", "reboot")
			return err
		}
	}
	if err := tui.Run(w, rebootFn); err != nil {
		logger.Error("TUI error", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("knuckle finished")
}

// runHeadless loads a JSON config and runs the install without TUI.
func runHeadless(configPath string, dryRun bool, logFile string) {
	// Set up logging
	logWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logWriter.Close() }()

	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Load config
	cfg, err := headless.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Override dry-run from CLI flag
	if dryRun {
		cfg.DryRun = true
	}

	// Set up runner
	var cmdRunner runner.Runner
	if cfg.DryRun {
		cmdRunner = runner.NewDryRunner(logger)
	} else {
		cmdRunner = runner.NewRealRunner(logger)
	}

	installer := install.NewFlatcarInstaller(cmdRunner, logger)

	// Run headless install with a 30-minute wall-clock timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := headless.Run(ctx, cfg, installer, logger); err != nil {
		fmt.Fprintf(os.Stderr, "\n❌ %v\n", err)
		os.Exit(1)
	}

	// Handle reboot through runner (keeps DryRunner/SpyRunner semantics)
	if cfg.Reboot && !cfg.DryRun {
		if _, err := cmdRunner.Run(context.Background(), "systemctl", "reboot"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: reboot failed: %v\n", err)
			os.Exit(1)
		}
	}
}
