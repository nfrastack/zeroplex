// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package runner

import (
	"zeroplex/pkg/config"
	"zeroplex/pkg/daemon"
	"zeroplex/pkg/dns"
	"zeroplex/pkg/log"
	"zeroplex/pkg/modes"
	"zeroplex/pkg/utils"

	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

// Runner manages the execution of the ZeroPlex in both one-shot and daemon modes
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
		logger: log.NewScopedLogger("[runner]", cfg.Default.Log.Level),
	}
}

// Run executes the application based on configuration
func (r *Runner) Run() error {
	r.logger.Info("[debug] Entered Runner.Run() (TOP)")
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

	// Start DNS watchdog if enabled
	go r.startDNSWatchdog()

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
	r.logger.Verbose("Running in daemon mode with interval: %s", r.cfg.Default.Daemon.PollInterval)

	// Start D-Bus sleep/resume watcher with structured logging
	r.logger.Debug("About to start sleep watcher goroutine (PRE)")
	go func(logger func(string, ...interface{})) {
		ctx := context.Background()
		StartSleepResumeWatcher(ctx, logger, func() {
			r.logger.Verbose("System resume detected (D-Bus), triggering DNS/interface re-check with backoff")
			go r.retryUntilDNSOk(context.Background(), "resume event")
		})
	}(r.logger.Debug)
	r.logger.Debug("After starting sleep watcher goroutine (POST)")

	// Start interface watcher if enabled
	r.logger.Debug("Interface watch mode: %s", r.cfg.Default.InterfaceWatch.Mode)
	if r.cfg.Default.InterfaceWatch.Mode == "event" {
		r.ifaceWatchStop = make(chan struct{})
		err := utils.WatchInterfacesNetlink(r.handleInterfaceEvent, r.ifaceWatchStop, r.cfg.Default.Log.Level)
		if err != nil {
			r.logger.Error("Netlink watcher failed: %v. Falling back to polling mode.", err)
			go utils.PollInterfaces(5*time.Second, r.handleInterfaceEvent, r.ifaceWatchStop, r.cfg.Default.Log.Level)
		}
	} else if r.cfg.Default.InterfaceWatch.Mode == "poll" {
		r.ifaceWatchStop = make(chan struct{})
		go utils.PollInterfaces(5*time.Second, r.handleInterfaceEvent, r.ifaceWatchStop, r.cfg.Default.Log.Level)
		// No error to check for goroutine
		// Optionally log after a short delay
	}

	// Parse interval
	interval, err := time.ParseDuration(r.cfg.Default.Daemon.PollInterval)
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
	if r.cfg.Default.Features.RestoreOnExit {
		r.logger.Info("restore_on_exit enabled: restoring DNS for all managed interfaces...")
		saved := dns.GetSavedDNSState()
		for iface := range saved {
			r.logger.Info("Restoring DNS for interface %s", iface)
			dns.RestoreSavedDNS(iface, r.cfg.Default.Log.Level)
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

func (r *Runner) executeTask(ctx context.Context) error {
	taskLogger := log.NewScopedLogger("[runner/task]", r.cfg.Default.Log.Level)

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
	isZT := strings.HasPrefix(ev.Name, "zt") // Only act on ZeroTier interfaces
	if isZT {
		r.logger.Info("ZeroTier interface %s event (%s), checking readiness and applying DNS if ready", ev.Name, ev.Type)
		retryCfg := r.cfg.Default.InterfaceWatch.Retry
		var backoffSeq []time.Duration
		if len(retryCfg.Backoff) > 0 {
			for _, s := range retryCfg.Backoff {
				d, err := time.ParseDuration(s)
				if err == nil {
					backoffSeq = append(backoffSeq, d)
				}
			}
		}
		maxTotal := 2 * time.Minute
		if retryCfg.MaxTotal != "" {
			if d, err := time.ParseDuration(retryCfg.MaxTotal); err == nil {
				maxTotal = d
			}
		}
		startTime := time.Now()
		var lastErr error
		attempt := 0
		for {
			if len(backoffSeq) > 0 {
				if attempt >= len(backoffSeq) {
					break
				}
			} else {
				if attempt > retryCfg.Count {
					break
				}
			}
			if time.Since(startTime) > maxTotal {
				r.logger.Warn("ZeroTier interface %s did not become ready after %.0fs (max_total), skipping DNS apply", ev.Name, maxTotal.Seconds())
				break
			}
			ready, status, err := isZTInterfaceReady(r.cfg, ev.Name)
			if err != nil {
				lastErr = err
				// Log detailed diagnostics for readiness errors
				if status == "iface_not_found" {
					r.logger.Warn("[retry %d] Interface %s not found: %v", attempt+1, ev.Name, err)
				} else if status == "iface_down" {
					r.logger.Warn("[retry %d] Interface %s exists but is down", attempt+1, ev.Name)
				} else if status == "api_unreachable" {
					r.logger.Warn("[retry %d] ZeroTier API unreachable for %s: %v", attempt+1, ev.Name, err)
				} else {
					r.logger.Warn("[retry %d] Error checking ZeroTier interface %s readiness (status=%s): %v", attempt+1, ev.Name, status, err)
				}
			} else if ready {
				r.logger.Info("ZeroTier interface %s is ready (status=%s), applying DNS", ev.Name, status)
				_ = r.executeTask(context.Background())
				r.logger.Info("DNS applied for ZeroTier interface %s after %d attempt(s), total wait %.1fs", ev.Name, attempt+1, time.Since(startTime).Seconds())
				return
			} else {
				if attempt == 0 || (len(backoffSeq) > 0 && attempt == len(backoffSeq)-1) || (len(backoffSeq) == 0 && attempt == retryCfg.Count) || attempt%3 == 0 {
					r.logger.Debug("[retry %d] ZeroTier interface %s not ready (status=%s), will retry", attempt+1, ev.Name, status)
				}
			}
			var d time.Duration
			if len(backoffSeq) > 0 {
				d = backoffSeq[attempt]
			} else {
				baseDelay, err := time.ParseDuration(retryCfg.Delay)
				if err != nil || baseDelay <= 0 {
					baseDelay = 2 * time.Second
				}
				maxDelay := 1 * time.Minute
				d = baseDelay << attempt // exponential backoff
				if d > maxDelay {
					d = maxDelay
				}
			}
			time.Sleep(d)
			attempt++
		}
		if lastErr != nil {
			r.logger.Warn("ZeroTier interface %s did not become ready after %d retries, last error: %v", ev.Name, attempt, lastErr)
		} else {
			r.logger.Warn("ZeroTier interface %s did not become ready after %d retries, skipping DNS apply", ev.Name, attempt)
		}
	} else {
		r.logger.Trace("Non-ZeroTier interface %s event (%s), ignoring", ev.Name, ev.Type)
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
	if r.cfg.Default.Log.Timestamps {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("%s Starting ZeroPlex version: %s\n", timestamp, utils.GetVersion())
	} else {
		fmt.Printf("Starting ZeroPlex version: %s\n", utils.GetVersion())
	}
}

// startDNSWatchdog launches a goroutine that pings the watchdog_ip and triggers a poll on failure
func (r *Runner) startDNSWatchdog() {
	cfg := r.cfg.Default.Features
	interval := time.Minute
	if cfg.WatchdogInterval != "" {
		if d, err := time.ParseDuration(cfg.WatchdogInterval); err == nil {
			interval = d
		}
	}
	backoff := []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second}
	if len(cfg.WatchdogBackoff) > 0 {
		parsed := []time.Duration{}
		for _, s := range cfg.WatchdogBackoff {
			if d, err := time.ParseDuration(s); err == nil {
				parsed = append(parsed, d)
			}
		}
		if len(parsed) > 0 {
			backoff = parsed
		}
	}
	var watchdogIP string = cfg.WatchdogIP
	if watchdogIP == "" {
		if len(r.cfg.Default.Client.Host) > 0 {
			watchdogIP = r.cfg.Default.Client.Host
		}
	}
	watchdogHostname := cfg.WatchdogHostname
	watchdogExpectedIP := cfg.WatchdogExpectedIP
	if strings.Contains(watchdogHostname, "%domain%") {
		networks, err := getZTNetworksDomains(r.cfg)
		if err != nil {
			r.logger.Warn("DNS watchdog: failed to get ZeroTier domains for %%domain%% substitution: %v", err)
			return
		}
		if len(networks) == 0 {
			r.logger.Warn("DNS watchdog: no ZeroTier networks with domains found for %%domain%% substitution")
			return
		}
		for _, netinfo := range networks {
			host := strings.ReplaceAll(watchdogHostname, "%domain%", netinfo.Domain)
			r.logger.Info("DNS watchdog (hostname) for interface %s: Hostname=%s, ExpectedIP=%s, interval=%s, backoff=%v", netinfo.Interface, host, watchdogExpectedIP, interval, backoff)
			go func(host, expectedIP, iface string) {
				for {
					ok := false
					ips, err := net.LookupHost(host)
					if err == nil {
						for _, ip := range ips {
							if ip == expectedIP {
								ok = true
								break
							}
						}
					}
					if ok {
						r.logger.Trace("DNS watchdog: %s resolves to %s", host, expectedIP)
						time.Sleep(interval)
						continue
					}
					r.logger.Warn("DNS watchdog: %s does not resolve to %s (got: %v, err: %v), triggering poll and backoff", host, expectedIP, ips, err)
					go r.retryUntilDNSOk(context.Background(), "watchdog-hostname failure")
					for _, bo := range backoff {
						ips, err := net.LookupHost(host)
						ok := false
						if err == nil {
							for _, ip := range ips {
								if ip == expectedIP {
									ok = true
									break
								}
							}
						}
						if ok {
							r.logger.Info("DNS watchdog: %s resolves to %s after backoff", host, expectedIP)
							break
						}
						r.logger.Warn("DNS watchdog: %s still does not resolve to %s, waiting %s", host, expectedIP, bo)
						_ = r.executeTask(context.Background())
						time.Sleep(bo)
					}
				}
			}(host, watchdogExpectedIP, netinfo.Interface)
		}
		return
	} else if watchdogIP != "" {
		r.logger.Info("DNS watchdog enabled: IP=%s, interval=%s, backoff=%v", watchdogIP, interval, backoff)
		for {
			if utils.Ping(watchdogIP) {
				r.logger.Trace("DNS watchdog: %s is reachable", watchdogIP)
				time.Sleep(interval)
				continue
			}
			r.logger.Warn("DNS watchdog: %s unreachable, triggering poll and backoff", watchdogIP)
			go r.retryUntilDNSOk(context.Background(), "watchdog-ip failure")
			for _, bo := range backoff {
				if utils.Ping(watchdogIP) {
					r.logger.Info("DNS watchdog: %s is reachable after backoff", watchdogIP)
					break
				}
				r.logger.Warn("DNS watchdog: %s still unreachable, waiting %s", watchdogIP, bo)
				_ = r.executeTask(context.Background())
				time.Sleep(bo)
			}
		}
	} else {
		r.logger.Warn("No watchdog_ip or hostname configured and no DNS server found; DNS watchdog disabled")
		return
	}
}

// retryUntilDNSOk aggressively retries DNS/interface re-checks with backoff until success or max retries/time.
func (r *Runner) retryUntilDNSOk(ctx context.Context, reason string) {
	r.logger.Debug("retryUntilDNSOk called with reason: %s", reason)
	retryCfg := r.cfg.Default.InterfaceWatch.Retry
	var backoffSeq []time.Duration
	if len(retryCfg.Backoff) > 0 {
		for _, s := range retryCfg.Backoff {
			d, err := time.ParseDuration(s)
			if err == nil {
				backoffSeq = append(backoffSeq, d)
			}
		}
	}
	maxTotal := 2 * time.Minute
	if retryCfg.MaxTotal != "" {
		if d, err := time.ParseDuration(retryCfg.MaxTotal); err == nil {
			maxTotal = d
		}
	}
	startTime := time.Now()
	attempt := 0
	for {
		if len(backoffSeq) > 0 {
			if attempt >= len(backoffSeq) {
				break
			}
		} else {
			if attempt > retryCfg.Count {
				break
			}
		}
		if time.Since(startTime) > maxTotal {
			r.logger.Warn("%s: did not succeed after %.0fs (max_total), giving up", reason, maxTotal.Seconds())
			break
		}
		err := r.executeTask(ctx)
		if err == nil {
			r.logger.Verbose("%s: DNS/interface re-check succeeded after %d attempt(s), total wait %.1fs", reason, attempt+1, time.Since(startTime).Seconds())
			return
		} else {
			r.logger.Warn("%s: attempt %d failed: %v", reason, attempt+1, err)
		}
		var d time.Duration
		if len(backoffSeq) > 0 {
			d = backoffSeq[attempt]
		} else {
			baseDelay, err := time.ParseDuration(retryCfg.Delay)
			if err != nil || baseDelay <= 0 {
				baseDelay = 2 * time.Second
			}
			maxDelay := 1 * time.Minute
			d = baseDelay << attempt // exponential backoff
			if d > maxDelay {
				d = maxDelay
			}
		}
		time.Sleep(d)
		attempt++
	}
}

// StartSleepResumeWatcher listens for system sleep/resume events and triggers the callback on resume.
// Accepts a logger for consistent logging.
func StartSleepResumeWatcher(ctx context.Context, logger func(msg string, args ...interface{}), onResume func()) {
	logger("Sleep watcher goroutine started")
	defer func() {
		if r := recover(); r != nil {
			logger("PANIC: %v", r)
		}
	}()
	conn, err := dbus.SystemBus()
	if err != nil {
		logger("Failed to connect to system D-Bus: %v", err)
		return
	}
	ch := make(chan *dbus.Signal, 10)
	conn.Signal(ch)
	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
		dbus.WithMatchSender("org.freedesktop.login1"),
	)
	if err != nil {
		logger("Failed to add D-Bus match rule: %v", err)
		return
	}
	logger("Subscribed to D-Bus signals for org.freedesktop.login1.Manager/PrepareForSleep on /org/freedesktop/login1")
	for {
		select {
		case sig := <-ch:
			// Only check Path, Name, and Body
			logger("D-Bus signal received: Path=%v, Member=%v, Sender=%v, Body=%+v", sig.Path, sig.Name, sig.Sender, sig.Body)
			if sig.Path == "/org/freedesktop/login1" && sig.Name == "org.freedesktop.login1.Manager.PrepareForSleep" && len(sig.Body) == 1 {
				if sleeping, ok := sig.Body[0].(bool); ok {
					if sleeping {
						logger("System is preparing to sleep (PrepareForSleep=true)")
					} else {
						logger("System resume detected via D-Bus (PrepareForSleep=false)")
						defer func() {
							if r := recover(); r != nil {
								logger("PANIC in onResume callback: %v", r)
							}
						}()
						onResume()
						logger("onResume callback returned")
					}
				}
			}
		case <-ctx.Done():
			logger("Sleep watcher goroutine exiting due to context cancellation")
			return
		}
	}
}

// ZTNetworkInfo and getZTNetworksDomains merged from zt_domains.go

type ZTNetworkInfo struct {
	Interface string
	Domain    string
}

func getZTNetworksDomains(cfg config.Config) ([]ZTNetworkInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s:%d/networks", strings.TrimRight(cfg.Default.Client.Host, "/"), cfg.Default.Client.Port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	token := cfg.Default.Client.TokenFile
	if token != "" {
		content, err := os.ReadFile(token)
		if err == nil {
			req.Header.Add("X-ZT1-Auth", strings.TrimSpace(string(content)))
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var networks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return nil, err
	}
	var result []ZTNetworkInfo
	for _, nw := range networks {
		iface, _ := nw["portDeviceName"].(string)
		dns, _ := nw["dns"].(map[string]interface{})
		var domain string
		if dns != nil {
			domain, _ = dns["domain"].(string)
		}
		if iface != "" && domain != "" {
			result = append(result, ZTNetworkInfo{Interface: iface, Domain: domain})
		}
	}
	return result, nil
}

// isZTInterfaceReady merged from zt_ready.go

func isZTInterfaceReady(cfg config.Config, ifaceName string) (bool, string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return false, "iface_not_found", fmt.Errorf("interface %s not found: %w", ifaceName, err)
	}
	if iface.Flags&net.FlagUp == 0 {
		return false, "iface_down", fmt.Errorf("interface %s exists but is down", ifaceName)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s:%d/networks", strings.TrimRight(cfg.Default.Client.Host, "/"), cfg.Default.Client.Port)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, "api_error", err
	}
	token := cfg.Default.Client.TokenFile
	if token != "" {
		content, err := os.ReadFile(token)
		if err == nil {
			req.Header.Add("X-ZT1-Auth", strings.TrimSpace(string(content)))
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, "api_unreachable", fmt.Errorf("ZeroTier API unreachable: %w (iface %s is up)", err, ifaceName)
	}
	defer resp.Body.Close()
	var networks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
		return false, "api_decode_error", err
	}
	for _, nw := range networks {
		if nw["portDeviceName"] == ifaceName {
			status, _ := nw["status"].(string)
			dns, _ := nw["dns"].(map[string]interface{})
			servers, _ := dns["servers"].([]interface{})
			if status == "OK" && len(servers) > 0 {
				return true, status, nil
			}
			return false, status, nil
		}
	}
	return false, "not_found", nil
}
