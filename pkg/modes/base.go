// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zeroflex/pkg/client"
	"zeroflex/pkg/config"
	"zeroflex/pkg/filters"
	"zeroflex/pkg/log"
	"zeroflex/pkg/utils"

	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/zerotier/go-zerotier-one/service"
)

// BaseMode provides common functionality for all mode implementations
type BaseMode struct {
	cfg    config.Config
	dryRun bool
	mode   string
}

// NewBaseMode creates a new base mode instance
func NewBaseMode(cfg config.Config, dryRun bool, mode string) *BaseMode {
	return &BaseMode{
		cfg:    cfg,
		dryRun: dryRun,
		mode:   mode,
	}
}

// FetchNetworks retrieves networks from ZeroTier API
func (b *BaseMode) FetchNetworks(ctx context.Context) (*service.GetNetworksResponse, error) {
	logger := log.NewScopedLogger("[api]", b.cfg.Default.LogLevel)

	// Create API client
	sAPI, err := client.NewServiceAPI(b.cfg.Default.TokenFile)
	if err != nil {
		logger.Error("Failed to create service API client: %v", err)
		return nil, fmt.Errorf("failed to create service API client: %w", err)
	}

	// Create ZeroTier client
	ztBaseURL := fmt.Sprintf("%s:%d", b.cfg.Default.Host, b.cfg.Default.Port)
	logger.Debug("Creating ZeroTier client with URL: %s", ztBaseURL)
	ztClient, err := service.NewClient(ztBaseURL, service.WithHTTPClient(sAPI))
	if err != nil {
		logger.Error("Failed to create ZeroTier client: %v", err)
		return nil, fmt.Errorf("failed to create ZeroTier client: %w", err)
	}

	// Fetch networks
	logger.Trace("Making API request to fetch networks (GET %s/networks)", ztBaseURL)
	resp, err := ztClient.GetNetworks(ctx)
	if err != nil {
		logger.Error("Failed to get networks: %v (could not access the ZeroTier API server)", err)
		return nil, fmt.Errorf("failed to get networks: %w", err)
	}

	// Log raw response body (truncate if very large)
	var respBodyBytes []byte
	if resp != nil && resp.Body != nil {
		respBodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Failed to read API response body: %v", err)
			return nil, fmt.Errorf("failed to read API response body: %w", err)
		}
		if len(respBodyBytes) > 2048 {
			logger.Trace("Raw API response (truncated to 2KB): %s...", string(respBodyBytes[:2048]))
		} else {
			logger.Trace("Raw API response: %s", string(respBodyBytes))
		}
		// Replace resp.Body so it can be read again
		resp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))
	}

	logger.Trace("Parsing API response")
	networks, err := service.ParseGetNetworksResponse(resp)
	if err != nil {
		logger.Error("Failed to parse networks response: %v", err)
		return nil, fmt.Errorf("failed to parse networks response: %w", err)
	}

	return networks, nil
}

// ApplyFilters applies configured filters to networks
func (b *BaseMode) ApplyFilters(networks *service.GetNetworksResponse) {
	filters.ApplyFilters(networks, b.cfg.Default)
}

