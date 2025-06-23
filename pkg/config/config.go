// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type LogConfig struct {
	Level      string `yaml:"level"`
	Type       string `yaml:"type"`
	File       string `yaml:"file"`
	Timestamps bool   `yaml:"timestamps"`
}

type DaemonConfig struct {
	Enabled      bool   `yaml:"enabled"`
	PollInterval string `yaml:"poll_interval"`
}

type ClientConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	TokenFile string `yaml:"token_file"`
}

type FeaturesConfig struct {
	DNSOverTLS        bool `yaml:"dns_over_tls"`
	AddReverseDomains bool `yaml:"add_reverse_domains"`
	MulticastDNS      bool `yaml:"multicast_dns"`
	RestoreOnExit     bool `yaml:"restore_on_exit"`
}

type NetworkdConfig struct {
	AutoRestart bool `yaml:"auto_restart"`
	Reconcile   bool `yaml:"reconcile"`
}

type InterfaceWatchRetry struct {
	Count int    `yaml:"count"`
	Delay string `yaml:"delay"`
}

type InterfaceWatch struct {
	Mode  string              `yaml:"mode"`
	Retry InterfaceWatchRetry `yaml:"retry"`
}

type Profile struct {
	Mode           string                   `yaml:"mode"`
	Log            LogConfig                `yaml:"log"`
	Daemon         DaemonConfig             `yaml:"daemon"`
	Client         ClientConfig             `yaml:"client"`
	Features       FeaturesConfig           `yaml:"features"`
	Networkd       NetworkdConfig           `yaml:"networkd"`
	InterfaceWatch InterfaceWatch           `yaml:"interface_watch"`
	Filters        []map[string]interface{} `yaml:"filters,omitempty"`
}

type Config struct {
	Default  Profile            `yaml:"default"`
	Profiles map[string]Profile `yaml:"profiles"`
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
			Mode: "auto",
			Log: LogConfig{
				Level:      "verbose",
				Type:       "console",
				File:       "/var/log/zeroplex.log",
				Timestamps: false,
			},
			Daemon: DaemonConfig{
				Enabled:      true,
				PollInterval: "1m",
			},
			Client: ClientConfig{
				Host:      "http://localhost",
				Port:      9993,
				TokenFile: "/var/lib/zerotier-one/authtoken.secret",
			},
			Networkd: NetworkdConfig{
				AutoRestart: true,
				Reconcile:   true,
			},
			Features: FeaturesConfig{
				DNSOverTLS:        false,
				AddReverseDomains: false,
				MulticastDNS:      false,
				RestoreOnExit:     false,
			},
			InterfaceWatch: InterfaceWatch{
				Mode: "off",
				Retry: InterfaceWatchRetry{
					Count: 10,
					Delay: "10s",
				},
			},
		},
		Profiles: make(map[string]Profile),
	}
}

func LoadConfig(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var config Config

	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".yaml", ".yml":
		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&config); err != nil {
			return Config{}, fmt.Errorf("failed to parse YAML config: %w", err)
		}

	default:
		return Config{}, fmt.Errorf("unsupported config file format: %s (supported: .yaml, .yml)", ext)
	}

	return config, nil
}

