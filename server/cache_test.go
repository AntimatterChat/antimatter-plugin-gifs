// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	now := time.Unix(1000, 0)
	c := newCache(80)
	c.now = func() time.Time { return now }

	c.set("a", make([]byte, 10), "image/gif", time.Minute)
	c.set("b", make([]byte, 10), "", time.Minute)
	e, ok := c.get("a")
	require.True(t, ok)
	require.Equal(t, "image/gif", e.contentType)

	// Too large for the cache.
	c.set("big", make([]byte, 11), "", time.Minute)
	_, ok = c.get("big")
	require.False(t, ok)

	// Evicts the least recently used entries.
	for _, k := range []string{"c", "d", "e", "f", "g", "h", "i"} {
		c.set(k, make([]byte, 10), "", time.Minute)
	}
	_, ok = c.get("b")
	require.False(t, ok, "b should be evicted")
	_, ok = c.get("a")
	require.True(t, ok, "a was used recently")
	require.LessOrEqual(t, c.size, int64(80))

	// Expires.
	now = now.Add(2 * time.Minute)
	_, ok = c.get("a")
	require.False(t, ok)

	// Replacing keeps the size right.
	c.set("x", make([]byte, 5), "", time.Minute)
	c.set("x", make([]byte, 7), "", time.Minute)
	e, ok = c.get("x")
	require.True(t, ok)
	require.Len(t, e.body, 7)
}
