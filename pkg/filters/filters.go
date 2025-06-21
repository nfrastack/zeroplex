// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package filters

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"

	"strings"

	"github.com/zerotier/go-zerotier-one/service"
)

func ApplyUnifiedFilters(networks *service.GetNetworksResponse, cfg config.Profile) {
	filterType := strings.ToLower(cfg.FilterType)

	if filterType == "" || filterType == "none" {
		logger.Debugf("No filtering applied (FilterType=%s)", filterType)
		return
	}

	// Apply the appropriate filter based on FilterType
	switch filterType {
	case "interface":
		logger.Debugf("Using interface-based filtering")
		genericFilter(networks, cfg.FilterInclude, cfg.FilterExclude, func(network service.Network) *string {
			return network.PortDeviceName
		})
	case "network":
		logger.Debugf("Using network name-based filtering")
		genericFilter(networks, cfg.FilterInclude, cfg.FilterExclude, func(network service.Network) *string {
			return network.Name
		})
	case "network_id":
		logger.Debugf("Using network ID-based filtering")
		genericFilter(networks, cfg.FilterInclude, cfg.FilterExclude, func(network service.Network) *string {
			return network.Id
		})
	default:
		logger.Debugf("Unknown FilterType '%s'. No filtering will be applied.", filterType)
	}
}

func genericFilter(networks *service.GetNetworksResponse, include, exclude []string, getKey func(network service.Network) *string) {
	logger.Debugf("Filtering with include: %v, exclude: %v", include, exclude)

	includeAll := hasSpecialIncludeValue(include)
	excludeNone := hasSpecialExcludeValue(exclude)

	if includeAll && excludeNone {
		logger.Debugf("No filtering needed - will process all items")
		return
	}

	filteredNetworks := []service.Network{}
	for _, network := range *networks.JSON200 {
		key := getKey(network)
		if key == nil {
			logger.Debugf("Skipping network without required key")
			continue
		}

		keyValue := *key
		logger.Debugf("Evaluating item: %s", keyValue)

		if !includeAll && !utils.Contains(include, keyValue) {
			logger.Debugf("Item %s not in include list - skipping", keyValue)
			continue
		}

		if !excludeNone && utils.Contains(exclude, keyValue) {
			logger.Debugf("Item %s is in exclude list - skipping", keyValue)
			continue
		}

		filteredNetworks = append(filteredNetworks, network)
	}

	logger.Debugf("After filtering: %d of %d networks remain", len(filteredNetworks), len(*networks.JSON200))
	*networks.JSON200 = filteredNetworks
}

func hasSpecialIncludeValue(values []string) bool {
	if len(values) == 0 {
		return true // Empty slice means "include all"
	}

	for _, value := range values {
		if config.IncludeAllValues[strings.ToLower(value)] {
			return true
		}
	}
	return false
}

func hasSpecialExcludeValue(values []string) bool {
	if len(values) == 0 {
		return true // Empty slice means "exclude none"
	}

	for _, value := range values {
		if config.ExcludeNoneValues[strings.ToLower(value)] {
			return true
		}
	}
	return false
}

func FindInterfaceInNetworks(networks *service.GetNetworksResponse, interfaceNames []string) []string {
	found := []string{}

	// Debug all available interface names for reference
	if len(interfaceNames) > 0 {
		availableInterfaces := []string{}
		for _, network := range *networks.JSON200 {
			if network.PortDeviceName != nil {
				availableInterfaces = append(availableInterfaces, *network.PortDeviceName)
			}
		}
		logger.Debugf("Interface filtering requested for: %v", interfaceNames)
		logger.Debugf("Available interfaces: %v", availableInterfaces)
	}

	// Check if any interfaces match our filter list
	for _, name := range interfaceNames {
		matching := false
		for _, network := range *networks.JSON200 {
			if network.PortDeviceName != nil && *network.PortDeviceName == name {
				matching = true
				found = append(found, name)
				break
			}
		}
		if !matching {
			logger.Debugf("Warning: Interface %s specified in filter was not found in ZeroTier networks", name)
		}
	}

	return found
}