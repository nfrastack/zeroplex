// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package filters

import (
	"fmt"
	"strings"

	"github.com/zerotier/go-zerotier-one/service"
	"zt-dns-companion/pkg/config"
	"zt-dns-companion/pkg/logger"
)

// FilterRule represents a filter rule with conditions
type FilterRule struct {
	Type       string            `yaml:"type"`       // "name", "online", "assigned", "address", "interface", "route"
	Operation  string            `yaml:"operation"`  // "AND" or "OR" - how this rule combines with others
	Negate     bool              `yaml:"negate"`     // whether to negate the result
	Conditions []FilterCondition `yaml:"conditions"` // list of conditions
}

// AdvancedFilterEngine processes filters
type AdvancedFilterEngine struct {
	rules []FilterRule
}

// NewAdvancedFilterEngine creates a new filter engine from config
func NewAdvancedFilterEngine(profile config.Profile) (*AdvancedFilterEngine, error) {
	if !profile.HasAdvancedFilters() {
		return nil, fmt.Errorf("no advanced filters configured")
	}

	var rules []FilterRule
	for _, filterMap := range profile.Filters {
		rule, err := parseFilterRule(filterMap)
		if err != nil {
			return nil, fmt.Errorf("failed to parse filter rule: %w", err)
		}
		rules = append(rules, rule)
	}

	return &AdvancedFilterEngine{rules: rules}, nil
}

// parseFilterRule converts a map to a FilterRule
func parseFilterRule(filterMap map[string]interface{}) (FilterRule, error) {
	rule := FilterRule{}

	// Extract type
	if t, ok := filterMap["type"].(string); ok {
		rule.Type = t
	} else {
		return rule, fmt.Errorf("missing or invalid 'type' field")
	}

	// Extract operation (default to AND)
	if op, ok := filterMap["operation"].(string); ok {
		rule.Operation = strings.ToUpper(op)
	} else {
		rule.Operation = "AND"
	}

	// Extract negate (default to false)
	if negate, ok := filterMap["negate"].(bool); ok {
		rule.Negate = negate
	}

	// Extract conditions
	if conditionsRaw, ok := filterMap["conditions"]; ok {
		switch conditionsSlice := conditionsRaw.(type) {
		case []interface{}:
			for _, condRaw := range conditionsSlice {
				if condMap, ok := condRaw.(map[string]interface{}); ok {
					condition := FilterCondition{}
					if value, ok := condMap["value"].(string); ok {
						condition.Value = value
					}
					if logic, ok := condMap["logic"].(string); ok {
						condition.Logic = strings.ToLower(logic)
					} else {
						condition.Logic = "and" // default
					}
					rule.Conditions = append(rule.Conditions, condition)
				}
			}
		default:
			return rule, fmt.Errorf("invalid conditions format")
		}
	} else {
		return rule, fmt.Errorf("missing 'conditions' field")
	}

	return rule, nil
}

// Apply applies all filter rules to the networks
func (afe *AdvancedFilterEngine) Apply(networks *service.GetNetworksResponse) error {
	logger.Debugf("Applying %d advanced filter rules", len(afe.rules))

	originalCount := len(*networks.JSON200)

	// Create a slice to track which networks pass all filters
	var filteredNetworks []service.Network

	for _, network := range *networks.JSON200 {
		if afe.evaluateNetwork(network) {
			filteredNetworks = append(filteredNetworks, network)
		}
	}

	// Replace the original networks with filtered ones
	*networks.JSON200 = filteredNetworks

	logger.Debugf("Advanced filters: %d -> %d networks", originalCount, len(filteredNetworks))
	return nil
}

// evaluateNetwork evaluates all filter rules against a single network
func (afe *AdvancedFilterEngine) evaluateNetwork(network service.Network) bool {
	if len(afe.rules) == 0 {
		return true // No filters = include all
	}

	// Start with true for AND operations, false for OR operations
	result := true

	for i, rule := range afe.rules {
		ruleResult := afe.evaluateRule(rule, network)

		if i == 0 {
			// First rule sets the initial result
			result = ruleResult
		} else {
			// Combine with previous results based on operation
			switch rule.Operation {
			case "AND":
				result = result && ruleResult
			case "OR":
				result = result || ruleResult
			default:
				logger.Debugf("Unknown operation %s, defaulting to AND", rule.Operation)
				result = result && ruleResult
			}
		}
	}

	return result
}

