// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	// mediaTTL is how long media files stay in the proxy's cache.
	mediaTTL = 24 * time.Hour
	// maxMediaSize is the largest media file the proxy serves.
	maxMediaSize = 16 << 20
	// maxConcurrentMediaFetches is how many media files the proxy downloads at once: each is held
	// in memory, up to maxMediaSize (128 MB for 8).
	maxConcurrentMediaFetches = 8
	// mediaFetchWait is how long a download waits for its turn before a 503.
	mediaFetchWait = 30 * time.Second
)

// mediaCall is the download of a media file, which the requests for that file wait for.
type mediaCall struct {
	done   chan struct{}
	entry  *cacheEntry
	status int
}

// mediaFetcher downloads each media file once for the requests that want it at the same time, and
// a few files at a time: requests for many large files missing from the cache held them all.
type mediaFetcher struct {
	mu    sync.Mutex
	calls map[string]*mediaCall
	slots chan struct{}
}

func newMediaFetcher(concurrency int) *mediaFetcher {
	return &mediaFetcher{calls: map[string]*mediaCall{}, slots: make(chan struct{}, concurrency)}
}

// do returns the result of fetch for key, joining a download of it under way. The download goes on
// when the request ends (others may wait for it, and it fills the cache).
func (f *mediaFetcher) do(ctx context.Context, key string, fetch func(context.Context) (*cacheEntry, int)) (*cacheEntry, int) {
	f.mu.Lock()
	c, ok := f.calls[key]
	if !ok {
		c = &mediaCall{done: make(chan struct{})}
		f.calls[key] = c
		go f.run(key, c, fetch)
	}
	f.mu.Unlock()
	select {
	case <-c.done:
		return c.entry, c.status
	case <-ctx.Done():
		return nil, http.StatusServiceUnavailable
	}
}

func (f *mediaFetcher) run(key string, c *mediaCall, fetch func(context.Context) (*cacheEntry, int)) {
	defer func() {
		if v := recover(); v != nil {
			c.entry, c.status = nil, http.StatusInternalServerError
		}
		f.mu.Lock()
		delete(f.calls, key)
		f.mu.Unlock()
		close(c.done)
	}()
	wait := time.NewTimer(mediaFetchWait)
	defer wait.Stop()
	select {
	case f.slots <- struct{}{}:
		defer func() { <-f.slots }()
	case <-wait.C:
		c.status = http.StatusServiceUnavailable
		return
	}
	c.entry, c.status = fetch(context.Background())
}

// mediaCSP keeps proxied files inert when opened directly: they come from the Antimatter origin.
const mediaCSP = "default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'; sandbox"

// handleMedia proxies a media file of the GIF service, so that clients only talk to the
// Antimatter server, and caches it.
func (p *Plugin) handleMedia(w http.ResponseWriter, r *http.Request) {
	s := p.getService()
	if !s.configured() || !s.proxyMedia || s.mediaBase == "" {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/media/")
	clean := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if rel == "" || clean != rel || strings.ContainsAny(rel, "?#\\") {
		http.NotFound(w, r)
		return
	}

	cacheKey := "media:" + s.mediaBase + rel
	entry, ok := s.cache.get(cacheKey)
	if !ok {
		var status int
		entry, status = s.media.do(r.Context(), cacheKey, func(ctx context.Context) (*cacheEntry, int) {
			fetched, fetchStatus := p.fetchMedia(ctx, s, rel)
			if fetched != nil {
				s.cache.set(cacheKey, fetched.body, fetched.contentType, mediaTTL)
			}
			return fetched, fetchStatus
		})
		if entry == nil {
			http.Error(w, http.StatusText(status), status)
			return
		}
	}

	h := w.Header()
	h.Set("Content-Type", entry.contentType)
	h.Set("Cache-Control", "private, max-age=86400")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", mediaCSP)
	// ServeContent answers range requests, which Safari needs to play videos.
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(entry.body))
}

// fetchMedia downloads a media file; it only accepts images and videos.
func (p *Plugin) fetchMedia(ctx context.Context, s *gifService, rel string) (*cacheEntry, int) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.mediaBase+rel, nil)
	if err != nil {
		return nil, http.StatusBadRequest
	}
	resp, err := s.http.Do(req)
	if err != nil {
		p.API.LogWarn("Failed to fetch a media file from the GIF service", "path", rel, "err", err.Error())
		return nil, http.StatusBadGateway
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, http.StatusNotFound
	}
	if resp.StatusCode != http.StatusOK {
		p.API.LogWarn("The GIF service didn't serve a media file", "path", rel, "status", resp.Status)
		return nil, http.StatusBadGateway
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(contentType, "video/") {
		return nil, http.StatusUnsupportedMediaType
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize+1))
	if err != nil {
		return nil, http.StatusBadGateway
	}
	if len(body) > maxMediaSize {
		return nil, http.StatusRequestEntityTooLarge
	}
	return &cacheEntry{body: body, contentType: contentType}, http.StatusOK
}
