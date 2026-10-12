// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
)

const (
	gifTrigger     = "gif"
	stickerTrigger = "sticker"
	// previewResults is how many results Shuffle goes through.
	previewResults = 20
)

func gifCommands() []*model.Command {
	return []*model.Command{
		{
			Trigger:          gifTrigger,
			DisplayName:      "GIF",
			Description:      "Post a GIF",
			AutoComplete:     true,
			AutoCompleteDesc: "Search a GIF to post",
			AutoCompleteHint: "[search terms]",
		},
		{
			Trigger:          stickerTrigger,
			DisplayName:      "Sticker",
			Description:      "Post a sticker",
			AutoComplete:     true,
			AutoCompleteDesc: "Search a sticker to post",
			AutoCompleteHint: "[search terms]",
		},
	}
}

func (p *Plugin) registerCommands() error {
	for _, cmd := range gifCommands() {
		if err := p.API.RegisterCommand(cmd); err != nil {
			return errors.Wrapf(err, "failed to register the /%s command", cmd.Trigger)
		}
	}
	return nil
}

func ephemeralResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}
}

// ExecuteCommand runs /gif and /sticker: they show a preview of the first result, which the
// user can send, shuffle or cancel. (The webapp opens its picker instead.)
func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	trigger, query, _ := strings.Cut(strings.TrimSpace(args.Command), " ")
	query = strings.TrimSpace(query)
	kind := kindGIF
	noun := "GIF"
	switch strings.TrimPrefix(trigger, "/") {
	case gifTrigger:
	case stickerTrigger:
		kind, noun = kindSticker, "sticker"
		if !p.getConfiguration().EnableStickers {
			return ephemeralResponse("Stickers are turned off."), nil
		}
	default:
		return ephemeralResponse("Unknown command."), nil
	}
	if query == "" {
		return ephemeralResponse(fmt.Sprintf("Search a %s to post with `%s <search terms>`, e.g. `%s thank you`.", noun, trigger, trigger)), nil
	}
	if !p.getService().configured() {
		return ephemeralResponse("GIFs aren't set up yet: ask a system admin to configure the GIFs plugin."), nil
	}
	if err := p.checkCanPost(args.UserId, args.ChannelId, args.RootId); err != nil {
		return ephemeralResponse("You can't post in this channel."), nil
	}

	page, err := p.getService().Search(context.Background(), kind, query, "", "", previewResults)
	if err != nil {
		p.API.LogWarn("GIF search failed", "err", err.Error())
		return ephemeralResponse(fmt.Sprintf("The %s search failed, try again later.", noun)), nil
	}
	if len(page.Items) == 0 {
		return ephemeralResponse(fmt.Sprintf("No %s found for “%s”.", noun, query)), nil
	}
	preview := previewContext{Kind: kind, Query: query, RootID: args.RootId}
	p.API.SendEphemeralPost(args.UserId, p.previewPost(model.NewId(), args.UserId, args.ChannelId, preview, &page.Items[0]))
	return &model.CommandResponse{}, nil
}

// previewContext is carried by the buttons of a preview.
type previewContext struct {
	Kind   string `json:"kind"`
	Query  string `json:"query"`
	Index  int    `json:"index"`
	RootID string `json:"root_id"`
	ID     string `json:"id"`
}

func (c previewContext) toMap() map[string]any {
	return map[string]any{"kind": c.Kind, "query": c.Query, "index": strconv.Itoa(c.Index), "root_id": c.RootID, "id": c.ID}
}

func previewContextFrom(m map[string]any) previewContext {
	str := func(k string) string {
		s, _ := m[k].(string)
		return s
	}
	index, _ := strconv.Atoi(str("index"))
	return previewContext{Kind: str("kind"), Query: str("query"), Index: index, RootID: str("root_id"), ID: str("id")}
}

// previewPost is the ephemeral post showing a result with Send, Shuffle and Cancel buttons.
func (p *Plugin) previewPost(postID, userID, channelID string, c previewContext, it *Item) *model.Post {
	c.ID = it.ID
	action := func(id, name, style string) *model.PostAction {
		return &model.PostAction{
			Id:    id,
			Name:  name,
			Type:  model.PostActionTypeButton,
			Style: style,
			Integration: &model.PostActionIntegration{
				URL:     "/plugins/" + pluginID + "/api/v1/actions/" + id,
				Context: c.toMap(),
			},
		}
	}
	title := it.Title
	if title == "" {
		title = c.Query
	}
	post := &model.Post{
		Id:        postID,
		UserId:    userID,
		ChannelId: channelID,
		RootId:    c.RootID,
		CreateAt:  model.GetMillis(),
	}
	model.ParseMessageAttachment(post, []*model.MessageAttachment{{
		Fallback: title,
		Title:    title,
		ImageURL: p.absoluteURL(it.imageURL()),
		Footer:   it.Attribution,
		Actions: []*model.PostAction{
			action("send", "Send", "primary"),
			action("shuffle", "Shuffle", "default"),
			action("cancel", "Cancel", "default"),
		},
	}})
	return post
}

// handleAction handles the buttons of a /gif or /sticker preview.
func (p *Plugin) handleAction(w http.ResponseWriter, r *http.Request) {
	var req model.PostActionIntegrationRequest
	if err := p.readJSON(r, &req); err != nil {
		p.writeError(w, err)
		return
	}
	// The server sets the header to the user who clicked.
	userID := r.Header.Get("Mattermost-User-Id")
	c := previewContextFrom(req.Context)
	respond := func(text string) {
		resp := &model.PostActionIntegrationResponse{}
		if text != "" {
			resp.EphemeralText = text
		}
		p.writeJSON(w, resp)
	}

	action := mux.Vars(r)["action"]
	if action == "cancel" {
		p.API.DeleteEphemeralPost(userID, req.PostId)
		respond("")
		return
	}
	// The endpoint can be called directly, with any context: check the kind like the API does
	// (stickers may have been turned off since the preview, too).
	kind, err := p.kindParam(c.Kind)
	if err != nil {
		respond(err.Error())
		return
	}
	c.Kind = kind
	switch action {
	case "shuffle":
		page, err := p.getService().Search(r.Context(), c.Kind, c.Query, "", "", previewResults)
		if err != nil || len(page.Items) == 0 {
			respond("The search failed, try again later.")
			return
		}
		c.Index = (c.Index + 1) % len(page.Items)
		p.API.UpdateEphemeralPost(userID, p.previewPost(req.PostId, userID, req.ChannelId, c, &page.Items[c.Index]))
		respond("")
	case "send":
		if err := p.checkCanPost(userID, req.ChannelId, c.RootID); err != nil {
			respond("You can't post in this channel.")
			return
		}
		it, err := p.getService().Get(r.Context(), c.Kind, c.ID)
		if err != nil {
			respond("This GIF isn't available anymore.")
			return
		}
		if _, err := p.postItem(userID, req.ChannelId, c.RootID, it, c.Query); err != nil {
			p.API.LogWarn("Failed to post a GIF", "err", err.Error())
			respond("The GIF couldn't be posted.")
			return
		}
		p.API.DeleteEphemeralPost(userID, req.PostId)
		respond("")
	}
}
