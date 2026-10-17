// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {GlobalState} from '@mattermost/types/store';

import {getCurrentUserLocale} from 'mattermost-redux/selectors/entities/i18n';

import * as client from '../client';
import {messages} from '../messages';
import {isFavorite, loadFavorites, toggleFavorite, useClientConfig, useFavorites, type PickerRequest} from '../state';
import type {Item, Kind} from '../types/gifs';
import {classNames} from '../web_ui';

import {useDebounced, useLoad, usePagedList} from './hooks';
import {SearchIcon, StarIcon} from './icons';
import MediaView from './media_view';
import {AddButton} from './upload_dialog';

type ClassNames = ReturnType<typeof classNames>;

// The width of a column of the GIF grid: the picker is 390px wide, with 10px of padding and an
// 8px gap between its two columns.
const GIF_COLUMN_WIDTH = 181;

// How many categories show as chips.
const MAX_CATEGORIES = 8;

type Common = {
    k: ClassNames;
    fusion: boolean;
    locale: string;
    onSend: (item: Item) => void;
};

function tileHeight(item: Item) {
    const {width, height} = item.preview;
    if (!width || !height) {
        return 120;
    }
    return Math.round(Math.min(260, Math.max(60, (GIF_COLUMN_WIDTH * height) / width)));
}

function FavoriteButton({item, k, fusion, small}: {item: Item; k: ClassNames; fusion: boolean; small?: boolean}) {
    const intl = useIntl();
    const favorites = useFavorites();
    const on = isFavorite(favorites, item);
    const label = intl.formatMessage(on ? messages.removeFavorite : messages.addFavorite);
    return (
        <button
            type='button'
            className={k('gif-star', on && 'on') + (small ? ' amgifs-star-small' : '')}
            aria-pressed={on}
            aria-label={label}
            title={label}
            onClick={(e) => {
                e.stopPropagation();
                toggleFavorite(item).catch(() => null);
            }}
        >
            <StarIcon className={fusion ? 'am-ic' : 'amgifs-ic'}/>
        </button>
    );
}

function GifCell({item, k, fusion, onSend}: Omit<Common, 'locale'> & {item: Item}) {
    const intl = useIntl();
    const title = item.title || item.description || intl.formatMessage(messages.gif);
    const heightVar = fusion ? '--am-h' : '--amgifs-h';
    return (
        <div className={k('gif-cell')}>
            <button
                type='button'
                className={k('gif-tile')}
                style={{[heightVar]: `${tileHeight(item)}px`} as React.CSSProperties}
                aria-label={intl.formatMessage(messages.sendGif, {title})}
                title={title}
                onClick={() => onSend(item)}
            >
                <MediaView
                    media={item.preview}
                    label=''
                    className='amgifs-fill'
                />
                <span className={k('gif-badge')}>{'GIF'}</span>
            </button>
            <FavoriteButton
                item={item}
                k={k}
                fusion={fusion}
            />
        </div>
    );
}

function StickerCell({item, k, fusion, onSend}: Omit<Common, 'locale'> & {item: Item}) {
    const intl = useIntl();
    const title = item.title || item.description || '';
    return (
        <div className='amgifs-sticker-cell'>
            <button
                type='button'
                className={k('sticker')}
                aria-label={intl.formatMessage(messages.sendSticker, {title})}
                title={title}
                onClick={() => onSend(item)}
            >
                <MediaView
                    media={item.preview}
                    label=''
                    className='amgifs-contain'
                />
            </button>
            <FavoriteButton
                item={item}
                k={k}
                fusion={fusion}
                small={true}
            />
        </div>
    );
}

// onNearBottom calls load when a scrolled element nears its end.
function onNearBottom(load: () => void) {
    return (e: React.UIEvent<HTMLElement>) => {
        const el = e.currentTarget;
        if (el.scrollTop + el.clientHeight > el.scrollHeight - 200) {
            load();
        }
    };
}

function Empty({k, children}: {k: ClassNames; children: React.ReactNode}) {
    return <div className={k('empty')}>{children}</div>;
}

type GifSource = {type: 'favorites'} | {type: 'recent'} | {type: 'trending'} | {type: 'search'; query: string};