func LoadConfiguration(configFile string) Config {
	if configFile != "" {
		_, err := os.Stat(configFile)
		if err == nil {
			loadedConfig, err := LoadConfig(configFile)
			if err != nil {
				if configFile != "/etc/zeroplex.yaml" {
					fmt.Fprintf(os.Stderr, "ERROR: Configuration file %s not found: %v\n", configFile, err)
					os.Exit(1)
				}
				return DefaultConfig()
			}

			defaultConfig := DefaultConfig()

			// Apply token file default if not set in config
			if loadedConfig.Default.Client.TokenFile == "" {
				loadedConfig.Default.Client.TokenFile = defaultConfig.Default.Client.TokenFile
			}

			return loadedConfig
		} else if os.IsNotExist(err) {
			if configFile != "/etc/zeroplex.yaml" {
				fmt.Fprintf(os.Stderr, "ERROR: Configuration file %s not found: %v\n", configFile, err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Checking configuration file existence: %v\n", err)
			os.Exit(1)
		}
	}

	return DefaultConfig()
}

func ValidateConfig(cfg *Config) error {
	if cfg.Default.Client.Host == "" {
		return fmt.Errorf("missing required configuration: client.host")
	}
	if cfg.Default.Client.Port == 0 {
		return fmt.Errorf("missing required configuration: client.port")
	}

	mode := strings.ToLower(cfg.Default.Mode)
	if mode != "auto" && mode != "networkd" && mode != "resolved" {
		return fmt.Errorf("invalid mode: %s (must be auto, networkd, or resolved)", cfg.Default.Mode)
	}

	logLevel := strings.ToLower(cfg.Default.Log.Level)
	if logLevel != "error" && logLevel != "warn" && logLevel != "info" && logLevel != "verbose" && logLevel != "debug" && logLevel != "trace" {
		return fmt.Errorf("invalid log level: %s (must be error, warn, info, verbose, debug, or trace)", cfg.Default.Log.Level)
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

		if profile.Log.Level != "" {
			logLevel = strings.ToLower(profile.Log.Level)
			if logLevel != "error" && logLevel != "warn" && logLevel != "info" && logLevel != "verbose" && logLevel != "debug" && logLevel != "trace" {
				return fmt.Errorf("invalid log level in profile %s: %s (must be error, warn, info, verbose, debug, or trace)",
					name, profile.Log.Level)
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

	// Merge Log Config
	if selectedProfile.Log.Level != "" {
		mergedProfile.Log.Level = selectedProfile.Log.Level
	}
	if selectedProfile.Log.Type != "" {
		mergedProfile.Log.Type = selectedProfile.Log.Type
	}
	if selectedProfile.Log.File != "" {
		mergedProfile.Log.File = selectedProfile.Log.File
	}
	mergedProfile.Log.Timestamps = mergedProfile.Log.Timestamps || selectedProfile.Log.Timestamps

	// Merge Daemon Config
	if selectedProfile.Daemon.Enabled {
		mergedProfile.Daemon.Enabled = true
	}
	if selectedProfile.Daemon.PollInterval != "" {
		mergedProfile.Daemon.PollInterval = selectedProfile.Daemon.PollInterval
	}

	// Merge Client Config
	if selectedProfile.Client.Host != "" {
		mergedProfile.Client.Host = selectedProfile.Client.Host
	}
	if selectedProfile.Client.Port != 0 {
		mergedProfile.Client.Port = selectedProfile.Client.Port
	}
	if selectedProfile.Client.TokenFile != "" {
		mergedProfile.Client.TokenFile = selectedProfile.Client.TokenFile
	} else if mergedProfile.Client.TokenFile == "" {
		mergedProfile.Client.TokenFile = "/var/lib/zerotier-one/authtoken.secret"
	}

	// Merge Networkd Config
	mergedProfile.Networkd.AutoRestart = mergedProfile.Networkd.AutoRestart || selectedProfile.Networkd.AutoRestart
	mergedProfile.Networkd.Reconcile = mergedProfile.Networkd.Reconcile || selectedProfile.Networkd.Reconcile

	// Merge Features Config
	if selectedProfile.Features.DNSOverTLS {
		mergedProfile.Features.DNSOverTLS = true
	}
	if selectedProfile.Features.AddReverseDomains {
		mergedProfile.Features.AddReverseDomains = true
	}
	if selectedProfile.Features.MulticastDNS {
		mergedProfile.Features.MulticastDNS = true
	}
	if selectedProfile.Features.RestoreOnExit {
		mergedProfile.Features.RestoreOnExit = true
	}

	// Copy Filters
	if len(selectedProfile.Filters) > 0 {
		mergedProfile.Filters = selectedProfile.Filters
	}

	// Interface Watch
	if selectedProfile.InterfaceWatch.Mode != "" {
		mergedProfile.InterfaceWatch.Mode = selectedProfile.InterfaceWatch.Mode
	}
	if selectedProfile.InterfaceWatch.Retry.Count != 0 {
		mergedProfile.InterfaceWatch.Retry.Count = selectedProfile.InterfaceWatch.Retry.Count
	}
	if selectedProfile.InterfaceWatch.Retry.Delay != "" {
		mergedProfile.InterfaceWatch.Retry.Delay = selectedProfile.InterfaceWatch.Retry.Delay
	}

	return mergedProfile
}
