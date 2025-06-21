// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
// SPDX-FileCopyrightText: © 2021 Zerotier Inc.
// SPDX-FileCopyright: BSD-3-Clause
//
// This file includes code derived from Zerotier Systemd Manager.
// The original code is licensed under the BSD-3-Clause license.
// See the LICENSE file or https://opensource.org/licenses/BSD-3-Clause for details.

package main

import (
	"zt-dns-companion/pkg/app"

	"fmt"
	"log"
	"os"
)

// Version information
var (
	// Version is the current version of the application
	Version = "development"

	// BuildTime is when the application was built
	BuildTime = "unknown"
)

// versionString returns a string representation of the version information
func versionString(showBuild bool) string {
	if showBuild {
		return fmt.Sprintf("%s (built: %s)", Version, BuildTime)
	}
	return Version
}

// IsRunningUnderSystemd detects if the application is running under systemd
func IsRunningUnderSystemd() (system, user bool) {
	invocation := os.Getenv("INVOCATION_ID") != ""
	journal := os.Getenv("JOURNAL_STREAM") != ""
	if invocation || journal {
		return true, false
	}
	return false, false
}

// showBanner displays the application banner if not running under systemd
func showBanner() {
		fmt.Println()
		fmt.Println("             .o88o.                                 .                       oooo")
		fmt.Println("             888 \"\"                                .o8                       888")
		fmt.Println("ooo. .oo.   o888oo  oooo d8b  .oooo.    .oooo.o .o888oo  .oooo.    .ooooo.   888  oooo")
		fmt.Println("`888P\"Y88b   888    `888\"\"8P `P  )88b  d88(  \"8   888   `P  )88b  d88' \"Y8  888 .8P'")
		fmt.Println(" 888   888   888     888      .oP\"888  \"\"Y88b.    888    .oP\"888  888        888888.")
		fmt.Println(" 888   888   888     888     d8(  888  o.  )88b   888 . d8(  888  888   .o8  888 `88b.")
		fmt.Println("o888o o888o o888o   d888b    `Y888\"\"8o 8\"\"888P'   \"888\" `Y888\"\"8o `Y8bod8P' o888o o888o")
		fmt.Println()
}

func main() {
	application := app.New()

	if err := application.Run(); err != nil {
		if err.Error() == "help requested" {
			os.Exit(0)
		}
		if err.Error() == "version requested" {
			fmt.Println(versionString(true))
			os.Exit(0)
		}

		log.Fatal(err)
	}
}


