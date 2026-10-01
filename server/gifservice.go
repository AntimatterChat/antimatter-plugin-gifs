// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Kinds of items.
const (
	kindGIF     = "gif"
	kindSticker = "sticker"
)

// clientKey tells the GIF service which integration calls it, like Tenor's client_key.
const clientKey = "antimatter"

// errNotConfigured is returned while no GIF service is set up.
var errNotConfigured = newAPIError(http.StatusServiceUnavailable, "the GIF service isn't configured")

// Media is a media file of an item. URL is a path on the Antimatter server (under the plugin's
// /media/ route) when the plugin proxies the media, else the service's URL.
type Media struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	// Video is true for MP4 or WebM files, to play muted and looped.
	Video bool `json:"video,omitempty"`
}

// Item is a GIF or sticker, in the few formats the webapp uses.
type Item struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	// Preview is the small animated image of the picker.
	Preview Media `json:"preview"`
	// Post is shown in messages: an MP4 for GIFs when available, a transparent image for stickers.
	Post Media `json:"post"`
	// Image is the GIF of a GIF whose Post is a video, for places that only show images.
	Image *Media `json:"image,omitempty"`
	// Still is a still frame, for people who prefer reduced motion.
	Still       *Media `json:"still,omitempty"`
	Attribution string `json:"attribution,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
}

// imageURL is the URL of an image of the item (not a video).
func (it *Item) imageURL() string {
	if !it.Post.Video {
		return it.Post.URL
	}
	if it.Image != nil {
		return it.Image.URL
	}
	return it.Preview.URL
}

// Page is a page of results.
type Page struct {
	Items []Item `json:"items"`
	Next  string `json:"next,omitempty"`
}

// Category is a GIF category or a sticker pack.
type Category struct {
	Name       string `json:"name"`
	SearchTerm string `json:"searchterm"`
	Image      string `json:"image,omitempty"`
}

// Format preferences, best first.
var (
	gifPreviewFormats     = []string{"tinygif", "nanogif", "mediumgif", "gif"}
	gifVideoFormats       = []string{"mp4", "tinymp4", "loopedmp4", "webm", "tinywebm"}
	gifImageFormats       = []string{"gif", "mediumgif", "tinygif"}
	stickerPreviewFormats = []string{"tinywebp_transparent", "tinygif_transparent", "nanowebp_transparent", "nanogif_transparent", "webp_transparent", "gif_transparent"}
	stickerPostFormats    = []string{"tinywebp_transparent", "tinygif_transparent", "webp_transparent", "gif_transparent"}
	stillFormats          = []string{"preview", "gifpreview", "tinygifpreview"}
)

// mediaFilter asks the service for the formats the plugin uses only.
var mediaFilter = func() string {
	seen := map[string]bool{}
	var formats []string
	for _, list := range [][]string{gifPreviewFormats, gifVideoFormats, gifImageFormats, stickerPreviewFormats, stickerPostFormats, stillFormats} {
		for _, f := range list {
			if !seen[f] {
				seen[f] = true
				formats = append(formats, f)
			}
		}
	}
	return strings.Join(formats, ",")
}()

// tenorResult is a Tenor response object.
type tenorResult struct {
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	ContentDescription string                `json:"content_description"`
	ItemURL            string                `json:"itemurl"`
	Flags              []string              `json:"flags"`
	MediaFormats       map[string]tenorMedia `json:"media_formats"`
	Attribution        *struct {
		Source string `json:"source"`
		URL    string `json:"url"`
		Text   string `json:"text"`
	} `json:"attribution"`
}

type tenorMedia struct {
	URL  string `json:"url"`
	Dims []int  `json:"dims"`
}

// gifService calls the Tenor API of the GIF service, with a cache.
type gifService struct {
	base          string
	key           string
	contentFilter string
	mediaBase     string
	proxyMedia    bool
	stickers      bool
	http          *http.Client
	cache         *cache
}

func newGIFService(c *configuration, client *http.Client) *gifService {
	return &gifService{
		base:          c.serviceBase(),
		key:           strings.TrimSpace(c.APIKey),
		contentFilter: c.contentFilter(),
		mediaBase:     c.mediaBase(),
		proxyMedia:    c.ProxyMedia,
		stickers:      c.EnableStickers,
		http:          client,
		cache:         newCache(c.cacheBytes()),
	}
}

func (s *gifService) configured() bool {
	return s != nil && s.base != ""
}

// apiTTL is how long API responses are cached.
const apiTTL = 5 * time.Minute

// get calls an endpoint and decodes its JSON response, from the cache when possible.
func (s *gifService) get(ctx context.Context, endpoint string, params url.Values, out any) error {
	if !s.configured() {
		return errNotConfigured
	}
	params.Set("client_key", clientKey)
	params.Set("contentfilter", s.contentFilter)
	cacheKey := "api:" + endpoint + "?" + params.Encode()
	if e, ok := s.cache.get(cacheKey); ok {
		return json.Unmarshal(e.body, out)
	}
	if s.key != "" {
		params.Set("key", s.key)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/"+endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		// Don't leak the key of the URL in errors.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return newAPIError(http.StatusBadGateway, "the GIF service can't be reached: "+err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return newAPIError(http.StatusBadGateway, "reading the GIF service's response: "+err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		msg := resp.Status
		if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
			msg = e.Error.Message
		}
		return newAPIError(http.StatusBadGateway, "the GIF service answered: "+msg)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return newAPIError(http.StatusBadGateway, "invalid response from the GIF service")
	}
	s.cache.set(cacheKey, body, "application/json", apiTTL)
	return nil
}

// listParams are the parameters of search and featured requests.
func (s *gifService) listParams(kind, pos, locale string, limit int) url.Values {
	params := url.Values{}
	params.Set("media_filter", mediaFilter)
	params.Set("limit", fmt.Sprint(limit))
	if kind == kindSticker {
		params.Set("searchfilter", "sticker")
	}
	if pos != "" {
		params.Set("pos", pos)
	}
	if locale != "" {
		params.Set("locale", locale)
	}
	return params
}

func (s *gifService) page(ctx context.Context, endpoint string, params url.Values, kind string) (*Page, error) {
	var res struct {
		Results []tenorResult `json:"results"`
		Next    string        `json:"next"`
	}
	if err := s.get(ctx, endpoint, params, &res); err != nil {
		return nil, err
	}
	page := &Page{Items: []Item{}, Next: res.Next}
	for _, r := range res.Results {
		if it, ok := s.item(r, kind); ok {
			page.Items = append(page.Items, it)
		}
	}
	return page, nil
}

// Search searches GIFs or stickers.
func (s *gifService) Search(ctx context.Context, kind, query, pos, locale string, limit int) (*Page, error) {
	params := s.listParams(kind, pos, locale, limit)
	params.Set("q", query)
	return s.page(ctx, "search", params, kind)
}

// Featured lists the featured (trending) GIFs or stickers.
func (s *gifService) Featured(ctx context.Context, kind, pos, locale string, limit int) (*Page, error) {
	return s.page(ctx, "featured", s.listParams(kind, pos, locale, limit), kind)
}

// Get returns one item, checking its kind.
func (s *gifService) Get(ctx context.Context, kind, id string) (*Item, error) {
	params := url.Values{}
	params.Set("ids", id)
	params.Set("media_filter", mediaFilter)
	var res struct {
		Results []tenorResult `json:"results"`
	}
	if err := s.get(ctx, "posts", params, &res); err != nil {
		return nil, err
	}
	for _, r := range res.Results {
		if r.ID != id {
			continue
		}
		isSticker := false
		for _, f := range r.Flags {
			isSticker = isSticker || f == "sticker"
		}
		// Services that don't flag stickers still give them transparent formats.
		if !isSticker {
			_, isSticker = firstFormat(r.MediaFormats, stickerPostFormats)
		}
		if isSticker != (kind == kindSticker) {
			break
		}
		if it, ok := s.item(r, kind); ok {
			return &it, nil
		}
	}
	return nil, newAPIError(http.StatusNotFound, "GIF not found")
}

// Categories lists the GIF categories or the sticker packs.
func (s *gifService) Categories(ctx context.Context, kind, locale string) ([]Category, error) {
	params := url.Values{}
	params.Set("type", "featured")
	if kind == kindSticker {
		params.Set("searchfilter", "sticker")
	}
	if locale != "" {
		params.Set("locale", locale)
	}
	var res struct {
		Tags []struct {
			SearchTerm string `json:"searchterm"`
			Name       string `json:"name"`
			Image      string `json:"image"`
		} `json:"tags"`
	}
	if err := s.get(ctx, "categories", params, &res); err != nil {
		return nil, err
	}
	cats := []Category{}
	for _, t := range res.Tags {
		if t.SearchTerm == "" {
			continue
		}
		cats = append(cats, Category{Name: strings.TrimPrefix(t.Name, "#"), SearchTerm: t.SearchTerm, Image: s.mediaURL(t.Image)})
	}
	return cats, nil
}

// Terms calls autocomplete, search_suggestions or trending_terms.
func (s *gifService) Terms(ctx context.Context, endpoint, query, locale string, limit int) ([]string, error) {
	params := url.Values{}
	params.Set("limit", fmt.Sprint(limit))
	if query != "" {
		params.Set("q", query)
	}
	if locale != "" {
		params.Set("locale", locale)
	}
	var res struct {
		Results []string `json:"results"`
	}
	if err := s.get(ctx, endpoint, params, &res); err != nil {
		return nil, err
	}
	if res.Results == nil {
		res.Results = []string{}
	}
	return res.Results, nil
}

// RegisterShare tells the service an item was shared, for its trending lists.
func (s *gifService) RegisterShare(ctx context.Context, id, query string) error {
	if !s.configured() {
		return errNotConfigured
	}
	params := url.Values{}
	params.Set("id", id)
	params.Set("client_key", clientKey)
	if query != "" {
		params.Set("q", query)
	}
	if s.key != "" {
		params.Set("key", s.key)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+"/registershare?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("registershare failed")
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registershare: %s", resp.Status)
	}
	return nil
}

func firstFormat(formats map[string]tenorMedia, names []string) (Media, bool) {
	for _, name := range names {
		if m, ok := formats[name]; ok && m.URL != "" {
			media := Media{URL: m.URL}
			if len(m.Dims) == 2 {
				media.Width, media.Height = m.Dims[0], m.Dims[1]
			}
			return media, true
		}
	}
	return Media{}, false
}

// item converts a Tenor result; results without usable media are dropped.
func (s *gifService) item(r tenorResult, kind string) (Item, bool) {
	it := Item{ID: r.ID, Kind: kind, Title: r.Title, Description: r.ContentDescription}
	if it.ID == "" {
		return it, false
	}
	var ok bool
	if kind == kindSticker {
		if it.Preview, ok = firstFormat(r.MediaFormats, stickerPreviewFormats); !ok {
			return it, false
		}
		it.Post, _ = firstFormat(r.MediaFormats, stickerPostFormats)
	} else {
		image, hasImage := firstFormat(r.MediaFormats, gifImageFormats)
		if it.Preview, ok = firstFormat(r.MediaFormats, gifPreviewFormats); !ok {
			return it, false
		}
		if video, hasVideo := firstFormat(r.MediaFormats, gifVideoFormats); hasVideo {
			video.Video = true
			it.Post = video
			if hasImage {
				it.Image = &image
			}
		} else if hasImage {
			it.Post = image
		} else {
			it.Post = it.Preview
		}
	}
	if still, ok := firstFormat(r.MediaFormats, stillFormats); ok {
		it.Still = &still
	}
	for _, m := range []*Media{&it.Preview, &it.Post, it.Image, it.Still} {
		if m != nil {
			m.URL = s.mediaURL(m.URL)
		}
	}
	if r.Attribution != nil {
		it.Attribution = r.Attribution.Text
		it.SourceURL = r.Attribution.URL
	}
	return it, true
}

// mediaPrefix is the plugin's media route, relative to the site URL.
const mediaPrefix = "/plugins/" + pluginID + "/media/"

// mediaURL rewrites the URL of a media file of the service to the plugin's proxy.
func (s *gifService) mediaURL(u string) string {
	if s.proxyMedia && s.mediaBase != "" && strings.HasPrefix(u, s.mediaBase) {
		return mediaPrefix + strings.TrimPrefix(u, s.mediaBase)
	}
	return u
}
