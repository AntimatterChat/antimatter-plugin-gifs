// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The people of the permission tests. The plugin knows system admins by manage_system; members
// and guests are both people without it (guests may upload too when everyone may).
const (
	otherUserID = "oooooooooooooooooooooooooo"
	adminUserID = "aaaaaaaaaaaaaaaaaaaaaaaaaa"
	guestUserID = "gggggggggggggggggggggggggg"
)

// withPeople mocks who is a system admin, and the uploaders' accounts.
func (env *testEnv) withPeople() *testEnv {
	for user, admin := range map[string]bool{testUserID: false, otherUserID: false, guestUserID: false, adminUserID: true} {
		env.api.On("HasPermissionTo", user, model.PermissionManageSystem).Return(admin).Maybe()
	}
	for user, name := range map[string]string{testUserID: "maya", otherUserID: "nina", guestUserID: "gus", adminUserID: "root"} {
		env.api.On("GetUser", user).Return(&model.User{Id: user, Username: name}, nil).Maybe()
	}
	return env
}

func (env *testEnv) listUploads(t *testing.T, userID, query string) UploadList {
	t.Helper()
	rec := env.do(t, http.MethodGet, "/api/v1/uploads?"+query, userID, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decode[UploadList](t, rec)
}

func ids(list UploadList) []string {
	out := []string{}
	for _, e := range list.Uploads {
		out = append(out, e.ID)
	}
	return out
}

func TestUploadsAreListed(t *testing.T) {
	env := newTestEnv(t, withUploads("everyone")).withPeople()
	require.Equal(t, http.StatusOK, env.doUploadAs(t, testUserID, map[string]string{"title": "Lab wave", "tags": "hello"}, []byte("GIF89a")).Code)
	require.Equal(t, http.StatusOK, env.doUploadAs(t, otherUserID, map[string]string{"title": "Dance"}, []byte("GIF89a")).Code)
	require.Equal(t, http.StatusOK, env.doUploadAs(t, testUserID, map[string]string{"kind": "sticker", "title": "Dot", "pack": "Tiny"}, []byte("GIF89a")).Code)

	// Newest first, per kind, with the uploader and whether the user may delete each.
	gifs := env.listUploads(t, testUserID, "kind=gif")
	assert.Equal(t, []string{"up2", "up1"}, ids(gifs))
	assert.Equal(t, "nina", gifs.Uploads[0].Username)
	assert.False(t, gifs.Uploads[0].CanDelete)
	assert.Equal(t, "maya", gifs.Uploads[1].Username)
	assert.True(t, gifs.Uploads[1].CanDelete)
	assert.NotZero(t, gifs.Uploads[1].CreatedAt)
	require.NotNil(t, gifs.Uploads[1].Item)
	assert.Equal(t, mediaPrefix+"up1/tinygif.gif", gifs.Uploads[1].Item.Preview.URL)
	assert.Equal(t, []string{"up3"}, ids(env.listUploads(t, testUserID, "kind=sticker")))

	// Searched by title, tags and pack, and paged.
	assert.Equal(t, []string{"up1"}, ids(env.listUploads(t, testUserID, "kind=gif&q=HELLO")))
	assert.Equal(t, []string{"up3"}, ids(env.listUploads(t, testUserID, "kind=sticker&q=tiny")))
	assert.Empty(t, ids(env.listUploads(t, testUserID, "kind=gif&q=nothing")))
	page := env.listUploads(t, testUserID, "kind=gif&per_page=1")
	assert.Equal(t, []string{"up2"}, ids(page))
	assert.True(t, page.HasMore)
	page = env.listUploads(t, testUserID, "kind=gif&per_page=1&page=1")
	assert.Equal(t, []string{"up1"}, ids(page))
	assert.False(t, page.HasMore)

	// Admins may delete anyone's.
	for _, e := range env.listUploads(t, adminUserID, "kind=gif").Uploads {
		assert.True(t, e.CanDelete, e.ID)
	}

	// The same file again keeps its first uploader.
	require.Equal(t, http.StatusOK, env.doUploadAs(t, otherUserID, map[string]string{"title": "Wave again", "tags": "id:up1"}, []byte("GIF89a")).Code)
	gifs = env.listUploads(t, testUserID, "kind=gif")
	assert.Equal(t, []string{"up1", "up2"}, ids(gifs))
	assert.Equal(t, "Wave again", gifs.Uploads[0].Title)
	assert.Equal(t, testUserID, gifs.Uploads[0].UserID)
}

// TestUploadAndDeletePermissions checks who may add and delete GIFs and stickers, for each kind:
// members, guests and admins, with uploads open to everyone or to admins only.
func TestUploadAndDeletePermissions(t *testing.T) {
	for _, kind := range []string{kindGIF, kindSticker} {
		t.Run(kind, func(t *testing.T) {
			fields := func(title string) map[string]string { return map[string]string{"kind": kind, "title": title} }
			del := func(env *testEnv, userID, id string) int {
				return env.do(t, http.MethodDelete, "/api/v1/uploads/"+kind+"/"+id, userID, nil).Code
			}

			t.Run("everyone may upload", func(t *testing.T) {
				env := newTestEnv(t, withUploads("everyone")).withPeople()
				require.Equal(t, http.StatusOK, env.doUploadAs(t, testUserID, fields("Member's"), []byte("GIF89a")).Code) // up1
				require.Equal(t, http.StatusOK, env.doUploadAs(t, guestUserID, fields("Guest's"), []byte("GIF89a")).Code) // up2
				require.Equal(t, http.StatusOK, env.doUploadAs(t, otherUserID, fields("Other's"), []byte("GIF89a")).Code) // up3

				// A member can't delete someone else's, nor a guest.
				assert.Equal(t, http.StatusForbidden, del(env, testUserID, "up3"))
				assert.Equal(t, http.StatusForbidden, del(env, guestUserID, "up1"))
				assert.Empty(t, env.service.deletedIDs())

				// Each deletes their own; an admin anyone's.
				assert.Equal(t, http.StatusNoContent, del(env, testUserID, "up1"))
				assert.Equal(t, http.StatusNoContent, del(env, guestUserID, "up2"))
				assert.Equal(t, http.StatusNoContent, del(env, adminUserID, "up3"))
				assert.Equal(t, []string{"up1", "up2", "up3"}, env.service.deletedIDs())
				assert.Empty(t, ids(env.listUploads(t, testUserID, "kind="+kind)))

				// Gone: not found.
				assert.Equal(t, http.StatusNotFound, del(env, adminUserID, "up3"))
				assert.Equal(t, http.StatusNotFound, del(env, adminUserID, "never"))
			})

			t.Run("admins only", func(t *testing.T) {
				env := newTestEnv(t, withUploads("everyone")).withPeople()
				require.Equal(t, http.StatusOK, env.doUploadAs(t, testUserID, fields("Member's"), []byte("GIF89a")).Code) // up1

				// Uploads become admins' only: members and guests can neither add nor delete,
				// even what they added before.
				env.p.setConfiguration(&configuration{ServiceURL: env.service.URL + "/v2", APIKey: "secret", EnableStickers: true, ProxyMedia: true, UploadKey: "up-secret", WhoCanUpload: "admins"})
				for _, user := range []string{testUserID, guestUserID} {
					assert.Equal(t, http.StatusForbidden, env.doUploadAs(t, user, fields("No"), []byte("GIF89a")).Code, user)
					cfg := decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", user, nil))
					assert.False(t, cfg.CanUpload, user)
					assert.False(t, cfg.CanDeleteOthers, user)
				}
				assert.Equal(t, http.StatusForbidden, del(env, testUserID, "up1"))
				assert.False(t, env.listUploads(t, testUserID, "kind="+kind).Uploads[0].CanDelete)

				cfg := decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", adminUserID, nil))
				assert.True(t, cfg.CanUpload)
				assert.True(t, cfg.CanDeleteOthers)
				require.Equal(t, http.StatusOK, env.doUploadAs(t, adminUserID, fields("Admin's"), []byte("GIF89a")).Code)
				assert.Equal(t, http.StatusNoContent, del(env, adminUserID, "up1"))
			})

			t.Run("uploads off", func(t *testing.T) {
				// Without an upload key, nobody adds or deletes, admins neither.
				env := newTestEnv(t, nil).withPeople()
				for _, user := range []string{testUserID, guestUserID, adminUserID} {
					assert.Equal(t, http.StatusForbidden, env.doUploadAs(t, user, fields("No"), []byte("GIF89a")).Code, user)
					assert.Equal(t, http.StatusForbidden, del(env, user, "up1"), user)
					cfg := decode[ClientConfig](t, env.do(t, http.MethodGet, "/api/v1/config", user, nil))
					assert.False(t, cfg.CanUpload, user)
					assert.False(t, cfg.CanDeleteOthers, user)
				}
				assert.Zero(t, env.service.count("/v2/upload"))
			})

			t.Run("service not configured", func(t *testing.T) {
				env := newTestEnv(t, func(c *configuration) {
					withUploads("everyone")(c)
					c.ServiceURL = ""
				}).withPeople()
				assert.Equal(t, http.StatusForbidden, env.doUploadAs(t, adminUserID, fields("No"), []byte("GIF89a")).Code)
				assert.Equal(t, http.StatusForbidden, del(env, adminUserID, "up1"))
				assert.Equal(t, http.StatusServiceUnavailable, env.do(t, http.MethodGet, "/api/v1/uploads?kind="+kind, testUserID, nil).Code)
			})
		})
	}

	// Stickers turned off: none added, listed or deleted, GIFs still are.
	env := newTestEnv(t, func(c *configuration) {
		withUploads("everyone")(c)
		c.EnableStickers = false
	}).withPeople()
	assert.Equal(t, http.StatusForbidden, env.doUploadAs(t, adminUserID, map[string]string{"kind": kindSticker, "title": "No"}, []byte("GIF89a")).Code)
	assert.Equal(t, http.StatusForbidden, env.do(t, http.MethodGet, "/api/v1/uploads?kind=sticker", adminUserID, nil).Code)
	assert.Equal(t, http.StatusForbidden, env.do(t, http.MethodDelete, "/api/v1/uploads/sticker/up1", adminUserID, nil).Code)
	require.Equal(t, http.StatusOK, env.doUploadAs(t, testUserID, map[string]string{"title": "Yes"}, []byte("GIF89a")).Code)
	assert.Equal(t, http.StatusNoContent, env.do(t, http.MethodDelete, "/api/v1/uploads/gif/up1", testUserID, nil).Code)
}

func TestDeleteUploadCleansUp(t *testing.T) {
	env := newTestEnv(t, withUploads("everyone")).withPeople()
	require.Equal(t, http.StatusOK, env.doUploadAs(t, testUserID, map[string]string{"title": "Wave", "tags": "id:gone"}, []byte("GIF89a")).Code)
	require.Len(t, decode[[]Item](t, env.do(t, http.MethodGet, "/api/v1/favorites", testUserID, nil)), 1)

	// The service no longer knowing it is fine: the record and the favourite go anyway.
	assert.Equal(t, http.StatusNoContent, env.do(t, http.MethodDelete, "/api/v1/uploads/gif/gone", testUserID, nil).Code)
	assert.Empty(t, ids(env.listUploads(t, testUserID, "kind=gif")))
	assert.Empty(t, decode[[]Item](t, env.do(t, http.MethodGet, "/api/v1/favorites", testUserID, nil)))
	item, err := env.p.store.UploadItem(kindGIF, "gone")
	require.NoError(t, err)
	assert.Nil(t, item)

	// Anonymous requests are refused.
	assert.Equal(t, http.StatusUnauthorized, env.do(t, http.MethodGet, "/api/v1/uploads?kind=gif", "", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, env.do(t, http.MethodDelete, "/api/v1/uploads/gif/up1", "", nil).Code)
}

func TestAddUploadKeepsAtMostMaxUploads(t *testing.T) {
	old := maxUploads
	maxUploads = 3
	t.Cleanup(func() { maxUploads = old })
	store := NewStore(newFakeKV())
	for _, id := range []string{"u1", "u2", "u3", "u4", "u5"} {
		require.NoError(t, store.AddUpload(Upload{ID: id, Kind: kindGIF, UserID: testUserID}, Item{ID: id, Kind: kindGIF}))
	}
	uploads, err := store.Uploads(kindGIF)
	require.NoError(t, err)
	got := []string{}
	for _, up := range uploads {
		got = append(got, up.ID)
	}
	assert.Equal(t, []string{"u5", "u4", "u3"}, got)
	// The oldest records went with their snapshots.
	item, err := store.UploadItem(kindGIF, "u1")
	require.NoError(t, err)
	assert.Nil(t, item)
	item, err = store.UploadItem(kindGIF, "u3")
	require.NoError(t, err)
	require.NotNil(t, item)
}
