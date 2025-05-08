// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
// SPDX-FileCopyrightText: © 2021 Zerotier Inc.
// SPDX-FileCopyright: BSD-3-Clause
//
// This file includes code derived from Zerotier Systemd Manager.
// The original code is licensed under the BSD-3-Clause license.
// See the LICENSE file or https://opensource.org/licenses/BSD-3-Clause for details.

package main

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pelletier/go-toml"
	"github.com/zerotier/go-zerotier-one/service"
)

var (
	Version           = "dev"
	currentLogLevel   = "info"
	logTimestamps     bool
	IncludeAllValues  = map[string]bool{"any": true, "ignore": true, "all": true, "": true} // Values that mean "include everything"
	ExcludeNoneValues = map[string]bool{"none": true, "ignore": true, "": true}             // Values that mean "exclude nothing"
)

func init() {
	// By default, remove timestamp from log output to match our custom logging behavior
	log.SetFlags(0)
}

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

type serviceAPIClient struct {
	apiKey string
	client *http.Client
}

func (c *serviceAPIClient) Do(req *http.Request) (*http.Response, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("empty API key, authentication failed")
	}
	req.Header.Add("X-ZT1-Auth", c.apiKey)
	return c.client.Do(req)
}

type templateScaffold struct {
	FileHeader  string
	ZTInterface string
	ZTNetwork   string
	DNS         []string
	Domain      string
	DNS_TLS     bool
	MDNS        bool
}

func applyUnifiedFilters(networks *service.GetNetworksResponse, config Profile) {
	filterType := strings.ToLower(config.FilterType)

	if filterType == "" || filterType == "none" {
		Debugf("No filtering applied (FilterType=%s)", filterType)
		return
	}

	// Apply the appropriate filter based on FilterType
	switch filterType {
	case "interface":
		Debugf("Using interface-based filtering")
		genericFilter(networks, config.FilterInclude, config.FilterExclude, func(network service.Network) *string {
			return network.PortDeviceName
		})
	case "network":
		Debugf("Using network name-based filtering")
		genericFilter(networks, config.FilterInclude, config.FilterExclude, func(network service.Network) *string {
			return network.Name
		})
	case "network_id":
		Debugf("Using network ID-based filtering")
		genericFilter(networks, config.FilterInclude, config.FilterExclude, func(network service.Network) *string {
			return network.Id
		})
	default:
		Debugf("Unknown FilterType '%s'. No filtering will be applied.", filterType)
	}
}

func calculateReverseDomains(assignedAddresses *[]string) []string {
	reverseDomains := []string{}
	if assignedAddresses == nil || len(*assignedAddresses) == 0 {
		return reverseDomains
	}

	for _, addr := range *assignedAddresses {
		ip, ipnet, err := net.ParseCIDR(addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not parse CIDR %q: %v\n", addr, err)
			continue
		}

		used, total := ipnet.Mask.Size()
		bits := int(math.Ceil(float64(total) / float64(used)))

		octets := make([]byte, bits+1)
		if total == 32 { // IPv4
			ip = ip.To4()
		}

		for i := 0; i <= bits; i++ {
			octets[i] = ip[i]
		}

		searchLine := "~"
		for i := len(octets) - 1; i >= 0; i-- {
			if total > 32 { // IPv6
				searchLine += fmt.Sprintf("%x.", (octets[i] & 0xf))
				searchLine += fmt.Sprintf("%x.", ((octets[i] >> 4) & 0xf))
			} else { // IPv4
				searchLine += fmt.Sprintf("%d.", octets[i])
			}
		}

		if total == 32 {
			searchLine += "in-addr.arpa"
		} else {
			searchLine += "ip6.arpa"
		}

		reverseDomains = append(reverseDomains, searchLine)
	}

	return reverseDomains
}

