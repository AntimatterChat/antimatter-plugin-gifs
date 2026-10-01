// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {Client4} from 'mattermost-redux/client';

import manifest from './manifest';
import type {Category, ClientConfig, Item, Kind, Page, Upload} from './types/gifs';

export class ClientError extends Error {
    status: number;

    constructor(message: string, status: number) {
        super(message);
        this.status = status;
    }
}

function apiURL(path: string, params?: Record<string, string | number | undefined>) {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(params || {})) {
        if (typeof value !== 'undefined' && value !== '') {
            query.set(key, String(value));
        }
    }
    const qs = query.toString();
    return `${window.basename || ''}/plugins/${manifest.id}/api/v1${path}${qs ? '?' + qs : ''}`;
}

async function request<T>(method: string, url: string, body?: unknown): Promise<T> {
    const options: {method: string; body?: string | FormData} = {method};
    if (body instanceof FormData) {
        options.body = body;
    } else if (typeof body !== 'undefined') {
        options.body = JSON.stringify(body);
    }

    // With FormData, Client4 lets the browser set the multipart type and its boundary.
    const response = await fetch(url, Client4.getOptions(options));
    if (!response.ok) {
        let message = response.statusText;
        try {
            const data = await response.json();
            message = data.error || message;
        } catch {
            // Not JSON
        }
        throw new ClientError(message, response.status);
    }
    return response.json();
}

export function getConfig() {
    return request<ClientConfig>('GET', apiURL('/config'));
}

export function search(kind: Kind, q: string, pos?: string, locale?: string, limit?: number) {
    return request<Page>('GET', apiURL('/search', {kind, q, pos, locale, limit}));
}

export function featured(kind: Kind, pos?: string, locale?: string, limit?: number) {
    return request<Page>('GET', apiURL('/featured', {kind, pos, locale, limit}));
}

export function categories(kind: Kind, locale?: string) {
    return request<Category[]>('GET', apiURL('/categories', {kind, locale}));
}

export function autocomplete(q: string, locale?: string) {
    return request<string[]>('GET', apiURL('/autocomplete', {q, locale, limit: 6}));
}

export function getFavorites() {
    return request<Item[]>('GET', apiURL('/favorites'));
}

export function addFavorite(item: Item) {
    return request<Item[]>('POST', apiURL('/favorites'), {kind: item.kind, id: item.id});
}

export function removeFavorite(item: Item) {
    return request<Item[]>('DELETE', apiURL(`/favorites/${item.kind}/${encodeURIComponent(item.id)}`));
}

export function getRecent() {
    return request<Item[]>('GET', apiURL('/recent'));
}

export function postItem(item: Item, channelId: string, rootId?: string, query?: string) {
    return request<unknown>('POST', apiURL('/post'), {kind: item.kind, id: item.id, channel_id: channelId, root_id: rootId || '', query: query || ''});
}

export function upload({kind, file, title, tags, pack}: Upload) {
    const form = new FormData();
    form.append('kind', kind);
    form.append('title', title);
    form.append('tags', tags);
    if (pack) {
        form.append('pack', pack);
    }
    form.append('file', file, file.name);
    return request<Item>('POST', apiURL('/upload'), form);
}
