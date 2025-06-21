// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package daemon

import (
	"context"
	"sync"
	"time"

	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"
)

// TaskFunc represents a function that the daemon will execute periodically
type TaskFunc func(ctx context.Context) error

// Daemon represents a background service that runs tasks at specified intervals
type Daemon struct {
	interval    time.Duration
	task        TaskFunc
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
	mu          sync.RWMutex
	ticker      *time.Ticker
	logPrefix   string
}

// New creates a new daemon instance
func New(intervalStr string, task TaskFunc) (*Daemon, error) {
	interval, err := utils.ParseInterval(intervalStr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		interval:  interval,
		task:      task,
		ctx:       ctx,
		cancel:    cancel,
		logPrefix: "[daemon]",
	}, nil
}

// SetLogPrefix sets a custom log prefix for this daemon
func (d *Daemon) SetLogPrefix(prefix string) {
	d.logPrefix = prefix
}

// Start begins the daemon's execution loop
func (d *Daemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return nil // Already running
	}

	if d.interval == 0 {
		logger.Infof("%s Daemon disabled (interval: 0)", d.logPrefix)
		return nil
	}

	d.running = true
	logger.Infof("%s Starting daemon with interval: %s", d.logPrefix, utils.FormatInterval(d.interval))

	// Run initial task immediately
	go func() {
		logger.Debugf("%s Running initial task", d.logPrefix)
		if err := d.task(d.ctx); err != nil {
			logger.Errorf("%s Initial task failed: %v", d.logPrefix, err)
		}

		// Start periodic execution
		d.ticker = time.NewTicker(d.interval)
		defer d.ticker.Stop()

		logger.Debugf("%s Entering main execution loop", d.logPrefix)
		for {
			select {
			case <-d.ctx.Done():
				logger.Debugf("%s Context cancelled, stopping daemon", d.logPrefix)
				return
			case <-d.ticker.C:
				logger.Debugf("%s Executing periodic task", d.logPrefix)
				if err := d.task(d.ctx); err != nil {
					logger.Errorf("%s Task execution failed: %v", d.logPrefix, err)
				}
			}
		}
	}()

	return nil
}

// Stop gracefully stops the daemon
func (d *Daemon) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	logger.Infof("%s Stopping daemon", d.logPrefix)
	d.cancel()
	if d.ticker != nil {
		d.ticker.Stop()
	}
	d.running = false
}

// IsRunning returns whether the daemon is currently running
func (d *Daemon) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// GetInterval returns the daemon's configured interval
func (d *Daemon) GetInterval() time.Duration {
	return d.interval
}