func compareDNS(current, desired []string) bool {
	if len(current) != len(desired) {
		return false
	}
	currentSet := make(map[string]struct{})
	for _, item := range current {
		currentSet[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range desired {
		if _, exists := currentSet[item]; !exists {
			return false
		}
	}
	return true
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func configureDNSAndSearchDomains(interfaceName string, dnsServers, searchKeys []string, dryRun bool) {
	if dryRun {
		DryRunf("Would set Interface: %s Search Domain: %s and DNS: %s", interfaceName, strings.Join(searchKeys, ", "), strings.Join(dnsServers, ", "))
		return
	}

	output, err := executeCommand("resolvectl", "dns", interfaceName)
	if err != nil {
		Debugf("Failed to query DNS via resolvectl for interface %s: %v", interfaceName, err)
		Debugf("Command output: %s", output)
		fmt.Fprintf(os.Stderr, "Could not query DNS for interface %s. Please ensure the interface exists and resolvectl is configured correctly.\n", interfaceName)
		return
	}
	currentDNS := parseResolvectlOutput(output, "Link ")
	Debugf("Parsed current DNS for interface %s: %v", interfaceName, currentDNS)

	output, err = executeCommand("resolvectl", "domain", interfaceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query search domains via resolvectl for interface %s: %v\n", interfaceName, err)
		return
	}
	currentDomains := parseResolvectlOutput(output, "Link ")
	Debugf("Parsed current search domains for interface %s: %v", interfaceName, currentDomains)

	Debugf("Desired DNS for interface %s: %v", interfaceName, dnsServers)
	Debugf("Desired search domains for interface %s: %v", interfaceName, searchKeys)

	sameDNS := compareDNS(currentDNS, dnsServers)
	sameDomains := compareDNS(currentDomains, searchKeys)

	Debugf("Comparison result for interface %s: sameDNS=%v, sameDomains=%v", interfaceName, sameDNS, sameDomains)

	if sameDNS && sameDomains {
		Infof("No changes needed for interface %s; DNS and search domains are already up-to-date", interfaceName)
		return
	}

	// Attempt to configure via D-Bus
	conn, err := dbus.SystemBus()
	if err == nil {
		interfaceObj, err := net.InterfaceByName(interfaceName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get interface index for interface %s: %v\n", interfaceName, err)
			return
		}

		resolved := conn.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")

		// Prepare DNS server arguments
		var dnsEntries []struct {
			Family  int32
			Address []byte
		}
		for _, server := range dnsServers {
			ip := net.ParseIP(server)
			if ip == nil {
				fmt.Fprintf(os.Stderr, "Invalid DNS server IP: %s\n", server)
				continue
			}
			family := int32(2) // AF_INET
			address := ip.To4()
			if address == nil {
				family = 10 // AF_INET6
				address = ip.To16()
			}
			dnsEntries = append(dnsEntries, struct {
				Family  int32
				Address []byte
			}{Family: family, Address: address})
		}
		call := resolved.Call("org.freedesktop.resolve1.Manager.SetLinkDNS", 0, interfaceObj.Index, dnsEntries)
		if call.Err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set DNS via D-Bus for interface %s: %v\n", interfaceName, call.Err)
		}

		if len(searchKeys) > 0 {
			domains := []struct {
				Domain      string
				RoutingOnly bool
			}{}
			for _, domain := range searchKeys {
				domains = append(domains, struct {
					Domain      string
					RoutingOnly bool
				}{Domain: strings.TrimPrefix(domain, "~"), RoutingOnly: strings.HasPrefix(domain, "~")})
			}

			call = resolved.Call("org.freedesktop.resolve1.Manager.SetLinkDomains", 0, interfaceObj.Index, domains)
			if call.Err != nil {
				fmt.Fprintf(os.Stderr, "Failed to set search domains via D-Bus for interface %s: %v\n", interfaceName, call.Err)
			}
		}

		if call.Err == nil {
			if len(searchKeys) > 0 {
				Infof("Configured via D-Bus for Interface: %s DNS: %s Search Domain: %s", interfaceName, strings.Join(dnsServers, ", "), strings.Join(searchKeys, ", "))
			} else {
				Infof("Configured via D-Bus for Interface: %s DNS: %s", interfaceName, strings.Join(dnsServers, ", "))
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "D-Bus connection failed: %v\n", err)
	}
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
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

func ErrorHandler(context string, err error, exit bool) {
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("ERROR: %s: %v", context, err)
		} else {
			if context != "" {
				log.Fatalf("ERROR: %s: %v", context, err)
			} else {
				log.Fatalf("ERROR: %v", err)
			}
		}
	} else if context != "" {
		log.Printf("ERROR: %s", context)
	}

	if exit {
		os.Exit(1)
	}
}

func executeCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %s %v\nOutput: %s", name, args, string(output))
	}
	return string(output), nil
}

func findInterfaceInNetworks(networks *service.GetNetworksResponse, interfaceNames []string) []string {
	found := []string{}

	// Debug all available interface names for reference
	if len(interfaceNames) > 0 {
		availableInterfaces := []string{}
		for _, network := range *networks.JSON200 {
			if network.PortDeviceName != nil {
				availableInterfaces = append(availableInterfaces, *network.PortDeviceName)
			}
		}
		Debugf("Interface filtering requested for: %v", interfaceNames)
		Debugf("Available interfaces: %v", availableInterfaces)
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
			Debugf("Warning: Interface %s specified in filter was not found in ZeroTier networks", name)
		}
	}

	return found
}

func formatSliceDebug(slice []string, isExcludeFilter bool) string {
	if len(slice) == 0 {
		if isExcludeFilter {
			return "none (no filter applied)"
		} else {
			return "any (no filter applied)"
		}
	}
	return strings.Join(slice, ", ")
}

