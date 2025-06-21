// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package logger

import (
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	currentLogLevel = "info"
	logTimestamps   bool
)

func init() {
	// By default, remove timestamp from log output to match our custom logging behavior
	log.SetFlags(0)
}

func SetLogLevel(level string) {
	if strings.ToLower(level) == "debug" {
		currentLogLevel = "debug"
	} else {
		currentLogLevel = "info"
	}

	// Set log package flags based on the logTimestamps setting
	if logTimestamps {
		log.SetFlags(log.LstdFlags)
	} else {
		log.SetFlags(0)
	}
}

// SetTimestamps enables or disables timestamps in log output
func SetTimestamps(enabled bool) {
	logTimestamps = enabled
	// Update the Go log package to match our setting
	if enabled {
		log.SetFlags(log.LstdFlags)
	} else {
		log.SetFlags(0)
	}
}

func logWithLevel(level string, format string, args ...interface{}) {
	if (level == "info" && (currentLogLevel == "info" || currentLogLevel == "debug")) ||
		(level == "debug" && currentLogLevel == "debug") ||
		(level == "dryrun") {
		if logTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			fmt.Printf("%s %s: "+format+"\n", append([]interface{}{timestamp, strings.ToUpper(level)}, args...)...)
		} else {
			fmt.Printf("%s: "+format+"\n", append([]interface{}{strings.ToUpper(level)}, args...)...)
		}
	}
}

func Infof(format string, args ...interface{}) {
	logWithLevel("info", format, args...)
}

func Debugf(format string, args ...interface{}) {
	logWithLevel("debug", format, args...)
}

func DryRunf(format string, args ...interface{}) {
	logWithLevel("dryrun", format, args...)
}

func Errorf(format string, args ...interface{}) {
	log.Printf("ERROR: "+format, args...)
}