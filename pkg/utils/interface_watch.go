// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package utils

import (
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"zeroflex/pkg/log"
)

// InterfaceEventType represents the type of interface event
// (add, remove, up, down)
type InterfaceEventType string

const (
	InterfaceAdded   InterfaceEventType = "added"
	InterfaceRemoved InterfaceEventType = "removed"
	InterfaceUp      InterfaceEventType = "up"
	InterfaceDown    InterfaceEventType = "down"
)

// InterfaceEvent represents an interface event
// Name: interface name, Type: event type
// Index: interface index
// Link: netlink.Link object (may be nil for removed)
type InterfaceEvent struct {
	Name  string
	Type  InterfaceEventType
	Index int
	Link  netlink.Link
}

// WatchInterfacesNetlink watches for interface add/remove/up/down events using netlink.
// Calls the callback for each event.
func WatchInterfacesNetlink(callback func(InterfaceEvent), stopCh <-chan struct{}, logLevel string) error {
	logger := log.NewScopedLogger("[interface_watch]", logLevel)
	logger.Verbose("Netlink watcher started")
	ch := make(chan netlink.LinkUpdate)
	done := make(chan struct{})
	if err := netlink.LinkSubscribe(ch, done); err != nil {
		logger.Error("Netlink LinkSubscribe failed: %v", err)
		return err
	}
	go func() {
		for {
			select {
			case update := <-ch:
				var eventType InterfaceEventType
				if update.Header.Type == unix.RTM_DELLINK {
					eventType = InterfaceRemoved
				} else if update.Header.Type == unix.RTM_NEWLINK {
					if update.Link.Attrs().OperState == netlink.OperUp {
						eventType = InterfaceUp
					} else {
						eventType = InterfaceDown
					}
				}
				logger.Debug("Event: %s %s (index %d)", eventType, update.Link.Attrs().Name, update.Link.Attrs().Index)
				callback(InterfaceEvent{
					Name:  update.Link.Attrs().Name,
					Type:  eventType,
					Index: update.Link.Attrs().Index,
					Link:  update.Link,
				})
			case <-stopCh:
				close(done)
				logger.Verbose("Netlink watcher stopped")
				return
			}
		}
	}()
	return nil
}

// PollInterfaces periodically lists interfaces and calls the callback for add/remove events.
// interval: polling interval
type InterfacePollState struct {
	Known map[string]struct{}
}

func NewInterfacePollState() *InterfacePollState {
	return &InterfacePollState{Known: make(map[string]struct{})}
}

func PollInterfaces(interval time.Duration, callback func(InterfaceEvent), stopCh <-chan struct{}, logLevel string) {
	logger := log.NewScopedLogger("[interface_watch]", logLevel)
	state := NewInterfacePollState()
	logger.Verbose("Polling watcher started (interval: %s)", interval)
	for {
		select {
		case <-stopCh:
			logger.Verbose("Polling watcher stopped")
			return
		case <-time.After(interval):
			links, err := netlink.LinkList()
			if err != nil {
				logger.Warn("Poll error: %v", err)
				continue
			}
			current := make(map[string]netlink.Link)
			for _, link := range links {
				current[link.Attrs().Name] = link
			}
			// Detect added
			for name, link := range current {
				if _, ok := state.Known[name]; !ok {
					logger.Debug("Event: added %s (index %d)", name, link.Attrs().Index)
					callback(InterfaceEvent{Name: name, Type: InterfaceAdded, Index: link.Attrs().Index, Link: link})
				}
				state.Known[name] = struct{}{}
			}
			// Detect removed
			for name := range state.Known {
				if _, ok := current[name]; !ok {
					logger.Debug("Event: removed %s", name)
					callback(InterfaceEvent{Name: name, Type: InterfaceRemoved, Index: 0, Link: nil})
					delete(state.Known, name)
				}
			}
		}
	}
}
