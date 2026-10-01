// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';

import type {Post} from '@mattermost/types/posts';

import {messages} from '../messages';
import {isFavorite, loadFavorites, toggleFavorite, useFavorites} from '../state';
import type {Item, Media} from '../types/gifs';
import {mediaSrc, safeLink} from '../urls';
import {classNames, isFusionUI} from '../web_ui';

import {StarIcon} from './icons';
import MediaView from './media_view';

// The post prop holding the item (see server/post.go).
export const POST_PROP = 'antimatter_gif';

// The largest width of a GIF in a post, as in the mockup.
const MAX_WIDTH = 320;

function isMedia(m: unknown): m is Media {
    return Boolean(m) && typeof (m as Media).url === 'string';
}

// parseItem checks the item of a post's props.
export function parseItem(value: unknown): Item | null {
    const it = value as Item;
    if (!it || typeof it !== 'object' || typeof it.id !== 'string' || (it.kind !== 'gif' && it.kind !== 'sticker')) {
        return null;
    }
    if (!isMedia(it.post) || !mediaSrc(it.post.url)) {
        return null;
    }
    const item: Item = {...it, preview: isMedia(it.preview) ? it.preview : it.post};
    if (!isMedia(item.image)) {
        delete item.image;
    }
    if (!isMedia(item.still)) {
        delete item.still;
    }
    return item;
}

// GifPost renders a GIF or sticker posted with the plugin (custom_antimatter_gif posts).
export default function GifPost({post}: {post: Post}) {
    const intl = useIntl();
    const fusion = isFusionUI();
    const k = classNames(fusion);
    const favorites = useFavorites();
    const item = parseItem(post.props?.[POST_PROP]);

    useEffect(() => {
        loadFavorites();
    }, []);

    if (!item) {
        return <p>{post.message}</p>;
    }

    const title = item.title || item.description || '';
    if (item.kind === 'sticker') {
        const label = intl.formatMessage(messages.stickerLabel, {title});
        return (
            <div
                className={k('sticker-msg')}
                role='img'
                aria-label={label}
                title={item.attribution ? `${title} · ${item.attribution}` : title}
            >
                <MediaView
                    media={item.post}
                    still={item.still}
                    label={label}
                    className='amgifs-contain'
                />
            </div>
        );
    }

    const label = intl.formatMessage(messages.gifLabel, {title: title || intl.formatMessage(messages.gif)});
    const {width, height} = item.post;
    const shownWidth = Math.min(MAX_WIDTH, width || MAX_WIDTH);
    const shownHeight = width && height ? Math.round((shownWidth * height) / width) : 180;
    const fav = isFavorite(favorites, item);
    const favLabel = intl.formatMessage(fav ? messages.removeFavorite : messages.addFavorite);
    const source = safeLink(item.source_url);
    return (
        <div
            className={k('gif-msg-wrap')}
            style={{width: shownWidth, maxWidth: '100%'}}
        >
            <div
                className={k('gif-msg')}
                style={{height: shownHeight, maxHeight: 400}}
            >
                <MediaView
                    media={item.post}
                    still={item.still}
                    label={label}
                    className='amgifs-fill'
                />
                <span className={k('gif-badge')}>{'GIF'}</span>
            </div>
            <button
                type='button'
                className={k('gif-star', fav && 'on')}
                aria-pressed={fav}
                aria-label={favLabel}
                title={favLabel}
                onClick={() => toggleFavorite(item).catch(() => null)}
            >
                <StarIcon className={fusion ? 'am-ic' : 'amgifs-ic'}/>
            </button>
            {item.attribution && (
                <div className='amgifs-attribution'>
                    {source ? (
                        <a
                            href={source}
                            target='_blank'
                            rel='noopener noreferrer'
                        >
                            {item.attribution}
                        </a>
                    ) : item.attribution}
                </div>
            )}
        </div>
    );
}