// LogNetworkDiscovery logs the network discovery process
func (b *BaseMode) LogNetworkDiscovery(networks *service.GetNetworksResponse, preFilter bool) {
	logger := log.NewScopedLogger(fmt.Sprintf("[modes/%s]", b.mode), b.cfg.Default.LogLevel)

	if preFilter {
		logger.Debug("Retrieved %d networks from ZeroTier", len(*networks.JSON200))

		// Log each network found (before filtering)
		for i, network := range *networks.JSON200 {
			logger.Trace("Network %d: ID=%s, Name=%s, Interface=%s, Status=%s",
				i+1,
				utils.GetString(network.Id),
				utils.GetString(network.Name),
				utils.GetString(network.PortDeviceName),
				utils.GetString(network.Status))
		}
	} else {
		logger.Debug("After filtering: %d networks to process", len(*networks.JSON200))

		// Log each network that will be processed (after filtering)
		for i, network := range *networks.JSON200 {
			logger.Debug("Processing network %d: ID=%s, Name=%s, Interface=%s",
				i+1,
				utils.GetString(network.Id),
				utils.GetString(network.Name),
				utils.GetString(network.PortDeviceName))

			networkNameOrID := utils.GetString(network.Name)
			if networkNameOrID == "" {
				networkNameOrID = utils.GetString(network.Id)
			}

			if network.Dns != nil && network.Dns.Servers != nil {
				logger.Debug("ZeroTier network [%s]: DNS servers: %v", networkNameOrID, *network.Dns.Servers)
			}
			if network.Dns != nil && network.Dns.Domain != nil {
				logger.Debug("ZeroTier network [%s]: DNS domain: %s", networkNameOrID, *network.Dns.Domain)
			}
			if network.AssignedAddresses != nil {
				logger.Debug("ZeroTier network [%s]: Assigned addresses: %v", networkNameOrID, *network.AssignedAddresses)
			}
		}
	}
}

// LogConfiguration logs the configuration details
func (b *BaseMode) LogConfiguration() {
	logger := log.NewScopedLogger("[config]", b.cfg.Default.LogLevel)
	logger.Debug("Host: %s, Port: %d, TokenFile: %s",
		b.cfg.Default.Host, b.cfg.Default.Port, b.cfg.Default.TokenFile)
}

// GetConfig returns the configuration
func (b *BaseMode) GetConfig() config.Config {
	return b.cfg
}

// IsDryRun returns whether this is a dry run
func (b *BaseMode) IsDryRun() bool {
	return b.dryRun
}

// GetModeName returns the mode name
func (b *BaseMode) GetModeName() string {
	return b.mode
}

// GetNetworkName returns a display name for the network
func GetNetworkName(network service.Network) string {
	if network.Name != nil && *network.Name != "" {
		return *network.Name
	}
	if network.Id != nil {
		return *network.Id
	}
	return "unknown"
}

// ValidateNetwork performs common network validation
func (b *BaseMode) ValidateNetwork(network service.Network) error {
	if network.Id == nil {
		return fmt.Errorf("network ID is required")
	}

	if network.PortDeviceName == nil || *network.PortDeviceName == "" {
		return fmt.Errorf("network interface name is required for network %s", utils.GetString(network.Id))
	}

	return nil
}

// GetDNSServers extracts DNS servers from a network
func (b *BaseMode) GetDNSServers(network service.Network) []string {
	if network.Dns == nil || network.Dns.Servers == nil {
		return nil
	}
	return *network.Dns.Servers
}

// GetDNSDomain extracts DNS domain from a network
func (b *BaseMode) GetDNSDomain(network service.Network) string {
	if network.Dns == nil || network.Dns.Domain == nil {
		return ""
	}
	return *network.Dns.Domain
}

// ProcessNetworks handles the common network processing workflow
func (b *BaseMode) ProcessNetworks(ctx context.Context) (*service.GetNetworksResponse, error) {
	logger := log.NewScopedLogger(fmt.Sprintf("[modes/%s]", b.mode), b.cfg.Default.LogLevel)

	// Log configuration
	b.LogConfiguration()

	// Fetch networks
	logger.Debug("Fetching networks from ZeroTier API")
	networks, err := b.FetchNetworks(ctx)
	if err != nil {
		return nil, err
	}

	// Log discovery (before filtering)
	b.LogNetworkDiscovery(networks, true)

	// Apply filters
	logger.Trace("Applying network filters")
	b.ApplyFilters(networks)

	// Log discovery (after filtering)
	b.LogNetworkDiscovery(networks, false)

	// Validate networks
	for _, network := range *networks.JSON200 {
		if err := b.ValidateNetwork(network); err != nil {
			logger.Warn("Skipping invalid network: %v", err)
			continue
		}
	}

	return networks, nil
}