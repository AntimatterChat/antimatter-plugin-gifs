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
	"time"
)

const (
	// mediaTTL is how long media files stay in the proxy's cache.
	mediaTTL = 24 * time.Hour
	// maxMediaSize is the largest media file the proxy serves.
	maxMediaSize = 16 << 20
)

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
		entry, status = p.fetchMedia(r.Context(), s, rel)
		if entry == nil {
			http.Error(w, http.StatusText(status), status)
			return
		}
		s.cache.set(cacheKey, entry.body, entry.contentType, mediaTTL)
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
	defer resp.Body.Close()
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
