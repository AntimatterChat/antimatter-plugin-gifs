// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"container/list"
	"sync"
	"time"
)

// cacheEntry is a cached response.
type cacheEntry struct {
	key         string
	body        []byte
	contentType string
	expires     time.Time
}

// cache is an LRU cache of responses, bounded by the size of their bodies.
type cache struct {
	mu       sync.Mutex
	maxBytes int64
	size     int64
	order    *list.List // front is the most recently used
	entries  map[string]*list.Element
	now      func() time.Time
}

func newCache(maxBytes int64) *cache {
	return &cache{maxBytes: maxBytes, order: list.New(), entries: map[string]*list.Element{}, now: time.Now}
}

// get returns a fresh entry.
func (c *cache) get(key string) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*cacheEntry)
	if c.now().After(e.expires) {
		c.remove(el)
		return nil, false
	}
	c.order.MoveToFront(el)
	return e, true
}

// set stores a response for ttl; bodies over an eighth of the cache aren't kept.
func (c *cache) set(key string, body []byte, contentType string, ttl time.Duration) {
	size := int64(len(body))
	if size > c.maxBytes/8 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.remove(el)
	}
	c.entries[key] = c.order.PushFront(&cacheEntry{key: key, body: body, contentType: contentType, expires: c.now().Add(ttl)})
	c.size += size
	for c.size > c.maxBytes {
		c.remove(c.order.Back())
	}
}

func (c *cache) remove(el *list.Element) {
	e := el.Value.(*cacheEntry)
	c.order.Remove(el)
	delete(c.entries, e.key)
	c.size -= int64(len(e.body))
}