// genericFilter applies filtering based on the provided criteria
func genericFilter(networks *service.GetNetworksResponse, include, exclude []string, getKey func(network service.Network) *string) {
	Debugf("Filtering with include: %v, exclude: %v", include, exclude)

	includeAll := hasSpecialIncludeValue(include)
	excludeNone := hasSpecialExcludeValue(exclude)

	if includeAll && excludeNone {
		Debugf("No filtering needed - will process all items")
		return
	}

	filteredNetworks := []service.Network{}
	for _, network := range *networks.JSON200 {
		key := getKey(network)
		if key == nil {
			Debugf("Skipping network without required key")
			continue
		}

		keyValue := *key
		Debugf("Evaluating item: %s", keyValue)

		if !includeAll && !contains(include, keyValue) {
			Debugf("Item %s not in include list - skipping", keyValue)
			continue
		}

		if !excludeNone && contains(exclude, keyValue) {
			Debugf("Item %s is in exclude list - skipping", keyValue)
			continue
		}

		filteredNetworks = append(filteredNetworks, network)
	}

	Debugf("After filtering: %d of %d networks remain", len(filteredNetworks), len(*networks.JSON200))
	*networks.JSON200 = filteredNetworks
}

func getString(ptr *string) string {
	if ptr == nil {
		return "<nil>"
	}
	return *ptr
}

func hasSpecialIncludeValue(values []string) bool {
	if len(values) == 0 {
		return true // Empty slice means "include all"
	}

	for _, value := range values {
		if IncludeAllValues[strings.ToLower(value)] {
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
		if ExcludeNoneValues[strings.ToLower(value)] {
			return true
		}
	}
	return false
}

func loadAPIToken(tokenFile, tokenArg string) string {
	if tokenArg != "" {
		return tokenArg
	}
	content, err := os.ReadFile(tokenFile)
	if err != nil {
		Debugf("Failed to read token file %s: %v", tokenFile, err)
		return ""
	}
	return strings.TrimSpace(string(content))
}

func LoadConfig(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		Debugf("Error opening configuration file %s: %v", filePath, err)
		return Config{}, err
	}
	defer file.Close()

	Debugf("Successfully opened configuration file: %s", filePath)

	var config Config
	decoder := toml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		Debugf("Error decoding configuration file %s: %v", filePath, err)
		return Config{}, err
	}

	Debugf("Configuration loaded successfully from file: %s", filePath)

	return config, nil
}

func loadConfiguration(configFile string) Config {
	if configFile != "" {
		_, err := os.Stat(configFile)
		if err == nil {
			Debugf("Configuration file %s found, loading...", configFile)
			loadedConfig, err := LoadConfig(configFile)
			if err != nil {
				Debugf("Error loading configuration file %s: %v", configFile, err)
				ErrorHandler("Loading configuration file", err, true)
				return DefaultConfig()
			}

			// Apply application defaults for any unset fields in the loaded config
			defaultConfig := DefaultConfig()

			// Apply token file default if not set in config
			if loadedConfig.Default.TokenFile == "" {
				Debugf("TokenFile not set in config, using default: %s", defaultConfig.Default.TokenFile)
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
			Debugf("Configuration file %s does not exist", configFile)
			if configFile != "/etc/zt-dns-companion.conf" {
				Debugf("Explicitly provided configuration file %s not found", configFile)
				ErrorHandler(fmt.Sprintf("Configuration file %s not found", configFile), err, true)
			}
			Debugf("Default configuration file not found, using in-app defaults - Create it by passing -configFile /etc/zt-dns-companion.conf")
		} else {
			Debugf("Error checking configuration file %s: %v", configFile, err)
			ErrorHandler("Checking configuration file existence", err, true)
		}
	}

	Debugf("Using in-app defaults for configuration")
	return DefaultConfig()
}

func logWithLevel(level string, format string, args ...interface{}) {
	if (level == "info" && (currentLogLevel == "info" || currentLogLevel == "debug")) ||
		(level == "debug" && currentLogLevel == "debug") ||
		(level == "dryrun") {
		if logTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("%s %s: "+format+"\n", append([]interface{}{timestamp, strings.ToUpper(level)}, args...)...)
		} else {
			fmt.Printf("%s: "+format+"\n", append([]interface{}{strings.ToUpper(level)}, args...)...)
		}
	}
}

func Infof(format string, args ...interface{}) {
	logWithLevel("info", format, args...)
}

func Debugf(format string, args ...interface{}) {
	logWithLevel("debug", format, args...)
}

func DryRunf(format string, args ...interface{}) {
	logWithLevel("dryrun", format, args...)
}

