// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"zt-dns-companion/pkg/dns"
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"

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
		logger.Debugf("Template parsing error: %v", err)
		utils.ErrorHandler("Failed to parse template", err, true)
	}

	serviceAvailable := utils.ServiceExists("systemd-networkd.service")
	if !serviceAvailable {
		logger.Debugf("systemd-networkd.service is not available; changes will not trigger a service restart")
	} else {
		logger.Debugf("systemd-networkd.service is available")
	}

	found := map[string]struct{}{}
	var changed bool

	for _, network := range *networks.JSON200 {
		logger.Debugf("Processing network: Interface=%s, Name=%s, ID=%s",
			utils.GetString(network.PortDeviceName), utils.GetString(network.Name), utils.GetString(network.Id))

		fn := fmt.Sprintf("%s/99-%s.network", "/etc/systemd/network", *network.PortDeviceName)

		delete(found, path.Base(fn))

		search := map[string]struct{}{}

		if network.Dns.Domain != nil {
			search[*network.Dns.Domain] = struct{}{}
			logger.Debugf("Added DNS domain to search: %s, DNS servers: %v", *network.Dns.Domain, *network.Dns.Servers)
		}

		if addReverseDomains {
			reverseDomains := dns.CalculateReverseDomains(network.AssignedAddresses)
			for _, domain := range reverseDomains {
				search[domain] = struct{}{}
				logger.Debugf("Added reverse domain to search: %s", domain)
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
			DNS_TLS:     dnsOverTLS,
			MDNS:        multicastDNS,
		}

		buf := bytes.NewBuffer(nil)
		if err := t.Execute(buf, out); err != nil {
			logger.Debugf("Error executing template for %q: %v", fn, err)
			utils.ErrorHandler(fmt.Sprintf("Failed to execute template for %q", fn), err, true)
		}

		if dryRun {
			logger.DryRunf("Would generate %q with DNS servers: %s and search domains: %s", fn, strings.Join(out.DNS, ", "), out.Domain)
			continue
		}

		if _, err := os.Stat(fn); err == nil {
			content, err := os.ReadFile(fn)
			if err != nil {
				logger.Debugf("Error reading file %s: %v", fn, err)
				utils.ErrorHandler(fmt.Sprintf("Failed to read file %q", fn), err, true)
			}

			if bytes.Equal(content, buf.Bytes()) {
				logger.Infof("No changes needed for file %s; already up-to-date", fn)
				continue
			}
		}

		logger.Debugf("Creating or overwriting file %s", fn)
		f, err := os.Create(fn)
		if err != nil {
			logger.Debugf("Error creating file %s: %v", fn, err)
			utils.ErrorHandler(fmt.Sprintf("Failed to create file %q", fn), err, true)
		}
		logger.Debugf("Successfully created file %s", fn)

		if _, err := f.Write(buf.Bytes()); err != nil {
			logger.Debugf("Error writing to file %s: %v", fn, err)
			utils.ErrorHandler("Failed to write to file", err, true)
		}
		logger.Debugf("Successfully wrote to file %s", fn)

		f.Close()
		logger.Debugf("Closed file %s", fn)

		changed = true

		if changed {
			logger.Infof("Processed Interface=%s, Network=%s, ID=%s, DNS Search Domain=%s, DNS Servers=%v, wrote to /etc/systemd/network/99-%s.network",
				utils.GetString(network.PortDeviceName), utils.GetString(network.Name), utils.GetString(network.Id),
				utils.GetString(network.Dns.Domain), *network.Dns.Servers, utils.GetString(network.PortDeviceName))
		}
	}

	if len(found) > 0 && reconcile {
		logger.Infof("Found unused networks, reconciling...")

		for fn := range found {
			logger.Infof("Removing %q", fn)

			if dryRun {
				logger.DryRunf("Would remove %q", fn)
				continue
			}

			if err := os.Remove(filepath.Join("/etc/systemd/network", fn)); err != nil {
				utils.ErrorHandler(fmt.Sprintf("Failed to remove file %q", fn), err, true)
			}
		}
	}

	if (changed || len(found) > 0) && autoRestart && serviceAvailable {
		logger.Infof("Files changed; reloading systemd-networkd...")

		if dryRun {
			logger.DryRunf("Would reload systemd-networkd")
			return
		}

		if err := exec.Command("networkctl", "reload").Run(); err != nil {
			utils.ErrorHandler("Failed to reload systemd-networkd", err, true)
		}
	}
}

func RunResolvedMode(networks *service.GetNetworksResponse, addReverseDomains, dryRun bool) {
	if !utils.CommandExists("resolvectl") {
		utils.ErrorHandler("resolvectl is required for systemd-resolved but is not available on this system", nil, true)
	}
	logger.Debugf("resolvectl is available for systemd-resolved commands")

	for _, network := range *networks.JSON200 {
		logger.Debugf("Processing network: Interface=%s, Name=%s, ID=%s", utils.GetString(network.PortDeviceName), utils.GetString(network.Name), utils.GetString(network.Id))

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

			dns.ConfigureDNSAndSearchDomains(interfaceName, dnsServers, searchKeys, dryRun)
		}
	}
}