// evaluateRule evaluates a single filter rule against a network
func (afe *AdvancedFilterEngine) evaluateRule(rule FilterRule, network service.Network) bool {
	ruleResult := false

	// Evaluate all conditions within this rule
	for i, condition := range rule.Conditions {
		conditionResult := afe.evaluateCondition(rule.Type, condition, network)

		if i == 0 {
			ruleResult = conditionResult
		} else {
			// Combine conditions based on their logic
			switch condition.Logic {
			case "or":
				ruleResult = ruleResult || conditionResult
			case "and":
				ruleResult = ruleResult && conditionResult
			default:
				ruleResult = ruleResult && conditionResult // default to AND
			}
		}
	}

	// Apply negation if specified
	if rule.Negate {
		ruleResult = !ruleResult
	}

	logger.Debugf("Rule %s: %t (negate: %t)", rule.Type, ruleResult, rule.Negate)
	return ruleResult
}

// evaluateCondition evaluates a single condition against a network
func (afe *AdvancedFilterEngine) evaluateCondition(filterType string, condition FilterCondition, network service.Network) bool {
	switch filterType {
	case "name":
		return afe.matchesPattern(getNetworkName(network), condition.Value)

	case "online":
		online := getNetworkOnlineStatus(network)
		return strings.ToLower(condition.Value) == strings.ToLower(fmt.Sprintf("%t", online))

	case "assigned":
		assigned := getNetworkAssignedStatus(network)
		return strings.ToLower(condition.Value) == strings.ToLower(fmt.Sprintf("%t", assigned))

	case "address":
		addresses := getNetworkAddresses(network)
		for _, addr := range addresses {
			if afe.matchesPattern(addr, condition.Value) {
				return true
			}
		}
		return false

	case "interface":
		interfaceName := getNetworkInterface(network)
		return afe.matchesPattern(interfaceName, condition.Value)

	case "route":
		routes := getNetworkRoutes(network)
		for _, route := range routes {
			if afe.matchesPattern(route, condition.Value) {
				return true
			}
		}
		return false

	default:
		logger.Debugf("Unknown filter type: %s", filterType)
		return false
	}
}

// matchesPattern checks if a value matches a pattern (supports wildcards)
func (afe *AdvancedFilterEngine) matchesPattern(value, pattern string) bool {
	// Simple wildcard matching (* and ?)
	if pattern == "*" {
		return true
	}

	// Convert shell-style wildcards to Go regexp
	// This is a simplified implementation
	if strings.Contains(pattern, "*") {
		// For now, just do prefix/suffix matching
		if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
			// *text* -> contains
			substring := strings.Trim(pattern, "*")
			return strings.Contains(value, substring)
		} else if strings.HasPrefix(pattern, "*") {
			// *text -> ends with
			suffix := strings.TrimPrefix(pattern, "*")
			return strings.HasSuffix(value, suffix)
		} else if strings.HasSuffix(pattern, "*") {
			// text* -> starts with
			prefix := strings.TrimSuffix(pattern, "*")
			return strings.HasPrefix(value, prefix)
		}
	}

	// Exact match
	return value == pattern
}

// Helper functions to extract network properties
func getNetworkName(network service.Network) string {
	if network.Name != nil {
		return *network.Name
	}
	return ""
}

func getNetworkOnlineStatus(network service.Network) bool {
	if network.Status != nil {
		return *network.Status == "OK"
	}
	return false
}

func getNetworkAssignedStatus(network service.Network) bool {
	if network.AssignedAddresses != nil {
		return len(*network.AssignedAddresses) > 0
	}
	return false
}

func getNetworkAddresses(network service.Network) []string {
	var addresses []string
	if network.AssignedAddresses != nil {
		for _, addr := range *network.AssignedAddresses {
			addresses = append(addresses, addr)
		}
	}
	return addresses
}

func getNetworkInterface(network service.Network) string {
	if network.PortDeviceName != nil {
		return *network.PortDeviceName
	}
	return ""
}

func getNetworkRoutes(network service.Network) []string {
	var routes []string
	if network.Routes != nil {
		for _, route := range *network.Routes {
			if route.Target != nil {
				routes = append(routes, *route.Target)
			}
		}
	}
	return routes
}