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

// NetworkdMode handles systemd-networkd integration
type NetworkdMode struct {
	*BaseMode
}

// NewNetworkdMode creates a new networkd mode runner
func NewNetworkdMode(cfg config.Config, dryRun bool) (*NetworkdMode, error) {
	// Verify systemd-networkd is available
	if !utils.ServiceExists("systemd-networkd.service") {
		return nil, fmt.Errorf("systemd-networkd.service is not available")
	}

	return &NetworkdMode{
		BaseMode: NewBaseMode(cfg, dryRun, "networkd"),
	}, nil
}

// GetMode returns the mode name
func (n *NetworkdMode) GetMode() string {
	return "networkd"
}

// Run executes the networkd mode logic
func (n *NetworkdMode) Run(ctx context.Context) error {
	logger.Trace(">>> NetworkdMode.Run() started")
	logger.Debugf("Running in networkd mode (dry-run: %t)", n.IsDryRun())
	logger.Verbose("Starting ZeroTier network discovery and processing")

	// Log configuration details
	n.LogConfiguration()
	logger.Verbose("DNS settings - OverTLS: %t, AutoRestart: %t, AddReverseDomains: %t, MulticastDNS: %t, Reconcile: %t",
		n.GetConfig().Default.DNSOverTLS, n.GetConfig().Default.AutoRestart, n.GetConfig().Default.AddReverseDomains,
		n.GetConfig().Default.MulticastDNS, n.GetConfig().Default.Reconcile)

	// Get ZeroTier networks
	logger.Trace("Fetching networks from ZeroTier API")
	logger.Verbose("Connecting to ZeroTier API for network discovery...")
	networks, err := n.FetchNetworks(ctx)
	if err != nil {
		logger.Error("Failed to fetch networks: %v", err)
		return fmt.Errorf("failed to fetch networks: %w", err)
	}

	// Log networks before filtering
	n.LogNetworkDiscovery(networks, true)

	// Apply filters
	logger.Trace("Applying network filters")
	logger.Verbose("Starting network filtering process...")
	n.ApplyFilters(networks)

	// Log networks after filtering
	n.LogNetworkDiscovery(networks, false)

	// Process networks for networkd
	logger.Verbose("Processing networks for systemd-networkd configuration")
	logger.Trace("Calling processNetworks() for systemd-networkd integration")
	err = n.processNetworks(ctx, networks)
	if err != nil {
		logger.Error("Failed to process networks: %v", err)
		return err
	}

	logger.Info("Networkd mode execution completed successfully")
	logger.Trace("<<< NetworkdMode.Run() completed")
	return nil
}

// processNetworks handles the actual network processing for networkd
func (n *NetworkdMode) processNetworks(ctx context.Context, networks *service.GetNetworksResponse) error {
	// Call the existing networkd implementation directly
	RunNetworkdMode(networks, n.GetConfig().Default.AddReverseDomains, n.GetConfig().Default.AutoRestart,
		n.GetConfig().Default.DNSOverTLS, n.IsDryRun(), n.GetConfig().Default.MulticastDNS, n.GetConfig().Default.Reconcile)

	return nil
}