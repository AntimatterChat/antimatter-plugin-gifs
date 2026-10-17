// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doUpload posts a multipart upload to the plugin as the test user.
func (env *testEnv) doUpload(t *testing.T, fields map[string]string, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	return env.doUploadAs(t, testUserID, fields, file)
}

// doUploadAs posts a multipart upload to the plugin as a user.
func (env *testEnv) doUploadAs(t *testing.T, userID string, fields map[string]string, file []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		require.NoError(t, mw.WriteField(k, v))
	}
	if file != nil {
		fw, err := mw.CreateFormFile("file", "dance.gif")
		require.NoError(t, err)
		_, _ = fw.Write(file)
	}
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Mattermost-User-Id", userID)
	rec := httptest.NewRecorder()
	env.p.ServeHTTP(nil, rec, req)
	return rec
}

func withUploads(who string) func(c *configuration) {
	return func(c *configuration) {
		c.UploadKey = "up-secret"
		c.WhoCanUpload = who
	}
}

func TestUploadPermissions(t *testing.T) {
	// Without an upload key, nobody can upload.
	env := newTestEnv(t, nil)
	assert.False(t, decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", testUserID, nil)).CanUpload)
	assert.Equal(t, http.StatusForbidden, env.doUpload(t, map[string]string{"title": "Dance"}, []byte("GIF89a")).Code)

	// Admins only, by default.
	env = newTestEnv(t, withUploads(""))
	env.api.On("HasPermissionTo", testUserID, model.PermissionManageSystem).Return(false)
	assert.False(t, decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", testUserID, nil)).CanUpload)
	assert.Equal(t, http.StatusForbidden, env.doUpload(t, map[string]string{"title": "Dance"}, []byte("GIF89a")).Code)
	assert.Equal(t, 0, env.service.count("/v2/upload"))

	env = newTestEnv(t, withUploads("admins"))
	env.api.On("HasPermissionTo", testUserID, model.PermissionManageSystem).Return(true)
	assert.True(t, decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", testUserID, nil)).CanUpload)

	env = newTestEnv(t, withUploads("everyone"))
	env.api.On("HasPermissionTo", testUserID, model.PermissionManageSystem).Return(false)
	cfg := decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", testUserID, nil))
	assert.True(t, cfg.CanUpload)
	assert.False(t, cfg.CanDeleteOthers)
	assert.Equal(t, int64(uploadMaxSizeBytes), cfg.UploadMaxBytes)
}

func TestUpload(t *testing.T) {
	env := newTestEnv(t, withUploads("everyone"))

	// A search is cached before the upload, which makes the service's lists change.
	env.do(t, http.MethodGet, "/api/v1/search?q=cat", testUserID, nil)

	rec := env.doUpload(t, map[string]string{"kind": "sticker", "title": "  Dance ", "tags": "dance, party", "pack": "Moves"}, []byte("GIF89a..."))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	it := decode[Item](t, rec)
	assert.Equal(t, "up1", it.ID)
	assert.Equal(t, kindSticker, it.Kind)
	assert.Equal(t, mediaPrefix+"up1/tinywebp_transparent.webp", it.Preview.URL)

	// The service got the fields, the file and the upload key.
	up := env.service.lastUpload()
	assert.Equal(t, "Bearer up-secret", up.auth)
	assert.Equal(t, map[string]string{"kind": "sticker", "title": "Dance", "tags": "dance, party", "pack": "Moves"}, up.fields)
	assert.Equal(t, "GIF89a...", up.file)
	assert.Equal(t, "dance.gif", up.fileName)

	// It's among the user's favourites, and searches are asked again.
	favs := decode[[]Item](t, env.do(t, http.MethodGet, "/api/v1/favorites", testUserID, nil))
	require.Len(t, favs, 1)
	assert.Equal(t, "up1", favs[0].ID)
	env.do(t, http.MethodGet, "/api/v1/search?q=cat", testUserID, nil)
	assert.Equal(t, 2, env.service.count("/v2/search"))

	// GIFs don't go in packs.
	require.Equal(t, http.StatusOK, env.doUpload(t, map[string]string{"title": "Dance", "pack": "Moves"}, []byte("GIF89a")).Code)
	assert.Equal(t, map[string]string{"kind": "gif", "title": "Dance"}, env.service.lastUpload().fields)
}

func TestUploadErrors(t *testing.T) {
	env := newTestEnv(t, withUploads("everyone"))
	assert.Equal(t, http.StatusBadRequest, env.doUpload(t, map[string]string{"title": " "}, []byte("GIF89a")).Code)
	assert.Equal(t, http.StatusBadRequest, env.doUpload(t, map[string]string{"title": "Dance"}, nil).Code)
	assert.Equal(t, http.StatusBadRequest, env.doUpload(t, map[string]string{"title": "Dance", "kind": "video"}, []byte("GIF89a")).Code)
	assert.Equal(t, 0, env.service.count("/v2/upload"))

	// The service's errors about the file are the user's; others are the server's.
	rec := env.doUpload(t, map[string]string{"title": "unconvertible"}, []byte("GIF89a"))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Contains(t, rec.Body.String(), "couldn't be converted")

	env = newTestEnv(t, func(c *configuration) {
		c.UploadKey = "wrong"
		c.WhoCanUpload = "everyone"
	})
	rec = env.doUpload(t, map[string]string{"title": "Dance"}, []byte("GIF89a"))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing a valid upload key")

	// Stickers can be turned off.
	env = newTestEnv(t, func(c *configuration) {
		withUploads("everyone")(c)
		c.EnableStickers = false
	})
	assert.Equal(t, http.StatusForbidden, env.doUpload(t, map[string]string{"title": "Dance", "kind": "sticker"}, []byte("GIF89a")).Code)
}
