// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseInterval parses interval strings like "60", "1m", "5h", "2d", etc.
// Supports:
// - Plain numbers (default to seconds): "60" -> 60s
// - Time units: s, m, h, d (seconds, minutes, hours, days)
// - Special values: "0", "false", "disabled" -> 0 (disabled)
// Returns 0 for disabled, or the parsed duration
func ParseInterval(intervalStr string) (time.Duration, error) {
	if intervalStr == "" {
		return 0, fmt.Errorf("empty interval string")
	}

	// Handle special disabled values
	switch strings.ToLower(intervalStr) {
	case "0", "false", "disabled", "off":
		return 0, nil
	}

	// Try parsing as Go duration first (supports "1m", "30s", "1h", etc.)
	if duration, err := time.ParseDuration(intervalStr); err == nil {
		return duration, nil
	}

	// Try parsing as plain number (assume seconds)
	if num, err := strconv.ParseInt(intervalStr, 10, 64); err == nil {
		if num <= 0 {
			return 0, nil
		}
		return time.Duration(num) * time.Second, nil
	}

	// Try parsing number with unit suffix that Go doesn't support (like 'd' for days)
	if len(intervalStr) > 1 {
		unit := strings.ToLower(intervalStr[len(intervalStr)-1:])
		numStr := intervalStr[:len(intervalStr)-1]

		if num, err := strconv.ParseFloat(numStr, 64); err == nil && num > 0 {
			switch unit {
			case "d":
				return time.Duration(num * 24 * float64(time.Hour)), nil
			case "w":
				return time.Duration(num * 7 * 24 * float64(time.Hour)), nil
			}
		}
	}

	return 0, fmt.Errorf("invalid interval format: %s (supported: 60, 1m, 1h, 1d, disabled)", intervalStr)
}

// FormatInterval formats a duration in a human-readable way
func FormatInterval(d time.Duration) string {
	if d == 0 {
		return "disabled"
	}

	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}