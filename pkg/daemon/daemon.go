// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package daemon

import (
	"zt-dns-companion/pkg/logger"
	"zt-dns-companion/pkg/utils"

	"context"
	"sync"
	"time"
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
	logger.Trace("Creating new daemon instance with interval: %s", intervalStr)

	interval, err := utils.ParseInterval(intervalStr)
	if err != nil {
		logger.Error("Failed to parse daemon interval '%s': %v", intervalStr, err)
		return nil, err
	}

	logger.Trace("Parsed daemon interval: %s -> %v", intervalStr, interval)

	ctx, cancel := context.WithCancel(context.Background())

	daemon := &Daemon{
		interval:  interval,
		task:      task,
		ctx:       ctx,
		cancel:    cancel,
		logPrefix: "[daemon]",
	}

	logger.Debug("Daemon instance created successfully")
	return daemon, nil
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
		logger.InfoWithPrefix("daemon", "Daemon disabled (interval: 0)")
		return nil
	}

	d.running = true
	logger.InfoWithPrefix("daemon", "Starting daemon with interval: %s", utils.FormatInterval(d.interval))

	// Run initial task immediately
	go func() {
		logger.DebugWithPrefix("daemon", "Running initial task")
		if err := d.task(d.ctx); err != nil {
			logger.ErrorWithPrefix("daemon", "Initial task failed: %v", err)
		}

		// Start periodic execution
		d.ticker = time.NewTicker(d.interval)
		defer d.ticker.Stop()

		logger.DebugWithPrefix("daemon", "Entering main execution loop")
		for {
			select {
			case <-d.ctx.Done():
				logger.DebugWithPrefix("daemon", "Context cancelled, stopping daemon")
				return
			case <-d.ticker.C:
				logger.DebugWithPrefix("daemon", "Executing periodic task")
				if err := d.task(d.ctx); err != nil {
					logger.ErrorWithPrefix("daemon", "Task execution failed: %v", err)
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

	logger.InfoWithPrefix("daemon", "Stopping daemon")
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