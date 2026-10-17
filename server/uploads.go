// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
)

// maxUploads is how many uploads of a kind the plugin keeps a record of: past it, the oldest
// records go (their GIFs or stickers stay in the service, without an uploader to delete them).
// A variable for the tests.
var maxUploads = 5000

// Upload records a GIF or sticker someone added from the picker: who added it and when, so that
// the picker's manage view can list the uploads and let people delete their own.
type Upload struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Tags   string `json:"tags,omitempty"`
	Pack   string `json:"pack,omitempty"`
	UserID string `json:"user_id"`
	// CreatedAt is in milliseconds since the epoch.
	CreatedAt int64 `json:"created_at"`
}

func uploadsKey(kind string) string        { return "uploads_" + kind }
func uploadItemKey(kind, id string) string { return "upload_" + kind + "_" + id }

// Uploads returns the records of the uploads of a kind, newest first.
func (s *Store) Uploads(kind string) ([]Upload, error) {
	data, appErr := s.api.KVGet(uploadsKey(kind))
	if appErr != nil {
		return nil, appErr
	}
	uploads := []Upload{}
	if data != nil {
		if err := json.Unmarshal(data, &uploads); err != nil {
			return nil, err
		}
	}
	return uploads, nil
}

// UploadItem returns the snapshot of an uploaded item, nil when there's none.
func (s *Store) UploadItem(kind, id string) (*Item, error) {
	data, appErr := s.api.KVGet(uploadItemKey(kind, id))
	if appErr != nil {
		return nil, appErr
	}
	if data == nil {
		return nil, nil
	}
	var it Item
	if err := json.Unmarshal(data, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

// updateUploads changes the records of a kind atomically, retrying on concurrent changes.
func (s *Store) updateUploads(kind string, change func([]Upload) []Upload) error {
	key := uploadsKey(kind)
	for range 5 {
		old, appErr := s.api.KVGet(key)
		if appErr != nil {
			return appErr
		}
		uploads := []Upload{}
		if old != nil {
			if err := json.Unmarshal(old, &uploads); err != nil {
				return err
			}
		}
		data, err := json.Marshal(change(uploads))
		if err != nil {
			return err
		}
		ok, appErr := s.api.KVSetWithOptions(key, data, model.PluginKVSetOptions{Atomic: true, OldValue: old})
		if appErr != nil {
			return appErr
		}
		if ok {
			return nil
		}
	}
	return errTooManyUpdates
}

// AddUpload records an upload and keeps a snapshot of its item. The same file uploaded again (the
// service gives it the same ID) keeps its first uploader and date, with the new title and tags.
func (s *Store) AddUpload(up Upload, it Item) error {
	data, err := json.Marshal(it)
	if err != nil {
		return err
	}
	if _, appErr := s.api.KVSetWithOptions(uploadItemKey(up.Kind, up.ID), data, model.PluginKVSetOptions{}); appErr != nil {
		return appErr
	}
	var dropped []Upload
	err = s.updateUploads(up.Kind, func(uploads []Upload) []Upload {
		out := []Upload{up}
		for _, other := range uploads {
			if other.ID == up.ID {
				out[0].UserID, out[0].CreatedAt = other.UserID, other.CreatedAt
				continue
			}
			out = append(out, other)
		}
		dropped = nil
		if len(out) > maxUploads {
			dropped = append(dropped, out[maxUploads:]...)
			out = out[:maxUploads]
		}
		return out
	})
	if err != nil {
		return err
	}
	for _, d := range dropped {
		_, _ = s.api.KVSetWithOptions(uploadItemKey(d.Kind, d.ID), nil, model.PluginKVSetOptions{})
	}
	return nil
}

// RemoveUpload forgets an upload and its snapshot.
func (s *Store) RemoveUpload(kind, id string) error {
	err := s.updateUploads(kind, func(uploads []Upload) []Upload {
		out := []Upload{}
		for _, up := range uploads {
			if up.ID != id {
				out = append(out, up)
			}
		}
		return out
	})
	if err != nil {
		return err
	}
	if _, appErr := s.api.KVSetWithOptions(uploadItemKey(kind, id), nil, model.PluginKVSetOptions{}); appErr != nil {
		return appErr
	}
	return nil
}

// isAdmin tells whether a user is a system admin.
func (p *Plugin) isAdmin(userID string) bool {
	return p.API.HasPermissionTo(userID, model.PermissionManageSystem)
}

// canDeleteUpload tells whether a user may delete an upload: their own while they may upload (so
// never a guest), anyone's for system admins. Nobody can while the service takes no uploads.
func (p *Plugin) canDeleteUpload(userID string, up Upload) bool {
	if !p.getService().canUpload() {
		return false
	}
	if up.UserID == userID && p.canUpload(userID) {
		return true
	}
	return p.isAdmin(userID)
}

// UploadEntry is an upload in the list of GET /api/v1/uploads.
type UploadEntry struct {
	Upload
	Item *Item `json:"item"`
	// Username is the uploader's, empty when the account is gone.
	Username  string `json:"username,omitempty"`
	CanDelete bool   `json:"can_delete"`
}

// UploadList is a page of uploads.
type UploadList struct {
	Uploads []UploadEntry `json:"uploads"`
	HasMore bool          `json:"has_more"`
}

func (up *Upload) matches(query string) bool {
	if query == "" {
		return true
	}
	for _, field := range []string{up.Title, up.Tags, up.Pack} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func intParam(r *http.Request, name string, def, maxValue int) int {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || n < 0 {
		return def
	}
	if maxValue > 0 {
		return min(n, maxValue)
	}
	return n
}

// handleListUploads lists the uploads of a kind, newest first, searched by title, tags or pack,
// a page at a time (page from 0, per_page up to 60), with who added them and whether the user may
// delete each.
func (p *Plugin) handleListUploads(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	kind, err := p.kindParam(r.URL.Query().Get("kind"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	if !p.getService().configured() {
		p.writeError(w, errNotConfigured)
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if len(query) > 100 {
		p.writeError(w, newAPIError(http.StatusBadRequest, "the search can have up to 100 characters"))
		return
	}
	page := intParam(r, "page", 0, 0)
	perPage := intParam(r, "per_page", 30, 60)
	if perPage == 0 {
		perPage = 30
	}

	all, err := p.store.Uploads(kind)
	if err != nil {
		p.writeError(w, err)
		return
	}
	var found []Upload
	for _, up := range all {
		if up.matches(query) {
			found = append(found, up)
		}
	}
	list := UploadList{Uploads: []UploadEntry{}}
	start := page * perPage
	if start < len(found) {
		end := min(start+perPage, len(found))
		list.HasMore = end < len(found)
		names := map[string]string{}
		admin, adminKnown := false, false
		for _, up := range found[start:end] {
			entry := UploadEntry{Upload: up}
			if entry.Item, err = p.store.UploadItem(kind, up.ID); err != nil {
				p.writeError(w, err)
				return
			}
			name, ok := names[up.UserID]
			if !ok {
				if u, appErr := p.API.GetUser(up.UserID); appErr == nil && u != nil {
					name = u.Username
				}
				names[up.UserID] = name
			}
			entry.Username = name
			if up.UserID == userID {
				entry.CanDelete = p.canDeleteUpload(userID, up)
			} else if p.getService().canUpload() {
				if !adminKnown {
					admin, adminKnown = p.isAdmin(userID), true
				}
				entry.CanDelete = admin
			}
			list.Uploads = append(list.Uploads, entry)
		}
	}
	p.writeJSON(w, list)
}

// handleDeleteUpload deletes an upload from the GIF service, for its uploader (while they may
// upload) or a system admin, then forgets it and removes it from the user's favourites.
func (p *Plugin) handleDeleteUpload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	vars := mux.Vars(r)
	kind, err := p.kindParam(vars["kind"])
	if err != nil {
		p.writeError(w, err)
		return
	}
	if !p.getService().canUpload() {
		p.writeError(w, newAPIError(http.StatusForbidden, "the GIF service doesn't take uploads"))
		return
	}
	id := vars["id"]
	uploads, err := p.store.Uploads(kind)
	if err != nil {
		p.writeError(w, err)
		return
	}
	var found *Upload
	for i := range uploads {
		if uploads[i].ID == id {
			found = &uploads[i]
			break
		}
	}
	if found == nil {
		p.writeError(w, newAPIError(http.StatusNotFound, "no such upload"))
		return
	}
	if !p.canDeleteUpload(userID, *found) {
		p.writeError(w, newAPIError(http.StatusForbidden, "you can't delete this upload"))
		return
	}
	if err := p.getService().DeleteUpload(r.Context(), id); err != nil {
		p.writeError(w, err)
		return
	}
	if err := p.store.RemoveUpload(kind, id); err != nil {
		p.writeError(w, err)
		return
	}
	if _, err := p.store.RemoveFavorite(userID, kind, id); err != nil {
		p.API.LogWarn("Failed to remove a deleted upload from the favourites", "err", err.Error())
	}
	p.API.LogInfo("GIF deleted", "user_id", userID, "kind", kind, "id", id, "uploader_id", found.UserID)
	w.WriteHeader(http.StatusNoContent)
}
