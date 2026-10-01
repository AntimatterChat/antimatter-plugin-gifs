// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeKV is an in-memory kvAPI.
type fakeKV struct {
	mu   sync.Mutex
	data map[string][]byte
	// conflicts makes the next atomic sets fail, like concurrent updates would.
	conflicts int
}

func newFakeKV() *fakeKV {
	return &fakeKV{data: map[string][]byte{}}
}

func (f *fakeKV) KVGet(key string) ([]byte, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.data[key]; ok {
		return bytes.Clone(v), nil
	}
	return nil, nil
}

func (f *fakeKV) KVSetWithOptions(key string, value []byte, options model.PluginKVSetOptions) (bool, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if options.Atomic {
		if f.conflicts > 0 {
			f.conflicts--
			return false, nil
		}
		current, exists := f.data[key]
		if options.OldValue == nil && exists || options.OldValue != nil && (!exists || !bytes.Equal(current, options.OldValue)) {
			return false, nil
		}
	}
	if value == nil {
		delete(f.data, key)
	} else {
		f.data[key] = bytes.Clone(value)
	}
	return true, nil
}

// fakeService is a GIF service with the Tenor API: GIFs g1 (all formats) and g2 (a GIF only),
// and sticker s1.
type fakeService struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
	shares   []string
	uploads  []fakeUpload
}

// fakeUpload is a request to POST /v2/upload.
type fakeUpload struct {
	auth     string
	fields   map[string]string
	file     string
	fileName string
}

func (f *fakeService) lastUpload() fakeUpload {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.uploads[len(f.uploads)-1]
}

// upload answers POST /v2/upload with item up1 of the uploaded kind.
func (f *fakeService) upload(w http.ResponseWriter, r *http.Request, media func(string, int, int) map[string]any) {
	up := fakeUpload{auth: r.Header.Get("Authorization"), fields: map[string]string{}}
	if up.auth != "Bearer up-secret" {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"code":403,"message":"The request is missing a valid upload key.","status":"PERMISSION_DENIED"}}`)
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for k, v := range r.MultipartForm.Value {
		up.fields[k] = v[0]
	}
	if file, header, err := r.FormFile("file"); err == nil {
		data, _ := io.ReadAll(file)
		up.file, up.fileName = string(data), header.Filename
	}
	f.mu.Lock()
	f.uploads = append(f.uploads, up)
	f.mu.Unlock()
	if up.fields["title"] == "unconvertible" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"error":{"code":422,"message":"The file couldn't be converted: is it a valid image or video?","status":"INVALID_ARGUMENT"}}`)
		return
	}
	res := map[string]any{"id": "up1", "title": up.fields["title"], "media_formats": map[string]any{
		"tinygif": media("up1/tinygif.gif", 220, 124), "gif": media("up1/gif.gif", 498, 280),
	}}
	if up.fields["kind"] == "sticker" {
		res["flags"] = []string{"sticker"}
		res["media_formats"] = map[string]any{"tinywebp_transparent": media("up1/tinywebp_transparent.webp", 220, 220)}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{res}})
}

