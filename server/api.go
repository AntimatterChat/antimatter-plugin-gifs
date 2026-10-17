// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
)

const requestBodyMaxSizeBytes = 64 * 1024

// uploadMaxSizeBytes is the largest GIF or sticker users can add (the service's default limit).
const uploadMaxSizeBytes = 16 << 20

// ClientConfig is the part of the plugin configuration clients need.
type ClientConfig struct {
	Configured bool `json:"configured"`
	Stickers   bool `json:"stickers"`
	// CanUpload tells whether the user can add GIFs and stickers (and delete their own).
	CanUpload bool `json:"can_upload"`
	// CanDeleteOthers tells whether the user can delete anyone's uploads (system admins, while
	// the service takes uploads).
	CanDeleteOthers bool `json:"can_delete_others"`
	// UploadMaxBytes is the largest file an upload takes, when uploads are possible.
	UploadMaxBytes int64 `json:"upload_max_bytes,omitempty"`
}

// newRouter returns the router of the plugin's REST API, served under
// /plugins/com.antimatterchat.gifs/api/v1, and of its media proxy under /media/.
func (p *Plugin) newRouter() *mux.Router {
	router := mux.NewRouter()
	// The media proxy rejects unclean paths itself rather than redirecting them.
	router.SkipClean(true)
	router.Use(p.requireUser)

	router.PathPrefix("/media/").HandlerFunc(p.handleMedia).Methods(http.MethodGet, http.MethodHead)

	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/config", p.handleGetConfig).Methods(http.MethodGet)
	api.HandleFunc("/search", p.handleSearch).Methods(http.MethodGet)
	api.HandleFunc("/featured", p.handleFeatured).Methods(http.MethodGet)
	api.HandleFunc("/categories", p.handleCategories).Methods(http.MethodGet)
	api.HandleFunc("/autocomplete", p.handleTerms("autocomplete")).Methods(http.MethodGet)
	api.HandleFunc("/suggestions", p.handleTerms("search_suggestions")).Methods(http.MethodGet)
	api.HandleFunc("/trending_terms", p.handleTerms("trending_terms")).Methods(http.MethodGet)
	api.HandleFunc("/favorites", p.handleGetFavorites).Methods(http.MethodGet)
	api.HandleFunc("/favorites", p.handleAddFavorite).Methods(http.MethodPost)
	api.HandleFunc("/favorites/{kind:gif|sticker}/{id}", p.handleRemoveFavorite).Methods(http.MethodDelete)
	api.HandleFunc("/recent", p.handleGetRecent).Methods(http.MethodGet)
	api.HandleFunc("/post", p.handlePost).Methods(http.MethodPost)
	api.HandleFunc("/upload", p.handleUpload).Methods(http.MethodPost)
	api.HandleFunc("/uploads", p.handleListUploads).Methods(http.MethodGet)
	api.HandleFunc("/uploads/{kind:gif|sticker}/{id}", p.handleDeleteUpload).Methods(http.MethodDelete)
	api.HandleFunc("/actions/{action:send|shuffle|cancel}", p.handleAction).Methods(http.MethodPost)

	return router
}

func (p *Plugin) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Mattermost-User-Id") == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		p.API.LogWarn("Failed to write response", "err", err.Error())
	}
}

