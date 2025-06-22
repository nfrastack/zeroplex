// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zeroflex/pkg/dns"
	"zeroflex/pkg/log"
	"zeroflex/pkg/utils"

	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zerotier/go-zerotier-one/service"
)

type templateScaffold struct {
	FileHeader  string
	ZTInterface string
	ZTNetwork   string
	DNS         []string
	Domain      string
	DNS_TLS     bool
	MDNS        bool
}

func RunNetworkdMode(networks *service.GetNetworksResponse, addReverseDomains, autoRestart, dnsOverTLS, dryRun, multicastDNS, reconcile bool) {
	logger := log.NewScopedLogger("[networkd]", "info")

	const fileheader = "--- Managed by zeroflex. Do not remove this comment. ---"
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

	logger.Trace(">>> RunNetworkdMode() started")
	logger.Debug("RunNetworkdMode parameters: addReverse=%t, autoRestart=%t, dnsOverTLS=%t, dryRun=%t, mDNS=%t, reconcile=%t",
		addReverseDomains, autoRestart, dnsOverTLS, dryRun, multicastDNS, reconcile)

	t, err := template.New("network").Parse(networkTemplate)
	if err != nil {
		logger.Debug("Template parsing error: %v", err)
		utils.ErrorHandler("Failed to parse template", err, true)
	}

	serviceAvailable := utils.ServiceExists("systemd-networkd.service")
	if !serviceAvailable {
		logger.Debug("systemd-networkd.service is not available; changes will not trigger a service restart")
	} else {
		logger.Debug("systemd-networkd.service is available")
	}

	found := map[string]struct{}{}
	var changed bool

	logger.Verbose("Processing %d networks for networkd configuration", len(*networks.JSON200))

	for i, network := range *networks.JSON200 {
		logger.Debug("Processing network %d/%d: Interface=%s, Name=%s, ID=%s",
			i+1, len(*networks.JSON200),
			utils.GetString(network.PortDeviceName), utils.GetString(network.Name), utils.GetString(network.Id))

		fn := fmt.Sprintf("%s/99-%s.network", "/etc/systemd/network", *network.PortDeviceName)
		logger.Trace("Target file: %s", fn)

		delete(found, path.Base(fn))

		search := map[string]struct{}{}

		if network.Dns.Domain != nil {
			search[*network.Dns.Domain] = struct{}{}
			logger.Debug("Added DNS domain to search: %s, DNS servers: %v", *network.Dns.Domain, *network.Dns.Servers)
		}

		if addReverseDomains {
			logger.Trace("Calculating reverse domains for assigned addresses")
			reverseDomains := dns.CalculateReverseDomains(network.AssignedAddresses)
			for _, domain := range reverseDomains {
				search[domain] = struct{}{}
				logger.Debug("Added reverse domain to search: %s", domain)
			}
		}

		searchkeys := []string{}
		for key := range search {
			searchkeys = append(searchkeys, key)
		}
		sort.Strings(searchkeys)
		logger.Verbose("Search domains for %s: %v", utils.GetString(network.PortDeviceName), searchkeys)

		out := templateScaffold{
			ZTInterface: *network.PortDeviceName,
			ZTNetwork:   *network.Name,
			DNS:         *network.Dns.Servers,
			Domain:      strings.Join(searchkeys, " "),
			FileHeader:  fileheader,
			DNS_TLS:     dnsOverTLS,
			MDNS:        multicastDNS,
		}

		buf := bytes.NewBuffer(nil)
		if err := t.Execute(buf, out); err != nil {
			logger.Debug("Error executing template for %q: %v", fn, err)
			utils.ErrorHandler(fmt.Sprintf("Failed to execute template for %q", fn), err, true)
		}
		logger.Trace("Template executed successfully for %s", fn)

		if dryRun {
			logger.Debug("Would generate %q with DNS servers: %s and search domains: %s", fn, strings.Join(out.DNS, ", "), out.Domain)
			continue
		}

		if _, err := os.Stat(fn); err == nil {
			content, err := os.ReadFile(fn)
			if err != nil {
				logger.Debug("Error reading file %s: %v", fn, err)
				utils.ErrorHandler(fmt.Sprintf("Failed to read file %q", fn), err, true)
			}

			if bytes.Equal(content, buf.Bytes()) {
				logger.Info("No changes needed for file %s; already up-to-date", fn)
				continue
			}
			logger.Debug("File %s needs updating", fn)
		} else {
			logger.Debug("File %s does not exist, will create", fn)
		}

		logger.Debug("Creating or overwriting file %s", fn)
		f, err := os.Create(fn)
		if err != nil {
			logger.Debug("Error creating file %s: %v", fn, err)
			utils.ErrorHandler(fmt.Sprintf("Failed to create file %q", fn), err, true)
		}
		logger.Debug("Successfully created file %s", fn)

		if _, err := f.Write(buf.Bytes()); err != nil {
			logger.Debug("Error writing to file %s: %v", fn, err)
			utils.ErrorHandler("Failed to write to file", err, true)
		}
		logger.Debug("Successfully wrote to file %s", fn)

		f.Close()
		logger.Debug("Closed file %s", fn)

		changed = true

		if changed {
			logger.Info("Processed Interface=%s, Network=%s, ID=%s, DNS Search Domain=%s, DNS Servers=%v, wrote to /etc/systemd/network/99-%s.network",
				utils.GetString(network.PortDeviceName), utils.GetString(network.Name), utils.GetString(network.Id),
				utils.GetString(network.Dns.Domain), *network.Dns.Servers, utils.GetString(network.PortDeviceName))
		}
	}

	if len(found) > 0 && reconcile {
		logger.Info("Found unused networks, reconciling...")

		for fn := range found {
			logger.Info("Removing %q", fn)

			if dryRun {
				logger.Debug("Would remove %q", fn)
				continue
			}

			if err := os.Remove(filepath.Join("/etc/systemd/network", fn)); err != nil {
				utils.ErrorHandler(fmt.Sprintf("Failed to remove file %q", fn), err, true)
			}
		}
	}

	if (changed || len(found) > 0) && autoRestart && serviceAvailable {
		logger.Info("Files changed; reloading systemd-networkd...")

		if dryRun {
			logger.Debug("Would reload systemd-networkd")
			return
		}

		if err := exec.Command("networkctl", "reload").Run(); err != nil {
			utils.ErrorHandler("Failed to reload systemd-networkd", err, true)
		}
	}

	logger.Trace("<<< RunNetworkdMode() completed")
}