function GifTab({k, fusion, locale, onSend, query, onQuery}: Common & {query: string; onQuery: (q: string) => void}) {
    const intl = useIntl();
    const [chip, setChip] = useState<string>('trending');
    const favorites = useFavorites();
    const categories = useLoad(`gif:${locale}`, () => client.categories('gif', locale));
    const recent = useLoad(chip === 'recent' ? 'recent' : null, client.getRecent);
    const suggestions = useLoad(query.length >= 2 ? `ac:${query}` : null, () => client.autocomplete(query, locale));

    let source: GifSource;
    if (query) {
        source = {type: 'search', query};
    } else if (chip === 'favorites' || chip === 'recent' || chip === 'trending') {
        source = {type: chip};
    } else {
        source = {type: 'search', query: chip.replace(/^c:/, '')};
    }

    let listKey: string | null = null;
    if (source.type === 'search') {
        listKey = `search:${source.query}:${locale}`;
    } else if (source.type === 'trending') {
        listKey = `featured:${locale}`;
    }
    const searchQuery = source.type === 'search' ? source.query : '';
    const list = usePagedList(listKey, (pos) => (searchQuery ? client.search('gif', searchQuery, pos, locale) : client.featured('gif', pos, locale)));

    let items: Item[] = list.items;
    let empty = intl.formatMessage(messages.noGifs);
    if (source.type === 'favorites') {
        items = (favorites || []).filter((it) => it.kind === 'gif');
        empty = intl.formatMessage(messages.noFavorites);
    } else if (source.type === 'recent') {
        items = (recent.data || []).filter((it) => it.kind === 'gif');
        empty = intl.formatMessage(messages.noRecent);
    }
    const loading = listKey ? list.loading : source.type === 'recent' && recent.loading;
    const failed = (listKey && list.error) || (source.type === 'recent' && recent.error);

    const chips: Array<{id: string; label: string; fav?: boolean}> = query ? (suggestions.data || []).
        filter((s) => s !== query).
        map((s) => ({id: s, label: s})) : [
        {id: 'favorites', label: intl.formatMessage(messages.favorites), fav: true},
        {id: 'recent', label: intl.formatMessage(messages.recent)},
        {id: 'trending', label: intl.formatMessage(messages.trending)},
        ...(categories.data || []).slice(0, MAX_CATEGORIES).map((c) => ({id: 'c:' + c.searchterm, label: c.name})),
    ];

    return (
        <div style={{display: 'flex', flexDirection: 'column', flex: 1, minWidth: 0, minHeight: 0}}>
            {chips.length > 0 && (
                <div
                    className={k('gif-cats')}
                    role='group'
                    aria-label={intl.formatMessage(query ? messages.suggestions : messages.categories)}
                >
                    {chips.map((c) => (
                        <button
                            type='button'
                            key={c.id}
                            className={k(!query && chip === c.id && 'on', c.fav && 'fav')}
                            aria-pressed={!query && chip === c.id}
                            onClick={() => (query ? onQuery(c.id) : setChip(c.id))}
                        >
                            {c.fav && <StarIcon className={fusion ? 'am-ic' : 'amgifs-ic'}/>}
                            {c.label}
                        </button>
                    ))}
                </div>
            )}
            <div
                className={k('gif-scroll')}
                onScroll={onNearBottom(list.loadMore)}
            >
                <div className={k('gif-grid')}>
                    {items.map((item) => (
                        <GifCell
                            key={item.id}
                            item={item}
                            k={k}
                            fusion={fusion}
                            onSend={onSend}
                        />
                    ))}
                </div>
                {items.length === 0 && !loading && <Empty k={k}>{failed ? intl.formatMessage(messages.loadFailed) : empty}</Empty>}
                {loading && <Empty k={k}>{intl.formatMessage(messages.loading)}</Empty>}
                <div className={k('gif-note')}>{intl.formatMessage(messages.note)}</div>
            </div>
        </div>
    );
}

function StickerSection({k, fusion, onSend, title, items}: Omit<Common, 'locale'> & {title: string; items: Item[]}) {
    if (items.length === 0) {
        return null;
    }
    return (
        <>
            <h5>{title}</h5>
            <div className={k('sticker-grid')}>
                {items.map((item) => (
                    <StickerCell
                        key={item.id}
                        item={item}
                        k={k}
                        fusion={fusion}
                        onSend={onSend}
                    />
                ))}
            </div>
        </>
    );
}

// StickerPack lists the stickers of a pack: the curated results of its search term.
function StickerPack({k, fusion, locale, onSend, name, term}: Common & {name: string; term: string}) {
    const pack = useLoad(`pack:${term}:${locale}`, () => client.search('sticker', term, '', locale, 50));
    return (
        <StickerSection
            k={k}
            fusion={fusion}
            onSend={onSend}
            title={name}
            items={pack.data?.items || []}
        />
    );
}

