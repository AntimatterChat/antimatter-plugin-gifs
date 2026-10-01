// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import manifest from './manifest';

// mediaSrc returns the URL to load a media file from: the plugin's media route (under the site's
// subpath) or an http(s) URL of the GIF service. Anything else, e.g. a javascript: URL in the
// props of a forged post, gives null.
export function mediaSrc(url: string | undefined): string | null {
    if (!url) {
        return null;
    }
    if (url.startsWith(`/plugins/${manifest.id}/media/`) && !url.includes('..')) {
        return (window.basename || '') + url;
    }
    if ((/^https?:\/\//i).test(url)) {
        return url;
    }
    return null;
}

// safeLink returns an http(s) URL, or null.
export function safeLink(url: string | undefined): string | null {
    return url && (/^https?:\/\//i).test(url) ? url : null;
}
