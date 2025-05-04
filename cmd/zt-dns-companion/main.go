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
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/nfrastack/zt-dns-companion/internal/config"
	"github.com/zerotier/go-zerotier-one/service"
)

var (
	currentLogLevel = "info"
	parsedTemplate  *template.Template
	templateOnce    sync.Once
	Version         = "dev" // Default value
)

func loadTemplateFromFile() string {
	basePath, err := os.Getwd()
	if err != nil {
		logErrorAndExit("Failed to get working directory", err)
	}

	templatePath := filepath.Join(basePath, "templates", "networkd", "template.network")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		logErrorAndExit("Failed to read template file", err)
	}

	return string(content)
}

func SetLogLevel(level string) {
	if strings.ToLower(level) == "debug" {
		currentLogLevel = "debug"
	} else {
		currentLogLevel = "info"
	}
}

func Infof(format string, args ...interface{}) {
	if currentLogLevel == "info" || currentLogLevel == "debug" {
		fmt.Printf("INFO: "+format+"\n", args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if currentLogLevel == "debug" {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

// commandExists checks if a command exists in the system
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// serviceExists checks if a service exists in systemd
func serviceExists(serviceName string) bool {
	cmd := exec.Command("systemctl", "status", serviceName)
	return cmd.Run() == nil
}

// logErrorAndExit logs an error and exits the program
func logErrorAndExit(message string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
		os.Exit(1)
	}
}

// errExit logs an error message and exits the program
func errExit(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}

// parseResolvectlOutput parses the output of resolvectl commands
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

// parseTemplate ensures the template is parsed only once
func parseTemplate() *template.Template {
	templateOnce.Do(func() {
		var err error
		parsedTemplate, err = template.New("network").Parse(loadTemplateFromFile())
		if err != nil {
			errExit(fmt.Errorf("failed to parse template: %w", err))
		}
	})
	return parsedTemplate
}

// calculateReverseDomains calculates in-addr.arpa and ip6.arpa domains
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

// serviceAPIClient wraps the ZeroTier service API client
type serviceAPIClient struct {
	apiKey string
	client *http.Client
}

// Do initiates a client transaction
func (c *serviceAPIClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-ZT1-Auth", c.apiKey)
	return c.client.Do(req)
}

// main contains the primary logic for the application
func main() {
	// Parse command-line arguments
	configFile := flag.String("config", "/etc/zt-dns-companion.conf", "Path to the configuration file")
	addReverseDomains_arg := flag.Bool("add-reverse-domains", false, "Add ip6.arpa and in-addr.arpa search domains")
	autoRestart_arg := flag.Bool("auto-restart", true, "Automatically restart systemd-resolved when things change")
	dnsOverTLS_arg := flag.Bool("dns-over-tls", false, "Automatically prefer DNS-over-TLS. Requires ZeroNSd v0.4 or better")
	dryRun_arg := flag.Bool("dry-run", false, "Simulate changes without applying them")
	host_arg := flag.String("host", "http://localhost", "ZeroTier client host address")
	logLevel_arg := flag.String("log-level", "info", "Set the logging level (info or debug)")
	mode_arg := flag.String("mode", "networkd", "Mode of operation: networkd or resolved")
	multicastDNS_arg := flag.Bool("multicast-dns", false, "Enable mDNS resolution on the zerotier interface.")
	port_arg := flag.Int("port", 9993, "ZeroTier client port number")
	reconcile_arg := flag.Bool("reconcile", true, "Automatically remove left networks from systemd-networkd configuration")
	tokenFile_arg := flag.String("token-file", "/var/lib/zerotier-one/authtoken.secret", "Path to the ZeroTier authentication token file")
	token_arg := flag.String("token", "", "ZeroTier authentication token (overrides token-file if provided")
	version_arg := flag.Bool("version", false, "Print the version and exit")

	// Add a custom usage function to display the header and flags
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ZT DNS Companion\nCopyright © 2025 nfrastack <https://nfrastack.com>\nCopyright © 2021 ZeroTier Inc. <https://zerotier.com>\n")
		fmt.Fprintf(os.Stderr, "Version: %s\n\n", Version)
		fmt.Fprintf(os.Stderr, "This program is designed to populate per interface entries with upstream DNS information as provided by a ZeroTier controller.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}

	flag.Parse()

	if *version_arg {
		fmt.Printf("%s\n", Version)
		os.Exit(0)
	}

	// Load configuration
	cfg := config.DefaultConfig()
	if _, err := os.Stat(*configFile); err == nil {
		loadedConfig, err := config.LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config file %s: %v\n", *configFile, err)
			os.Exit(1)
		}
		cfg = config.MergeConfig(cfg, loadedConfig)
	} else if *configFile != "/etc/zt-dns-companion.conf" {
		// Create default config if custom file doesn't exist
		if err := config.SaveConfig(*configFile, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create config file %s: %v\n", *configFile, err)
			os.Exit(1)
		}
		fmt.Printf("Configuration file %s not found. A default one has been created.\n", *configFile)
	}

	// Override with command-line arguments
	cmdOverrides := config.Config{
		AddReverseDomains: *addReverseDomains_arg,
		AutoRestart:       *autoRestart_arg,
		DNSOverTLS:        *dnsOverTLS_arg,
		DryRun:            *dryRun_arg,
		Host:              *host_arg,
		LogLevel:          *logLevel_arg,
		Mode:              *mode_arg,
		MulticastDNS:      *multicastDNS_arg,
		Port:              *port_arg,
		Reconcile:         *reconcile_arg,
		TokenFile:         *tokenFile_arg,
		Token:             *token_arg,
	}
	cfg = config.MergeConfig(cfg, cmdOverrides)

	// Set log level
	SetLogLevel(cfg.LogLevel)

	// Check for root privileges
	if os.Geteuid() != 0 {
		errExit(fmt.Errorf("ERROR: You need to be root to run this program"))
	}

	// Check for Linux OS
	if runtime.GOOS != "linux" {
		errExit(fmt.Errorf("ERROR: This tool is only needed on Linux"))
	}

	// Use the configuration values in the application logic
	apiToken := cfg.Token
	if apiToken == "" {
		content, err := os.ReadFile(cfg.TokenFile)
		if err != nil {
			errExit(fmt.Errorf("failed to read token file %s: %w", cfg.TokenFile, err))
		}
		apiToken = strings.TrimSpace(string(content))
	}

	sAPI := &serviceAPIClient{apiKey: apiToken, client: &http.Client{}}

	ztBaseURL := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := service.NewClient(ztBaseURL, service.WithHTTPClient(sAPI))
	if err != nil {
		errExit(err)
	}

	resp, err := client.GetNetworks(context.Background())
	if err != nil {
		errExit(err)
	}

	networks, err := service.ParseGetNetworksResponse(resp)
	if err != nil {
		errExit(err)
	}

	switch cfg.Mode {
	case "networkd":
		runNetworkdMode(networks, &cfg.AddReverseDomains, &cfg.AutoRestart, &cfg.DNSOverTLS, &cfg.DryRun, &cfg.MulticastDNS, &cfg.Reconcile)
	case "resolved":
		runResolvedMode(networks, &cfg.AddReverseDomains, &cfg.DryRun)
	default:
		errExit(fmt.Errorf("invalid mode. Supported modes are: networkd, resolved"))
	}
}

// runNetworkdMode handles the logic for networkd mode
func runNetworkdMode(networks *service.GetNetworksResponse, addReverseDomains_arg, autoRestart_arg, dnsOverTLS_arg, dryRun_arg, multicastDNS_arg, reconcile_arg *bool) {
	const fileheader = "--- Managed by zt-dns-companion. Do not remove this comment. ---"

	serviceAvailable := serviceExists("systemd-networkd.service")
	if !serviceAvailable {
		Debugf("systemd-networkd.service is not available; changes will not trigger a service restart")
	} else {
		Debugf("systemd-networkd.service is available")
	}

	Debugf("Running in networkd mode")

	t := parseTemplate()

	dir, err := os.ReadDir("/etc/systemd/network")
	if err != nil {
		errExit(err)
	}

	found := map[string]struct{}{}

	for _, item := range dir {
		if item.Type().IsRegular() && strings.HasSuffix(item.Name(), ".network") {
			content, err := os.ReadFile(filepath.Join("/etc/systemd/network", item.Name()))
			if err != nil {
				errExit(err)
			}

			if bytes.Contains(content, []byte(fileheader)) {
				found[item.Name()] = struct{}{}
			}
		}
	}

	var changed bool

	for _, network := range *networks.JSON200 {
		if network.Dns != nil && len(*network.Dns.Servers) != 0 {
			fn := fmt.Sprintf("%s/99-%s.network", "/etc/systemd/network", *network.PortDeviceName)

			delete(found, path.Base(fn))

			search := map[string]struct{}{}

			if network.Dns.Domain != nil {
				search[*network.Dns.Domain] = struct{}{}
			}

			// This calculates in-addr.arpa and ip6.arpa search domains by calculating them from the IP assignments.
			if *addReverseDomains_arg {
				reverseDomains := calculateReverseDomains(network.AssignedAddresses)
				for _, domain := range reverseDomains {
					search[domain] = struct{}{}
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
				errExit(fmt.Errorf("%q: %w", fn, err))
			}

			if *dryRun_arg {
				Infof("Dry-run: Would generate %q with DNS servers: %s and search domains: %s", fn, strings.Join(out.DNS, ", "), out.Domain)
				continue
			}

			if _, err := os.Stat(fn); err == nil {
				content, err := os.ReadFile(fn)
				if err != nil {
					errExit(fmt.Errorf("in %v: %w", fn, err))
				}

				if bytes.Equal(content, buf.Bytes()) {
					Infof("No changes made to %q; already up-to-date", fn)
					continue
				}
			}

			Infof("Generating %q with DNS servers: %s and search domains: %s", fn, strings.Join(out.DNS, ", "), out.Domain)
			Infof("Generating %q", fn)
			f, err := os.Create(fn)
			if err != nil {
				errExit(fmt.Errorf("%q: %w", fn, err))
			}

			if _, err := f.Write(buf.Bytes()); err != nil {
				errExit(err)
			}

			f.Close()

			changed = true
		}
	}

	if len(found) > 0 && *reconcile_arg {
		Infof("Found unused networks, reconciling...")

		for fn := range found {
			Infof("Removing %q", fn)

			if *dryRun_arg {
				Infof("Dry-run: Would remove %q", fn)
				continue
			}

			if err := os.Remove(filepath.Join("/etc/systemd/network", fn)); err != nil {
				errExit(fmt.Errorf("while removing %q: %w", fn, err))
			}
		}
	}

	if (changed || len(found) > 0) && *autoRestart_arg && serviceAvailable {
		Infof("Files changed; reloading systemd-networkd...")

		if *dryRun_arg {
			Infof("Dry-run: Would reload systemd-networkd")
			return
		}

		if err := exec.Command("networkctl", "reload").Run(); err != nil {
			errExit(fmt.Errorf("while reloading systemd-networkd: %v", err))
		}
	}
}

// runResolvedMode handles the logic for resolved mode
func runResolvedMode(networks *service.GetNetworksResponse, addReverseDomains_arg, dryRun_arg *bool) {
	if !commandExists("resolvectl") {
		logErrorAndExit("resolvectl is required for systemd-resolved but is not available on this system", nil)
	}
	Debugf("resolvectl is available for systemd-resolved commands")
	Debugf("Running in resolved mode")

	for _, network := range *networks.JSON200 {
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

			if *dryRun_arg {
				Infof("Dry-run: Would set DNS for %s: %s", interfaceName, strings.Join(dnsServers, ", "))
				Infof("Dry-run: Would set search domains for %s: %s", interfaceName, strings.Join(searchKeys, ", "))
				continue
			}

			// Query current settings using resolvectl
			cmd := exec.Command("resolvectl", "dns", interfaceName)
			output, err := cmd.Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to query DNS via resolvectl for %s: %v\n", interfaceName, err)
				continue
			}
			currentDNS := parseResolvectlOutput(string(output), "Link ")
			Debugf("Parsed current DNS for %s: %v", interfaceName, currentDNS)

			cmd = exec.Command("resolvectl", "domain", interfaceName)
			output, err = cmd.Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to query search domains via resolvectl for %s: %v\n", interfaceName, err)
				continue
			}
			currentDomains := parseResolvectlOutput(string(output), "Link ")
			Debugf("Parsed current search domains for %s: %v", interfaceName, currentDomains)

			// Compare current and desired settings
			Debugf("Desired DNS for %s: %v", interfaceName, dnsServers)
			Debugf("Desired search domains for %s: %v", interfaceName, searchKeys)

			sameDNS := len(currentDNS) == len(dnsServers)
			if sameDNS {
				dnsSet := make(map[string]struct{})
				for _, current := range currentDNS {
					dnsSet[strings.TrimSpace(current)] = struct{}{}
				}
				for _, server := range dnsServers {
					if _, exists := dnsSet[server]; !exists {
						sameDNS = false
						break
					}
				}
			}

			sameDomains := len(currentDomains) == len(searchKeys)
			if sameDomains {
				domainSet := make(map[string]struct{})
				for _, current := range currentDomains {
					domainSet[strings.TrimSpace(current)] = struct{}{}
				}
				for _, domain := range searchKeys {
					if _, exists := domainSet[domain]; !exists {
						sameDomains = false
						break
					}
				}
			}

			Debugf("Comparison result for %s: sameDNS=%v, sameDomains=%v", interfaceName, sameDNS, sameDomains)

			if sameDNS && sameDomains {
				Infof("No changes needed for %s; DNS and search domains are already up-to-date", interfaceName)
				continue
			}

			// Attempt to configure via D-Bus
			conn, err := dbus.SystemBus()
			if err == nil {
				interfaceObj, err := net.InterfaceByName(interfaceName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to get interface index for %s: %v\n", interfaceName, err)
					continue
				}

				resolved := conn.Object("org.freedesktop.resolve1", "/org/freedesktop/resolve1")

				// Prepare DNS server arguments
				Infof("Setting DNS for %s: %s", interfaceName, strings.Join(dnsServers, ", "))
				Debugf("Setting DNS with arguments: %v", dnsEntries)
				call := resolved.Call("org.freedesktop.resolve1.Manager.SetLinkDNS", 0, interfaceObj.Index, dnsEntries)
				if call.Err != nil {
					fmt.Fprintf(os.Stderr, "Failed to set DNS via D-Bus for %s: %v\n", interfaceName, call.Err)
				} else {
					Infof("Configured DNS via D-Bus for %s", interfaceName)
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

					Infof("Setting search domains for %s: %s", interfaceName, strings.Join(searchKeys, ", "))
					Debugf("Setting search domains with arguments: %v", domains)
					call = resolved.Call("org.freedesktop.resolve1.Manager.SetLinkDomains", 0, interfaceObj.Index, domains)
					if call.Err != nil {
						fmt.Fprintf(os.Stderr, "Failed to set search domains via D-Bus for %s: %v\n", interfaceName, call.Err)
					} else {
						Infof("Configured search domains via D-Bus for %s", interfaceName)
					}
				}
			} else {
				fmt.Fprintf(os.Stderr, "D-Bus connection failed: %v\n", err)
				// Fallback to resolvectl
				cmd := exec.Command("resolvectl", "dns", interfaceName, strings.Join(dnsServers, " "))
				Infof("Executing: %s", strings.Join(cmd.Args, " "))
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to set DNS via resolvectl for %s: %v\n", interfaceName, err)
				} else {
					Infof("Configured DNS via resolvectl for %s", interfaceName)
				}

				if len(searchKeys) > 0 {
					cmd = exec.Command("resolvectl", "domain", interfaceName, strings.Join(searchKeys, " "))
					Infof("Executing: %s", strings.Join(cmd.Args, " "))
					if err := cmd.Run(); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to set search domains via resolvectl for %s: %v\n", interfaceName, err)
					} else {
						Infof("Configured search domains via resolvectl for %s", interfaceName)
					}
				}
			}
		}
	}
}

// templateScaffold represents the parameter list for multiple template operations
type templateScaffold struct {
	FileHeader  string
	ZTInterface string
	ZTNetwork   string
	DNS         []string
	Domain      string
	DNS_TLS     bool
	MDNS        bool
}
