// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	// PostType is the type of the posts of GIFs and stickers, rendered by the webapp.
	PostType = "custom_antimatter_gif"
	// PostPropKey is the post prop holding the Item.
	PostPropKey = "antimatter_gif"
)

// checkCanPost checks that the user can post in the channel (and the thread).
func (p *Plugin) checkCanPost(userID, channelID, rootID string) error {
	if !model.IsValidId(channelID) || (rootID != "" && !model.IsValidId(rootID)) {
		return newAPIError(http.StatusBadRequest, "invalid channel or thread")
	}
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return newAPIError(http.StatusForbidden, "you can't post in this channel")
	}
	return nil
}

// absoluteURL makes a URL of the Antimatter server absolute, for clients that need it.
func (p *Plugin) absoluteURL(u string) string {
	if !strings.HasPrefix(u, "/") {
		return u
	}
	if cfg := p.API.GetConfig(); cfg != nil && cfg.ServiceSettings.SiteURL != nil {
		return strings.TrimRight(*cfg.ServiceSettings.SiteURL, "/") + u
	}
	return u
}

// itemProps converts an item to plain JSON values, which the plugin RPC can carry in post props.
func itemProps(it *Item) (map[string]any, error) {
	data, err := json.Marshal(it)
	if err != nil {
		return nil, err
	}
	var props map[string]any
	err = json.Unmarshal(data, &props)
	return props, err
}

// postItem posts a GIF or sticker as the user. The message is a Markdown image, for clients
// without the plugin; the webapp renders the item of the post's props.
func (p *Plugin) postItem(userID, channelID, rootID string, it *Item, query string) (*model.Post, error) {
	props, err := itemProps(it)
	if err != nil {
		return nil, err
	}
	alt := it.Title
	if alt == "" {
		alt = map[string]string{kindGIF: "GIF", kindSticker: "Sticker"}[it.Kind]
	}
	alt = strings.NewReplacer("[", "", "]", "", "\n", " ").Replace(alt)
	post := &model.Post{
		UserId:    userID,
		ChannelId: channelID,
		RootId:    rootID,
		Type:      PostType,
		Message:   "![" + alt + "](" + p.absoluteURL(it.imageURL()) + ")",
	}
	post.AddProp(PostPropKey, props)
	created, appErr := p.API.CreatePost(post)
	if appErr != nil {
		if appErr.StatusCode >= 400 && appErr.StatusCode < 500 {
			return nil, newAPIError(appErr.StatusCode, appErr.Message)
		}
		return nil, appErr
	}

	if err := p.store.AddRecent(userID, *it); err != nil {
		p.API.LogWarn("Failed to save a recently used GIF", "err", err.Error())
	}
	if s := p.getService(); s.configured() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := s.RegisterShare(ctx, it.ID, query); err != nil {
				p.API.LogDebug("Failed to register a share with the GIF service", "err", err.Error())
			}
		}()
	}
	return created, nil
}
