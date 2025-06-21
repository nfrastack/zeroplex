// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"

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
	// Verify systemd-resolved is available and running
	logger.Trace("Checking systemd-resolved service status")
	output, err := utils.ExecuteCommand("systemctl", "is-active", "systemd-resolved.service")
	if err != nil || output != "active\n" {
		logger.Errorf("systemd-resolved service check failed: %v", err)
		return nil, fmt.Errorf("systemd-resolved is not running")
	}
	logger.Debugf("systemd-resolved service is active")

	// Verify resolvectl is available
	logger.Trace("Checking if resolvectl command is available")
	if !utils.CommandExists("resolvectl") {
		logger.Errorf("resolvectl command not found")
		return nil, fmt.Errorf("resolvectl is required for systemd-resolved but is not available")
	}
	logger.Debugf("resolvectl command is available")

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
	logger.Trace(">>> ResolvedMode.Run() started")
	logger.Debugf("Running in resolved mode (dry-run: %t)", r.IsDryRun())
	logger.Verbose("Starting ZeroTier network discovery and processing for systemd-resolved")

	// Log configuration details
	r.LogConfiguration()
	logger.Verbose("DNS settings - AddReverseDomains: %t", r.GetConfig().Default.AddReverseDomains)

	// Get ZeroTier networks
	logger.Trace("Fetching networks from ZeroTier API")
	logger.Verbose("Connecting to ZeroTier API for network discovery...")
	networks, err := r.FetchNetworks(ctx)
	if err != nil {
		logger.Error("Failed to fetch networks: %v", err)
		return fmt.Errorf("failed to fetch networks: %w", err)
	}

	// Log networks before filtering
	r.LogNetworkDiscovery(networks, true)

	// Apply filters
	logger.Trace("Applying network filters")
	logger.Verbose("Starting network filtering process...")
	r.ApplyFilters(networks)

	// Log networks after filtering
	r.LogNetworkDiscovery(networks, false)

	// Process networks for resolved
	logger.Verbose("Processing networks for systemd-resolved configuration")
	logger.Trace("Calling processNetworks() for systemd-resolved integration")
	err = r.processNetworks(ctx, networks)
	if err != nil {
		logger.Error("Failed to process networks: %v", err)
		return err
	}

	logger.Info("Resolved mode execution completed successfully")
	logger.Trace("<<< ResolvedMode.Run() completed")
	return nil
}

// processNetworks handles the actual network processing for resolved
func (r *ResolvedMode) processNetworks(ctx context.Context, networks *service.GetNetworksResponse) error {
	// Call the existing resolved implementation directly
	RunResolvedMode(networks, r.GetConfig().Default.AddReverseDomains, r.IsDryRun())
	return nil
}