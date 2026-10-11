// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		{"/media/g1/gif.gif", "", http.StatusUnauthorized},
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

func TestFavorites(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := env.do(t, http.MethodGet, "/api/v1/favorites", testUserID, nil)
	assert.Equal(t, []Item{}, decode[[]Item](t, rec))

	rec = env.do(t, http.MethodPost, "/api/v1/favorites", testUserID, itemRequest{Kind: kindGIF, ID: "g1"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = env.do(t, http.MethodPost, "/api/v1/favorites", testUserID, itemRequest{Kind: kindSticker, ID: "s1"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	favs := decode[[]Item](t, rec)
	require.Len(t, favs, 2)
	assert.Equal(t, "s1", favs[0].ID)
	assert.Equal(t, mediaPrefix+"g1/tinygif.gif", favs[1].Preview.URL)

	// Only items of the service, of the right kind.
	assert.Equal(t, http.StatusNotFound, env.do(t, http.MethodPost, "/api/v1/favorites", testUserID, itemRequest{Kind: kindGIF, ID: "nope"}).Code)
	assert.Equal(t, http.StatusNotFound, env.do(t, http.MethodPost, "/api/v1/favorites", testUserID, itemRequest{Kind: kindGIF, ID: "s1"}).Code)
	assert.Equal(t, http.StatusBadRequest, env.do(t, http.MethodPost, "/api/v1/favorites", testUserID, itemRequest{Kind: kindGIF, ID: "a,b"}).Code)
	assert.Equal(t, http.StatusBadRequest, env.do(t, http.MethodPost, "/api/v1/favorites", testUserID, "not an object").Code)

	rec = env.do(t, http.MethodDelete, "/api/v1/favorites/gif/g1", testUserID, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decode[[]Item](t, rec), 1)

	// Another user has their own.
	assert.Equal(t, []Item{}, decode[[]Item](t, env.do(t, http.MethodGet, "/api/v1/favorites", "otheruserxxxxxxxxxxxxxxxxx", nil)))
}

func TestPost(t *testing.T) {
	env := newTestEnv(t, nil)
	env.api.On("HasPermissionToChannel", testUserID, testChannelID, model.PermissionCreatePost).Return(true)
	env.api.On("HasPermissionToChannel", mock.Anything, mock.Anything, model.PermissionCreatePost).Return(false)
	var created *model.Post
	env.api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(func(post *model.Post) (*model.Post, *model.AppError) {
		created = post.Clone()
		created.Id = model.NewId()
		return created, nil
	})

	rec := env.do(t, http.MethodPost, "/api/v1/post", testUserID, postRequest{
		itemRequest: itemRequest{Kind: kindGIF, ID: "g1"}, ChannelID: testChannelID, RootID: testRootID, Query: "cat",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, created)
	assert.Equal(t, PostType, created.Type)
	assert.Equal(t, testUserID, created.UserId)
	assert.Equal(t, testChannelID, created.ChannelId)
	assert.Equal(t, testRootID, created.RootId)
	assert.Equal(t, "![Happy cat](https://chat.example.com"+mediaPrefix+"g1/gif.gif)", created.Message)
	props, ok := created.GetProp(PostPropKey).(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "g1", props["id"])
	assert.Equal(t, mediaPrefix+"g1/mp4.mp4", props["post"].(map[string]any)["url"])
	assert.Equal(t, true, props["post"].(map[string]any)["video"])

	// The share is registered with the search, and the GIF is recently used.
	assert.Eventually(t, func() bool {
		env.service.mu.Lock()
		defer env.service.mu.Unlock()
		return len(env.service.shares) == 1 && env.service.shares[0] == "g1:cat"
	}, 2*time.Second, 10*time.Millisecond)
	recent := decode[[]Item](t, env.do(t, http.MethodGet, "/api/v1/recent", testUserID, nil))
	require.Len(t, recent, 1)
	assert.Equal(t, "g1", recent[0].ID)

	// Stickers post their image.
	rec = env.do(t, http.MethodPost, "/api/v1/post", testUserID, postRequest{itemRequest: itemRequest{Kind: kindSticker, ID: "s1"}, ChannelID: testChannelID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "![It works!](https://chat.example.com"+mediaPrefix+"s1/tinywebp_transparent.webp)", created.Message)
	assert.Equal(t, "", created.RootId)

	// Checks.
	assert.Equal(t, http.StatusForbidden, env.do(t, http.MethodPost, "/api/v1/post", testUserID, postRequest{
		itemRequest: itemRequest{Kind: kindGIF, ID: "g1"}, ChannelID: "otherchannelxxxxxxxxxxxxxx",
	}).Code)
	assert.Equal(t, http.StatusBadRequest, env.do(t, http.MethodPost, "/api/v1/post", testUserID, postRequest{
		itemRequest: itemRequest{Kind: kindGIF, ID: "g1"}, ChannelID: "bad",
	}).Code)
	assert.Equal(t, http.StatusNotFound, env.do(t, http.MethodPost, "/api/v1/post", testUserID, postRequest{
		itemRequest: itemRequest{Kind: kindSticker, ID: "g1"}, ChannelID: testChannelID,
	}).Code)
}

func TestMediaProxy(t *testing.T) {
	env := newTestEnv(t, nil)
	rec := env.do(t, http.MethodGet, "/media/g1/gif.gif", testUserID, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "MEDIA:g1/gif.gif", rec.Body.String())
	assert.Equal(t, "image/gif", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, rec.Header().Get("Cache-Control"), "private")

	// Cached.
	env.do(t, http.MethodGet, "/media/g1/gif.gif", testUserID, nil)
	assert.Equal(t, 1, env.service.count("/media/g1/gif.gif"))

	// Range requests, for videos.
	req := httptest.NewRequest(http.MethodGet, "/media/g1/mp4.mp4", nil)
	req.Header.Set("Mattermost-User-Id", testUserID)
	req.Header.Set("Range", "bytes=0-4")
	rec = httptest.NewRecorder()
	env.p.ServeHTTP(nil, rec, req)
	assert.Equal(t, http.StatusPartialContent, rec.Code)
	assert.Equal(t, "MEDIA", rec.Body.String())
	assert.Equal(t, "video/mp4", rec.Header().Get("Content-Type"))

	for target, status := range map[string]int{
		"/media/missing.gif":      http.StatusNotFound,
		"/media/page.html":        http.StatusUnsupportedMediaType,
		"/media/g1/../g1/gif.gif": http.StatusNotFound,
		"/media/g1//gif.gif":      http.StatusNotFound,
		"/media/":                 http.StatusNotFound,
	} {
		rec := env.do(t, http.MethodGet, target, testUserID, nil)
		assert.Equal(t, status, rec.Code, target)
	}
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

func TestConfigurationEnvironment(t *testing.T) {
	env := map[string]string{
		"AM_GIFS_SERVICE_URL": " http://gifs-env:8080/v2 ",
		"AM_GIFS_API_KEY":     "env-key",
		"AM_GIFS_UPLOAD_KEY":  "env-upload",
	}
	getenv := func(name string) string { return env[name] }

	t.Run("empty settings come from the environment", func(t *testing.T) {
		c := &configuration{}
		c.applyEnvironment(getenv)
		assert.Equal(t, "http://gifs-env:8080/v2", c.ServiceURL)
		assert.Equal(t, "env-key", c.APIKey)
		assert.Equal(t, "env-upload", c.UploadKey)
	})

	t.Run("System Console values win", func(t *testing.T) {
		c := &configuration{ServiceURL: "http://gifs:8080/v2", APIKey: "console-key", UploadKey: " "}
		c.applyEnvironment(getenv)
		assert.Equal(t, "http://gifs:8080/v2", c.ServiceURL)
		assert.Equal(t, "console-key", c.APIKey)
		assert.Equal(t, "env-upload", c.UploadKey, "a blank setting counts as empty")
	})

	t.Run("OnConfigurationChange reads the process environment", func(t *testing.T) {
		t.Setenv("AM_GIFS_SERVICE_URL", "http://gifs-env:8080/v2")
		t.Setenv("AM_GIFS_API_KEY", "env-key")
		t.Setenv("AM_GIFS_UPLOAD_KEY", "")

		api := &plugintest.API{}
		api.On("LoadPluginConfiguration", mock.Anything).Run(func(args mock.Arguments) {
			args.Get(0).(*configuration).UploadKey = "console-upload"
		}).Return(nil)
		p := &Plugin{}
		p.SetAPI(api)

		require.NoError(t, p.OnConfigurationChange())
		c := p.getConfiguration()
		assert.Equal(t, "http://gifs-env:8080/v2", c.serviceBase())
		assert.Equal(t, "env-key", c.APIKey)
		assert.Equal(t, "console-upload", c.UploadKey)
		assert.NotNil(t, p.getService())
	})
}
