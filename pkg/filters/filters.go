// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package filters

import (
	"zeroflex/pkg/config"
	"zeroflex/pkg/log"

	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zerotier/go-zerotier-one/service"
)

// FilterType represents the type of filter to apply
type FilterType string

// Filter operation types
const (
	FilterOperationAND = "AND"
	FilterOperationOR  = "OR"
	FilterOperationNOT = "NOT"
)

// ZeroTier-specific filter types
const (
	FilterTypeNone       FilterType = "none"
	FilterTypeName       FilterType = "name"
	FilterTypeInterface  FilterType = "interface"
	FilterTypeNetwork    FilterType = "network"
	FilterTypeNetworkID  FilterType = "network_id"
	FilterTypeOnline     FilterType = "online"
	FilterTypeAssigned   FilterType = "assigned"
	FilterTypeAddress    FilterType = "address"
	FilterTypeRoute      FilterType = "route"
)

// Filter defines a filter for ZeroTier networks
type Filter struct {
	Type       FilterType        `yaml:"type" mapstructure:"type"`
	Value      string            `yaml:"value,omitempty" mapstructure:"value,omitempty"` // For simple filters
	Operation  string            `yaml:"operation,omitempty" mapstructure:"operation,omitempty"` // AND, OR, NOT (defaults to AND)
	Negate     bool              `yaml:"negate,omitempty" mapstructure:"negate,omitempty"` // Invert the filter result
	Conditions []FilterCondition `yaml:"conditions,omitempty" mapstructure:"conditions,omitempty"` // Filter conditions
}

// FilterCondition represents individual filter criteria
type FilterCondition struct {
	Value string `yaml:"value" mapstructure:"value"`
	Logic string `yaml:"logic,omitempty" mapstructure:"logic,omitempty"` // and, or (defaults to and)
}

// FilterConfig contains multiple filters
type FilterConfig struct {
	Filters []Filter `yaml:"filters,omitempty" mapstructure:"filters,omitempty"`
}

// DefaultFilterConfig returns a default filter configuration
func DefaultFilterConfig() FilterConfig {
	return FilterConfig{
		Filters: []Filter{{Type: FilterTypeNone, Value: ""}},
	}
}

// ApplyFilters applies filtering
func ApplyFilters(networks *service.GetNetworksResponse, profile config.Profile) {
	logger := log.NewLogger("[filters]", profile.LogLevel)
	logger.Trace("ApplyFilters() started")

	if !profile.HasAdvancedFilters() {
		logger.Debug("No filters configured - processing all networks")
		return
	}

	logger.Debug("Applying filtering")
	logger.Verbose("Profile has %d filter configurations", len(profile.Filters))

	filterOptions, err := profile.GetAdvancedFilterConfig()
	if err != nil {
		logger := log.NewScopedLogger("[filters]", "error")
		logger.Error("Failed to get advanced filter config: %v", err)
		return
	}

	logger.Trace("Converting filter options to FilterConfig")
	filterConfig, err := NewFilterFromStructuredOptions(filterOptions)
	if err != nil {
		logger := log.NewScopedLogger("[filters]", "error")
		logger.Error("Failed to parse advanced filters: %v", err)
		return
	}

	logger.Debug("Parsed %d filters from configuration", len(filterConfig.Filters))
	ApplyAdvancedFilters(networks, filterConfig)
}

// ApplyAdvancedFilters applies filtering with multiple filters and AND/OR operations
func ApplyAdvancedFilters(networks *service.GetNetworksResponse, filterConfig FilterConfig) {
	logger := log.NewScopedLogger("[filters]", "debug")

	if len(filterConfig.Filters) == 0 || (len(filterConfig.Filters) == 1 && filterConfig.Filters[0].Type == FilterTypeNone) {
		logger.Debug("No filtering applied - no filters configured")
		return
	}

	logger.Debug("Applying filtering with %d filters", len(filterConfig.Filters))

	filteredNetworks := []service.Network{}
	for _, network := range *networks.JSON200 {
		// Use evaluation system
		if filterConfig.Evaluate(network, evaluateZTFilter) {
			filteredNetworks = append(filteredNetworks, network)
			logger.Debug("Network %s passed filtering", getNetworkDisplayName(network))
		} else {
			logger.Debug("Network %s filtered out by filtering", getNetworkDisplayName(network))
		}
	}

	logger.Debug("filtering: %d of %d networks passed", len(filteredNetworks), len(*networks.JSON200))
	*networks.JSON200 = filteredNetworks
}

