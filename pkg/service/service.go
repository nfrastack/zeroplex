package service

import (
	"strings"

	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"
)

// DetectMode automatically detects which systemd service is running
func DetectMode() (string, bool) {
	networkdOutput, networkdErr := utils.ExecuteCommand("systemctl", "is-active", "systemd-networkd.service")
	networkdActive := networkdErr == nil && strings.TrimSpace(networkdOutput) == "active"

	resolvedOutput, resolvedErr := utils.ExecuteCommand("systemctl", "is-active", "systemd-resolved.service")
	resolvedActive := resolvedErr == nil && strings.TrimSpace(resolvedOutput) == "active"

	if networkdActive {
		return "networkd", true
	} else if resolvedActive {
		return "resolved", true
	} else {
		utils.ErrorHandler("Neither systemd-networkd nor systemd-resolved is running. Please manually set the mode using the -mode flag or configuration file.", nil, true)
		return "", false
	}
}

// ValidateAndLoadConfig validates and loads configuration from file
func ValidateAndLoadConfig(configFile string) config.Config {
	logger.Debugf("Loading configuration from file: %s", configFile)
	cfg := config.LoadConfiguration(configFile)

	err := config.ValidateConfig(&cfg)
	if err != nil {
		logger.Debugf("Configuration validation failed: %v", err)
		utils.ErrorHandler("Validating configuration", err, true)
	} else {
		logger.Debugf("Configuration validation succeeded")
	}

	return cfg
}