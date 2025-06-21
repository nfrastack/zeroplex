// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
// SPDX-FileCopyrightText: © 2021 Zerotier Inc.
// SPDX-FileCopyright: BSD-3-Clause
//
// This file includes code derived from Zerotier Systemd Manager.
// The original code is licensed under the BSD-3-Clause license.
// See the LICENSE file or https://opensource.org/licenses/BSD-3-Clause for details.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"zt-dns-companion/pkg/app"
)

// Version information
var (
	// Version is the current version of the application
	Version = "development"

	// BuildTime is when the application was built
	BuildTime = "unknown"
)

func main() {
	application := app.New()

	if err := application.Run(); err != nil {
		if err.Error() == "help requested" {
			flag.Usage()
			os.Exit(0)
		}
		log.Fatal(err)
	}
}


