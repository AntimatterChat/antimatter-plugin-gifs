// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {useSyncExternalStore} from 'react';

import * as client from './client';
import type {ClientConfig, Item} from './types/gifs';

// A tiny observable value, shared by the picker, the posts and the plugin's registrations.
class Observable<T> {
    private value: T;
    private listeners = new Set<() => void>();

    constructor(value: T) {
        this.value = value;
    }

    get = () => this.value;

    set(value: T) {
        this.value = value;
        this.listeners.forEach((l) => l());
    }

    subscribe = (listener: () => void) => {
        this.listeners.add(listener);
        return () => {
            this.listeners.delete(listener);
        };
    };
}

function useObservable<T>(o: Observable<T>): T {
    return useSyncExternalStore(o.subscribe, o.get);
}

// The plugin configuration the clients see.
// null until loaded.
const config = new Observable<ClientConfig | null>(null);

export async function loadConfig() {
    try {
        config.set(await client.getConfig());
    } catch {
        // Keep the previous configuration
    }
}

export function getClientConfig() {
    return config.get();
}

export function useClientConfig() {
    return useObservable(config);
}

// The user's favourites, loaded once.
const favorites = new Observable<Item[] | null>(null);
let favoritesLoading: Promise<void> | null = null;

export function loadFavorites() {
    if (!favoritesLoading) {
        favoritesLoading = client.getFavorites().then(
            (items) => favorites.set(items),
            () => {
                favoritesLoading = null;
            },
        );
    }
    return favoritesLoading;
}

export function resetFavorites() {
    favorites.set(null);
    favoritesLoading = null;
}

export function isFavorite(list: Item[] | null, item: Item) {
    return Boolean(list?.some((f) => f.id === item.id && f.kind === item.kind));
}

export async function toggleFavorite(item: Item) {
    const list = favorites.get();
    const next = isFavorite(list, item) ? await client.removeFavorite(item) : await client.addFavorite(item);
    favorites.set(next);
}

export function useFavorites() {
    return useObservable(favorites);
}
