// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	kv := newFakeKV()
	s := NewStore(kv)

	favs, err := s.Favorites("u1")
	require.NoError(t, err)
	assert.Empty(t, favs)

	_, err = s.AddFavorite("u1", Item{ID: "a", Kind: kindGIF})
	require.NoError(t, err)
	_, err = s.AddFavorite("u1", Item{ID: "a", Kind: kindSticker})
	require.NoError(t, err)
	favs, err = s.AddFavorite("u1", Item{ID: "a", Kind: kindGIF, Title: "again"})
	require.NoError(t, err)
	require.Len(t, favs, 2)
	assert.Equal(t, "again", favs[0].Title)
	assert.Equal(t, kindSticker, favs[1].Kind)

	favs, err = s.RemoveFavorite("u1", kindSticker, "a")
	require.NoError(t, err)
	assert.Len(t, favs, 1)
	favs, err = s.RemoveFavorite("u1", kindSticker, "a")
	require.NoError(t, err)
	assert.Len(t, favs, 1)

	// Recent items are capped, newest first.
	for i := range maxRecent + 5 {
		require.NoError(t, s.AddRecent("u1", Item{ID: fmt.Sprint(i), Kind: kindGIF}))
	}
	recent, err := s.Recent("u1")
	require.NoError(t, err)
	require.Len(t, recent, maxRecent)
	assert.Equal(t, fmt.Sprint(maxRecent+4), recent[0].ID)

	// Concurrent updates are retried, then given up.
	kv.conflicts = 2
	require.NoError(t, s.AddRecent("u1", Item{ID: "x", Kind: kindGIF}))
	kv.conflicts = 10
	assert.Error(t, s.AddRecent("u1", Item{ID: "y", Kind: kindGIF}))

	other, err := s.Recent("u2")
	require.NoError(t, err)
	assert.Empty(t, other)
}