func main() {
	version_arg := flag.Bool("version", false, "Print the version and exit")
	help_arg := flag.Bool("help", false, "Show help message and exit")
	configFile := flag.String("config-file", "/etc/zt-dns-companion.conf", "Path to the configuration file")
	dryRun_arg := flag.Bool("dry-run", false, "Enable dry-run mode. No changes will be made.")
	mode_arg := flag.String("mode", "auto", "Mode of operation (networkd, resolved, or auto).")
	host_arg := flag.String("host", "http://localhost", "ZeroTier client host address. Default: http://localhost")
	port_arg := flag.Int("port", 9993, "ZeroTier client port number. Default: 9993")
	logLevel_arg := flag.String("log-level", "info", "Set the logging level (info or debug). Default: info")
	logTimestamps_arg := flag.Bool("log-timestamps", false, "Enable timestamps in logs. Default: false")
	tokenFile_arg := flag.String("token-file", "/var/lib/zerotier-one/authtoken.secret", "Path to the ZeroTier authentication token file. Default: /var/lib/zerotier-one/authtoken.secret")

	filterType_arg := flag.String("filter-type", "none", "Type of filter to apply (interface, network, network_id, or none). Default: none")
	filterInclude_arg := flag.String("filter-include", "", "Comma-separated list of items to include based on filter-type. Empty means 'all'.")
	filterExclude_arg := flag.String("filter-exclude", "", "Comma-separated list of items to exclude based on filter-type. Empty means 'none'.")

	addReverseDomains_arg := flag.Bool("add-reverse-domains", false, "Add ip6.arpa and in-addr.arpa search domains. Default: false")
	autoRestart_arg := flag.Bool("auto-restart", true, "Automatically restart systemd-networkd when things change. Default: true")
	dnsOverTLS_arg := flag.Bool("dns-over-tls", false, "Automatically prefer DNS-over-TLS. Default: false")
	selectedProfile_arg := flag.String("profile", "", "Specify a profile to use from the configuration file. Default: none")
	multicastDNS_arg := flag.Bool("multicast-dns", false, "Enable Multicast DNS (mDNS). Default: false")
	reconcile_arg := flag.Bool("reconcile", true, "Automatically remove left networks from systemd-networkd configuration")
	token_arg := flag.String("token", "", "API token to use. Overrides token-file if provided.")

	flag.Parse()

	// Check for args that could be flags
	for i, arg := range os.Args {
		if i > 0 && strings.HasPrefix(arg, "-") {
			// Check if this is a flag without value
			if strings.HasPrefix(arg, "--") || (len(arg) > 1 && arg[1] != '-') {
				flagName := strings.TrimLeft(arg, "-")
				// If a flag that needs a value but doesn't have one, print an error
				if flagName == "log-level" || flagName == "mode" || flagName == "profile" ||
					flagName == "filter-type" || flagName == "filter-include" || flagName == "filter-exclude" ||
					flagName == "host" || flagName == "token" || flagName == "token-file" || flagName == "config-file" {

					// Check if there's a next arg that doesn't start with - (which would be the value)
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

	logTimestamps = *logTimestamps_arg

	SetLogLevel(*logLevel_arg)

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitFlags[f.Name] = true
		Debugf("Explicit flag detected: %s = %s", f.Name, f.Value.String())
	})

	var cfg Config = validateAndLoadConfig(*configFile)

	logTimestamps = *logTimestamps_arg || cfg.Default.LogTimestamps

	if *dryRun_arg {
		Debugf("Dry-run mode enabled")
	}

	// Apply all explicitly set flags before final configuration dump
	if explicitFlags["add-reverse-domains"] {
		cfg.Default.AddReverseDomains = *addReverseDomains_arg
	}
	if explicitFlags["auto-restart"] {
		cfg.Default.AutoRestart = *autoRestart_arg
	}
	if explicitFlags["dns-over-tls"] {
		cfg.Default.DNSOverTLS = *dnsOverTLS_arg
	}
	if explicitFlags["host"] {
		cfg.Default.Host = *host_arg
	}
	if explicitFlags["log-level"] {
		cfg.Default.LogLevel = *logLevel_arg
	}
	if explicitFlags["log-timestamps"] {
		cfg.Default.LogTimestamps = *logTimestamps_arg
	}
	if explicitFlags["mode"] {
		cfg.Default.Mode = *mode_arg
	}
	if explicitFlags["multicast-dns"] {
		cfg.Default.MulticastDNS = *multicastDNS_arg
	}
	if explicitFlags["port"] {
		cfg.Default.Port = *port_arg
	}
	if explicitFlags["reconcile"] {
		cfg.Default.Reconcile = *reconcile_arg
	}
	if explicitFlags["token"] {
		// Token is handled separately via loadAPIToken
	}
	if explicitFlags["token-file"] {
		cfg.Default.TokenFile = *tokenFile_arg
	}
	if explicitFlags["filter-type"] {
		cfg.Default.FilterType = *filterType_arg
	}

	// Handle filter-include and filter-exclude flags
	if explicitFlags["filter-include"] {
		// Empty string means "all" so only split if the string isn't empty
		if *filterInclude_arg != "" {
			cfg.Default.FilterInclude = strings.Split(*filterInclude_arg, ",")
		} else {
			// Empty string means use empty slice to indicate "all" interfaces/networks
			cfg.Default.FilterInclude = []string{}
		}
	}

	if explicitFlags["filter-exclude"] {
		// Empty string means "none" so only split if the string isn't empty
		if *filterExclude_arg != "" {
			cfg.Default.FilterExclude = strings.Split(*filterExclude_arg, ",")
		} else {
			// Empty string means use empty slice to indicate "none" excluded
			cfg.Default.FilterExclude = []string{}
		}
	}

	// Debugf("*** Final Effective Configuration ***")
	// Debugf("*  DryRun: %t", *dryRun_arg)
	// if *selectedProfile_arg != "" {
	// 	Debugf("*  Selected Profile: %s", *selectedProfile_arg)
	// } else {
	// 	Debugf("*  Selected Profile: default (no profile specified)")
	// }
	// Debugf("*  AddReverseDomains: %t (default: false, explicitly set: %t)", cfg.Default.AddReverseDomains, explicitFlags["add-reverse-domains"])
	// Debugf("*  AutoRestart: %t (default: true, explicitly set: %t)", cfg.Default.AutoRestart, explicitFlags["auto-restart"])
	// Debugf("*  DNSOverTLS: %t (default: false, explicitly set: %t)", cfg.Default.DNSOverTLS, explicitFlags["dns-over-tls"])
	// Debugf("*  FilterType: %s (default: none, explicitly set: %t)", cfg.Default.FilterType, explicitFlags["filter-type"])
	// Debugf("*  FilterInclude: %s", formatSliceDebug(cfg.Default.FilterInclude, false))
	// Debugf("*  FilterExclude: %s", formatSliceDebug(cfg.Default.FilterExclude, true))
	// Debugf("*  Host: %s", cfg.Default.Host)
	// Debugf("*  LogLevel: %s (default: info)", cfg.Default.LogLevel)
	// Debugf("*  LogTimestamps: %t (default: false, explicitly set: %t)", cfg.Default.LogTimestamps, explicitFlags["log-timestamps"])
	// Debugf("*  Mode: %s", cfg.Default.Mode)
	// Debugf("*  MulticastDNS: %t (default: false, explicitly set: %t)", cfg.Default.MulticastDNS, explicitFlags["multicast-dns"])
	// Debugf("*  Port: %d (default: 9993)", cfg.Default.Port)
	// Debugf("*  Reconcile: %t (default: true, explicitly set: %t)", cfg.Default.Reconcile, explicitFlags["reconcile"])
	// Debugf("*  TokenFile: %s (default: /var/lib/zerotier-one/authtoken.secret)", cfg.Default.TokenFile)
	// Debugf("*****************************************")

	if *version_arg {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	if *help_arg {
		flag.Usage()
		os.Exit(0)
	}

	if os.Geteuid() != 0 {
		ErrorHandler("You need to be root to run this program", nil, true)
	}

	if runtime.GOOS != "linux" {
		ErrorHandler("This tool is only needed on Linux", nil, true)
	}

	profileNames := []string{}
	for name := range cfg.Profiles {
		profileNames = append(profileNames, name)
	}

	if len(cfg.Profiles) > 0 {
		Debugf("Profiles found in configuration: %v", profileNames)
		if *selectedProfile_arg == "" {
			Debugf("Loading default profile.")
		} else if selectedProfile, ok := cfg.Profiles[*selectedProfile_arg]; ok {
			Debugf("Applying selected profile: %s", *selectedProfile_arg)
			cfg.Default = mergeProfiles(cfg.Default, selectedProfile)
		} else {
			Debugf("Selected profile '%s' not found. Using default profile.", *selectedProfile_arg)
		}
	} else {
		Debugf("Using default profile")
	}

	// Override configuration with explicitly passed flags
	if explicitFlags["port"] {
		cfg.Default.Port = *port_arg
	}
	if explicitFlags["host"] {
		cfg.Default.Host = *host_arg
	}
	if explicitFlags["mode"] {
		cfg.Default.Mode = *mode_arg
	}
	if explicitFlags["log-level"] {
		cfg.Default.LogLevel = *logLevel_arg
	}
	if explicitFlags["token-file"] {
		cfg.Default.TokenFile = *tokenFile_arg
	}
	if explicitFlags["add-reverse-domains"] {
		cfg.Default.AddReverseDomains = *addReverseDomains_arg
	}
	if explicitFlags["filter-type"] {
		cfg.Default.FilterType = *filterType_arg
	}
	if explicitFlags["filter-include"] {
		if *filterInclude_arg != "" {
			cfg.Default.FilterInclude = strings.Split(*filterInclude_arg, ",")
		} else {
			cfg.Default.FilterInclude = []string{}
		}
	}
	if explicitFlags["filter-exclude"] {
		if *filterExclude_arg != "" {
			cfg.Default.FilterExclude = strings.Split(*filterExclude_arg, ",")
		} else {
			cfg.Default.FilterExclude = []string{}
		}
	}

	if explicitFlags["auto-restart"] {
		cfg.Default.AutoRestart = *autoRestart_arg
	}
	if explicitFlags["dns-over-tls"] {
		cfg.Default.DNSOverTLS = *dnsOverTLS_arg
	}
	if explicitFlags["log-timestamps"] {
		cfg.Default.LogTimestamps = *logTimestamps_arg
	}

	var modeDetected bool
	if *mode_arg == "auto" || cfg.Default.Mode == "auto" {
		networkdOutput, networkdErr := executeCommand("systemctl", "is-active", "systemd-networkd.service")
		networkdActive := networkdErr == nil && strings.TrimSpace(networkdOutput) == "active"

		resolvedOutput, resolvedErr := executeCommand("systemctl", "is-active", "systemd-resolved.service")
		resolvedActive := resolvedErr == nil && strings.TrimSpace(resolvedOutput) == "active"

		if networkdActive {
			cfg.Default.Mode = "networkd"
			modeDetected = true
		} else if resolvedActive {
			cfg.Default.Mode = "resolved"
			modeDetected = true
		} else {
			ErrorHandler("Neither systemd-networkd nor systemd-resolved is running. Please manually set the mode using the -mode flag or configuration file.", nil, true)
		}
	} else if *mode_arg != "" {
		cfg.Default.Mode = *mode_arg
	} else {
		modeDetected = false
	}

	if *port_arg == 9993 {
		if cfg.Default.Port != 0 {
			*port_arg = cfg.Default.Port
		}
	}

	ztBaseURL := fmt.Sprintf("%s:%d", cfg.Default.Host, *port_arg)

	apiToken := loadAPIToken(cfg.Default.TokenFile, *token_arg)
	_ = apiToken // Placeholder to ensure the variable is used without exposing it in logs or functionality
	sAPI, err := newServiceAPI(cfg.Default.TokenFile)
	if err != nil {
		ErrorHandler(fmt.Sprintf("Failed to initialize service API client: %v", err), err, true)
	}

	client, err := service.NewClient(ztBaseURL, service.WithHTTPClient(sAPI))
	if err != nil {
		ErrorHandler(fmt.Sprintf("Failed to create ZeroTier client: %v", err), err, true)
	}

	Debugf("Fetching networks from ZeroTier API using base URL: %s", ztBaseURL)
	resp, err := client.GetNetworks(context.Background())
	if err != nil {
		ErrorHandler("Failed to get networks from ZeroTier client", err, true)
	}

	networks, err := service.ParseGetNetworksResponse(resp)
	if err != nil {
		Debugf("Failed to parse networks response: %v", err)
		ErrorHandler("Failed to parse networks response", err, true)
	}

	// Apply unified filters to the networks
	applyUnifiedFilters(networks, cfg.Default)

	switch cfg.Default.Mode {
	case "networkd":
		Debugf("Running in networkd mode%s", func() string {
			if modeDetected {
				return " (detected)"
			}
			return ""
		}())
		runNetworkdMode(networks, addReverseDomains_arg, autoRestart_arg, dnsOverTLS_arg, dryRun_arg, multicastDNS_arg, reconcile_arg)
	case "resolved":
		output, err := executeCommand("systemctl", "is-active", "systemd-resolved.service")
		if err != nil || strings.TrimSpace(output) != "active" {
			ErrorHandler("systemd-resolved is not running. Resolved mode requires systemd-resolved to be active.", err, true)
		}
		Debugf("Running in resolved mode%s", func() string {
			if modeDetected {
				return " (detected)"
			}
			return ""
		}())
		Debugf("systemd-resolved is running and active")
		runResolvedMode(networks, addReverseDomains_arg, dryRun_arg)
	default:
		ErrorHandler("Invalid mode specified in configuration", nil, true)
	}
}

func mergeProfiles(defaultProfile, selectedProfile Profile) Profile {
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
		Debugf("Applying filter include from profile: %v", selectedProfile.FilterInclude)
		mergedProfile.FilterInclude = selectedProfile.FilterInclude
	}
	if len(selectedProfile.FilterExclude) > 0 {
		Debugf("Applying filter exclude from profile: %v", selectedProfile.FilterExclude)
		mergedProfile.FilterExclude = selectedProfile.FilterExclude
	}
	if selectedProfile.FilterType != "" {
		Debugf("Applying filter type from profile: %s", selectedProfile.FilterType)
		mergedProfile.FilterType = selectedProfile.FilterType
	}

	if !selectedProfile.AutoRestart {
		Debugf("Profile is disabling AutoRestart")
		mergedProfile.AutoRestart = false
	}
	if !selectedProfile.Reconcile {
		Debugf("Profile is disabling Reconcile")
		mergedProfile.Reconcile = false
	}

	if selectedProfile.DNSOverTLS {
		Debugf("Profile is enabling DNSOverTLS")
		mergedProfile.DNSOverTLS = true
	}
	if selectedProfile.AddReverseDomains {
		Debugf("Profile is enabling AddReverseDomains")
		mergedProfile.AddReverseDomains = true
	}
	if selectedProfile.LogTimestamps {
		Debugf("Profile is enabling LogTimestamps")
		mergedProfile.LogTimestamps = true
	}
	if selectedProfile.MulticastDNS {
		Debugf("Profile is enabling MulticastDNS")
		mergedProfile.MulticastDNS = true
	}

	return mergedProfile
}

func newServiceAPI(tokenFile string) (*serviceAPIClient, error) {
	content, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file %s: %w", tokenFile, err)
	}

	return &serviceAPIClient{
		apiKey: strings.TrimSpace(string(content)),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func parseResolvectlOutput(output string, prefix string) []string {
	parsed := []string{}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) > 1 {
				value := strings.TrimSpace(parts[1])
				if value != "" {
					parsed = append(parsed, value)
				}
			}
		}
	}
	return parsed
}

