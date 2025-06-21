// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package runner

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/daemon"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/modes"
	"zt-dns-companion/pkg/service"


	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// Runner manages the execution of the ZT DNS Companion in both one-shot and daemon modes
type Runner struct {
	cfg      config.Config
	daemon   *daemon.Daemon
	dryRun   bool
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
	// Validate runtime environment
	if err := r.validateEnvironment(); err != nil {
		return err
	}

	// Auto-detect mode if needed
	if r.cfg.Default.Mode == "auto" {
		detectedMode, detected := service.DetectMode()
		if detected {
			r.cfg.Default.Mode = detectedMode
			logger.Debugf("Auto-detected mode: %s", detectedMode)
		}
	}

	// Check if daemon mode is enabled
	if r.cfg.Default.DaemonMode {
		return r.runDaemon()
	} else {
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
	logger.Infof("Running in daemon mode with interval: %s", r.cfg.Default.DaemonInterval)

	// Create daemon
	d, err := daemon.New(r.cfg.Default.DaemonInterval, r.executeTask)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	d.SetLogPrefix("[zt-dns-daemon]")
	r.daemon = d

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon
	if err := d.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Infof("Received signal %s, shutting down gracefully...", sig)

	// Stop daemon
	d.Stop()
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