// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package dns

import (
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"

	"fmt"
	"math"
	"net"
	"os"
	"strings"
)

func CalculateReverseDomains(assignedAddresses *[]string) []string {
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

func CompareDNS(current, desired []string) bool {
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

func ConfigureDNSAndSearchDomains(interfaceName string, dnsServers, searchKeys []string, dryRun bool) {
	logger.Trace("ConfigureDNSAndSearchDomains() started for interface: %s", interfaceName)
	logger.Debug("Configuring DNS for interface: %s", interfaceName)
	logger.Verbose("DNS servers to configure: %v", dnsServers)
	logger.Verbose("Search domains to configure: %v", searchKeys)

	if dryRun {
		logger.DryRunWithPrefix("dns", "Would set Interface: %s Search Domain: %s and DNS: %s", interfaceName, strings.Join(searchKeys, ", "), strings.Join(dnsServers, ", "))
		return
	}

	logger.Trace("Querying current DNS configuration via resolvectl")
	logger.Trace("Executing command: resolvectl dns %s", interfaceName)
	output, err := utils.ExecuteCommand("resolvectl", "dns", interfaceName)
	if err != nil {
		logger.DebugWithPrefix("dns", "Failed to query DNS via resolvectl for interface %s: %v", interfaceName, err)
		logger.DebugWithPrefix("dns", "Command output: %s", output)
		fmt.Fprintf(os.Stderr, "Could not query DNS for interface %s. Please ensure the interface exists and resolvectl is configured correctly.\n", interfaceName)
		return
	}
	logger.Debug("Command succeeded: resolvectl dns %s", interfaceName)
	logger.Verbose("Command output length: %d characters", len(output))
	currentDNS := utils.ParseResolvectlOutput(output, "Link ")
	logger.DebugWithPrefix("dns", "Parsed current DNS for interface %s: %v", interfaceName, currentDNS)

	logger.Trace("Querying current search domains via resolvectl")
	logger.Trace("Executing command: resolvectl domain %s", interfaceName)
	output, err = utils.ExecuteCommand("resolvectl", "domain", interfaceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query search domains via resolvectl for interface %s: %v\n", interfaceName, err)
		return
	}
	logger.Debug("Command succeeded: resolvectl domain %s", interfaceName)
	logger.Verbose("Command output length: %d characters", len(output))
	currentDomains := utils.ParseResolvectlOutput(output, "Link ")
	logger.DebugWithPrefix("dns", "Parsed current search domains for interface %s: %v", interfaceName, currentDomains)

	logger.DebugWithPrefix("dns", "Desired DNS for interface %s: %v", interfaceName, dnsServers)
	logger.DebugWithPrefix("dns", "Desired search domains for interface %s: %v", interfaceName, searchKeys)

	sameDNS := CompareDNS(currentDNS, dnsServers)
	sameDomains := CompareDNS(currentDomains, searchKeys)

	logger.DebugWithPrefix("dns", "Comparison result for interface %s: sameDNS=%v, sameDomains=%v", interfaceName, sameDNS, sameDomains)

	if sameDNS && sameDomains {
		logger.InfoWithPrefix("dns", "No changes needed for interface %s; DNS and search domains are already up-to-date", interfaceName)
		return
	}

	logger.Verbose("DNS configuration changes needed for interface %s", interfaceName)
	// Configure DNS and domains using resolvectl
	configureViaDbus(interfaceName, dnsServers, searchKeys)
}

func configureViaDbus(interfaceName string, dnsServers, searchKeys []string) {
	// Import dbus here to keep it contained to this function
	conn, err := net.Dial("unix", "/run/systemd/resolve/io.systemd.Resolve")
	if err != nil {
		// Fallback to using resolvectl commands
		configureViaResolvectl(interfaceName, dnsServers, searchKeys)
		return
	}
	defer conn.Close()

	// For now, use resolvectl as fallback until we implement full D-Bus
	configureViaResolvectl(interfaceName, dnsServers, searchKeys)
}

func configureViaResolvectl(interfaceName string, dnsServers, searchKeys []string) {
	// Set DNS servers
	if len(dnsServers) > 0 {
		args := append([]string{"dns", interfaceName}, dnsServers...)
		_, err := utils.ExecuteCommand("resolvectl", args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set DNS servers for %s: %v\n", interfaceName, err)
			return
		}
	}

	// Set search domains
	if len(searchKeys) > 0 {
		args := append([]string{"domain", interfaceName}, searchKeys...)
		_, err := utils.ExecuteCommand("resolvectl", args...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set search domains for %s: %v\n", interfaceName, err)
			return
		}
	}

	if len(searchKeys) > 0 {
		logger.Infof("Configured for Interface: %s DNS: %s Search Domain: %s", interfaceName, strings.Join(dnsServers, ", "), strings.Join(searchKeys, ", "))
	} else {
		logger.Infof("Configured for Interface: %s DNS: %s", interfaceName, strings.Join(dnsServers, ", "))
	}
}