func (f *fakeService) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func newFakeService(t *testing.T) *fakeService {
	t.Helper()
	f := &fakeService{}
	media := func(path string, w, h int) map[string]any {
		return map[string]any{"url": f.URL + "/media/" + path, "dims": []int{w, h}, "size": 10}
	}
	var results map[string]map[string]any
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Path+"?"+r.URL.RawQuery)
		f.mu.Unlock()
		q := r.URL.Query()

		if rest, ok := strings.CutPrefix(r.URL.Path, "/media/"); ok {
			switch {
			case rest == "page.html":
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<script>")
			case strings.HasPrefix(rest, "g1/") || strings.HasPrefix(rest, "s1/"):
				w.Header().Set("Content-Type", map[bool]string{true: "video/mp4", false: "image/gif"}[strings.HasSuffix(rest, ".mp4")])
				fmt.Fprint(w, "MEDIA:"+rest)
			default:
				http.NotFound(w, r)
			}
			return
		}

		if r.URL.Path == "/v2/upload" {
			f.upload(w, r, media)
			return
		}
		if q.Get("key") != "secret" || q.Get("client_key") != clientKey {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"error":{"code":403,"message":"The request is missing a valid API key.","status":"PERMISSION_DENIED"}}`)
			return
		}
		sticker := q.Get("searchfilter") == "sticker"
		var out any
		switch r.URL.Path {
		case "/v2/search", "/v2/featured":
			list := []any{results["g1"], results["g2"]}
			if sticker {
				list = []any{results["s1"]}
			}
			if q.Get("q") == "nothing" {
				list = []any{}
			}
			out = map[string]any{"results": list, "next": "2"}
		case "/v2/posts":
			var list []any
			for _, id := range strings.Split(q.Get("ids"), ",") {
				if res, ok := results[id]; ok {
					list = append(list, res)
				}
			}
			out = map[string]any{"results": list}
		case "/v2/categories":
			name, term, image := "#cats", "cats", "g1/tinygif.gif"
			if sticker {
				name, term, image = "Lab crew", "lab crew", "s1/tinywebp_transparent.webp"
			}
			out = map[string]any{"locale": "en", "tags": []any{map[string]any{"name": name, "searchterm": term, "image": f.URL + "/media/" + image}}}
		case "/v2/autocomplete", "/v2/search_suggestions", "/v2/trending_terms":
			out = map[string]any{"locale": "en", "results": []string{"cat", "cats"}}
		case "/v2/registershare":
			f.mu.Lock()
			f.shares = append(f.shares, q.Get("id")+":"+q.Get("q"))
			f.mu.Unlock()
			out = map[string]any{}
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(f.Close)
	results = map[string]map[string]any{
		"g1": {"id": "g1", "title": "Happy cat", "content_description": "A happy cat", "media_formats": map[string]any{
			"tinygif": media("g1/tinygif.gif", 220, 124), "gif": media("g1/gif.gif", 498, 280),
			"mp4": media("g1/mp4.mp4", 498, 280), "preview": media("g1/preview.png", 498, 280),
		}, "attribution": map[string]any{"source": "tenor", "url": "https://tenor.com/view/g1", "text": "Via Tenor"}},
		"g2": {"id": "g2", "title": "", "media_formats": map[string]any{"gif": media("g2/gif.gif", 200, 200)}},
		"s1": {"id": "s1", "title": "It works!", "flags": []string{"sticker"}, "media_formats": map[string]any{
			"tinywebp_transparent": media("s1/tinywebp_transparent.webp", 220, 220), "gif_transparent": media("s1/gif_transparent.gif", 512, 512),
		}},
	}
	return f
}

type testEnv struct {
	p       *Plugin
	api     *plugintest.API
	kv      *fakeKV
	service *fakeService
}

const (
	testUserID    = "uuuuuuuuuuuuuuuuuuuuuuuuuu"
	testChannelID = "cccccccccccccccccccccccccc"
	testRootID    = "rrrrrrrrrrrrrrrrrrrrrrrrrr"
)

// newTestEnv sets up the plugin with a fake GIF service; change edits the configuration.
func newTestEnv(t *testing.T, change func(c *configuration)) *testEnv {
	t.Helper()
	env := &testEnv{api: &plugintest.API{}, kv: newFakeKV(), service: newFakeService(t)}
	for _, level := range []string{"LogDebug", "LogInfo", "LogWarn", "LogError"} {
		for n := 1; n <= 7; n += 2 {
			args := make([]any, n)
			for i := range args {
				args[i] = mock.Anything
			}
			env.api.On(level, args...).Maybe()
		}
	}
	siteURL := "https://chat.example.com"
	env.api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: &siteURL}}).Maybe()

	cfg := &configuration{
		ServiceURL:     env.service.URL + "/v2",
		APIKey:         "secret",
		ContentFilter:  "high",
		EnableStickers: true,
		ProxyMedia:     true,
	}
	if change != nil {
		change(cfg)
	}
	require.NoError(t, cfg.IsValid())
	p := &Plugin{httpClient: env.service.Client()}
	p.SetAPI(env.api)
	p.store = NewStore(env.kv)
	p.router = p.newRouter()
	p.setConfiguration(cfg)
	p.setService(newGIFService(cfg, p.client()))
	env.p = p
	return env
}

// do sends a request to the plugin as the test user (or anonymously when userID is empty).
func (env *testEnv) do(t *testing.T, method, target, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, target, reader)
	if userID != "" {
		req.Header.Set("Mattermost-User-Id", userID)
	}
	rec := httptest.NewRecorder()
	env.p.ServeHTTP(nil, rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v), rec.Body.String())
	return v
}
