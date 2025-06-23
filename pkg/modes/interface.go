// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package modes

import (
	"context"
)

// ModeRunner defines the interface for different operation modes
type ModeRunner interface {
	Run(ctx context.Context) error
	GetMode() string
}
