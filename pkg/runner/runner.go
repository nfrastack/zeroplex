// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package runner

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/modes"
	"zt-dns-companion/pkg/service"

	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// Daemon interface for managing daemon functionality
type Daemon interface {
	Start() error
	Stop()
	IsRunning() bool
	SetLogPrefix(string)
}

// SimpleDaemon implements basic daemon functionality
type SimpleDaemon struct {
	interval    time.Duration
	task        func(context.Context) error
	ticker      *time.Ticker
	stopChan    chan struct{}
	running     bool
	logPrefix   string
}

// NewSimpleDaemon creates a new daemon instance
func NewSimpleDaemon(interval time.Duration, task func(context.Context) error) *SimpleDaemon {
	return &SimpleDaemon{
		interval: interval,
		task:     task,
		stopChan: make(chan struct{}),
	}
}

func (d *SimpleDaemon) Start() error {
	if d.running {
		return fmt.Errorf("daemon already running")
	}

	d.running = true
	d.ticker = time.NewTicker(d.interval)

	go func() {
		for {
			select {
			case <-d.ticker.C:
				if err := d.task(context.Background()); err != nil {
					logger.Error("%sTask execution failed: %v", d.logPrefix, err)
				}
			case <-d.stopChan:
				d.ticker.Stop()
				return
			}
		}
	}()

	return nil
}

func (d *SimpleDaemon) Stop() {
	if !d.running {
		return
	}

	close(d.stopChan)
	d.running = false
}

func (d *SimpleDaemon) IsRunning() bool {
	return d.running
}

func (d *SimpleDaemon) SetLogPrefix(prefix string) {
	d.logPrefix = prefix
}

// Runner manages the execution of the ZT DNS Companion in both one-shot and daemon modes
type Runner struct {
	cfg      config.Config
	dryRun   bool
	ticker   *time.Ticker
	stopChan chan struct{}
	daemon   Daemon
}

// New creates a new runner instance
func New(cfg config.Config, dryRun bool) *Runner {
	return &Runner{
		cfg:      cfg,
		dryRun:   dryRun,
		stopChan: make(chan struct{}),
	}
}

// Run executes the application based on configuration
func (r *Runner) Run() error {
	logger.Trace("Runner.Run() started")

	// Show banner and startup message only when actually running
	r.showStartupBanner()

	// Validate runtime environment
	logger.Trace("Validating runtime environment")
	if err := r.validateEnvironment(); err != nil {
		logger.Error("Environment validation failed: %v", err)
		return err
	}
	logger.Trace("Runtime environment validation passed")

	// Auto-detect mode if needed
	if r.cfg.Default.Mode == "auto" {
		detectedMode, detected := service.DetectMode()
		if detected {
			r.cfg.Default.Mode = detectedMode
			logger.Verbose("Auto-detected mode: %s", detectedMode)
		} else {
			logger.Warn("Failed to auto-detect mode, keeping 'auto'")
		}
	} else {
		logger.Verbose("Using configured mode: %s", r.cfg.Default.Mode)
	}

	// Check if daemon mode is enabled
	if r.cfg.Default.DaemonMode {
		logger.Verbose("Starting in daemon mode")
		return r.runDaemon()
	} else {
		logger.Verbose("Starting in one-shot mode")
		return r.runOnce()
	}
}

// validateEnvironment checks if the runtime environment is suitable
func (r *Runner) validateEnvironment() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("ERROR You need to be root to run this program")
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("ERROR This tool is only needed on Linux")
	}

	return nil
}

// runOnce executes the application once and exits
func (r *Runner) runOnce() error {
	logger.Infof("Running in one-shot mode")
	return r.executeTask(context.Background())
}

// runDaemon starts the application in daemon mode
func (r *Runner) runDaemon() error {
	logger.Verbose("Running in daemon mode with interval: %s", r.cfg.Default.PollInterval)

	// Parse interval
	interval, err := time.ParseDuration(r.cfg.Default.PollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval: %w", err)
	}

	// Create daemon
	r.daemon = NewSimpleDaemon(interval, r.executeTask)
	r.daemon.SetLogPrefix("[zt-dns-daemon] ")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon
	if err := r.daemon.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Infof("Received signal %s, shutting down gracefully...", sig)

	// Stop daemon
	r.daemon.Stop()
	logger.Infof("Daemon stopped successfully")

	return nil
}

// executeTask performs the actual ZT DNS companion work
func (r *Runner) executeTask(ctx context.Context) error {
	if r.dryRun {
		logger.Infof("DRY RUN MODE: No actual changes will be made")
	}

	// Create the appropriate mode runner
	var modeRunner modes.ModeRunner
	var err error

	switch r.cfg.Default.Mode {
	case "networkd":
		modeRunner, err = modes.NewNetworkdMode(r.cfg, r.dryRun)
	case "resolved":
		modeRunner, err = modes.NewResolvedMode(r.cfg, r.dryRun)
	default:
		return fmt.Errorf("invalid mode: %s", r.cfg.Default.Mode)
	}

	if err != nil {
		return fmt.Errorf("failed to create mode runner: %w", err)
	}

	// Execute the mode-specific logic
	return modeRunner.Run(ctx)
}

// Stop gracefully stops the runner if it's in daemon mode
func (r *Runner) Stop() {
	if r.daemon != nil && r.daemon.IsRunning() {
		r.daemon.Stop()
	}
}

// showStartupBanner displays the application banner and startup message
func (r *Runner) showStartupBanner() {
	// Detect if running under systemd (import from main)
	invocation := os.Getenv("INVOCATION_ID") != ""
	journal := os.Getenv("JOURNAL_STREAM") != ""
	system := invocation || journal

	// Show banner if not running under systemd
	if !system {
		fmt.Println()
		fmt.Println("             .o88o.                                 .                       oooo")
		fmt.Println("             888 \"\"                                .o8                       888")
		fmt.Println("ooo. .oo.   o888oo  oooo d8b  .oooo.    .oooo.o .o888oo  .oooo.    .ooooo.   888  oooo")
		fmt.Println("`888P\"Y88b   888    `888\"\"8P `P  )88b  d88(  \"8   888   `P  )88b  d88' \"Y8  888 .8P'")
		fmt.Println(" 888   888   888     888      .oP\"888  \"\"Y88b.    888    .oP\"888  888        888888.")
		fmt.Println(" 888   888   888     888     d8(  888  o.  )88b   888 . d8(  888  888   .o8  888 `88b.")
		fmt.Println("o888o o888o o888o   d888b    `Y888\"\"8o 8\"\"888P'   \"888\" `Y888\"\"8o `Y8bod8P' o888o o888o")
		fmt.Println()
	}

	// TODO: Get version from build flags properly
	// For now, fallback to environment variable
	version := os.Getenv("ZT_DNS_VERSION")
	if version == "" {
		version = "development"
	}
	fmt.Printf("Starting ZeroTier DNS Companion version: %s\n", version)
}