var managedZTInterfaces = make(map[string]struct{})

func RunResolvedMode(networks *service.GetNetworksResponse, addReverseDomains, dryRun bool, logLevel string) {
	logger := log.NewScopedLogger("[resolved]", logLevel)

	if !utils.CommandExists("resolvectl") {
		utils.ErrorHandler("resolvectl is required for systemd-resolved but is not available on this system", nil, true)
	}
	logger.Trace("resolvectl is available for systemd-resolved commands")

	currentZT := make(map[string]struct{})
	for _, network := range *networks.JSON200 {
		if network.Dns != nil && len(*network.Dns.Servers) != 0 {
			interfaceName := *network.PortDeviceName
			currentZT[interfaceName] = struct{}{}
		}
	}

	// Restore DNS for interfaces we previously managed but are no longer present
	for iface := range managedZTInterfaces {
		if _, stillPresent := currentZT[iface]; !stillPresent {
			logger.Info("Interface %s no longer present in ZeroTier networks, restoring original DNS", iface)
			dns.RestoreSavedDNS(iface, logLevel)
			delete(managedZTInterfaces, iface)
		}
	}

	for _, network := range *networks.JSON200 {
		logger.Verbose("Processing network: Interface=%s, Name=%s, ID=%s", utils.GetString(network.PortDeviceName), utils.GetString(network.Name), utils.GetString(network.Id))

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

			if addReverseDomains {
				reverseDomains := dns.CalculateReverseDomains(network.AssignedAddresses)
				for _, domain := range reverseDomains {
					searchDomains[domain] = struct{}{}
				}
			}

			searchKeys := []string{}
			for key := range searchDomains {
				searchKeys = append(searchKeys, key)
			}
			sort.Strings(searchKeys)

			// Save original DNS before first change
			dns.SaveCurrentDNSIfNeeded(interfaceName, logLevel)
			managedZTInterfaces[interfaceName] = struct{}{}
			dns.ConfigureDNSAndSearchDomains(interfaceName, dnsServers, searchKeys, dryRun, logLevel)
		}
	}
}