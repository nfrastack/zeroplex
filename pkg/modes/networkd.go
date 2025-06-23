// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zeroflex/pkg/config"
	"zeroflex/pkg/log"
	"zeroflex/pkg/utils"

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
	logger := log.NewScopedLogger("[modes/networkd]", "info")
	// Verify systemd-networkd is available
	if !utils.ServiceExists("systemd-networkd.service") {
		logger.Error("systemd-networkd.service is not available")
		return nil, fmt.Errorf("systemd-networkd.service is not available")
	}

	return &NetworkdMode{
		BaseMode: NewBaseMode(cfg, dryRun, "networkd"),
	}, nil
}

// GetMode returns the mode name
func (n *NetworkdMode) GetMode() string {
	logger := log.NewScopedLogger("[modes/networkd]", "info")
	logger.Trace("GetMode called")
	return "networkd"
}

// Run executes the networkd mode logic
func (n *NetworkdMode) Run(ctx context.Context) error {
	logger := log.NewScopedLogger("[modes/networkd]", n.GetConfig().Default.Log.Level)
	logger.Trace(">>> NetworkdMode.Run() started")
	logger.Debug("Running in networkd mode (dry-run: %t)", n.IsDryRun())

	// Log configuration details
	n.LogConfiguration()
	logger.Debug("DNS settings - OverTLS: %t, AutoRestart: %t, AddReverseDomains: %t, MulticastDNS: %t, Reconcile: %t",
		n.GetConfig().Default.Features.DNSOverTLS, n.GetConfig().Default.Networkd.AutoRestart, n.GetConfig().Default.Features.AddReverseDomains,
		n.GetConfig().Default.Features.MulticastDNS, n.GetConfig().Default.Networkd.Reconcile)

	// Use BaseMode.ProcessNetworks for all network fetching, logging, and filtering
	networks, err := n.ProcessNetworks(ctx)
	if err != nil {
		logger.Error("Failed to process networks: %v", err)
		return fmt.Errorf("failed to process networks: %w", err)
	}

	// Process networks for networkd
	logger.Verbose("Processing networks for systemd-networkd configuration")
	logger.Trace("Calling processNetworks() for systemd-networkd integration")
	err = n.processNetworks(ctx, networks)
	if err != nil {
		logger.Error("Failed to process networks: %v", err)
		return err
	}

	logger.Trace("<<< NetworkdMode.Run() completed")
	return nil
}

// processNetworks handles the actual network processing for networkd
func (n *NetworkdMode) processNetworks(ctx context.Context, networks *service.GetNetworksResponse) error {
	logger := log.NewScopedLogger("[modes/networkd]", "info")
	logger.Trace("processNetworks called")
	// Call the existing networkd implementation directly
	RunNetworkdMode(networks, n.GetConfig().Default.Features.AddReverseDomains, n.GetConfig().Default.Networkd.AutoRestart,
		n.GetConfig().Default.Features.DNSOverTLS, n.IsDryRun(), n.GetConfig().Default.Features.MulticastDNS, n.GetConfig().Default.Networkd.Reconcile)

	return nil
}
