// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package utils

import "os"

// IsRunningUnderSystemd detects if the application is running under systemd
func IsRunningUnderSystemd() bool {
	invocation := os.Getenv("INVOCATION_ID") != ""
	journal := os.Getenv("JOURNAL_STREAM") != ""
	return invocation || journal
}

// GetVersion returns the application version from environment or default
func GetVersion() string {
	version := os.Getenv("ZT_DNS_VERSION")
	if version == "" {
		version = "development"
	}
	return version
}