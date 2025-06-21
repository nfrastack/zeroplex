// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package logging

import (
	"zt-dns-companion/pkg/logger"
)

// Common scoped loggers for consistent logging throughout the application
var (
	// Core components
	RunnerLogger = logger.NewScopedLogger("runner", "")
	DaemonLogger = logger.NewScopedLogger("daemon", "")
	ConfigLogger = logger.NewScopedLogger("config", "")
	
	// API and networking
	APILogger     = logger.NewScopedLogger("api", "")
	ClientLogger  = logger.NewScopedLogger("client", "")
	NetworkLogger = logger.NewScopedLogger("network", "")
	
	// DNS operations
	DNSLogger         = logger.NewScopedLogger("dns", "")
	ResolvedLogger    = logger.NewScopedLogger("dns/resolved", "")
	NetworkdLogger    = logger.NewScopedLogger("dns/networkd", "")
	
	// Filtering and processing
	FilterLogger  = logger.NewScopedLogger("filters", "")
	ServiceLogger = logger.NewScopedLogger("service", "")
	
	// System operations
	SystemLogger  = logger.NewScopedLogger("system", "")
	CommandLogger = logger.NewScopedLogger("system/cmd", "")
)

// GetModeLogger returns a scoped logger for a specific mode
func GetModeLogger(mode string) *logger.ScopedLogger {
	return logger.NewScopedLogger("modes/"+mode, "")
}

// GetSubLogger returns a scoped logger with a sub-component
func GetSubLogger(component, subComponent string) *logger.ScopedLogger {
	scope := component
	if subComponent != "" {
		scope = component + "/" + subComponent
	}
	return logger.NewScopedLogger(scope, "")
}