// getNetworkDisplayName returns a display name for a network (for logging)
func getNetworkDisplayName(network service.Network) string {
	if network.Name != nil && *network.Name != "" {
		return *network.Name
	}
	if network.Id != nil {
		return *network.Id
	}
	return "unknown"
}

// Evaluate evaluates all filters against a network using  logic
func (fc FilterConfig) Evaluate(network service.Network, evaluator func(Filter, service.Network) bool) bool {
	if len(fc.Filters) == 0 {
		return true // No filters = include all
	}

	// Start with the first filter, then combine with others
	result := evaluator(fc.Filters[0], network)

	for i := 1; i < len(fc.Filters); i++ {
		filter := fc.Filters[i]
		filterResult := evaluator(filter, network)

		// Apply negate if specified
		if filter.Negate {
			filterResult = !filterResult
		}

		// Combine with previous result based on operation
		switch strings.ToUpper(filter.Operation) {
		case FilterOperationOR:
			result = result || filterResult
		case FilterOperationNOT:
			result = result && !filterResult
		case FilterOperationAND, "":
			fallthrough
		default:
			result = result && filterResult
		}
	}

	return result
}

// evaluateZTFilter evaluates a single filter against a ZeroTier network
func evaluateZTFilter(filter Filter, network service.Network) bool {
	logger := log.NewScopedLogger("[filters]", "debug")

	switch filter.Type {
	case FilterTypeNone:
		return true

	case FilterTypeName:
		if network.Name == nil {
			return false
		}
		return matchesPattern(*network.Name, filter.Value, filter.Conditions)

	case FilterTypeInterface:
		if network.PortDeviceName == nil {
			return false
		}
		return matchesPattern(*network.PortDeviceName, filter.Value, filter.Conditions)

	case FilterTypeNetwork:
		if network.Name == nil {
			return false
		}
		return matchesPattern(*network.Name, filter.Value, filter.Conditions)

	case FilterTypeNetworkID:
		if network.Id == nil {
			return false
		}
		return matchesPattern(*network.Id, filter.Value, filter.Conditions)

	case FilterTypeOnline:
		online := network.Status != nil && *network.Status == "OK"
		return strings.ToLower(filter.Value) == strings.ToLower(fmt.Sprintf("%t", online))

	case FilterTypeAssigned:
		assigned := network.AssignedAddresses != nil && len(*network.AssignedAddresses) > 0
		return strings.ToLower(filter.Value) == strings.ToLower(fmt.Sprintf("%t", assigned))

	case FilterTypeAddress:
		if network.AssignedAddresses == nil {
			return false
		}
		for _, addr := range *network.AssignedAddresses {
			if matchesPattern(addr, filter.Value, filter.Conditions) {
				return true
			}
		}
		return false

	case FilterTypeRoute:
		if network.Routes == nil {
			return false
		}
		for _, route := range *network.Routes {
			if route.Target != nil && matchesPattern(*route.Target, filter.Value, filter.Conditions) {
				return true
			}
		}
		return false

	default:
		logger.Debug("Unknown filter type: %s", filter.Type)
		return false
	}
}

// matchesPattern checks if a value matches a pattern or conditions
func matchesPattern(value, pattern string, conditions []FilterCondition) bool {
	// If we have conditions, use them instead of the simple pattern
	if len(conditions) > 0 {
		return evaluateConditions(value, conditions)
	}

	// Simple pattern matching (wildcards)
	if pattern == "*" || pattern == "" {
		return true
	}

	// Use glob-style pattern matching
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		// Fallback to simple string matching if pattern is invalid
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	}

	return matched
}

// evaluateConditions evaluates multiple conditions against a value
func evaluateConditions(value string, conditions []FilterCondition) bool {
	if len(conditions) == 0 {
		return true
	}

	result := false
	for i, condition := range conditions {
		conditionResult := matchesSingleCondition(value, condition.Value)

		if i == 0 {
			result = conditionResult
		} else {
			switch strings.ToLower(condition.Logic) {
			case "or":
				result = result || conditionResult
			case "and", "":
				fallthrough
			default:
				result = result && conditionResult
			}
		}
	}

	return result
}

