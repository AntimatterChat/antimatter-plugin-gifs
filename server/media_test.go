// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Requests for many large media files missing from the cache each held one, up to 16 MB, at the
// same time. The proxy now downloads a few at a time, and each file once for the requests that
// want it together.
func TestHandleMediaLimitsConcurrentFetches(t *testing.T) {
	var inFlight, most atomic.Int32
	var mu sync.Mutex
	fetched := map[string]int{}
	release := make(chan struct{})
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		mu.Lock()
		fetched[r.URL.Path]++
		mu.Unlock()
		<-release
		w.Header().Set("Content-Type", "image/gif")
		fmt.Fprint(w, "GIF:"+r.URL.Path)
	}))
	defer media.Close()
	env := newTestEnv(t, func(c *configuration) { c.MediaURL = media.URL + "/media/" })

	paths := []string{}
	for i := range 3 * maxConcurrentMediaFetches {
		paths = append(paths, fmt.Sprintf("/media/many/%d.gif", i))
	}
	for range 10 {
		paths = append(paths, "/media/same.gif")
	}
	codes := make([]int, len(paths))
	bodies := make([]string, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, p, nil)
			req.Header.Set("Mattermost-User-Id", testUserID)
			rec := httptest.NewRecorder()
			env.p.ServeHTTP(nil, rec, req)
			codes[i], bodies[i] = rec.Code, rec.Body.String()
		}()
	}

	require.Eventually(t, func() bool { return inFlight.Load() == maxConcurrentMediaFetches }, 5*time.Second, 5*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, maxConcurrentMediaFetches, most.Load(), "at most this many downloads at once")
	close(release)
	wg.Wait()

	for i, p := range paths {
		assert.Equal(t, http.StatusOK, codes[i], p)
		assert.Equal(t, "GIF:"+p, bodies[i], p)
	}
	assert.LessOrEqual(t, most.Load(), int32(maxConcurrentMediaFetches))
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, fetched["/media/same.gif"], "the same file is downloaded once")
	for path, n := range fetched {
		assert.Equal(t, 1, n, path)
		assert.True(t, strings.HasPrefix(path, "/media/"), path)
	}
}

// A request that gives up doesn't stop the download others wait for.
func TestMediaFetcherOutlivesItsFirstRequest(t *testing.T) {
	f := newMediaFetcher(1)
	release := make(chan struct{})
	var fetches atomic.Int32
	fetch := func(context.Context) (*cacheEntry, int) {
		fetches.Add(1)
		<-release
		return &cacheEntry{body: []byte("x"), contentType: "image/gif"}, http.StatusOK
	}
	gone, cancel := context.WithCancel(context.Background())
	done := make(chan int)
	go func() {
		_, status := f.do(gone, "k", fetch)
		done <- status
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	assert.Equal(t, http.StatusServiceUnavailable, <-done)

	result := make(chan *cacheEntry)
	go func() {
		e, _ := f.do(context.Background(), "k", fetch)
		result <- e
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)
	e := <-result
	require.NotNil(t, e)
	assert.Equal(t, "x", string(e.body))
	assert.EqualValues(t, 1, fetches.Load())
}
