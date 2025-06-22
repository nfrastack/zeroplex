// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package runner

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/daemon"
	"zt-dns-companion/pkg/log"
	"zt-dns-companion/pkg/modes"
	"zt-dns-companion/pkg/utils"
	"zt-dns-companion/pkg/dns"

	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Runner manages the execution of the ZT DNS Companion in both one-shot and daemon modes
type Runner struct {
	cfg            config.Config
	dryRun         bool
	daemon         daemon.Interface
	logger         *log.Logger
	ifaceWatchStop chan struct{} // for stopping interface watcher
}

// New creates a new runner instance
func New(cfg config.Config, dryRun bool) *Runner {
	return &Runner{
		cfg:    cfg,
		dryRun: dryRun,
		logger: log.NewScopedLogger("[runner]", cfg.Default.LogLevel),
	}
}

// Run executes the application based on configuration
func (r *Runner) Run() error {
	r.logger.Info("[debug] Entered Runner.Run()")
	r.logger.Trace("Runner.Run() started")

	// Show banner and startup message only when actually running
	r.ShowStartupBanner()

	// Validate runtime environment
	r.logger.Trace("Validating runtime environment")
	if err := r.validateEnvironment(); err != nil {
		r.logger.Error("Environment validation failed: %v", err)
		return err
	}
	r.logger.Trace("Runtime environment validation passed")

	// Auto-detect mode if needed
	if r.cfg.Default.Mode == "auto" {
		detectedMode, detected := r.detectMode()
		if detected {
			r.cfg.Default.Mode = detectedMode
			r.logger.Info("Auto-detected mode: %s", detectedMode)
		} else {
			r.logger.Warn("Failed to auto-detect mode, keeping 'auto'")
		}
	} else {
		r.logger.Info("Using configured mode: %s", r.cfg.Default.Mode)
	}

	r.logger.Info("[debug] Exiting Runner.Run() (should not happen in daemon mode)")
	return nil
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

// detectMode automatically detects which systemd service is running
func (r *Runner) detectMode() (string, bool) {
	r.logger.Trace("DetectMode() - checking systemd services")

	r.logger.Debug("Checking systemd-networkd.service status...")
	networkdOutput, networkdErr := utils.ExecuteCommand("systemctl", "is-active", "systemd-networkd.service")
	networkdActive := networkdErr == nil && strings.TrimSpace(networkdOutput) == "active"
	r.logger.Debug("systemd-networkd.service active: %t", networkdActive)

	r.logger.Debug("Checking systemd-resolved.service status...")
	resolvedOutput, resolvedErr := utils.ExecuteCommand("systemctl", "is-active", "systemd-resolved.service")
	resolvedActive := resolvedErr == nil && strings.TrimSpace(resolvedOutput) == "active"
	r.logger.Debug("systemd-resolved.service active: %t", resolvedActive)

	if networkdActive {
		return "networkd", true
	} else if resolvedActive {
		return "resolved", true
	} else {
		r.logger.Error("Neither systemd-networkd nor systemd-resolved is running")
		utils.ErrorHandler("Neither systemd-networkd nor systemd-resolved is running. Please manually set the mode using the -mode flag or configuration file.", nil, true)
		return "", false
	}
}

// DetectMode exposes the detectMode method for external use
func (r *Runner) DetectMode() (string, bool) {
	return r.detectMode()
}

// runOnce executes the application once and exits
func (r *Runner) runOnce() error {
	r.logger.Info("Running in one-shot mode")
	return r.executeTask(context.Background())
}

// RunOnce executes the application once and exits
func (r *Runner) RunOnce() error {
	return r.runOnce()
}

// runDaemon starts the application in daemon mode
func (r *Runner) runDaemon() error {
	r.logger.Verbose("Running in daemon mode with interval: %s", r.cfg.Default.PollInterval)

	// Start interface watcher if enabled
	r.logger.Debug("Interface watch mode: %s", r.cfg.Default.InterfaceWatch.Mode)
	if r.cfg.Default.InterfaceWatch.Mode == "event" {

		r.ifaceWatchStop = make(chan struct{})
		err := utils.WatchInterfacesNetlink(r.handleInterfaceEvent, r.ifaceWatchStop, r.cfg.Default.LogLevel)
		if err != nil {
			r.logger.Error("Netlink watcher failed: %v. Falling back to polling mode.", err)
			go utils.PollInterfaces(5*time.Second, r.handleInterfaceEvent, r.ifaceWatchStop, r.cfg.Default.LogLevel)
		}
	} else if r.cfg.Default.InterfaceWatch.Mode == "poll" {
		r.ifaceWatchStop = make(chan struct{})
		go utils.PollInterfaces(5*time.Second, r.handleInterfaceEvent, r.ifaceWatchStop, r.cfg.Default.LogLevel)
		// No error to check for goroutine
		// Optionally log after a short delay
	}

	// Parse interval
	interval, err := time.ParseDuration(r.cfg.Default.PollInterval)
	if err != nil {
		return fmt.Errorf("invalid poll interval: %w", err)
	}

	// Create daemon
	r.daemon = daemon.NewSimple(interval, r.executeTask)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start daemon
	if err := r.daemon.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	r.logger.Info("Received signal %s, shutting down gracefully...", sig)

	// If restore_on_exit is enabled, restore DNS for all managed interfaces
	if r.cfg.Default.RestoreOnExit {
		r.logger.Info("restore_on_exit enabled: restoring DNS for all managed interfaces...")
		saved := dns.GetSavedDNSState()
		for iface := range saved {
			r.logger.Info("Restoring DNS for interface %s", iface)
			dns.RestoreSavedDNS(iface, r.cfg.Default.LogLevel)
		}
	}

	// Stop daemon
	r.daemon.Stop()
	return nil
}

// RunDaemon starts the application in daemon mode
func (r *Runner) RunDaemon() error {
	return r.runDaemon()
}

// executeTask performs the actual ZT DNS companion work
func (r *Runner) executeTask(ctx context.Context) error {
	taskLogger := log.NewScopedLogger("[runner/task]", r.cfg.Default.LogLevel)

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

	// Stop interface watcher if running
	if r.ifaceWatchStop != nil {
		close(r.ifaceWatchStop)
		r.ifaceWatchStop = nil
	}
}

// handleInterfaceEvent is called on interface add/remove/up/down
func (r *Runner) handleInterfaceEvent(ev utils.InterfaceEvent) {
	switch ev.Type {
	case utils.InterfaceRemoved:
		if dns.RestoreSavedDNS(ev.Name, r.cfg.Default.LogLevel) {
			r.logger.Info("Interface %s removed, DNS reverted to original settings", ev.Name)
		}
		// Optionally, trigger a poll/update here
	case utils.InterfaceAdded:
		r.logger.Info("Interface %s added, triggering DNS update", ev.Name)
		// Optionally, trigger a poll/update here
	}
}

// ShowStartupBanner displays the application banner and startup message
func (r *Runner) ShowStartupBanner() {
	fmt.Println()
	fmt.Println("             .o88o.                                 .                       oooo")
	fmt.Println("             888 \"\"                                .o8                       888")
	fmt.Println("ooo. .oo.   o888oo  oooo d8b  .oooo.    .oooo.o .o888oo  .oooo.    .ooooo.   888  oooo")
	fmt.Println("`888P\"Y88b   888    `888\"\"8P `P  )88b  d88(  \"8   888   `P  )88b  d88' \"Y8  888 .8P'")
	fmt.Println(" 888   888   888     888      .oP\"888  \"\"Y88b.    888    .oP\"888  888        888888.")
	fmt.Println(" 888   888   888     888     d8(  888  o.  )88b   888 . d8(  888  888   .o8  888 `88b.")
	fmt.Println("o888o o888o o888o   d888b    `Y888\"\"8o 8\"\"888P'   \"888\" `Y888\"\"8o `Y8bod8P' o888o o888o")
	fmt.Println()
	if r.cfg.Default.LogTimestamps {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("%s Starting ZeroTier DNS Companion version: %s\n", timestamp, utils.GetVersion())
	} else {
		fmt.Printf("Starting ZeroTier DNS Companion version: %s\n", utils.GetVersion())
	}
}