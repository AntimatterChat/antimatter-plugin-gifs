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
// /plugins/com.antimatterchat.gifs/api/v1.
func (p *Plugin) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.Use(p.requireUser)

	api := router.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/config", p.handleGetConfig).Methods(http.MethodGet)
	api.HandleFunc("/search", p.handleSearch).Methods(http.MethodGet)
	api.HandleFunc("/featured", p.handleFeatured).Methods(http.MethodGet)
	api.HandleFunc("/categories", p.handleCategories).Methods(http.MethodGet)
	api.HandleFunc("/autocomplete", p.handleTerms("autocomplete")).Methods(http.MethodGet)
	api.HandleFunc("/suggestions", p.handleTerms("search_suggestions")).Methods(http.MethodGet)
	api.HandleFunc("/trending_terms", p.handleTerms("trending_terms")).Methods(http.MethodGet)

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
