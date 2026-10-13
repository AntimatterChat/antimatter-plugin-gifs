// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"net/url"
	"os"
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

// configuration captures the plugin's external configuration as exposed in the Antimatter server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Antimatter server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
type configuration struct {
	// ServiceURL is the Tenor API v2 base URL of the GIF service, e.g. http://gifs-server:8080/v2.
	ServiceURL string
	APIKey     string
	// MediaURL is the base URL of the service's media files, which the plugin proxies; by
	// default the service's origin + /media/.
	MediaURL string
	// ContentFilter is Tenor's contentfilter: off, low, medium or high.
	ContentFilter  string
	EnableStickers bool
	// ProxyMedia serves the media through the plugin, so clients don't need to reach the service.
	ProxyMedia  bool
	CacheSizeMB int
	// UploadKey is an upload key of the GIF service (AM_GIFS_UPLOAD_KEYS of gifs-server): with
	// it, users can add GIFs and stickers from the picker.
	UploadKey string
	// WhoCanUpload is who can add GIFs and stickers: admins (system admins) or everyone.
	WhoCanUpload string
}

// The environment variables that configure the plugin when the matching setting is empty in the
// System Console, e.g. from a docker-compose file. Plugins inherit the server's environment.
const (
	envServiceURL = "AM_GIFS_SERVICE_URL"
	envAPIKey     = "AM_GIFS_API_KEY"
	envUploadKey  = "AM_GIFS_UPLOAD_KEY"
)

// applyEnvironment fills the service URL, API key and upload key left empty in the System Console
// from their environment variables, read with getenv. A value set in the System Console wins.
func (c *configuration) applyEnvironment(getenv func(string) string) {
	for name, setting := range map[string]*string{
		envServiceURL: &c.ServiceURL,
		envAPIKey:     &c.APIKey,
		envUploadKey:  &c.UploadKey,
	} {
		if strings.TrimSpace(*setting) == "" {
			*setting = strings.TrimSpace(getenv(name))
		}
	}
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	clone := *c
	return &clone
}

// serviceBase returns the API base URL without trailing slash, empty when not configured.
func (c *configuration) serviceBase() string {
	return strings.TrimRight(strings.TrimSpace(c.ServiceURL), "/")
}

// mediaBase returns the base URL of the media files, with a trailing slash.
func (c *configuration) mediaBase() string {
	if m := strings.TrimSpace(c.MediaURL); m != "" {
		return strings.TrimRight(m, "/") + "/"
	}
	u, err := url.Parse(c.serviceBase())
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/media/"
}

func (c *configuration) contentFilter() string {
	switch c.ContentFilter {
	case "off", "low", "medium", "high":
		return c.ContentFilter
	}
	return "medium"
}

// everyoneCanUpload tells whether all users, not only system admins, can add GIFs and stickers.
func (c *configuration) everyoneCanUpload() bool {
	return c.WhoCanUpload == "everyone"
}

func (c *configuration) cacheBytes() int64 {
	mb := c.CacheSizeMB
	if mb <= 0 {
		mb = 64
	}
	return int64(mb) << 20
}

// IsValid checks the configuration.
func (c *configuration) IsValid() error {
	for name, value := range map[string]string{"GIF service URL": c.serviceBase(), "Media URL": strings.TrimSpace(c.MediaURL)} {
		if value == "" {
			continue
		}
		u, err := url.Parse(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.Errorf("%s must be an http or https URL", name)
		}
	}
	return nil
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	configuration := new(configuration)

	// Load the public configuration fields from the Antimatter server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}
	configuration.applyEnvironment(os.Getenv)
	if err := configuration.IsValid(); err != nil {
		return errors.Wrap(err, "invalid plugin configuration")
	}

	p.setConfiguration(configuration)
	p.setService(newGIFService(configuration, p.client()))

	return nil
}
