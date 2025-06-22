// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"zeroflex/pkg/app"
)

// Version information
var (
	Version   = "development"
	BuildTime = "unknown"
)

func main() {
	app.Version = Version
	app.New().Run()
}
