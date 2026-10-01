// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {useCallback, useEffect, useRef, useState} from 'react';

import type {Item, Page} from '../types/gifs';

// useDebounced returns the value once it hasn't changed for delay ms.
export function useDebounced<T>(value: T, delay: number): T {
    const [debounced, setDebounced] = useState(value);
    useEffect(() => {
        const timer = setTimeout(() => setDebounced(value), delay);
        return () => clearTimeout(timer);
    }, [value, delay]);
    return debounced;
}

// useLoad runs load whenever key changes (not when null) and keeps its latest result.
export function useLoad<T>(key: string | null, load: () => Promise<T>): {data?: T; error: boolean; loading: boolean} {
    const [state, setState] = useState<{key: string | null; data?: T; error: boolean}>({key: null, error: false});
    const loadRef = useRef(load);
    loadRef.current = load;
    useEffect(() => {
        if (key === null) {
            return () => null;
        }
        let cancelled = false;
        loadRef.current().then(
            (data) => !cancelled && setState({key, data, error: false}),
            () => !cancelled && setState({key, error: true}),
        );
        return () => {
            cancelled = true;
        };
    }, [key]);
    const current = state.key === key;
    return {...(current ? {data: state.data} : {}), error: current && state.error, loading: key !== null && !current};
}

// appendNew appends the items that aren't in the list yet.
function appendNew(list: Item[], items: Item[]) {
    const seen = new Set(list.map((it) => it.id));
    return [...list, ...items.filter((it) => !seen.has(it.id))];
}

type PagedState = {
    key: string;
    items: Item[];
    next?: string;
    loading: boolean;
    error: boolean;
};

// usePagedList loads the pages of a list (search results, featured items) as the user scrolls.
export function usePagedList(key: string | null, fetchPage: (pos?: string) => Promise<Page>) {
    const [state, setState] = useState<PagedState>({key: '', items: [], loading: false, error: false});
    const fetchRef = useRef(fetchPage);
    fetchRef.current = fetchPage;
    const keyRef = useRef(key);
    keyRef.current = key;

    const load = useCallback(async (forKey: string, pos?: string) => {
        setState((s) => (s.key === forKey ? {...s, loading: true} : {key: forKey, items: [], loading: true, error: false}));
        let page: Page;
        try {
            page = await fetchRef.current(pos);
        } catch {
            if (keyRef.current === forKey) {
                setState((s) => ({...s, loading: false, error: true}));
            }
            return;
        }
        if (keyRef.current === forKey) {
            setState((s) => ({key: forKey, items: pos ? appendNew(s.items, page.items) : page.items, next: page.next, loading: false, error: false}));
        }
    }, []);

    useEffect(() => {
        if (key !== null) {
            load(key);
        }
    }, [key, load]);

    const current = key !== null && state.key === key;
    const loadMore = useCallback(() => {
        if (current && state.next && !state.loading) {
            load(state.key, state.next);
        }
    }, [current, state, load]);

    return {
        items: current ? state.items : [],
        loading: key !== null && (!current || state.loading),
        error: current && state.error,
        hasMore: current && Boolean(state.next),
        loadMore,
    };
}