// matchesSingleCondition checks if a value matches a single condition
func matchesSingleCondition(value, pattern string) bool {
	logger := log.NewScopedLogger("[filters]", "debug")

	// Support regex patterns if they start with ^
	if strings.HasPrefix(pattern, "^") {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			logger.Debug("Invalid regex pattern %s: %v", pattern, err)
			return false
		}
		return regex.MatchString(value)
	}

	// Glob-style pattern matching
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		// Fallback to substring matching
		return strings.Contains(strings.ToLower(value), strings.ToLower(pattern))
	}

	return matched
}

// LoadAdvancedFiltersFromYAML loads Filters from YAML configuration
func LoadAdvancedFiltersFromYAML(data []byte) (FilterConfig, error) {
	logger := log.NewScopedLogger("[filters]", "error")

	var config FilterConfig

	// Try to unmarshal as FilterConfig first
	if err := json.Unmarshal(data, &config); err == nil && len(config.Filters) > 0 {
		return config, nil
	}

	// Fallback to parsing as raw map for flexibility
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		logger.Error("Failed to parse filter configuration: %v", err)
		return FilterConfig{}, fmt.Errorf("failed to parse filter configuration: %w", err)
	}

	// Extract filters array
	if filtersRaw, ok := rawConfig["filters"]; ok {
		if filtersSlice, ok := filtersRaw.([]interface{}); ok {
			for _, filterRaw := range filtersSlice {
				if filterMap, ok := filterRaw.(map[string]interface{}); ok {
					filter, err := parseFilterFromMap(filterMap)
					if err != nil {
						logger.Error("Failed to parse filter: %v", err)
						return FilterConfig{}, fmt.Errorf("failed to parse filter: %w", err)
					}
					config.Filters = append(config.Filters, filter)
				}
			}
		}
	}

	return config, nil
}

// NewFilterFromStructuredOptions creates a FilterConfig from structured options
func NewFilterFromStructuredOptions(options map[string]interface{}) (FilterConfig, error) {
	logger := log.NewScopedLogger("[filters]", "error")

	config := FilterConfig{}

	// Extract filters array from options
	if filtersRaw, ok := options["filter"]; ok {
		if filtersSlice, ok := filtersRaw.([]map[string]interface{}); ok {
			for _, filterMap := range filtersSlice {
				filter, err := parseFilterFromMap(filterMap)
				if err != nil {
					logger.Error("Failed to parse filter: %v", err)
					return FilterConfig{}, fmt.Errorf("failed to parse filter: %w", err)
				}
				config.Filters = append(config.Filters, filter)
			}
		} else if filtersSlice, ok := filtersRaw.([]interface{}); ok {
			// Handle []interface{} case
			for _, filterRaw := range filtersSlice {
				if filterMap, ok := filterRaw.(map[string]interface{}); ok {
					filter, err := parseFilterFromMap(filterMap)
					if err != nil {
						logger.Error("Failed to parse filter: %v", err)
						return FilterConfig{}, fmt.Errorf("failed to parse filter: %w", err)
					}
					config.Filters = append(config.Filters, filter)
				}
			}
		}
	}

	if len(config.Filters) == 0 {
		// Default to no filtering
		config.Filters = []Filter{{Type: FilterTypeNone}}
	}

	return config, nil
}

// parseFilterFromMap converts a map to a Filter
func parseFilterFromMap(filterMap map[string]interface{}) (Filter, error) {
	filter := Filter{}

	// Extract type
	if t, ok := filterMap["type"].(string); ok {
		filter.Type = FilterType(t)
	} else {
		return filter, fmt.Errorf("missing or invalid 'type' field")
	}

	// Extract value (optional)
	if value, ok := filterMap["value"].(string); ok {
		filter.Value = value
	}

	// Extract operation (defaults to AND)
	if op, ok := filterMap["operation"].(string); ok {
		filter.Operation = strings.ToUpper(op)
	} else {
		filter.Operation = FilterOperationAND
	}

	// Extract negate (defaults to false)
	if negate, ok := filterMap["negate"].(bool); ok {
		filter.Negate = negate
	}

	// Extract conditions (optional)
	if conditionsRaw, ok := filterMap["conditions"]; ok {
		if conditionsSlice, ok := conditionsRaw.([]interface{}); ok {
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
					filter.Conditions = append(filter.Conditions, condition)
				}
			}
		}
	}

	return filter, nil
}