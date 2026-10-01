// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchAndFeatured(t *testing.T) {
	env := newTestEnv(t, nil)

	rec := env.do(t, http.MethodGet, "/api/v1/search?q=cat&locale=pt-BR&limit=5", testUserID, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	page := decode[Page](t, rec)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "2", page.Next)

	g1 := page.Items[0]
	assert.Equal(t, "g1", g1.ID)
	assert.Equal(t, kindGIF, g1.Kind)
	assert.Equal(t, "Happy cat", g1.Title)
	assert.Equal(t, Media{URL: mediaPrefix + "g1/tinygif.gif", Width: 220, Height: 124}, g1.Preview)
	assert.Equal(t, Media{URL: mediaPrefix + "g1/mp4.mp4", Width: 498, Height: 280, Video: true}, g1.Post)
	require.NotNil(t, g1.Image)
	assert.Equal(t, mediaPrefix+"g1/gif.gif", g1.Image.URL)
	require.NotNil(t, g1.Still)
	assert.Equal(t, mediaPrefix+"g1/preview.png", g1.Still.URL)
	assert.Equal(t, "Via Tenor", g1.Attribution)
	assert.Equal(t, "https://tenor.com/view/g1", g1.SourceURL)

	// A GIF without video or tiny formats uses its GIF everywhere.
	g2 := page.Items[1]
	assert.Equal(t, mediaPrefix+"g2/gif.gif", g2.Preview.URL)
	assert.Equal(t, Media{URL: mediaPrefix + "g2/gif.gif", Width: 200, Height: 200}, g2.Post)
	assert.Nil(t, g2.Image)

	// The service gets the key, the client key, the content filter and the formats.
	req := env.service.requests[0]
	for _, part := range []string{"/v2/search?", "q=cat", "key=secret", "client_key=antimatter", "contentfilter=high", "locale=pt_BR", "limit=5", "media_filter=tinygif"} {
		assert.Contains(t, req, part)
	}

	// Responses are cached.
	env.do(t, http.MethodGet, "/api/v1/search?q=cat&locale=pt-BR&limit=5", testUserID, nil)
	assert.Equal(t, 1, env.service.count("/v2/search"))

	rec = env.do(t, http.MethodGet, "/api/v1/featured?kind=sticker&pos=2", testUserID, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	page = decode[Page](t, rec)
	require.Len(t, page.Items, 1)
	s1 := page.Items[0]
	assert.Equal(t, kindSticker, s1.Kind)
	assert.Equal(t, mediaPrefix+"s1/tinywebp_transparent.webp", s1.Preview.URL)
	assert.Equal(t, mediaPrefix+"s1/tinywebp_transparent.webp", s1.Post.URL)
	assert.Contains(t, env.service.requests[len(env.service.requests)-1], "searchfilter=sticker")
	assert.Contains(t, env.service.requests[len(env.service.requests)-1], "pos=2")
}

func TestCategoriesAndTerms(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := env.do(t, http.MethodGet, "/api/v1/categories", testUserID, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []Category{{Name: "cats", SearchTerm: "cats", Image: mediaPrefix + "g1/tinygif.gif"}}, decode[[]Category](t, rec))

	rec = env.do(t, http.MethodGet, "/api/v1/categories?kind=sticker", testUserID, nil)
	assert.Equal(t, "Lab crew", decode[[]Category](t, rec)[0].Name)

	for _, target := range []string{"/api/v1/autocomplete?q=ca", "/api/v1/suggestions?q=cat", "/api/v1/trending_terms"} {
		rec = env.do(t, http.MethodGet, target, testUserID, nil)
		require.Equal(t, http.StatusOK, rec.Code, target)
		assert.Equal(t, []string{"cat", "cats"}, decode[[]string](t, rec))
	}
	assert.Equal(t, 1, env.service.count("/v2/search_suggestions"))
}

func TestMediaNotProxied(t *testing.T) {
	env := newTestEnv(t, func(c *configuration) { c.ProxyMedia = false })
	page := decode[Page](t, env.do(t, http.MethodGet, "/api/v1/search?q=cat", testUserID, nil))
	assert.Equal(t, env.service.URL+"/media/g1/tinygif.gif", page.Items[0].Preview.URL)
	assert.Equal(t, http.StatusNotFound, env.do(t, http.MethodGet, "/media/g1/gif.gif", testUserID, nil).Code)
}

func TestErrors(t *testing.T) {
	env := newTestEnv(t, func(c *configuration) { c.EnableStickers = false })
	for _, tc := range []struct {
		target string
		user   string
		status int
	}{
		{"/api/v1/search?q=cat", "", http.StatusUnauthorized},
		{"/api/v1/search?q=", testUserID, http.StatusBadRequest},
		{"/api/v1/search?q=" + strings.Repeat("a", 101), testUserID, http.StatusBadRequest},
		{"/api/v1/search?q=cat&kind=video", testUserID, http.StatusBadRequest},
		{"/api/v1/featured?kind=sticker", testUserID, http.StatusForbidden},
		{"/api/v1/autocomplete", testUserID, http.StatusBadRequest},
	} {
		rec := env.do(t, http.MethodGet, tc.target, tc.user, nil)
		assert.Equal(t, tc.status, rec.Code, tc.target)
	}

	cfg := decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", testUserID, nil))
	assert.Equal(t, ClientConfig{Configured: true, Stickers: false}, cfg)

	// A wrong key: the service's message, never the key.
	env = newTestEnv(t, func(c *configuration) { c.APIKey = "wrong" })
	rec := env.do(t, http.MethodGet, "/api/v1/featured", testUserID, nil)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing a valid API key")
	assert.NotContains(t, rec.Body.String(), "wrong")

	// Not configured.
	env = newTestEnv(t, func(c *configuration) { c.ServiceURL = "" })
	assert.Equal(t, http.StatusServiceUnavailable, env.do(t, http.MethodGet, "/api/v1/featured", testUserID, nil).Code)
	assert.False(t, decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", testUserID, nil)).Configured)

	// Unreachable.
	env = newTestEnv(t, nil)
	env.service.Close()
	rec = env.do(t, http.MethodGet, "/api/v1/featured", testUserID, nil)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret")
}

func TestConfiguration(t *testing.T) {
	c := &configuration{ServiceURL: " http://gifs:8080/v2/ "}
	assert.Equal(t, "http://gifs:8080/v2", c.serviceBase())
	assert.Equal(t, "http://gifs:8080/media/", c.mediaBase())
	assert.Equal(t, "medium", c.contentFilter())
	assert.Equal(t, int64(64<<20), c.cacheBytes())
	assert.NoError(t, c.IsValid())

	c.MediaURL = "https://cdn.example.com/gifs"
	assert.Equal(t, "https://cdn.example.com/gifs/", c.mediaBase())
	assert.NoError(t, c.IsValid())

	assert.Error(t, (&configuration{ServiceURL: "gifs:8080"}).IsValid())
	assert.Error(t, (&configuration{ServiceURL: "ftp://gifs/v2"}).IsValid())
	assert.Error(t, (&configuration{MediaURL: "/media"}).IsValid())
	assert.NoError(t, (&configuration{}).IsValid())
	assert.Equal(t, "", (&configuration{}).mediaBase())
}
