// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// pluginID is the ID of the plugin, as in plugin.json.
const pluginID = "com.antimatterchat.gifs"

// Plugin implements the interface expected by the Antimatter server to communicate between the
// server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	// serviceLock guards service, which is replaced when the configuration changes.
	serviceLock sync.RWMutex
	service     *gifService

	httpClient *http.Client
	store      *Store
	router     *mux.Router
}

// apiError is an error with the HTTP status to answer.
type apiError struct {
	status int
	err    error
}

func (e *apiError) Error() string {
	return e.err.Error()
}

func newAPIError(status int, msg string) *apiError {
	return &apiError{status: status, err: errors.New(msg)}
}

// statusFor returns the HTTP status for an error returned by the plugin's operations.
func statusFor(err error) int {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.status
	}
	return http.StatusInternalServerError
}

// OnActivate is invoked when the plugin is activated.
func (p *Plugin) OnActivate() error {
	p.store = NewStore(p.API)
	p.router = p.newRouter()
	return p.registerCommands()
}

func (p *Plugin) setService(s *gifService) {
	p.serviceLock.Lock()
	defer p.serviceLock.Unlock()
	p.service = s
}

func (p *Plugin) getService() *gifService {
	p.serviceLock.RLock()
	defer p.serviceLock.RUnlock()
	return p.service
}

// ServeHTTP serves the plugin's REST API and media proxy.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// client is the HTTP client calling the GIF service; tests replace it.
func (p *Plugin) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return defaultHTTPClient
}
