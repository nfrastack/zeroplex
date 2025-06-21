package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

var (
	IncludeAllValues  = map[string]bool{"any": true, "ignore": true, "all": true, "": true} // Values that mean "include everything"
	ExcludeNoneValues = map[string]bool{"none": true, "ignore": true, "": true}             // Values that mean "exclude nothing"
)

type Config struct {
	Default  Profile            `toml:"default"`
	Profiles map[string]Profile `toml:"profiles"`
}

type Profile struct {
	Mode              string   `toml:"mode"`
	LogLevel          string   `toml:"log_level"`
	Host              string   `toml:"host"`
	Port              int      `toml:"port"`
	DNSOverTLS        bool     `toml:"dns_over_tls"`
	AutoRestart       bool     `toml:"auto_restart"`
	AddReverseDomains bool     `toml:"add_reverse_domains"`
	LogTimestamps     bool     `toml:"log_timestamps"`
	MulticastDNS      bool     `toml:"multicast_dns"`
	Reconcile         bool     `toml:"reconcile"`
	FilterType        string   `toml:"filter_type"`    // "interface", "network", "network_id", or "none"
	FilterInclude     []string `toml:"filter_include"` // Items to include, depending on FilterType
	FilterExclude     []string `toml:"filter_exclude"` // Items to exclude, depending on FilterType
	TokenFile         string   `toml:"token_file"`
}

func DefaultConfig() Config {
	return Config{
		Default: Profile{
			Mode:              "auto",
			LogLevel:          "info",
			Host:              "http://localhost",
			Port:              9993,
			DNSOverTLS:        false,
			AutoRestart:       true,
			AddReverseDomains: false,
			LogTimestamps:     false,
			MulticastDNS:      false,
			Reconcile:         true,
			FilterType:        "none",
			FilterInclude:     []string{},
			FilterExclude:     []string{},
			TokenFile:         "/var/lib/zerotier-one/authtoken.secret",
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
	decoder := toml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, err
	}

	return config, nil
}

func LoadConfiguration(configFile string) Config {
	if configFile != "" {
		_, err := os.Stat(configFile)
		if err == nil {
			loadedConfig, err := LoadConfig(configFile)
			if err != nil {
				if configFile != "/etc/zt-dns-companion.conf" {
					fmt.Fprintf(os.Stderr, "ERROR: Configuration file %s not found: %v\n", configFile, err)
					os.Exit(1)
				}
				return DefaultConfig()
			}

			// Apply application defaults for any unset fields in the loaded config
			defaultConfig := DefaultConfig()

			// Apply token file default if not set in config
			if loadedConfig.Default.TokenFile == "" {
				loadedConfig.Default.TokenFile = defaultConfig.Default.TokenFile
			}

			// Apply MulticastDNS default if not set in config
			if !loadedConfig.Default.LogTimestamps && !loadedConfig.Default.MulticastDNS {
				loadedConfig.Default.MulticastDNS = defaultConfig.Default.MulticastDNS
			}

			// Apply Reconcile default if not set in config
			if !loadedConfig.Default.Reconcile {
				loadedConfig.Default.Reconcile = defaultConfig.Default.Reconcile
			}

			// Apply FilterType default if not set in config
			if loadedConfig.Default.FilterType == "" {
				loadedConfig.Default.FilterType = defaultConfig.Default.FilterType
			}

			return loadedConfig
		} else if os.IsNotExist(err) {
			if configFile != "/etc/zt-dns-companion.conf" {
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
	if logLevel != "info" && logLevel != "debug" {
		return fmt.Errorf("invalid log level: %s (must be info or debug)", cfg.Default.LogLevel)
	}

	filterType := strings.ToLower(cfg.Default.FilterType)
	if filterType != "" && filterType != "none" &&
		filterType != "interface" && filterType != "network" && filterType != "network_id" {
		return fmt.Errorf("invalid filter type: %s (must be none, interface, network, or network_id)", cfg.Default.FilterType)
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
			if logLevel != "info" && logLevel != "debug" {
				return fmt.Errorf("invalid log level in profile %s: %s (must be info or debug)",
					name, profile.LogLevel)
			}
		}

		if profile.FilterType != "" {
			filterType = strings.ToLower(profile.FilterType)
			if filterType != "none" && filterType != "interface" &&
				filterType != "network" && filterType != "network_id" {
				return fmt.Errorf("invalid filter type in profile %s: %s (must be none, interface, network, or network_id)",
					name, profile.FilterType)
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

	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(config); err != nil {
		return err
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

	if len(selectedProfile.FilterInclude) > 0 {
		mergedProfile.FilterInclude = selectedProfile.FilterInclude
	}
	if len(selectedProfile.FilterExclude) > 0 {
		mergedProfile.FilterExclude = selectedProfile.FilterExclude
	}
	if selectedProfile.FilterType != "" {
		mergedProfile.FilterType = selectedProfile.FilterType
	}

	if !selectedProfile.AutoRestart {
		mergedProfile.AutoRestart = false
	}
	if !selectedProfile.Reconcile {
		mergedProfile.Reconcile = false
	}

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

	return mergedProfile
}