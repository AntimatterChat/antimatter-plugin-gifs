// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import manifest from './manifest';
import type {ClientConfig} from './types/gifs';

// mediaSrc returns the URL to load a media file from: the plugin's media route (under the site's
// subpath) or, when allowRemote, an http(s) URL of the GIF service. Anything else, e.g. a
// javascript: URL in the props of a forged post, gives null. When the plugin serves the media (the
// default), the media of posts come from it: a forged post's URL elsewhere would have every
// viewer's browser call that host, telling it their address.
export function mediaSrc(url: string | undefined, allowRemote = false): string | null {
    if (!url) {
        return null;
    }
    if (url.startsWith(`/plugins/${manifest.id}/media/`) && !url.includes('..')) {
        return (window.basename || '') + url;
    }
    if (allowRemote && (/^https?:\/\//i).test(url)) {
        return url;
    }
    return null;
}

// remoteMediaAllowed tells whether media load from the GIF service's URLs: only once the
// configuration says the plugin doesn't serve them.
export function remoteMediaAllowed(config: ClientConfig | null | undefined): boolean {
    return config?.media_proxied === false;
}

// safeLink returns an http(s) URL, or null.
export function safeLink(url: string | undefined): string | null {
    return url && (/^https?:\/\//i).test(url) ? url : null;
}
