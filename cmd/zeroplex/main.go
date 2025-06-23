// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"zeroplex/pkg/app"
	"zeroplex/pkg/cli"
)

// Version information
var (
	Version   = "development"
	BuildTime = "unknown"
)

func main() {
	// Parse flags ONCE at program start
	cli.ParseFlags()
	app.Version = Version
	app.New().Run()
}
