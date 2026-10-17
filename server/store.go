// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/mattermost/mattermost/server/public/model"
)

// errTooManyUpdates is returned when a list kept changing under an update.
var errTooManyUpdates = errors.New("too many concurrent updates")

// Limits of the per-user lists.
const (
	maxFavorites = 200
	maxRecent    = 30
)

// kvAPI is the part of the plugin API the store uses.
type kvAPI interface {
	KVGet(key string) ([]byte, *model.AppError)
	KVSetWithOptions(key string, value []byte, options model.PluginKVSetOptions) (bool, *model.AppError)
}

// Store keeps the favourite and recently used GIFs and stickers of each user in the plugin's KV
// store, as snapshots of the items, newest first, so they show without asking the GIF service.
type Store struct {
	api kvAPI
}

// NewStore returns a store over the plugin's KV store.
func NewStore(api kvAPI) *Store {
	return &Store{api: api}
}

func favoritesKey(userID string) string { return "fav_" + userID }
func recentKey(userID string) string    { return "recent_" + userID }

func (s *Store) list(key string) ([]Item, []byte, error) {
	data, appErr := s.api.KVGet(key)
	if appErr != nil {
		return nil, nil, appErr
	}
	items := []Item{}
	if data != nil {
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, nil, err
		}
	}
	return items, data, nil
}

// update changes a list atomically, retrying when another request changed it meanwhile.
func (s *Store) update(key string, change func([]Item) []Item) ([]Item, error) {
	for range 5 {
		items, old, err := s.list(key)
		if err != nil {
			return nil, err
		}
		items = change(items)
		data, err := json.Marshal(items)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(data, old) {
			return items, nil
		}
		ok, appErr := s.api.KVSetWithOptions(key, data, model.PluginKVSetOptions{Atomic: true, OldValue: old})
		if appErr != nil {
			return nil, appErr
		}
		if ok {
			return items, nil
		}
	}
	return nil, errTooManyUpdates
}

// prepend puts an item first, removing its other copies, and truncates the list.
func prepend(items []Item, it Item, limit int) []Item {
	out := []Item{it}
	for _, other := range items {
		if other.ID != it.ID || other.Kind != it.Kind {
			out = append(out, other)
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Favorites returns the favourites of a user.
func (s *Store) Favorites(userID string) ([]Item, error) {
	items, _, err := s.list(favoritesKey(userID))
	return items, err
}

// AddFavorite adds (or moves first) a favourite.
func (s *Store) AddFavorite(userID string, it Item) ([]Item, error) {
	return s.update(favoritesKey(userID), func(items []Item) []Item {
		return prepend(items, it, maxFavorites)
	})
}

// RemoveFavorite removes a favourite.
func (s *Store) RemoveFavorite(userID, kind, id string) ([]Item, error) {
	return s.update(favoritesKey(userID), func(items []Item) []Item {
		out := []Item{}
		for _, it := range items {
			if it.ID != id || it.Kind != kind {
				out = append(out, it)
			}
		}
		return out
	})
}

// Recent returns the recently used items of a user.
func (s *Store) Recent(userID string) ([]Item, error) {
	items, _, err := s.list(recentKey(userID))
	return items, err
}

// AddRecent records the use of an item.
func (s *Store) AddRecent(userID string, it Item) error {
	_, err := s.update(recentKey(userID), func(items []Item) []Item {
		return prepend(items, it, maxRecent)
	})
	return err
}
