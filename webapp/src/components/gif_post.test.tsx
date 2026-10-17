// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {fireEvent, screen} from '@testing-library/react';
import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {makeGif, makeSticker, renderWithProviders} from '../../tests/utils';
import * as client from '../client';
import {resetFavorites} from '../state';

import GifPost, {parseItem} from './gif_post';

jest.mock('../client', () => ({
    getFavorites: jest.fn(() => Promise.resolve([])),
    addFavorite: jest.fn(() => Promise.resolve([])),
    removeFavorite: jest.fn(() => Promise.resolve([])),
}));

function makePost(item: unknown): Post {
    return {id: 'post1', message: '![Happy cat](https://chat.example.com/x.gif)', props: {antimatter_gif: item}} as unknown as Post;
}

describe('GifPost', () => {
    beforeEach(() => {
        resetFavorites();
        delete window.antimatterWebUI;
        (client.getFavorites as jest.Mock).mockResolvedValue([]);
    });

    test('plays the video of a GIF, with its badge, star and attribution', async () => {
        const gif = makeGif('g1');
        const {container} = renderWithProviders(<GifPost post={makePost(gif)}/>);
        const video = container.querySelector('video') as HTMLVideoElement;
        expect(video).toHaveAttribute('src', '/plugins/com.antimatterchat.gifs/media/g1/mp4.mp4');
        expect(video).toHaveAttribute('poster', '/plugins/com.antimatterchat.gifs/media/g1/preview.png');
        expect(video).toHaveAttribute('aria-label', 'GIF: Happy cat');
        expect(container.querySelector('.amgifs-gif-msg-wrap')).toHaveStyle({width: '320px'});
        expect(container.querySelector('.amgifs-gif-msg')).toHaveStyle({height: '180px'});
        expect(screen.getByText('GIF')).toHaveClass('amgifs-gif-badge');
        expect(screen.getByRole('link', {name: 'Via Tenor'})).toHaveAttribute('href', 'https://tenor.com/view/x');

        (client.addFavorite as jest.Mock).mockResolvedValue([gif]);
        fireEvent.click(screen.getByRole('button', {name: 'Add to favourites'}));
        expect(await screen.findByRole('button', {name: 'Remove from favourites'})).toHaveAttribute('aria-pressed', 'true');
        expect(client.addFavorite).toHaveBeenCalledWith(gif);
    });

    test('uses the Fusion classes under Fusion', () => {
        window.antimatterWebUI = 'fusion';
        const {container} = renderWithProviders(<GifPost post={makePost({...makeSticker('s1'), attribution: 'Antimatter stickers, CC BY 4.0'})}/>);
        const sticker = container.querySelector('.am-sticker-msg') as HTMLElement;
        expect(sticker).toHaveAttribute('aria-label', 'Sticker: It works!');
        expect(sticker).toHaveAttribute('data-am-tip', 'It works! · Antimatter stickers, CC BY 4.0');
        expect(sticker).not.toHaveAttribute('title');
        expect(sticker.querySelector('img')).toHaveAttribute('src', '/plugins/com.antimatterchat.gifs/media/s1/tinywebp_transparent.webp');
    });

    test('under Fusion, the star has Fusion\'s tooltip and icon', () => {
        window.antimatterWebUI = 'fusion';
        const {container} = renderWithProviders(<GifPost post={makePost(makeGif('g1'))}/>);
        const star = screen.getByRole('button', {name: 'Add to favorites'});
        expect(star).toHaveAttribute('data-am-tip', 'Add to favorites');
        expect(star).not.toHaveAttribute('title');
        expect(star.querySelector('use')).toHaveAttribute('href', '#am-i-star');
        expect(container.querySelector('[title]')).toBeNull();
    });

    test('under the classic UI, the star keeps the browser\'s tooltip and its own icon', () => {
        renderWithProviders(<GifPost post={makePost(makeGif('g1'))}/>);
        const star = screen.getByRole('button', {name: 'Add to favorites'});
        expect(star).toHaveAttribute('title', 'Add to favorites');
        expect(star).not.toHaveAttribute('data-am-tip');
        expect(star.querySelector('use')).toBeNull();
    });

    test('shows the message of posts with invalid props', () => {
        const scriptURL = ['javascript', 'alert(1)'].join(':');
        renderWithProviders(<GifPost post={makePost({id: 'x', kind: 'gif', post: {url: scriptURL}})}/>);
        expect(screen.getByText('![Happy cat](https://chat.example.com/x.gif)')).toBeInTheDocument();
    });
});

test('parseItem checks the props', () => {
    expect(parseItem(null)).toBeNull();
    expect(parseItem({id: 'x', kind: 'video', post: {url: '/plugins/com.antimatterchat.gifs/media/x.gif'}})).toBeNull();
    const item = parseItem({id: 'x', kind: 'gif', post: {url: 'https://media.tenor.com/x.gif'}, image: 'bad'});
    expect(item).toEqual({id: 'x', kind: 'gif', post: {url: 'https://media.tenor.com/x.gif'}, preview: {url: 'https://media.tenor.com/x.gif'}});
});