function StickerTab({k, fusion, locale, onSend, query}: Common & {query: string}) {
    const intl = useIntl();
    const favorites = useFavorites();
    const recent = useLoad(query ? null : 'recent', client.getRecent);
    const packs = useLoad(query ? null : `packs:${locale}`, () => client.categories('sticker', locale));
    const hasPacks = Boolean(packs.data?.length);
    const results = usePagedList(query ? `search:${query}:${locale}` : null, (pos) => client.search('sticker', query, pos, locale));
    const featured = useLoad(!query && packs.data && !hasPacks ? `featured:${locale}` : null, () => client.featured('sticker', '', locale, 48));
    const common = {k, fusion, onSend};

    if (query) {
        return (
            <div
                className={k('sticker-scroll')}
                onScroll={onNearBottom(results.loadMore)}
            >
                <div className={k('sticker-grid')}>
                    {results.items.map((item) => (
                        <StickerCell
                            key={item.id}
                            item={item}
                            {...common}
                        />
                    ))}
                </div>
                {results.items.length === 0 && !results.loading && <Empty k={k}>{intl.formatMessage(results.error ? messages.loadFailed : messages.noStickers)}</Empty>}
                {results.loading && <Empty k={k}>{intl.formatMessage(messages.loading)}</Empty>}
            </div>
        );
    }

    const favoriteStickers = (favorites || []).filter((it) => it.kind === 'sticker');
    return (
        <div className={k('sticker-scroll')}>
            <StickerSection
                {...common}
                title={intl.formatMessage(messages.recent)}
                items={(recent.data || []).filter((it) => it.kind === 'sticker').slice(0, 8)}
            />
            <StickerSection
                {...common}
                title={intl.formatMessage(messages.favorites)}
                items={favoriteStickers}
            />
            {(packs.data || []).map((p) => (
                <StickerPack
                    key={p.searchterm}
                    {...common}
                    locale={locale}
                    name={p.name}
                    term={p.searchterm}
                />
            ))}
            <StickerSection
                {...common}
                title={intl.formatMessage(messages.trending)}
                items={featured.data?.items || []}
            />
            {packs.error && <Empty k={k}>{intl.formatMessage(messages.loadFailed)}</Empty>}
            {packs.loading && <Empty k={k}>{intl.formatMessage(messages.loading)}</Empty>}
        </div>
    );
}

// JustAdded shows what was just added in the picker above the grid, with the focus on it, so that
// Enter sends it.
function JustAdded({item, onSend, onDismiss}: {item: Item; onSend: (item: Item) => void; onDismiss?: () => void}) {
    const intl = useIntl();
    const ref = useRef<HTMLButtonElement>(null);
    useEffect(() => {
        ref.current?.focus();
    }, [item.id]);
    const title = item.title || intl.formatMessage(messages.manageUntitled);
    return (
        <div
            className='amgifs-just-added'
            role='status'
        >
            <button
                ref={ref}
                type='button'
                className='amgifs-just-added-thumb'
                aria-label={intl.formatMessage(item.kind === 'sticker' ? messages.sendSticker : messages.sendGif, {title})}
                title={title}
                onClick={() => onSend(item)}
            >
                <MediaView
                    media={item.preview}
                    label=''
                    className='amgifs-contain'
                />
            </button>
            <span className='amgifs-just-added-text'>{intl.formatMessage(messages.justAdded, {title})}</span>
            {onDismiss && (
                <button
                    type='button'
                    className='amgifs-just-added-close'
                    aria-label={intl.formatMessage(messages.justAddedDismiss)}
                    title={intl.formatMessage(messages.justAddedDismiss)}
                    onClick={onDismiss}
                >
                    {'×'}
                </button>
            )}
        </div>
    );
}

type PanelProps = {
    kind: Kind;
    fusion: boolean;
    channelId: string;
    rootId?: string;

    // The text of the search box, and its changes.
    query: string;
    onQuery: (query: string) => void;

    // Called once the GIF or sticker is posted.
    onPosted: () => void;

    // Closes the picker, e.g. to open the dialog adding a GIF or sticker.
    onClose: () => void;

    // In the host's Fusion picker: its add and manage buttons, instead of the button opening the
    // upload dialog.
    tools?: React.ReactNode;

    // What was just added in the picker: shown first, ready to send, until dismissed.
    added?: Item | null;
    onDismissAdded?: () => void;
};

