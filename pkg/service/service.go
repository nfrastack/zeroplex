// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package service

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"

	"strings"
)

// DetectMode automatically detects which systemd service is running
func DetectMode() (string, bool) {
	logger.Trace("DetectMode() - checking systemd services")

	logger.Verbose("Checking systemd-networkd.service status...")
	networkdOutput, networkdErr := utils.ExecuteCommand("systemctl", "is-active", "systemd-networkd.service")
	networkdActive := networkdErr == nil && strings.TrimSpace(networkdOutput) == "active"
	logger.Debug("systemd-networkd.service active: %t", networkdActive)

	logger.Verbose("Checking systemd-resolved.service status...")
	resolvedOutput, resolvedErr := utils.ExecuteCommand("systemctl", "is-active", "systemd-resolved.service")
	resolvedActive := resolvedErr == nil && strings.TrimSpace(resolvedOutput) == "active"
	logger.Debug("systemd-resolved.service active: %t", resolvedActive)

	if networkdActive {
		logger.Verbose("Detected mode: networkd")
		return "networkd", true
	} else if resolvedActive {
		logger.Verbose("Detected mode: resolved")
		return "resolved", true
	} else {
		logger.Error("Neither systemd-networkd nor systemd-resolved is running")
		utils.ErrorHandler("Neither systemd-networkd nor systemd-resolved is running. Please manually set the mode using the -mode flag or configuration file.", nil, true)
		return "", false
	}
}

// ValidateAndLoadConfig validates and loads configuration from file
func ValidateAndLoadConfig(configFile string) config.Config {
	logger.Trace("ValidateAndLoadConfig() started with file: %s", configFile)
	logger.Debugf("Loading configuration from file: %s", configFile)
	cfg := config.LoadConfiguration(configFile)

	logger.Verbose("Validating loaded configuration...")
	err := config.ValidateConfig(&cfg)
	if err != nil {
		logger.Debugf("Configuration validation failed: %v", err)
		utils.ErrorHandler("Validating configuration", err, true)
	} else {
		logger.Debugf("Configuration validation succeeded")
	}

	logger.Debug("Configuration loaded and validated successfully")
	return cfg
}