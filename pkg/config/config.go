// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"zt-dns-companion/pkg/logger"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Default  Profile            `yaml:"default"`
	Profiles map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	DaemonMode        bool                     `yaml:"daemon_mode"`
	Mode              string                   `yaml:"mode"`
	LogLevel          string                   `yaml:"log_level"`
	Host              string                   `yaml:"host"`
	Port              int                      `yaml:"port"`
	DNSOverTLS        bool                     `yaml:"dns_over_tls"`
	AutoRestart       bool                     `yaml:"auto_restart"`
	AddReverseDomains bool                     `yaml:"add_reverse_domains"`
	LogTimestamps     bool                     `yaml:"log_timestamps"`
	MulticastDNS      bool                     `yaml:"multicast_dns"`
	Reconcile         bool                     `yaml:"reconcile"`
	TokenFile         string                   `yaml:"token_file"`
	PollInterval      string                   `yaml:"poll_interval"`
	Filters           []map[string]interface{} `yaml:"filters,omitempty"`
}

// HasAdvancedFilters checks if the profile has advanced filters configured
func (p Profile) HasAdvancedFilters() bool {
	return len(p.Filters) > 0
}

// GetAdvancedFilterConfig converts the profile's Filters to a FilterConfig
func (p Profile) GetAdvancedFilterConfig() (map[string]interface{}, error) {
	if !p.HasAdvancedFilters() {
		return nil, fmt.Errorf("no advanced filters configured")
	}

	// Convert []map[string]interface{} to the format expected by filters package
	structuredOptions := map[string]interface{}{
		"filter": p.Filters,
	}

	return structuredOptions, nil
}

func DefaultConfig() Config {
	return Config{
		Default: Profile{
			Mode:              "auto",
			LogLevel:          "verbose",
			Host:              "http://localhost",
			Port:              9993,
			DNSOverTLS:        false,
			AutoRestart:       true,
			AddReverseDomains: false,
			LogTimestamps:     false,
			MulticastDNS:      false,
			Reconcile:         true,
			TokenFile:         "/var/lib/zerotier-one/authtoken.secret",
	    	DaemonMode:        true,
	    	PollInterval:      "1m",
		},
		Profiles: make(map[string]Profile),
	}
}

func LoadConfig(filePath string) (Config, error) {
	logger.Trace(">>> LoadConfig(%s) called", filePath)
	logger.Debug("Opening configuration file: %s", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		logger.Debug("Failed to open config file: %v", err)
		return Config{}, err
	}
	defer file.Close()

	var config Config
	logger.Verbose("Parsing configuration file format...")

	ext := strings.ToLower(filepath.Ext(filePath))
	logger.Trace("Detected file extension: %s", ext)

	switch ext {
	case ".yaml", ".yml":
		logger.Debug("Using YAML decoder for configuration")
		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			logger.Error("Failed to parse YAML config: %v", err)
			return Config{}, fmt.Errorf("failed to parse YAML config: %w", err)
		}
		logger.Verbose("YAML configuration parsed successfully")

	default:
		logger.Error("Unsupported config file format: %s", ext)
		return Config{}, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml)", ext)
	}

	logger.Debug("Configuration loaded: mode=%s, log_level=%s, daemon_mode=%v",
		config.Default.Mode, config.Default.LogLevel, config.Default.DaemonMode)
	logger.Trace("<<< LoadConfig() completed successfully")
	return config, nil
}

func LoadConfiguration(configFile string) Config {
	if configFile != "" {
		logger.Verbose("Attempting to load configuration from: %s", configFile)
		_, err := os.Stat(configFile)
		if err == nil {
			logger.Debug("Configuration file found, loading...")
			loadedConfig, err := LoadConfig(configFile)
			if err != nil {
				if configFile != "/etc/zt-dns-companion.yaml" {
					fmt.Fprintf(os.Stderr, "ERROR: Configuration file %s not found: %v\n", configFile, err)
					os.Exit(1)
				}
				logger.Warn("Could not load config file, using defaults: %v", err)
				return DefaultConfig()
			}

			logger.Debug("Configuration loaded successfully")

			// Apply application defaults for any unset fields in the loaded config
			defaultConfig := DefaultConfig()

			// Apply token file default if not set in config
			if loadedConfig.Default.TokenFile == "" {
				loadedConfig.Default.TokenFile = defaultConfig.Default.TokenFile
				logger.Verbose("Applied default token file path: %s", defaultConfig.Default.TokenFile)
			}

			logger.Info("Using configuration from file: %s", configFile)
			return loadedConfig
		} else if os.IsNotExist(err) {
			if configFile != "/etc/zt-dns-companion.yaml" {
				fmt.Fprintf(os.Stderr, "ERROR: Configuration file %s not found: %v\n", configFile, err)
				os.Exit(1)
			}
			logger.Verbose("Default config file not found, using built-in defaults")
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Checking configuration file existence: %v\n", err)
			os.Exit(1)
		}
	} else {
		logger.Verbose("No configuration file specified, using defaults")
	}

	logger.Info("Using default configuration")
	return DefaultConfig()
}

