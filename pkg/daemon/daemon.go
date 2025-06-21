// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package daemon

import (
	"zt-dns-companion/pkg/logging"
	
	"context"
	"fmt"
	"time"
)

// Interface for managing daemon functionality
type Interface interface {
	Start() error
	Stop()
	IsRunning() bool
	SetLogPrefix(string)
}

// Simple implements basic daemon functionality
type Simple struct {
	interval    time.Duration
	task        func(context.Context) error
	ticker      *time.Ticker
	stopChan    chan struct{}
	running     bool
	logPrefix   string
}

// NewSimple creates a new daemon instance
func NewSimple(interval time.Duration, task func(context.Context) error) *Simple {
	return &Simple{
		interval: interval,
		task:     task,
		stopChan: make(chan struct{}),
	}
}

func (d *Simple) Start() error {
	if d.running {
		return fmt.Errorf("daemon already running")
	}

	d.running = true
	d.ticker = time.NewTicker(d.interval)

	go func() {
		defer func() {
			d.running = false
			if d.ticker != nil {
				d.ticker.Stop()
			}
		}()

		// Execute task immediately on start
		logging.DaemonLogger.Debug("Executing initial task")
		if err := d.task(context.Background()); err != nil {
			logging.DaemonLogger.Error("Initial task execution failed: %v", err)
		}

		// Then start the interval-based execution
		for {
			select {
			case <-d.ticker.C:
				logging.DaemonLogger.Debug("Executing scheduled task")
				if err := d.task(context.Background()); err != nil {
					logging.DaemonLogger.Error("Scheduled task execution failed: %v", err)
				}
			case <-d.stopChan:
				logging.DaemonLogger.Debug("Daemon stopping")
				return
			}
		}
	}()

	return nil
}

func (d *Simple) Stop() {
	if !d.running {
		return
	}

	close(d.stopChan)
	d.running = false
}

func (d *Simple) IsRunning() bool {
	return d.running
}

func (d *Simple) SetLogPrefix(prefix string) {
	d.logPrefix = prefix
}