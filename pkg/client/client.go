// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package client

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// ServiceAPIClient wraps http.Client with ZeroTier authentication
type ServiceAPIClient struct {
	apiKey string
	client *http.Client
}

// NewServiceAPI creates a new authenticated HTTP client for ZeroTier API
func NewServiceAPI(tokenFile string) (*ServiceAPIClient, error) {
	content, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read token file %s: %w", tokenFile, err)
	}

	return &ServiceAPIClient{
		apiKey: strings.TrimSpace(string(content)),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// Do executes HTTP requests with ZeroTier authentication
func (c *ServiceAPIClient) Do(req *http.Request) (*http.Response, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("empty API key, authentication failed")
	}

	req.Header.Add("X-ZT1-Auth", c.apiKey)
	return c.client.Do(req)
}

// LoadAPIToken loads API token from file or argument
func LoadAPIToken(tokenFile, tokenArg string) string {
	if tokenArg != "" {
		return tokenArg
	}

	content, err := os.ReadFile(tokenFile)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}