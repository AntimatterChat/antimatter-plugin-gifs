// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

const requestBodyMaxSizeBytes = 64 * 1024

// ClientConfig is the part of the plugin configuration clients need.
type ClientConfig struct {
	Configured bool `json:"configured"`
	Stickers   bool `json:"stickers"`
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

func (p *Plugin) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	c := p.getConfiguration()
	p.writeJSON(w, ClientConfig{Configured: c.serviceBase() != "", Stickers: c.EnableStickers})
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
