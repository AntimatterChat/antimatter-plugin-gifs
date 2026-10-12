// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func previewOf(t *testing.T, post *model.Post) (*model.SlackAttachment, map[string]any) {
	t.Helper()
	attachments := post.Attachments()
	require.Len(t, attachments, 1)
	a := attachments[0]
	require.Len(t, a.Actions, 3)
	return a, a.Actions[0].Integration.Context
}

func TestCommand(t *testing.T) {
	env := newTestEnv(t, nil)
	env.api.On("HasPermissionToChannel", testUserID, testChannelID, model.PermissionCreatePost).Return(true)
	var preview *model.Post
	env.api.On("SendEphemeralPost", testUserID, mock.AnythingOfType("*model.Post")).Return(func(_ string, post *model.Post) *model.Post {
		preview = post
		return post
	})

	run := func(command string) *model.CommandResponse {
		resp, appErr := env.p.ExecuteCommand(nil, &model.CommandArgs{Command: command, UserId: testUserID, ChannelId: testChannelID, RootId: testRootID})
		require.Nil(t, appErr)
		return resp
	}

	assert.Contains(t, run("/gif").Text, "/gif <search terms>")
	assert.Contains(t, run("/gif   nothing").Text, "No GIF found for “nothing”")

	resp := run("/gif happy cat")
	assert.Empty(t, resp.Text)
	require.NotNil(t, preview)
	assert.Equal(t, testRootID, preview.RootId)
	a, ctx := previewOf(t, preview)
	assert.Equal(t, "Happy cat", a.Title)
	assert.Equal(t, "https://chat.example.com"+mediaPrefix+"g1/gif.gif", a.ImageURL)
	assert.Equal(t, "Via Tenor", a.Footer)
	assert.Equal(t, "/plugins/"+pluginID+"/api/v1/actions/send", a.Actions[0].Integration.URL)
	assert.Equal(t, map[string]any{"kind": "gif", "query": "happy cat", "index": "0", "root_id": testRootID, "id": "g1"}, ctx)

	resp = run("/sticker works")
	assert.Empty(t, resp.Text)
	a, ctx = previewOf(t, preview)
	assert.Equal(t, "It works!", a.Title)
	assert.Equal(t, "sticker", ctx["kind"])

	env.p.getConfiguration().EnableStickers = false
	assert.Contains(t, run("/sticker works").Text, "turned off")

	env = newTestEnv(t, func(c *configuration) { c.ServiceURL = "" })
	resp, _ = env.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/gif cat", UserId: testUserID, ChannelId: testChannelID})
	assert.Contains(t, resp.Text, "aren't set up")
}

func TestPreviewActions(t *testing.T) {
	env := newTestEnv(t, nil)
	env.api.On("HasPermissionToChannel", testUserID, testChannelID, model.PermissionCreatePost).Return(true)
	var updated, created *model.Post
	env.api.On("UpdateEphemeralPost", testUserID, mock.AnythingOfType("*model.Post")).Return(func(_ string, post *model.Post) *model.Post {
		updated = post
		return post
	})
	env.api.On("DeleteEphemeralPost", testUserID, "previewpostidxxxxxxxxxxxxx").Return()
	env.api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(func(post *model.Post) (*model.Post, *model.AppError) {
		created = post
		return post, nil
	})

	action := func(name string, ctx map[string]any) *model.PostActionIntegrationResponse {
		rec := env.do(t, http.MethodPost, "/api/v1/actions/"+name, testUserID, model.PostActionIntegrationRequest{
			UserId: "someone-else", PostId: "previewpostidxxxxxxxxxxxxx", ChannelId: testChannelID, Context: ctx,
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decode[*model.PostActionIntegrationResponse](t, rec)
	}
	ctx := map[string]any{"kind": "gif", "query": "cat", "index": "0", "root_id": "", "id": "g1"}

	action("shuffle", ctx)
	require.NotNil(t, updated)
	assert.Equal(t, "previewpostidxxxxxxxxxxxxx", updated.Id)
	_, newCtx := previewOf(t, updated)
	assert.Equal(t, "1", newCtx["index"])
	assert.Equal(t, "g2", newCtx["id"])

	resp := action("send", newCtx)
	assert.Empty(t, resp.EphemeralText)
	require.NotNil(t, created)
	assert.Equal(t, testUserID, created.UserId)
	assert.Equal(t, "g2", created.GetProp(PostPropKey).(map[string]any)["id"])

	action("cancel", ctx)
	env.api.AssertNumberOfCalls(t, "DeleteEphemeralPost", 2)

	resp = action("send", map[string]any{"kind": "gif", "id": "missing"})
	assert.Contains(t, resp.EphemeralText, "isn't available")

	resp = action("send", map[string]any{"kind": "video", "id": "g1"})
	assert.Contains(t, resp.EphemeralText, "kind must be gif or sticker")
	env.p.getConfiguration().EnableStickers = false
	resp = action("shuffle", map[string]any{"kind": "sticker", "query": "works"})
	assert.Contains(t, resp.EphemeralText, "stickers are turned off")
}