// PickerPanel is the search box and the GIFs or stickers of the picker, below its tabs: in the
// plugin's own picker, or in a tab of the host's emoji picker.
export function PickerPanel({kind, fusion, channelId, rootId, query, onQuery, onPosted, onClose, tools, added, onDismissAdded}: PanelProps) {
    const intl = useIntl();
    const k = classNames(fusion);
    const config = useClientConfig();
    const locale = useSelector((state: GlobalState) => getCurrentUserLocale(state)) || 'en';
    const debouncedQuery = useDebounced(query.trim(), 300);
    const [error, setError] = useState('');

    useEffect(() => {
        loadFavorites();
    }, []);

    const onSend = useCallback((item: Item) => {
        setError('');
        client.postItem(item, channelId, rootId, debouncedQuery).then(
            onPosted,
            (e: Error) => setError(intl.formatMessage(messages.postFailed, {error: e.message})),
        );
    }, [channelId, rootId, debouncedQuery, onPosted, intl]);

    const placeholder = intl.formatMessage(kind === 'gif' ? messages.searchGifs : messages.searchStickers);
    const common = {k, fusion, locale, onSend};

    return (
        <>
            <div className={k('picker-search') + ' amgifs-search-row'}>
                <label>
                    <SearchIcon className={fusion ? 'am-ic am-sm' : 'amgifs-ic'}/>
                    <input
                        autoFocus={true}
                        value={query}
                        placeholder={placeholder}
                        aria-label={placeholder}
                        autoComplete='off'
                        onChange={(e) => onQuery(e.target.value)}
                    />
                </label>
                {typeof tools === 'undefined' && config?.can_upload && (
                    <AddButton
                        request={{kind, channelId, rootId}}
                        fusion={fusion}
                        onClose={onClose}
                    />
                )}
                {tools}
            </div>
            {added && added.kind === kind && (
                <JustAdded
                    item={added}
                    onSend={onSend}
                    onDismiss={onDismissAdded}
                />
            )}
            {error && (
                <div
                    className='amgifs-error'
                    role='alert'
                >
                    {error}
                </div>
            )}
            <div className={k('picker-body')}>
                {!config && <Empty k={k}>{intl.formatMessage(messages.loading)}</Empty>}
                {config && !config.configured && <Empty k={k}>{intl.formatMessage(messages.notConfigured)}</Empty>}
                {config?.configured && kind === 'gif' && (
                    <GifTab
                        {...common}
                        query={debouncedQuery}
                        onQuery={onQuery}
                    />
                )}
                {config?.configured && kind === 'sticker' && (
                    <StickerTab
                        {...common}
                        query={debouncedQuery}
                    />
                )}
            </div>
        </>
    );
}

type Props = {
    request: PickerRequest;
    fusion: boolean;
    onClose: () => void;
    popRef?: React.Ref<HTMLDivElement>;
    style?: React.CSSProperties;
};

// Picker is the GIF and sticker picker, with the markup of the Fusion UI mockup's picker: its
// am- classes under Fusion, the plugin's amgifs- classes under the classic UI.
export default function Picker({request, fusion, onClose, popRef, style}: Props) {
    const intl = useIntl();
    const k = classNames(fusion);
    const config = useClientConfig() || {configured: true, stickers: request.tab === 'sticker'};
    const [tab, setTab] = useState<Kind>(request.tab === 'sticker' && config.stickers ? 'sticker' : 'gif');
    const [query, setQuery] = useState(request.query);

    return (
        <div
            ref={popRef}
            className={k('pop', 'picker')}
            style={style}
            role='dialog'
            aria-label={intl.formatMessage(messages.dialog)}
        >
            {config.stickers && (
                <div
                    className={k('picker-tabs')}
                    role='tablist'
                >
                    {([['gif', messages.tabGifs], ['sticker', messages.tabStickers]] as const).map(([key, label]) => (
                        <button
                            type='button'
                            key={key}
                            role='tab'
                            aria-selected={tab === key}
                            className={k(tab === key && 'on')}
                            onClick={() => setTab(key)}
                        >
                            {intl.formatMessage(label)}
                        </button>
                    ))}
                </div>
            )}
            <PickerPanel
                kind={tab}
                fusion={fusion}
                channelId={request.channelId}
                rootId={request.rootId}
                query={query}
                onQuery={setQuery}
                onPosted={onClose}
                onClose={onClose}
            />
        </div>
    );
}
