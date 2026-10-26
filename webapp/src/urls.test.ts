// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {mediaSrc, remoteMediaAllowed, safeLink} from './urls';

// A script URL, which must never be used.
const scriptURL = ['javascript', 'alert(1)'].join(':');

describe('mediaSrc', () => {
    afterEach(() => {
        delete window.basename;
    });

    test('prefixes the plugin media route with the site subpath', () => {
        expect(mediaSrc('/plugins/com.antimatterchat.gifs/media/12/34/gif.gif')).toBe('/plugins/com.antimatterchat.gifs/media/12/34/gif.gif');
        window.basename = '/chat';
        expect(mediaSrc('/plugins/com.antimatterchat.gifs/media/12/34/gif.gif')).toBe('/chat/plugins/com.antimatterchat.gifs/media/12/34/gif.gif');
    });

    test('keeps the service URLs when the plugin doesn\'t serve the media', () => {
        expect(mediaSrc('https://media.tenor.com/x/tinygif.gif', true)).toBe('https://media.tenor.com/x/tinygif.gif');
    });

    // A forged post's URL elsewhere would have every viewer's browser call that host.
    test('mediaSrc rejects foreign hosts when proxied', () => {
        expect(mediaSrc('https://evil.example.com/x.mp4')).toBeNull();
        expect(mediaSrc('https://media.tenor.com/x/tinygif.gif', false)).toBeNull();
        expect(mediaSrc('/plugins/com.antimatterchat.gifs/media/12/34/gif.gif', false)).toBe('/plugins/com.antimatterchat.gifs/media/12/34/gif.gif');
    });

    test('remote media only once the configuration says the plugin doesn\'t serve them', () => {
        expect(remoteMediaAllowed(null)).toBe(false);
        expect(remoteMediaAllowed({configured: true, stickers: true})).toBe(false);
        expect(remoteMediaAllowed({configured: true, stickers: true, media_proxied: true})).toBe(false);
        expect(remoteMediaAllowed({configured: true, stickers: true, media_proxied: false})).toBe(true);
    });

    test('rejects anything else', () => {
        for (const url of ['', scriptURL, 'data:image/gif;base64,AAAA', '/api/v4/users/me', '/plugins/com.antimatterchat.gifs/media/../../api', '//evil.example.com/x.gif']) {
            expect(mediaSrc(url, true)).toBeNull();
        }
    });
});

test('safeLink keeps http(s) links only', () => {
    expect(safeLink('https://tenor.com/view/x')).toBe('https://tenor.com/view/x');
    expect(safeLink(scriptURL)).toBeNull();
    expect(safeLink('')).toBeNull();
});
