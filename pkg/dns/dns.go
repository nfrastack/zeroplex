// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package dns

import (
	"zeroflex/pkg/log"
	"zeroflex/pkg/utils"

	"fmt"
	"math"
	"net"
	"os"
	"strings"
)

type SavedDNS struct {
	DNS    []string
	Search []string
}

var savedDNSState = make(map[string]SavedDNS)

// Track interfaces that have actually been changed by this tool
var changedInterfaces = make(map[string]struct{})

// MarkInterfaceChanged records that an interface's DNS was changed by this tool
func MarkInterfaceChanged(interfaceName string) {
	changedInterfaces[interfaceName] = struct{}{}
}

// GetChangedInterfaces returns a list of interfaces changed by this tool
func GetChangedInterfaces() []string {
	keys := make([]string, 0, len(changedInterfaces))
	for k := range changedInterfaces {
		keys = append(keys, k)
	}
	return keys
}

// SaveCurrentDNSIfNeeded saves the current DNS/search domains for an interface if not already saved
func SaveCurrentDNSIfNeeded(interfaceName string, logLevel string) {
	if _, exists := savedDNSState[interfaceName]; exists {
		return
	}
	logger := log.NewScopedLogger("[dns]", logLevel)
	output, err := utils.ExecuteCommand("resolvectl", "dns", interfaceName)
	if err != nil {
		logger.Warn("Could not save original DNS for %s: %v", interfaceName, err)
		return
	}
	currentDNS := utils.ParseResolvectlOutput(output, "Link ")
	output, err = utils.ExecuteCommand("resolvectl", "domain", interfaceName)
	if err != nil {
		logger.Warn("Could not save original search domains for %s: %v", interfaceName, err)
		return
	}
	currentDomains := utils.ParseResolvectlOutput(output, "Link ")
	savedDNSState[interfaceName] = SavedDNS{DNS: currentDNS, Search: currentDomains}
	logger.Debug("Saved original DNS/search domains for %s: DNS=%v, Search=%v", interfaceName, currentDNS, currentDomains)
}

// RestoreSavedDNS restores the saved DNS/search domains for an interface, if present
// Returns true if a restore was performed, false otherwise
func RestoreSavedDNS(interfaceName string, logLevel string) bool {
	saved, exists := savedDNSState[interfaceName]
	logger := log.NewScopedLogger("[dns]", logLevel)
	if !exists {
		logger.Verbose("No saved DNS state for %s, nothing to restore (interface may have disappeared)", interfaceName)
		return false
	}
	if _, changed := changedInterfaces[interfaceName]; !changed {
		logger.Verbose("Interface %s was not changed by this tool, skipping restore", interfaceName)
		return false
	}
	logger.Info("Restoring original DNS/search domains for %s: DNS=%v, Search=%v", interfaceName, saved.DNS, saved.Search)

	// Use resolvectl revert for robust cleanup
	_, err := utils.ExecuteCommand("resolvectl", "revert", interfaceName)
	if err != nil {
		if strings.Contains(err.Error(), "No such device") {
			logger.Warn("Interface %s is gone (No such device) while reverting; skipping restore.", interfaceName)
			return false
		}
		logger.Warn("Failed to revert DNS settings for %s: %v", interfaceName, err)
		return false
	}
	logger.Info("Reverted all temporary DNS settings for %s using 'resolvectl revert'", interfaceName)
	return true
}

// GetSavedDNSState returns a copy of the saved DNS state map (interface names only)
func GetSavedDNSState() map[string]SavedDNS {
	copy := make(map[string]SavedDNS)
	for k, v := range savedDNSState {
		copy[k] = v
	}
	return copy
}

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

// Accept logLevel as a parameter
func ConfigureDNSAndSearchDomains(interfaceName string, dnsServers, searchKeys []string, dryRun bool, logLevel string) {
	logger := log.NewScopedLogger("[dns]", logLevel)
	logger.Trace("ConfigureDNSAndSearchDomains() started for interface: %s", interfaceName)
	logger.Debug("Configuring DNS for interface: %s", interfaceName)

	if dryRun {
		logger.Info("Would set Interface: %s Search Domain: %s and DNS: %s", interfaceName, strings.Join(searchKeys, ", "), strings.Join(dnsServers, ", "))
		return
	}

	SaveCurrentDNSIfNeeded(interfaceName, logLevel)

	logger.Debug("Querying current DNS configuration via resolvectl")
	logger.Trace("Executing command: resolvectl dns %s", interfaceName)
	output, err := utils.ExecuteCommand("resolvectl", "dns", interfaceName)
	logger.Trace("Command: resolvectl dns %s", interfaceName)
	logger.Trace("Command output: %s", output)
	if err != nil {
		logger.Error("Failed to query DNS via resolvectl for interface %s: %v", interfaceName, err)
		logger.Trace("Command output: %s", output)
		fmt.Fprintf(os.Stderr, "Could not query DNS for interface %s. Please ensure the interface exists and resolvectl is configured correctly.\n", interfaceName)
		return
	}
	logger.Trace("Command succeeded: resolvectl dns %s", interfaceName)
	logger.Trace("Command output length: %d characters", len(output))
	currentDNS := utils.ParseResolvectlOutput(output, "Link ")
	logger.Debug("Current systemd-resolved DNS for interface %s: %v", interfaceName, currentDNS)

	logger.Debug("Querying current search domains via resolvectl")
	logger.Trace("Executing command: resolvectl domain %s", interfaceName)
	output, err = utils.ExecuteCommand("resolvectl", "domain", interfaceName)
	logger.Trace("Command: resolvectl domain %s", interfaceName)
	logger.Trace("Command output: %s", output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to query search domains via resolvectl for interface %s: %v\n", interfaceName, err)
		return
	}
	logger.Trace("Command succeeded: resolvectl domain %s", interfaceName)
	logger.Trace("Command output length: %d characters", len(output))
	currentDomains := utils.ParseResolvectlOutput(output, "Link ")
	logger.Debug("Current systemd-resolved search domains for interface %s: %v", interfaceName, currentDomains)

	logger.Debug("Desired DNS for interface %s: %v", interfaceName, dnsServers)
	logger.Debug("Desired search domains for interface %s: %v", interfaceName, searchKeys)

	// Combine all verbose DNS/search log lines into one
	logger.Verbose("DNS config for %s: DNS(current)=%v, DNS(desired)=%v, Search(current)=%v, Search(desired)=%v", interfaceName, currentDNS, dnsServers, currentDomains, searchKeys)

	sameDNS := CompareDNS(currentDNS, dnsServers)
	sameDomains := CompareDNS(currentDomains, searchKeys)

	logger.Debug("Comparison result for interface %s: sameDNS=%v, sameDomains=%v", interfaceName, sameDNS, sameDomains)

	if sameDNS && sameDomains {
		logger.Verbose("No changes needed for interface %s; DNS and search domains are already up-to-date", interfaceName)
		return
	}

	logger.Info("DNS configuration changes needed for interface %s", interfaceName)
	// Configure DNS and domains using resolvectl
	configureViaDbus(interfaceName, dnsServers, searchKeys)
	// Mark as changed only if we actually updated
	MarkInterfaceChanged(interfaceName)
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
		log.NewScopedLogger("[dns]", "info").Info("Configured for Interface: %s DNS: %s Search Domain: %s", interfaceName, strings.Join(dnsServers, ", "), strings.Join(searchKeys, ", "))
	} else {
		log.NewScopedLogger("[dns]", "info").Info("Configured for Interface: %s DNS: %s", interfaceName, strings.Join(dnsServers, ", "))
	}
}
