// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package runner

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/daemon"
	"zt-dns-companion/pkg/logging"
	"zt-dns-companion/pkg/modes"
	"zt-dns-companion/pkg/service"
	"zt-dns-companion/pkg/utils"

	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// Runner manages the execution of the ZT DNS Companion in both one-shot and daemon modes
type Runner struct {
	cfg      config.Config
	dryRun   bool
	daemon   daemon.Interface
}

// New creates a new runner instance
func New(cfg config.Config, dryRun bool) *Runner {
	return &Runner{
		cfg:    cfg,
		dryRun: dryRun,
	}
}

// Run executes the application based on configuration
func (r *Runner) Run() error {
	logging.RunnerLogger.Trace("Runner.Run() started")

	// Show banner and startup message only when actually running
	r.showStartupBanner()

	// Validate runtime environment
	logging.RunnerLogger.Trace("Validating runtime environment")
	if err := r.validateEnvironment(); err != nil {
		logging.RunnerLogger.Error("Environment validation failed: %v", err)
		return err
	}
	logging.RunnerLogger.Trace("Runtime environment validation passed")

	// Auto-detect mode if needed
	if r.cfg.Default.Mode == "auto" {
		detectedMode, detected := service.DetectMode()
		if detected {
			r.cfg.Default.Mode = detectedMode
			logging.RunnerLogger.Verbose("Auto-detected mode: %s", detectedMode)
		} else {
			logging.RunnerLogger.Warn("Failed to auto-detect mode, keeping 'auto'")
		}
	} else {
		logging.RunnerLogger.Verbose("Using configured mode: %s", r.cfg.Default.Mode)
	}

	// Check if daemon mode is enabled
	if r.cfg.Default.DaemonMode {
		logging.RunnerLogger.Verbose("Starting in daemon mode")
		return r.runDaemon()
	} else {
		logging.RunnerLogger.Verbose("Starting in one-shot mode")
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
	logging.RunnerLogger.Info("Running in one-shot mode")
	return r.executeTask(context.Background())
}

// runDaemon starts the application in daemon mode
func (r *Runner) runDaemon() error {
	logging.RunnerLogger.Verbose("Running in daemon mode with interval: %s", r.cfg.Default.PollInterval)

	// Parse interval
	interval, err := time.ParseDuration(r.cfg.Default.PollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval: %w", err)
	}

	// Create daemon
	r.daemon = daemon.NewSimple(interval, r.executeTask)
	r.daemon.SetLogPrefix("[daemon] ")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon
	if err := r.daemon.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	logging.RunnerLogger.Info("Received signal %s, shutting down gracefully...", sig)

	// Stop daemon
	r.daemon.Stop()
	logging.RunnerLogger.Info("Daemon stopped successfully")

	return nil
}

// executeTask performs the actual ZT DNS companion work
func (r *Runner) executeTask(ctx context.Context) error {
	taskLogger := logging.GetSubLogger("runner", "task")
	
	if r.dryRun {
		taskLogger.Info("DRY RUN MODE: No actual changes will be made")
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
	// Show banner if not running under systemd
	if !utils.IsRunningUnderSystemd() {
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

	fmt.Printf("Starting ZeroTier DNS Companion version: %s\n", utils.GetVersion())
}