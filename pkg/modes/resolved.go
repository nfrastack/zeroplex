// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zeroplex/pkg/config"
	"zeroplex/pkg/dns"
	"zeroplex/pkg/log"
	"zeroplex/pkg/utils"

	"context"
	"fmt"

	"github.com/zerotier/go-zerotier-one/service"
)

// ResolvedMode handles systemd-resolved integration
type ResolvedMode struct {
	*BaseMode
}

// NewResolvedMode creates a new resolved mode runner
func NewResolvedMode(cfg config.Config, dryRun bool) (*ResolvedMode, error) {
	logger := log.NewScopedLogger("[modes/resolved]", cfg.Default.Log.Level)
	// Verify systemd-resolved is available and running
	logger.Trace("Checking systemd-resolved service status")
	output, err := utils.ExecuteCommand("systemctl", "is-active", "systemd-resolved.service")
	if err != nil || output != "active\n" {
		logger.Error("systemd-resolved service check failed: %v", err)
		return nil, fmt.Errorf("systemd-resolved is not running")
	}
	logger.Debug("systemd-resolved service is active")

	// Verify resolvectl is available
	logger.Trace("Checking if resolvectl command is available")
	if !utils.CommandExists("resolvectl") {
		logger.Error("resolvectl command not found")
		return nil, fmt.Errorf("resolvectl is required for systemd-resolved but is not available")
	}
	logger.Trace("resolvectl command is available")

	return &ResolvedMode{
		BaseMode: NewBaseMode(cfg, dryRun, "resolved"),
	}, nil
}

// GetMode returns the mode name
func (r *ResolvedMode) GetMode() string {
	return "resolved"
}

// Run executes the resolved mode logic
func (r *ResolvedMode) Run(ctx context.Context) error {
	logger := log.NewScopedLogger("[modes/resolved]", r.GetConfig().Default.Log.Level)
	logger.Trace(">>> ResolvedMode.Run() started")
	logger.Debug("Running in resolved mode (dry-run: %t)", r.IsDryRun())

	// Use BaseMode.ProcessNetworks for all network fetching, logging, and filtering
	networks, err := r.ProcessNetworks(ctx)
	if err != nil {
		logger.Error("Failed to process networks: %v", err)
		// Restore DNS for all interfaces with saved state
		logger.Warn("Restoring DNS for all managed interfaces due to ZeroTier API/network failure")
		for _, iface := range dns.GetChangedInterfaces() {
			dns.RestoreSavedDNS(iface, r.GetConfig().Default.Log.Level)
		}
		return err
	}

	// Process networks for resolved
	logger.Debug("Processing networks for systemd-resolved configuration")
	logger.Trace("Calling processNetworks() for systemd-resolved integration")
	err = r.processNetworks(ctx, networks)
	if err != nil {
		logger.Error("Failed to process networks: %v", err)
		return err
	}

	logger.Trace("<<< ResolvedMode.Run() completed")
	return nil
}

// processNetworks handles the actual network processing for resolved
func (r *ResolvedMode) processNetworks(ctx context.Context, networks *service.GetNetworksResponse) error {
	// Call the resolved implementation, passing all relevant feature toggles
	RunResolvedMode(
		networks,
		r.GetConfig().Default.Features.AddReverseDomains,
		r.GetConfig().Default.Features.DNSOverTLS,
		r.GetConfig().Default.Features.MulticastDNS,
		r.IsDryRun(),
		r.GetConfig().Default.Log.Level,
	)
	return nil
}
