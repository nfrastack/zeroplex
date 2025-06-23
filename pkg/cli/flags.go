// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package cli

import (
	"zeroflex/pkg/config"
	"zeroflex/pkg/log"

	"flag"
	"fmt"
	"os"
	"strings"
)

// Flags represents all command line flags
type Flags struct {
	Version                  *bool
	VersionShort             *bool
	VersionLong              *bool
	Help                     *bool
	HelpShort                *bool
	ConfigFile               *string
	ConfigFileShort          *string
	ConfigFileC              *string
	DryRun                   *bool
	Mode                     *string
	Host                     *string
	Port                     *int
	LogLevel                 *string
	LogTimestamps            *bool
	TokenFile                *string
	AddReverseDomains        *bool
	AutoRestart              *bool
	DNSOverTLS               *bool
	SelectedProfile          *string
	MulticastDNS             *bool
	Reconcile                *bool
	Token                    *string
	RestoreOnExit            *bool
	InterfaceWatchMode       *string
	InterfaceWatchRetryCount *int
	InterfaceWatchRetryDelay *string
	LogType                  *string
	LogFile                  *string
}

// ParseFlags initializes and parses command line flags
func ParseFlags() (*Flags, map[string]bool) {
	flags := &Flags{
		Version:                  flag.Bool("version", false, "Print the version and exit"),
		VersionShort:             flag.Bool("v", false, "Print the version and exit (alias)"),
		VersionLong:              flag.Bool("--version", false, "Print the version and exit (alias)"),
		Help:                     flag.Bool("help", false, "Show help message and exit"),
		HelpShort:                flag.Bool("h", false, "Show help message and exit (alias)"),
		AddReverseDomains:        flag.Bool("add-reverse-domains", false, "Add ip6.arpa and in-addr.arpa search domains. Default: false"),
		AutoRestart:              flag.Bool("auto-restart", true, "Automatically restart systemd-networkd when things change. Default: true"),
		ConfigFile:               flag.String("config-file", "/etc/zeroflex.conf", "Path to the configuration file"),
		ConfigFileC:              flag.String("c", "", "Path to the configuration file (alias)"),
		ConfigFileShort:          flag.String("config", "", "Path to the configuration file (alias)"),
		DNSOverTLS:               flag.Bool("dns-over-tls", false, "Automatically prefer DNS-over-TLS. Default: false"),
		DryRun:                   flag.Bool("dry-run", false, "Enable dry-run mode. No changes will be made."),
		Host:                     flag.String("host", "http://localhost", "ZeroTier client host address. Default: http://localhost"),
		InterfaceWatchMode:       flag.String("interface-watch-mode", "event", "Interface watch mode: event, poll, or off."),
		InterfaceWatchRetryCount: flag.Int("interface-watch-retry-count", 3, "Number of retries after interface event."),
		InterfaceWatchRetryDelay: flag.String("interface-watch-retry-delay", "2s", "Delay between interface event retries (e.g., 2s)."),
		LogFile:                  flag.String("log-file", "/var/log/zeroflex.log", "Log file path if log-type is file or both. Default: /var/log/zeroflex.log."),
		LogLevel:                 flag.String("log-level", "info", "Set the logging level (info or debug). Default: info"),
		LogTimestamps:            flag.Bool("log-timestamps", false, "Enable timestamps in logs. Default: false"),
		LogType:                  flag.String("log-type", "console", "Log output type: console, file, or both. Default: console."),
		Mode:                     flag.String("mode", "auto", "Mode of operation (networkd, resolved, or auto)."),
		MulticastDNS:             flag.Bool("multicast-dns", false, "Enable Multicast DNS (mDNS). Default: false"),
		Port:                     flag.Int("port", 9993, "ZeroTier client port number. Default: 9993"),
		Reconcile:                flag.Bool("reconcile", true, "Automatically remove left networks from systemd-networkd configuration"),
		RestoreOnExit:            flag.Bool("restore-on-exit", false, "Restore original DNS settings for all managed interfaces on exit (default: false)"),
		SelectedProfile:          flag.String("profile", "", "Specify a profile to use from the configuration file. Default: none"),
		Token:                    flag.String("token", "", "API token to use. Overrides token-file if provided."),
		TokenFile:                flag.String("token-file", "/var/lib/zerotier-one/authtoken.secret", "Path to the ZeroTier authentication token file. Default: /var/lib/zerotier-one/authtoken.secret"),
	}

	flag.Parse()

	// Validate flags that require values
	validateFlagsWithValues()

	// Track explicitly set flags
	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
		log.NewScopedLogger("[flag]", "debug").Debug("Explicit flag detected: %s = %s", f.Name, f.Value.String())
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
		cfg.Default.Features.AddReverseDomains = *flags.AddReverseDomains
	}
	if explicitFlags["auto-restart"] {
		cfg.Default.Networkd.AutoRestart = *flags.AutoRestart
	}
	if explicitFlags["dns-over-tls"] {
		cfg.Default.Features.DNSOverTLS = *flags.DNSOverTLS
	}
	if explicitFlags["host"] {
		cfg.Default.Client.Host = *flags.Host
	}
	if explicitFlags["log-level"] {
		cfg.Default.Log.Level = *flags.LogLevel
	}
	if explicitFlags["log-timestamps"] {
		cfg.Default.Log.Timestamps = *flags.LogTimestamps
	}
	if explicitFlags["mode"] {
		cfg.Default.Mode = *flags.Mode
	}
	if explicitFlags["multicast-dns"] {
		cfg.Default.Features.MulticastDNS = *flags.MulticastDNS
	}
	if explicitFlags["port"] {
		cfg.Default.Client.Port = *flags.Port
	}
	if explicitFlags["reconcile"] {
		cfg.Default.Networkd.Reconcile = *flags.Reconcile
	}
	if explicitFlags["token-file"] {
		cfg.Default.Client.TokenFile = *flags.TokenFile
	}
	if explicitFlags["restore-on-exit"] {
		cfg.Default.Features.RestoreOnExit = *flags.RestoreOnExit
	}
	if explicitFlags["interface-watch-mode"] {
		cfg.Default.InterfaceWatch.Mode = *flags.InterfaceWatchMode
	}
	if explicitFlags["interface-watch-retry-count"] {
		cfg.Default.InterfaceWatch.Retry.Count = *flags.InterfaceWatchRetryCount
	}
	if explicitFlags["interface-watch-retry-delay"] {
		cfg.Default.InterfaceWatch.Retry.Delay = *flags.InterfaceWatchRetryDelay
	}
	if explicitFlags["log-type"] {
		cfg.Default.Log.Type = *flags.LogType
	}
	if explicitFlags["log-file"] {
		cfg.Default.Log.File = *flags.LogFile
	}
}
