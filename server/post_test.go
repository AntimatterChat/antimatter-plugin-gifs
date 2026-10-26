// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The query of a post comes from the client, and went with the share for the service's trending
// terms: posting any GIF with a slur as its query showed the slur to everyone in the picker. It
// now goes only when the user searched it and found the item, and up to 100 characters.
func TestPostQueryBoundedAndRecordedOnlyIfSearched(t *testing.T) {
	env := newTestEnv(t, nil)
	env.api.On("HasPermissionToChannel", mock.Anything, testChannelID, model.PermissionCreatePost).Return(true)
	env.api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(func(post *model.Post) (*model.Post, *model.AppError) {
		created := post.Clone()
		created.Id = model.NewId()
		return created, nil
	})
	search := func(userID, query string) {
		require.Equal(t, http.StatusOK, env.do(t, http.MethodGet, "/api/v1/search?"+query, userID, nil).Code)
	}
	post := func(kind, id, query string) {
		rec := env.do(t, http.MethodPost, "/api/v1/post", testUserID, postRequest{itemRequest: itemRequest{Kind: kind, ID: id}, ChannelID: testChannelID, Query: query})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}

	post(kindGIF, "g1", "badword")  // never searched
	search(otherUserID, "q=cat")    // someone else's search
	post(kindGIF, "g1", "cat")      // by someone who didn't
	search(testUserID, "q=nothing") // found nothing
	post(kindGIF, "g1", "nothing")
	search(testUserID, "q=cat")                     // found g1 and g2, GIFs
	post(kindSticker, "s1", "cat")                  // not among them
	post(kindGIF, "g1", strings.Repeat("cat ", 30)) // too long
	post(kindGIF, "g2", " Cat ")                    // searched and found
	search(testUserID, "kind=sticker&q=works")      // stickers
	post(kindSticker, "s1", "works")

	want := []string{"g1:", "g1:", "g1:", "s1:", "g1:", "g2:Cat", "s1:works"}
	assert.Eventually(t, func() bool {
		env.service.mu.Lock()
		defer env.service.mu.Unlock()
		return len(env.service.shares) == len(want)
	}, 2*time.Second, 10*time.Millisecond)
	env.service.mu.Lock()
	defer env.service.mu.Unlock()
	assert.ElementsMatch(t, want, env.service.shares)
}

func TestShareQuery(t *testing.T) {
	s := &gifService{searched: newCache(searchMemoryBytes)}
	s.rememberSearch(testUserID, kindGIF, "happy  cat", []Item{{ID: "g1"}, {ID: "g2"}})
	s.rememberSearch(testUserID, kindGIF, "Happy Cat", []Item{{ID: "g3"}}) // its next page
	for _, id := range []string{"g1", "g2", "g3"} {
		assert.Equal(t, "HAPPY cat", s.shareQuery(testUserID, kindGIF, "HAPPY cat", id), id)
	}
	assert.Empty(t, s.shareQuery(testUserID, kindGIF, "happy cat", "g4"))
	assert.Empty(t, s.shareQuery(testUserID, kindSticker, "happy cat", "g1"))
	assert.Empty(t, s.shareQuery(otherUserID, kindGIF, "happy cat", "g1"))
	assert.Empty(t, s.shareQuery(testUserID, kindGIF, "", "g1"))

	// The items of a search are bounded.
	many := make([]Item, maxRememberedItems+10)
	for i := range many {
		many[i] = Item{ID: "m" + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + string(rune('a'+i/26))}
	}
	s.rememberSearch(testUserID, kindGIF, "many", many)
	e, ok := s.searched.get(searchMemoryKey(testUserID, kindGIF, "many"))
	require.True(t, ok)
	assert.LessOrEqual(t, len(strings.Split(string(e.body), "\n")), maxRememberedItems)
	var nilService *gifService
	assert.Empty(t, nilService.shareQuery(testUserID, kindGIF, "many", "g1"))
	nilService.rememberSearch(testUserID, kindGIF, "many", many)
}