func runNetworkdMode(networks *service.GetNetworksResponse, addReverseDomains_arg, autoRestart_arg, dnsOverTLS_arg, dryRun_arg, multicastDNS_arg, reconcile_arg *bool) {
	const fileheader = "--- Managed by zt-dns-companion. Do not remove this comment. ---"
	const networkTemplate = `# {{ .FileHeader }}
[Match]
Name={{ .ZTInterface }}

[Network]
Description={{ .ZTNetwork }}
DHCP=no
{{ range $key := .DNS -}}
DNS={{ $key }}
{{ end -}}
{{ if .DNS_TLS -}}
DNSOverTLS=yes
{{ end -}}
{{ if .MDNS -}}
MulticastDNS=yes
{{ end -}}
Domains=~{{ .Domain }}
ConfigureWithoutCarrier=true
KeepConfiguration=static
`

	t, err := template.New("network").Parse(networkTemplate)
	if err != nil {
		Debugf("Template parsing error: %v", err)
		ErrorHandler("Failed to parse template", err, true)
	}

	serviceAvailable := serviceExists("systemd-networkd.service")
	if !serviceAvailable {
		Debugf("systemd-networkd.service is not available; changes will not trigger a service restart")
	} else {
		Debugf("systemd-networkd.service is available")
	}

	found := map[string]struct{}{}

	var changed bool

	for _, network := range *networks.JSON200 {
		Debugf("Processing network: Interface=%s, Name=%s, ID=%s",
			getString(network.PortDeviceName), getString(network.Name), getString(network.Id))

		fn := fmt.Sprintf("%s/99-%s.network", "/etc/systemd/network", *network.PortDeviceName)

		delete(found, path.Base(fn))

		search := map[string]struct{}{}

		if network.Dns.Domain != nil {
			search[*network.Dns.Domain] = struct{}{}
			Debugf("Added DNS domain to search: %s, DNS servers: %v", *network.Dns.Domain, *network.Dns.Servers)
		}

		if *addReverseDomains_arg {
			reverseDomains := calculateReverseDomains(network.AssignedAddresses)
			for _, domain := range reverseDomains {
				search[domain] = struct{}{}
				Debugf("Added reverse domain to search: %s", domain)
			}
		}

		searchkeys := []string{}
		for key := range search {
			searchkeys = append(searchkeys, key)
		}
		sort.Strings(searchkeys)

		out := templateScaffold{
			ZTInterface: *network.PortDeviceName,
			ZTNetwork:   *network.Name,
			DNS:         *network.Dns.Servers,
			Domain:      strings.Join(searchkeys, " "),
			FileHeader:  fileheader,
			DNS_TLS:     *dnsOverTLS_arg,
			MDNS:        *multicastDNS_arg,
		}

		buf := bytes.NewBuffer(nil)
		if err := t.Execute(buf, out); err != nil {
			Debugf("Error executing template for %q: %v", fn, err)
			ErrorHandler(fmt.Sprintf("Failed to execute template for %q", fn), err, true)
		}

		if *dryRun_arg {
			DryRunf("Would generate %q with DNS servers: %s and search domains: %s", fn, strings.Join(out.DNS, ", "), out.Domain)
			continue
		}

		if _, err := os.Stat(fn); err == nil {
			content, err := os.ReadFile(fn)
			if err != nil {
				Debugf("Error reading file %s: %v", fn, err)
				ErrorHandler(fmt.Sprintf("Failed to read file %q", fn), err, true)
			}

			if bytes.Equal(content, buf.Bytes()) {
				Infof("No changes needed for file %s; already up-to-date", fn)
				continue
			}
		}

		Debugf("Creating or overwriting file %s", fn)
		f, err := os.Create(fn)
		if err != nil {
			Debugf("Error creating file %s: %v", fn, err)
			ErrorHandler(fmt.Sprintf("Failed to create file %q", fn), err, true)
		}
		Debugf("Successfully created file %s", fn)

		if _, err := f.Write(buf.Bytes()); err != nil {
			Debugf("Error writing to file %s: %v", fn, err)
			ErrorHandler("Failed to write to file", err, true)
		}
		Debugf("Successfully wrote to file %s", fn)

		f.Close()
		Debugf("Closed file %s", fn)

		changed = true

		if changed {
			Infof("Processed Interface=%s, Network=%s, ID=%s, DNS Search Domain=%s, DNS Servers=%v, wrote to /etc/systemd/network/99-%s.network",
				getString(network.PortDeviceName), getString(network.Name), getString(network.Id),
				getString(network.Dns.Domain), *network.Dns.Servers, getString(network.PortDeviceName))
		}
	}

	if len(found) > 0 && *reconcile_arg {
		Infof("Found unused networks, reconciling...")

		for fn := range found {
			Infof("Removing %q", fn)

			if *dryRun_arg {
				DryRunf("Would remove %q", fn)
				continue
			}

			if err := os.Remove(filepath.Join("/etc/systemd/network", fn)); err != nil {
				ErrorHandler(fmt.Sprintf("Failed to remove file %q", fn), err, true)
			}
		}
	}

	if (changed || len(found) > 0) && *autoRestart_arg && serviceAvailable {
		Infof("Files changed; reloading systemd-networkd...")

		if *dryRun_arg {
			DryRunf("Would reload systemd-networkd")
			return
		}

		if err := exec.Command("networkctl", "reload").Run(); err != nil {
			ErrorHandler("Failed to reload systemd-networkd", err, true)
		}
	}
}

