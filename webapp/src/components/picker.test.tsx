// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {act, fireEvent, screen, waitFor} from '@testing-library/react';
import React from 'react';

import {makeGif, makeSticker, renderWithProviders} from '../../tests/utils';
import * as client from '../client';
import {getPicker, loadConfig, openPicker, resetFavorites} from '../state';

import PickerRoot, {place} from './picker_root';

jest.mock('../client', () => ({
    getConfig: jest.fn(() => Promise.resolve({configured: true, stickers: true})),
    featured: jest.fn(),
    search: jest.fn(),
    categories: jest.fn(),
    autocomplete: jest.fn(() => Promise.resolve(['cats', 'cat dance'])),
    getFavorites: jest.fn(() => Promise.resolve([])),
    addFavorite: jest.fn(),
    removeFavorite: jest.fn(),
    getRecent: jest.fn(() => Promise.resolve([])),
    postItem: jest.fn(() => Promise.resolve({})),
}));

const mocked = client as jest.Mocked<typeof client>;

beforeEach(async () => {
    jest.clearAllMocks();
    resetFavorites();
    delete window.antimatterWebUI;
    mocked.featured.mockImplementation((kind) => Promise.resolve({items: kind === 'gif' ? [makeGif('g1'), makeGif('g2', 'Dance')] : [makeSticker('s9')], next: ''}));
    mocked.search.mockImplementation((kind, q) => Promise.resolve({items: kind === 'gif' ? [makeGif('found', q)] : [makeSticker(`pack-${q}`)]}));
    mocked.categories.mockImplementation((kind) => Promise.resolve(kind === 'gif' ? [{name: 'cats', searchterm: 'cats'}] : [{name: 'Lab crew', searchterm: 'lab crew'}]));
    await loadConfig();
});

afterEach(() => {
    act(() => {
        // Close the picker between tests
        if (getPicker()) {
            document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape'}));
        }
    });
});

test('shows the trending GIFs, posts one and closes', async () => {
    renderWithProviders(<PickerRoot/>);
    expect(screen.queryByRole('dialog')).toBeNull();

    act(() => openPicker({channelId: 'channel1', rootId: 'root1'}));
    expect(screen.getByRole('dialog', {name: 'GIFs and stickers'})).toHaveClass('amgifs-pop', 'amgifs-picker');
    expect(await screen.findByRole('button', {name: 'Send GIF: Happy cat'})).toBeInTheDocument();
    expect(mocked.featured.mock.calls[0][0]).toBe('gif');
    expect(mocked.featured.mock.calls[0][2]).toBe('fr');
    expect(screen.getByRole('button', {name: 'Trending'})).toHaveAttribute('aria-pressed', 'true');
    expect(await screen.findByRole('button', {name: 'cats'})).toBeInTheDocument();
    expect(screen.getByLabelText('Search GIFs')).toHaveFocus();

    fireEvent.click(screen.getByRole('button', {name: 'Send GIF: Dance'}));
    await waitFor(() => expect(getPicker()).toBeNull());
    expect(mocked.postItem).toHaveBeenCalledWith(expect.objectContaining({id: 'g2'}), 'channel1', 'root1', '');
});

test('searches, suggests and opens categories', async () => {
    renderWithProviders(<PickerRoot/>);
    act(() => openPicker({channelId: 'channel1', query: 'party'}));
    expect(screen.getByLabelText('Search GIFs')).toHaveValue('party');
    expect(await screen.findByRole('button', {name: 'Send GIF: party'})).toBeInTheDocument();
    expect(await screen.findByRole('button', {name: 'cat dance'})).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Search GIFs'), {target: {value: ''}});
    fireEvent.click(await screen.findByRole('button', {name: 'cats'}));
    expect(await screen.findByRole('button', {name: 'Send GIF: cats'})).toBeInTheDocument();
    expect(mocked.search.mock.calls.some(([kind, q, , locale]) => kind === 'gif' && q === 'cats' && locale === 'fr')).toBe(true);
});

test('shows favorites and keeps the dialog open when posting fails', async () => {
    mocked.getFavorites.mockResolvedValue([makeGif('fav1', 'Starred')]);
    mocked.postItem.mockRejectedValue(new Error('you can’t post in this channel'));
    renderWithProviders(<PickerRoot/>);
    act(() => openPicker({channelId: 'channel1'}));
    fireEvent.click(screen.getByRole('button', {name: 'Favourites'}));
    fireEvent.click(await screen.findByRole('button', {name: 'Send GIF: Starred'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Couldn’t send it: you can’t post in this channel');
    expect(getPicker()).not.toBeNull();
});

test('lists the sticker packs under Fusion', async () => {
    window.antimatterWebUI = 'fusion';
    renderWithProviders(<PickerRoot/>);
    act(() => openPicker({channelId: 'channel1', tab: 'sticker'}));
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveClass('am-pop', 'am-picker');
    expect(dialog.parentElement).toHaveClass('am-layer');
    expect(screen.getByRole('tab', {name: 'Stickers'})).toHaveAttribute('aria-selected', 'true');
    expect(await screen.findByText('Lab crew')).toBeInTheDocument();
    const sticker = await screen.findByRole('button', {name: 'Send sticker: It works!'});
    expect(sticker).toHaveClass('am-sticker');
    expect(mocked.search).toHaveBeenCalledWith('sticker', 'lab crew', '', 'fr', 50);

    fireEvent.pointerDown(document.body);
    expect(getPicker()).toBeNull();
});

test('says when GIFs are not set up', async () => {
    mocked.getConfig.mockResolvedValue({configured: false, stickers: false});
    await loadConfig();
    renderWithProviders(<PickerRoot/>);
    act(() => openPicker({channelId: 'channel1'}));
    expect(await screen.findByText(/GIFs aren’t set up yet/)).toBeInTheDocument();
    expect(screen.queryByRole('tab')).toBeNull();
});

test('place puts the picker above the message box', () => {
    const pop = {offsetWidth: 390, offsetHeight: 450};
    const rect = {top: 700, bottom: 760, left: 300, right: 1000} as DOMRect;
    expect(place(pop, rect, 1200, 800)).toEqual({left: 610, top: 242});
    expect(place(pop, {...rect, top: 100, bottom: 160} as DOMRect, 1200, 800)).toEqual({left: 610, top: 168});
    expect(place(pop, null, 1200, 800)).toEqual({left: 405, top: 175});
    expect(place(pop, {...rect, right: 200} as DOMRect, 1200, 800).left).toBe(8);
});