func ValidateConfig(cfg *Config) error {
	if cfg.Default.Host == "" {
		return fmt.Errorf("missing required configuration: host")
	}
	if cfg.Default.Port == 0 {
		return fmt.Errorf("missing required configuration: port")
	}

	mode := strings.ToLower(cfg.Default.Mode)
	if mode != "auto" && mode != "networkd" && mode != "resolved" {
		return fmt.Errorf("invalid mode: %s (must be auto, networkd, or resolved)", cfg.Default.Mode)
	}

	logLevel := strings.ToLower(cfg.Default.LogLevel)
	if logLevel != "error" && logLevel != "warn" && logLevel != "info" && logLevel != "verbose" && logLevel != "debug" && logLevel != "trace" {
		return fmt.Errorf("invalid log level: %s (must be error, warn, info, verbose, debug, or trace)", cfg.Default.LogLevel)
	}

	// Validate profiles
	for name, profile := range cfg.Profiles {
		if profile.Mode != "" {
			mode = strings.ToLower(profile.Mode)
			if mode != "auto" && mode != "networkd" && mode != "resolved" {
				return fmt.Errorf("invalid mode in profile %s: %s (must be auto, networkd, or resolved)",
					name, profile.Mode)
			}
		}

		if profile.LogLevel != "" {
			logLevel = strings.ToLower(profile.LogLevel)
			if logLevel != "error" && logLevel != "warn" && logLevel != "info" && logLevel != "verbose" && logLevel != "debug" && logLevel != "trace" {
				return fmt.Errorf("invalid log level in profile %s: %s (must be error, warn, info, verbose, debug, or trace)",
					name, profile.LogLevel)
			}
		}
	}

	return nil
}

func SaveConfig(filePath string, config Config) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(file)
		defer encoder.Close()
		encoder.SetIndent(2)
		if err := encoder.Encode(config); err != nil {
			return fmt.Errorf("failed to encode YAML config: %w", err)
		}

	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml)", ext)
	}

	return nil
}

func MergeProfiles(defaultProfile, selectedProfile Profile) Profile {
	mergedProfile := defaultProfile

	if selectedProfile.Mode != "" {
		mergedProfile.Mode = selectedProfile.Mode
	}
	if selectedProfile.LogLevel != "" {
		mergedProfile.LogLevel = selectedProfile.LogLevel
	}
	if selectedProfile.Host != "" {
		mergedProfile.Host = selectedProfile.Host
	}
	if selectedProfile.Port != 0 {
		mergedProfile.Port = selectedProfile.Port
	}
	if selectedProfile.TokenFile != "" {
		mergedProfile.TokenFile = selectedProfile.TokenFile
	} else if mergedProfile.TokenFile == "" {
		mergedProfile.TokenFile = "/var/lib/zerotier-one/authtoken.secret"
	}

	// Copy daemon configuration
	if selectedProfile.PollInterval != "" {
		mergedProfile.PollInterval = selectedProfile.PollInterval
	}

	// Copy Filters
	if len(selectedProfile.Filters) > 0 {
		mergedProfile.Filters = selectedProfile.Filters
	}

	// Boolean flags
	if selectedProfile.DNSOverTLS {
		mergedProfile.DNSOverTLS = true
	}
	if selectedProfile.AddReverseDomains {
		mergedProfile.AddReverseDomains = true
	}
	if selectedProfile.LogTimestamps {
		mergedProfile.LogTimestamps = true
	}
	if selectedProfile.MulticastDNS {
		mergedProfile.MulticastDNS = true
	}
	if selectedProfile.DaemonMode {
		mergedProfile.DaemonMode = true
	}
	if !selectedProfile.AutoRestart {
		mergedProfile.AutoRestart = false
	}
	if !selectedProfile.Reconcile {
		mergedProfile.Reconcile = false
	}

	return mergedProfile
}