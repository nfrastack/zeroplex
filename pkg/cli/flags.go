// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package cli

import (
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"

	"flag"
	"fmt"
	"os"
	"strings"
)

// Flags represents all command line flags
type Flags struct {
	Version           *bool
	Help              *bool
	ConfigFile        *string
	DryRun            *bool
	Mode              *string
	Host              *string
	Port              *int
	LogLevel          *string
	LogTimestamps     *bool
	TokenFile         *string
	FilterType        *string
	FilterInclude     *string
	FilterExclude     *string
	AddReverseDomains *bool
	AutoRestart       *bool
	DNSOverTLS        *bool
	SelectedProfile   *string
	MulticastDNS      *bool
	Reconcile         *bool
	Token             *string
}

// ParseFlags initializes and parses command line flags
func ParseFlags() (*Flags, map[string]bool) {
	flags := &Flags{
		Version:           flag.Bool("version", false, "Print the version and exit"),
		Help:              flag.Bool("help", false, "Show help message and exit"),
		ConfigFile:        flag.String("config-file", "/etc/zt-dns-companion.conf", "Path to the configuration file"),
		DryRun:            flag.Bool("dry-run", false, "Enable dry-run mode. No changes will be made."),
		Mode:              flag.String("mode", "auto", "Mode of operation (networkd, resolved, or auto)."),
		Host:              flag.String("host", "http://localhost", "ZeroTier client host address. Default: http://localhost"),
		Port:              flag.Int("port", 9993, "ZeroTier client port number. Default: 9993"),
		LogLevel:          flag.String("log-level", "info", "Set the logging level (info or debug). Default: info"),
		LogTimestamps:     flag.Bool("log-timestamps", false, "Enable timestamps in logs. Default: false"),
		TokenFile:         flag.String("token-file", "/var/lib/zerotier-one/authtoken.secret", "Path to the ZeroTier authentication token file. Default: /var/lib/zerotier-one/authtoken.secret"),
		FilterType:        flag.String("filter-type", "none", "Type of filter to apply (interface, network, network_id, or none). Default: none"),
		FilterInclude:     flag.String("filter-include", "", "Comma-separated list of items to include based on filter-type. Empty means 'all'."),
		FilterExclude:     flag.String("filter-exclude", "", "Comma-separated list of items to exclude based on filter-type. Empty means 'none'."),
		AddReverseDomains: flag.Bool("add-reverse-domains", false, "Add ip6.arpa and in-addr.arpa search domains. Default: false"),
		AutoRestart:       flag.Bool("auto-restart", true, "Automatically restart systemd-networkd when things change. Default: true"),
		DNSOverTLS:        flag.Bool("dns-over-tls", false, "Automatically prefer DNS-over-TLS. Default: false"),
		SelectedProfile:   flag.String("profile", "", "Specify a profile to use from the configuration file. Default: none"),
		MulticastDNS:      flag.Bool("multicast-dns", false, "Enable Multicast DNS (mDNS). Default: false"),
		Reconcile:         flag.Bool("reconcile", true, "Automatically remove left networks from systemd-networkd configuration"),
		Token:             flag.String("token", "", "API token to use. Overrides token-file if provided."),
	}

	flag.Parse()

	// Validate flags that require values
	validateFlagsWithValues()

	// Track explicitly set flags
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
		logger.DebugWithPrefix("flag", "Explicit flag detected: %s = %s", f.Name, f.Value.String())
	})

	return flags, explicitFlags
}

// validateFlagsWithValues checks that flags requiring values have them
func validateFlagsWithValues() {
	for i, arg := range os.Args {
		if i > 0 && strings.HasPrefix(arg, "-") {
			if strings.HasPrefix(arg, "--") || (len(arg) > 1 && arg[1] != '-') {
				flagName := strings.TrimLeft(arg, "-")
				if flagName == "log-level" || flagName == "mode" || flagName == "profile" ||
					flagName == "filter-type" || flagName == "filter-include" || flagName == "filter-exclude" ||
					flagName == "host" || flagName == "token" || flagName == "token-file" || flagName == "config-file" {

					hasValue := false
					if i+1 < len(os.Args) {
						nextArg := os.Args[i+1]
						hasValue = !strings.HasPrefix(nextArg, "-")
					}

					if !hasValue {
						fmt.Fprintf(os.Stderr, "Error: Flag -%s requires a value\n", flagName)
						flag.Usage()
						os.Exit(1)
					}
				}
			}
		}
	}
}

// ApplyExplicitFlags applies explicitly set command line flags to configuration
func ApplyExplicitFlags(cfg *config.Config, flags *Flags, explicitFlags map[string]bool) {
	if explicitFlags["add-reverse-domains"] {
		cfg.Default.AddReverseDomains = *flags.AddReverseDomains
	}
	if explicitFlags["auto-restart"] {
		cfg.Default.AutoRestart = *flags.AutoRestart
	}
	if explicitFlags["dns-over-tls"] {
		cfg.Default.DNSOverTLS = *flags.DNSOverTLS
	}
	if explicitFlags["host"] {
		cfg.Default.Host = *flags.Host
	}
	if explicitFlags["log-level"] {
		cfg.Default.LogLevel = *flags.LogLevel
	}
	if explicitFlags["log-timestamps"] {
		cfg.Default.LogTimestamps = *flags.LogTimestamps
	}
	if explicitFlags["mode"] {
		cfg.Default.Mode = *flags.Mode
	}
	if explicitFlags["multicast-dns"] {
		cfg.Default.MulticastDNS = *flags.MulticastDNS
	}
	if explicitFlags["port"] {
		cfg.Default.Port = *flags.Port
	}
	if explicitFlags["reconcile"] {
		cfg.Default.Reconcile = *flags.Reconcile
	}
	if explicitFlags["token-file"] {
		cfg.Default.TokenFile = *flags.TokenFile
	}
	if explicitFlags["filter-type"] {
		cfg.Default.FilterType = *flags.FilterType
	}

	// Handle filter-include and filter-exclude flags
	if explicitFlags["filter-include"] {
		if *flags.FilterInclude != "" {
			cfg.Default.FilterInclude = strings.Split(*flags.FilterInclude, ",")
		} else {
			cfg.Default.FilterInclude = []string{}
		}
	}

	if explicitFlags["filter-exclude"] {
		if *flags.FilterExclude != "" {
			cfg.Default.FilterExclude = strings.Split(*flags.FilterExclude, ",")
		} else {
			cfg.Default.FilterExclude = []string{}
		}
	}
}