func (p *Plugin) writeError(w http.ResponseWriter, err error) {
	status := statusFor(err)
	if status >= http.StatusInternalServerError {
		p.API.LogError("GIFs request failed", "err", err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// kindParam reads the kind of items asked for; stickers must be enabled.
func (p *Plugin) kindParam(kind string) (string, error) {
	switch kind {
	case "", kindGIF:
		return kindGIF, nil
	case kindSticker:
		if !p.getConfiguration().EnableStickers {
			return "", newAPIError(http.StatusForbidden, "stickers are turned off")
		}
		return kindSticker, nil
	}
	return "", newAPIError(http.StatusBadRequest, "kind must be gif or sticker")
}

func limitParam(r *http.Request) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n < 1 {
		return 24
	}
	return min(n, 50)
}

// localeParam turns a locale such as en or pt-BR into Tenor's xx_YY form.
func localeParam(r *http.Request) string {
	locale := r.URL.Query().Get("locale")
	if len(locale) > 10 {
		return ""
	}
	return strings.ReplaceAll(locale, "-", "_")
}

func (p *Plugin) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	c := p.getConfiguration()
	userID := r.Header.Get("Mattermost-User-Id")
	cfg := ClientConfig{
		Configured: c.serviceBase() != "",
		Stickers:   c.EnableStickers,
		CanUpload:  p.canUpload(userID),
	}
	if p.getService().canUpload() {
		cfg.CanDeleteOthers = p.isAdmin(userID)
		if cfg.CanUpload || cfg.CanDeleteOthers {
			cfg.UploadMaxBytes = uploadMaxSizeBytes
		}
	}
	p.writeJSON(w, cfg)
}

// canUpload tells whether a user can add GIFs and stickers: the service takes uploads, and the
// user is a system admin unless everyone may add them. "Everyone" leaves guests out, as custom
// emoji do: guests neither add nor delete GIFs and stickers.
func (p *Plugin) canUpload(userID string) bool {
	if !p.getService().canUpload() {
		return false
	}
	if p.isAdmin(userID) {
		return true
	}
	return p.getConfiguration().everyoneCanUpload() && !p.isGuest(userID)
}

// isGuest tells whether a user is a guest account. When the account can't be read, the user
// counts as one: nothing is allowed that wouldn't be for a guest.
func (p *Plugin) isGuest(userID string) bool {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil || user == nil {
		if appErr != nil {
			p.API.LogWarn("Failed to get the user to check their role", "user_id", userID, "err", appErr.Error())
		}
		return true
	}
	return user.IsGuest()
}

// uploadText reads a text field of an upload, checking its length.
func uploadText(r *http.Request, name string, maxLen int) (string, error) {
	v := strings.TrimSpace(r.FormValue(name))
	if utf8.RuneCountInString(v) > maxLen {
		return "", newAPIError(http.StatusBadRequest, name+" is too long")
	}
	return v, nil
}

// handleUpload adds a GIF or sticker to the GIF service (multipart/form-data: file, kind, title,
// tags, and pack for stickers) and to the user's favourites, and returns its item.
func (p *Plugin) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if !p.canUpload(userID) {
		p.writeError(w, newAPIError(http.StatusForbidden, "you can't add GIFs or stickers"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, uploadMaxSizeBytes+1<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			p.writeError(w, newAPIError(http.StatusRequestEntityTooLarge, "the file is larger than 16 MB"))
			return
		}
		p.writeError(w, newAPIError(http.StatusBadRequest, "invalid upload"))
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	kind, err := p.kindParam(r.FormValue("kind"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	up := UploadRequest{Kind: kind}
	for _, f := range []struct {
		into   *string
		name   string
		maxLen int
	}{{&up.Title, "title", 100}, {&up.Tags, "tags", 500}, {&up.Pack, "pack", 60}} {
		if *f.into, err = uploadText(r, f.name, f.maxLen); err != nil {
			p.writeError(w, err)
			return
		}
	}
	if up.Title == "" {
		p.writeError(w, newAPIError(http.StatusBadRequest, "a title is required"))
		return
	}
	if kind != kindSticker {
		up.Pack = ""
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		p.writeError(w, newAPIError(http.StatusBadRequest, "a file is required"))
		return
	}
	defer file.Close()
	if header.Size > uploadMaxSizeBytes {
		p.writeError(w, newAPIError(http.StatusRequestEntityTooLarge, "the file is larger than 16 MB"))
		return
	}
	up.FileName = header.Filename
	up.File = file

	it, err := p.getService().Upload(r.Context(), up)
	if err != nil {
		p.writeError(w, err)
		return
	}
	// The user finds it among their favourites right away, and in the uploads they can manage.
	if _, err := p.store.AddFavorite(userID, *it); err != nil {
		p.API.LogWarn("Failed to add an upload to the favourites", "err", err.Error())
	}
	record := Upload{ID: it.ID, Kind: it.Kind, Title: up.Title, Tags: up.Tags, Pack: up.Pack, UserID: userID, CreatedAt: model.GetMillis()}
	if err := p.store.AddUpload(record, *it); err != nil {
		p.API.LogWarn("Failed to record an upload", "err", err.Error())
	}
	p.API.LogInfo("GIF added", "user_id", userID, "kind", it.Kind, "id", it.ID)
	p.writeJSON(w, it)
}

func (p *Plugin) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, err := p.kindParam(q.Get("kind"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	query := strings.TrimSpace(q.Get("q"))
	if query == "" || len(query) > 100 {
		p.writeError(w, newAPIError(http.StatusBadRequest, "the search must have 1 to 100 characters"))
		return
	}
	page, err := p.getService().Search(r.Context(), kind, query, q.Get("pos"), localeParam(r), limitParam(r))
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, page)
}

func (p *Plugin) handleFeatured(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind, err := p.kindParam(q.Get("kind"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	page, err := p.getService().Featured(r.Context(), kind, q.Get("pos"), localeParam(r), limitParam(r))
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, page)
}

func (p *Plugin) handleCategories(w http.ResponseWriter, r *http.Request) {
	kind, err := p.kindParam(r.URL.Query().Get("kind"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	cats, err := p.getService().Categories(r.Context(), kind, localeParam(r))
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, cats)
}

func (p *Plugin) handleTerms(endpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if endpoint != "trending_terms" && (query == "" || len(query) > 100) {
			p.writeError(w, newAPIError(http.StatusBadRequest, "the search must have 1 to 100 characters"))
			return
		}
		terms, err := p.getService().Terms(r.Context(), endpoint, query, localeParam(r), min(limitParam(r), 20))
		if err != nil {
			p.writeError(w, err)
			return
		}
		p.writeJSON(w, terms)
	}
}

func (p *Plugin) handleGetFavorites(w http.ResponseWriter, r *http.Request) {
	items, err := p.store.Favorites(r.Header.Get("Mattermost-User-Id"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, items)
}

// itemRequest names an item of the GIF service.
type itemRequest struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// readJSON decodes a small JSON request body.
func (p *Plugin) readJSON(r *http.Request, into any) error {
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, requestBodyMaxSizeBytes)).Decode(into); err != nil {
		return newAPIError(http.StatusBadRequest, "invalid request body")
	}
	return nil
}

// fetchItem gets an item from the GIF service, so that clients can't make up media URLs.
func (p *Plugin) fetchItem(r *http.Request, req itemRequest) (*Item, error) {
	kind, err := p.kindParam(req.Kind)
	if err != nil {
		return nil, err
	}
	if req.ID == "" || len(req.ID) > 64 || strings.ContainsAny(req.ID, ",/?#") {
		return nil, newAPIError(http.StatusBadRequest, "invalid id")
	}
	return p.getService().Get(r.Context(), kind, req.ID)
}

func (p *Plugin) handleAddFavorite(w http.ResponseWriter, r *http.Request) {
	var req itemRequest
	if err := p.readJSON(r, &req); err != nil {
		p.writeError(w, err)
		return
	}
	it, err := p.fetchItem(r, req)
	if err != nil {
		p.writeError(w, err)
		return
	}
	items, err := p.store.AddFavorite(r.Header.Get("Mattermost-User-Id"), *it)
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, items)
}

func (p *Plugin) handleRemoveFavorite(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	items, err := p.store.RemoveFavorite(r.Header.Get("Mattermost-User-Id"), vars["kind"], vars["id"])
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, items)
}

func (p *Plugin) handleGetRecent(w http.ResponseWriter, r *http.Request) {
	items, err := p.store.Recent(r.Header.Get("Mattermost-User-Id"))
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, items)
}

// postRequest asks to post an item in a channel or thread.
type postRequest struct {
	itemRequest
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
	// Query is the search that found the item, for the service's trending terms.
	Query string `json:"query"`
}

func (p *Plugin) handlePost(w http.ResponseWriter, r *http.Request) {
	var req postRequest
	if err := p.readJSON(r, &req); err != nil {
		p.writeError(w, err)
		return
	}
	userID := r.Header.Get("Mattermost-User-Id")
	if err := p.checkCanPost(userID, req.ChannelID, req.RootID); err != nil {
		p.writeError(w, err)
		return
	}
	it, err := p.fetchItem(r, req.itemRequest)
	if err != nil {
		p.writeError(w, err)
		return
	}
	post, err := p.postItem(userID, req.ChannelID, req.RootID, it, req.Query)
	if err != nil {
		p.writeError(w, err)
		return
	}
	p.writeJSON(w, post)
}