func runResolvedMode(networks *service.GetNetworksResponse, addReverseDomains_arg, dryRun_arg *bool) {
	if !commandExists("resolvectl") {
		ErrorHandler("resolvectl is required for systemd-resolved but is not available on this system", nil, true)
	}
	Debugf("resolvectl is available for systemd-resolved commands")

	for _, network := range *networks.JSON200 {
		Debugf("Processing network: Interface=%s, Name=%s, ID=%s", getString(network.PortDeviceName), getString(network.Name), getString(network.Id))

		if network.Dns != nil && len(*network.Dns.Servers) != 0 {
			interfaceName := *network.PortDeviceName
			dnsServers := *network.Dns.Servers
			dnsSearch := ""

			if network.Dns.Domain != nil {
				dnsSearch = *network.Dns.Domain
			}

			// Calculate in-addr.arpa and ip6.arpa search domains
			searchDomains := map[string]struct{}{}
			if dnsSearch != "" {
				searchDomains[dnsSearch] = struct{}{}
			}

			if *addReverseDomains_arg {
				reverseDomains := calculateReverseDomains(network.AssignedAddresses)
				for _, domain := range reverseDomains {
					searchDomains[domain] = struct{}{}
				}
			}

			searchKeys := []string{}
			for key := range searchDomains {
				searchKeys = append(searchKeys, key)
			}
			sort.Strings(searchKeys)

			configureDNSAndSearchDomains(interfaceName, dnsServers, searchKeys, *dryRun_arg)
		}
	}
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

func SetLogLevel(level string) {
	if strings.ToLower(level) == "debug" {
		currentLogLevel = "debug"
	} else {
		currentLogLevel = "info"
	}

	// Set log package flags based on the logTimestamps setting
	if logTimestamps {
		log.SetFlags(log.LstdFlags)
	} else {
		log.SetFlags(0)
	}
}

func serviceExists(serviceName string) bool {
	cmd := exec.Command("systemctl", "status", serviceName)
	return cmd.Run() == nil
}

func validateAndLoadConfig(configFile string) Config {
	Debugf("Loading configuration from file: %s", configFile)
	cfg := loadConfiguration(configFile)

	err := ValidateConfig(&cfg)
	if err != nil {
		Debugf("Configuration validation failed: %v", err)
		ErrorHandler("Validating configuration", err, true)
	} else {
		Debugf("Configuration validation succeeded")
	}

	return cfg
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

	includeAll := hasSpecialIncludeValue(cfg.Default.FilterInclude)
	excludeNone := hasSpecialExcludeValue(cfg.Default.FilterExclude)

	if !includeAll && !excludeNone {
		for _, item := range cfg.Default.FilterInclude {
			if contains(cfg.Default.FilterExclude, item) {
				return fmt.Errorf("conflicting filter configuration: '%s' is both included and excluded", item)
			}
		}
	}

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

		includeAll = hasSpecialIncludeValue(profile.FilterInclude)
		excludeNone = hasSpecialExcludeValue(profile.FilterExclude)

		if !includeAll && !excludeNone && len(profile.FilterInclude) > 0 && len(profile.FilterExclude) > 0 {
			for _, item := range profile.FilterInclude {
				if contains(profile.FilterExclude, item) {
					return fmt.Errorf("conflicting filter configuration in profile %s: '%s' is both included and excluded",
						name, item)
				}
			}
		}
	}

	return nil
}
