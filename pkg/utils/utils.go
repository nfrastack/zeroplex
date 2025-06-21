// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func Contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func GetString(ptr *string) string {
	if ptr == nil {
		return "<nil>"
	}
	return *ptr
}

func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func ExecuteCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %s %v\nOutput: %s", name, args, string(output))
	}
	return string(output), nil
}

func ServiceExists(serviceName string) bool {
	cmd := exec.Command("systemctl", "status", serviceName)
	return cmd.Run() == nil
}

func ParseResolvectlOutput(output string, prefix string) []string {
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

func FormatSliceDebug(slice []string, isExcludeFilter bool) string {
	if len(slice) == 0 {
		if isExcludeFilter {
			return "none (no filter applied)"
		} else {
			return "any (no filter applied)"
		}
	}
	return strings.Join(slice, ", ")
}

func ErrorHandler(context string, err error, exit bool) {
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", context, err)
		} else {
			if context != "" {
				fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", context, err)
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
		}
	} else if context != "" {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", context)
	}

	if exit {
		os.Exit(1)
	}
}