package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	AddReverseDomains bool   `json:"add_reverse_domains"`
	AutoRestart       bool   `json:"auto_restart"`
	DNSOverTLS        bool   `json:"dns_over_tls"`
	DryRun            bool   `json:"dry_run"`
	Host              string `json:"host"`
	LogLevel          string `json:"log_level"`
	Mode              string `json:"mode"`
	MulticastDNS      bool   `json:"multicast_dns"`
	Port              int    `json:"port"`
	Reconcile         bool   `json:"reconcile"`
	TokenFile         string `json:"token_file"`
	Token             string `json:"token"`
}

// DefaultConfig returns the default configuration values
func DefaultConfig() Config {
	return Config{
		AddReverseDomains: false,
		AutoRestart:       true,
		DNSOverTLS:        false,
		DryRun:            false,
		Host:              "http://localhost",
		LogLevel:          "info",
		Mode:              "networkd",
		MulticastDNS:      false,
		Port:              9993,
		Reconcile:         true,
		TokenFile:         "/var/lib/zerotier-one/authtoken.secret",
		Token:             "",
	}
}

// LoadConfig reads the configuration from a file
func LoadConfig(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

// SaveConfig writes the configuration to a file
func SaveConfig(filePath string, config Config) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return err
	}

	return nil
}

// MergeConfig merges command-line arguments into the configuration
func MergeConfig(config Config, overrides Config) Config {
	if overrides.AddReverseDomains {
		config.AddReverseDomains = overrides.AddReverseDomains
	}
	if overrides.AutoRestart {
		config.AutoRestart = overrides.AutoRestart
	}
	if overrides.DNSOverTLS {
		config.DNSOverTLS = overrides.DNSOverTLS
	}
	if overrides.DryRun {
		config.DryRun = overrides.DryRun
	}
	if overrides.Host != "" {
		config.Host = overrides.Host
	}
	if overrides.LogLevel != "" {
		config.LogLevel = overrides.LogLevel
	}
	if overrides.Mode != "" {
		config.Mode = overrides.Mode
	}
	if overrides.MulticastDNS {
		config.MulticastDNS = overrides.MulticastDNS
	}
	if overrides.Port != 0 {
		config.Port = overrides.Port
	}
	if overrides.Reconcile {
		config.Reconcile = overrides.Reconcile
	}
	if overrides.TokenFile != "" {
		config.TokenFile = overrides.TokenFile
	}
	if overrides.Token != "" {
		config.Token = overrides.Token
	}

	return config
}
