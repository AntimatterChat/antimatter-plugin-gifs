# GIFs

An Antimatter plugin to post GIFs and stickers, from a GIF service with the
[Tenor API v2](https://developers.google.com/tenor/guides/endpoints): the self-hosted
[gifs-server](https://github.com/antimatterchat/gifs-server) service (with its original
sticker pack), or Tenor itself.

## Features

- **Picker**, after the Fusion UI mockup's GIF and sticker picker: GIFs and Stickers tabs of the
  message box's emoji picker (or the plugin's own picker), each with a search box; for GIFs, chips
  for Favorites, Recent, Trending and the service's categories (and suggested searches while
  searching) over a two-column grid that loads more as it scrolls; for stickers, the recently used
  and favourite stickers and a section per sticker pack.
- **Adding GIFs and stickers** (gifs-server with an upload key): a "+" button next to the
  search box opens a dialog taking a file (GIF, PNG, WebP, MP4 or WebM GIFs; PNG, WebP, GIF or SVG
  stickers, up to 16 MB), a title, tags and a sticker pack. The service converts it; it lands in the
  user's favourites and can be sent right away. System admins only by default.
- **Favourites** per user (the star on each GIF and sticker, in the picker and on posts) and
  **recently used** items, kept on the server.
- **Posts**: a GIF plays as a muted, looping video (or GIF) up to 320px wide with a GIF badge and the
  attribution of third-party content; a sticker shows as a 128px transparent image. People who
  prefer reduced motion see a still frame. Other clients (mobile, desktop without the plugin) see
  the message, a Markdown image of the GIF.
- **Commands**: `/gif <search>` and `/sticker <search>`. The webapp opens the picker on the search;
  other clients get an ephemeral preview with **Send**, **Shuffle** and **Cancel** buttons.
- **Privacy**: only the Antimatter server talks to the GIF service. It proxies the searches (the API
  key stays on the server) and, by default, the media files, so users' devices never connect to the
  service (or to Tenor) and the service can stay on a private network. Answers and media are cached
  in memory.
- **Fusion**: under the Fusion web UI (`isFusionUI()`), the picker and posts use the Fusion UI's own
  markup and classes (`am-pop am-picker`, `am-picker-tabs`, `am-gif-cats`, `am-gif-grid`,
  `am-gif-tile`, `am-gif-star`, `am-sticker-scroll`, `am-sticker-grid`, `am-gif-msg`,
  `am-sticker-msg`...) and render in its layer; under the classic UI, the plugin's own styles in the
  theme's colors.

## Setup

1. Run the GIF service, e.g. gifs-server with Docker:

   ```sh
   docker run --rm -v gifs-data:/data gifs-server stickers      # import the sticker pack
   docker run -d --name gifs-server -v gifs-data:/data -e AM_GIFS_API_KEYS=change-me gifs-server
   ```

   It only needs to be reachable from the Antimatter server.
2. Install and enable the plugin, then set **System Console > Plugins > GIFs**:

| Setting | Default | Meaning |
| --- | --- | --- |
| GIF service URL (`ServiceURL`) | empty | Base URL of the Tenor API v2: `http://gifs-server:8080/v2`, or `https://tenor.googleapis.com/v2`. When empty, `AM_GIFS_SERVICE_URL` is used; GIFs are off while both are empty. |
| API key (`APIKey`) | empty | The service's API key (one of `AM_GIFS_API_KEYS`, or your Tenor key). When empty, `AM_GIFS_API_KEY` is used. Stays on the server. |
| Enable stickers (`EnableStickers`) | true | Stickers tab, `/sticker`. |
| Content filter (`ContentFilter`) | medium | Tenor's `contentfilter`: high (G), medium (G, PG), low (up to PG-13), off. |
| Serve the media through Antimatter (`ProxyMedia`) | true | Media files are served under `/plugins/com.antimatterchat.gifs/media/` to signed-in users. When false, browsers load them from the service, which must then be reachable from them. |
| Media URL (`MediaURL`) | service origin + `/media/` | Base URL of the media files to proxy, when they aren't served by the service itself (a CDN; `https://media.tenor.com/` for Tenor). |
| Upload key (`UploadKey`) | empty | An upload key of gifs-server (`AM_GIFS_UPLOAD_KEYS`). With it, users can add GIFs and stickers; stays on the server. When empty, `AM_GIFS_UPLOAD_KEY` is used. |
| Who can add GIFs and stickers (`WhoCanUpload`) | admins | `admins` (system admins) or `everyone`. |
| Cache size (`CacheSizeMB`) | 64 | Memory for cached answers and media files. |

The service URL, API key and upload key can also come from environment variables of the Antimatter
server, which its plugins inherit, so a docker-compose file can set up GIFs without the System
Console. A value set in the System Console wins over the variable.

```yaml
services:
  antimatter:
    environment:
      AM_GIFS_SERVICE_URL: http://gifs-server:8080/v2
      AM_GIFS_API_KEY: change-me          # one of the service's AM_GIFS_API_KEYS
      AM_GIFS_UPLOAD_KEY: change-me-too   # optional, one of its AM_GIFS_UPLOAD_KEYS
```

The core GIF picker of the emoji picker (`ServiceSettings.EnableGifPicker`) uses Giphy directly from
browsers. On Antimatter, the plugin's GIFs tab replaces it while GIFs are set up (the Giphy tab is
hidden); on other hosts, turn it off when this plugin replaces it.

## Where the picker opens

| Host | Entry points |
| --- | --- |
| Antimatter with `registerEmojiPickerTab` | GIFs and Stickers tabs of the message box's emoji picker (before Emoji under Fusion, as in the mockup; after Emojis in the classic UI), `/gif`, `/sticker` |
| Antimatter with only `registerComposerMenuItem` | "GIF or sticker" in the message box's "+" menu (Fusion's sticker icon under Fusion), `/gif`, `/sticker` |
| Other hosts | A channel header button, `/gif`, `/sticker` |

`/gif` and `/sticker` open the plugin's own picker on the search. The plugin also sets
`window.antimatterGifs.openPicker({channelId, rootId?, tab?: 'gif' | 'sticker', query?})`, so the host
can open that picker too.

## How it works

### Server

REST API under `/plugins/com.antimatterchat.gifs` (user session required):

| Method & path | Response |
| --- | --- |
| `GET /api/v1/config` | `{"configured": bool, "stickers": bool, "can_upload": bool}` |
| `GET /api/v1/search?q=&kind=gif\|sticker&pos=&limit=&locale=` | `{"items": [Item], "next": "..."}` |
| `GET /api/v1/featured?kind=&pos=&limit=&locale=` | the same, for the trending items |
| `GET /api/v1/categories?kind=` | `[{"name", "searchterm", "image"}]`: GIF categories or sticker packs |
| `GET /api/v1/autocomplete?q=`, `/suggestions?q=`, `/trending_terms` | `["term"]` |
| `GET /api/v1/favorites`, `POST /api/v1/favorites` `{"kind", "id"}`, `DELETE /api/v1/favorites/{kind}/{id}` | the user's favourites, `[Item]` |
| `GET /api/v1/recent` | the user's recently used items |
| `POST /api/v1/post` `{"kind", "id", "channel_id", "root_id", "query"}` | the created post |
| `POST /api/v1/upload` (multipart: `file`, `kind`, `title`, `tags`, `pack`) | the new `Item`, also added to the favourites; `can_upload` of `/config` tells who may |
| `POST /api/v1/actions/{send,shuffle,cancel}` | the buttons of the `/gif` preview |
| `GET /media/{path}` | a media file of the service (cached, range requests, sandboxing CSP) |

An `Item` is `{"id", "kind", "title", "description", "preview", "post", "image", "still",
"attribution", "source_url"}`, where each media is `{"url", "width", "height", "video"}`:
`preview` for the picker (tinygif, tinywebp_transparent...), `post` for messages (an MP4 for GIFs
when there is one, a transparent WebP or GIF for stickers), `image` the GIF of a video, `still` a
still frame. Items are always fetched from the service before being favourited or posted, so
clients can't make up media URLs.

Posts have the type `custom_antimatter_gif` and the item in the `antimatter_gif` prop; their message
is a Markdown image for other clients. Posting registers the share with the service (`registershare`,
with the search), which feeds its trending lists. Favourites (≤ 200) and recently used items (≤ 30)
are snapshots of items in the KV store (`fav_<user id>`, `recent_<user id>`).

### Webapp

| Hook | Used for | Without it |
| --- | --- | --- |
| `registerEmojiPickerTab` (Antimatter) | GIFs and Stickers tabs of the emoji picker, replacing its Giphy tab | the "+" menu item |
| `registerComposerMenuItem` (Antimatter) | "+" menu item, without emoji picker tabs | a channel header button |
| `registerPostTypeComponent` | GIF and sticker posts | |
| `registerRootComponent` | the picker | |
| `registerSlashCommandWillBePostedHook` | `/gif` and `/sticker` open the picker | the server's preview |

## Development

```sh
make dist      # build the plugin bundle in dist/
make test      # run the server and webapp tests
make check-style
```

Set `MM_SERVICESETTINGS_ENABLEDEVELOPER=true` to only build the server for the current platform.

## Feedback and support

Found a bug in the GIFs plugin, or missing a feature? Please [open an
issue](https://github.com/AntimatterChat/community/issues/new/choose) in the [Antimatter community
repository](https://github.com/AntimatterChat/community), where the feedback for every part of
Antimatter is gathered. Questions and ideas are welcome in its
[Discussions](https://github.com/AntimatterChat/community/discussions). Please report security
problems privately, as explained in the [security
policy](https://github.com/AntimatterChat/.github/blob/main/SECURITY.md).

## License

GNU Affero General Public License v3.0, see [LICENSE.txt](LICENSE.txt). The build tooling is derived
from Apache-2.0 licensed Mattermost plugins, see [NOTICE.txt](NOTICE.txt).
